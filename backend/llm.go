// DeepSeek LLM 集成：按用户 AI 熟练度生成 skill 的个性化介绍
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

const (
	deepseekURL  = "https://api.deepseek.com/chat/completions"
	deepseekModel = "deepseek-chat"
)

// 解释缓存：按 (userID, skillID) 缓存，避免重复调用 LLM 产生费用
type explainEntry struct {
	Content   string
	CreatedAt time.Time
}

var (
	explainMu   sync.Mutex
	explainCache = map[string]explainEntry{}
)

// 对所有水平都生效的总要求：任何水平都必须先简要介绍 skill 是什么、能解决什么问题；
// 操作讲解的详细程度与用户 AI 水平成反比（水平越低讲得越细）。
const commonGuide = `【总要求】
- 所有用户（无论什么水平）都必须先用 1-3 句话简要介绍：这个 skill 是什么、能解决什么问题、适合谁用。
- 介绍内容必须严格基于【技能信息】中提供的真实信息（技能名称、分类、版本、标签、作者描述、包内文件清单）展开，不得编造该技能不具备的功能、不存在的文件或脚本名称。
- 用户水平越低，「怎么使用」的讲解越详细（分步骤说明每步在哪里操作、输入什么、会得到什么）；水平越高，使用引导越简练（点到为止即可）。`

// 熟练度 -> 面向该水平的介绍要求
func levelGuide(level string) string {
	switch level {
	case "never":
		return "用户从未用过任何 AI 工具（如 ChatGPT、Trae、Codex 等），甚至可能没有相关概念。请用最通俗易懂的日常语言，避免专业术语；把「怎么使用」讲到最细：分成 3-5 个基础步骤，每一步都说明在哪里操作、输入什么、会看到什么结果，让他照着做就能跑起来。"
	case "beginner":
		return "用户刚接触 AI 工具，用过简单的对话式 AI。先简要介绍这个 skill 的用途，再把「怎么使用」讲得细致一些：分步骤说明如何准备材料、如何向 AI 描述需求、如何检查生成结果，术语要有简短解释。"
	case "intermediate":
		return "用户熟悉常见 AI 工具，会用但想更深入。介绍这个 skill 的核心功能亮点、适用的典型场景、与普通方式的区别；「怎么使用」讲清关键流程即可，再补充上手时值得注意的要点。"
	case "advanced":
		return "用户是资深 AI 玩家，熟悉自动化、Agent、工作流等概念。先用一两句话介绍这个 skill 是什么，「怎么使用」只需一句简单引导（不展开步骤）；重点用专业精炼的语言讲：技术构成、目录结构与设计思路、可配置项与扩展点、最佳实践以及可能的改进方向。"
	default: // 未设置默认为 beginner 档
		return levelGuide("beginner")
	}
}

func levelLabel(level string) string {
	switch level {
	case "never":
		return "从未用过"
	case "beginner":
		return "初级"
	case "intermediate":
		return "中级"
	case "advanced":
		return "高级"
	default:
		return "初级"
	}
}

// officialAgentInfo 主流 AI 编码助手的官方渠道（注入 prompt，避免 LLM 编造下载链接）
const officialAgentInfo = `- Trae（字节跳动出品，中文友好、免费）：官网 https://www.trae.ai
- Cursor（AI 代码编辑器）：官网 https://www.cursor.com
- Codex（OpenAI）：在 ChatGPT 应用内开启 Codex 功能`

// 根据用户是否已安装 Agent 追加介绍要求
// parsed=false 表示问卷未知（老用户），保持原逻辑不额外引导
func agentInstallGuide(parsed, hasAgent bool) string {
	if !parsed {
		return ""
	}
	if hasAgent {
		return "用户电脑上已安装 AI 编码助手，介绍中不需要再讲如何安装工具。"
	}
	return fmt.Sprintf(`用户电脑上尚未安装任何 AI 编码助手（如 Trae、Codex、Cursor），不装的话无法运行本技能。
请在介绍最前面额外生成一个小节「开始之前：安装 AI 助手」，用最简洁的步骤（3 步以内）引导用户下载安装，并说明安装完成后回到本页再生成一次即可得到完整上手步骤。
下载渠道只能引用以下官方信息，禁止编造其他网址：
%s`, officialAgentInfo)
}

// parseAgentState 解析问卷，返回 (是否解析成功, 是否已安装 Agent)
func parseAgentState(user *User) (bool, bool) {
	if user == nil || user.AIQuiz == "" {
		return false, false
	}
	var q aiQuizInput
	if err := json.Unmarshal([]byte(user.AIQuiz), &q); err != nil {
		return false, false
	}
	return true, q.HasAgentInstalled
}

type chatMsg struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatFmt struct {
	Type string `json:"type"`
}

type chatReq struct {
	Model          string    `json:"model"`
	Messages       []chatMsg `json:"messages"`
	Temperature    float64   `json:"temperature"`
	ResponseFormat *chatFmt  `json:"response_format,omitempty"`
}

type chatResp struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}

// callDeepSeek 调用 DeepSeek 对话补全接口
func callDeepSeek(ctx context.Context, messages []chatMsg) (string, error) {
	return callDeepSeekWithKey(ctx, messages, "DEEPSEEK_API_KEY")
}

// callGuideDeepSeek 引导对话专用：优先使用引导 key，未配置则回退到解读 key
func callGuideDeepSeek(ctx context.Context, messages []chatMsg) (string, error) {
	key := os.Getenv("DEEPSEEK_GUIDE_API_KEY")
	env := "DEEPSEEK_GUIDE_API_KEY"
	if key == "" {
		env = "DEEPSEEK_API_KEY"
	}
	return callDeepSeekWithKey(ctx, messages, env)
}

// callDeepSeekWithKey 调用 DeepSeek 对话补全接口（key 来源由 env 指定）
func callDeepSeekWithKey(ctx context.Context, messages []chatMsg, env string) (string, error) {
	return callDeepSeekKeyTemp(ctx, messages, env, 0.7, false)
}

func callDeepSeekKeyTemp(ctx context.Context, messages []chatMsg, env string, temp float64, jsonMode bool) (string, error) {
	apiKey := os.Getenv(env)
	if apiKey == "" {
		return "", fmt.Errorf("%s 未配置", env)
	}
	reqBody := chatReq{
		Model:       deepseekModel,
		Messages:    messages,
		Temperature: temp,
	}
	if jsonMode {
		reqBody.ResponseFormat = &chatFmt{Type: "json_object"}
	}
	body, _ := json.Marshal(reqBody)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, deepseekURL, bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)

	client := &http.Client{Timeout: 120 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var out chatResp
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return "", fmt.Errorf("decode llm response: %v", err)
	}
	if out.Error != nil {
		return "", fmt.Errorf("deepseek error: %s", out.Error.Message)
	}
	if len(out.Choices) == 0 {
		return "", fmt.Errorf("deepseek returned no choices")
	}
	return strings.TrimSpace(out.Choices[0].Message.Content), nil
}

// ---------- 视觉模型（图片理解，OpenAI 兼容） ----------

// visionMsgContent OpenAI 兼容的多模态消息内容
type visionPart struct {
	Type     string `json:"type"`
	Text     string `json:"text,omitempty"`
	ImageURL struct {
		URL string `json:"url"`
	} `json:"image_url,omitempty"`
}

type visionMsg struct {
	Role    string       `json:"role"`
	Content []visionPart `json:"content"`
}

type visionReq struct {
	Model    string      `json:"model"`
	Messages []visionMsg `json:"messages"`
}

// callVisionLLM 调用视觉模型理解图片，返回文字描述
func callVisionLLM(ctx context.Context, mime, imageBase64, prompt string) (string, error) {
	apiKey := os.Getenv("VISION_API_KEY")
	if apiKey == "" {
		return "", fmt.Errorf("VISION_API_KEY 未配置")
	}
	baseURL := os.Getenv("VISION_BASE_URL")
	if baseURL == "" {
		baseURL = "https://api.siliconflow.cn/v1"
	}
	model := os.Getenv("VISION_MODEL")
	if model == "" {
		model = "Qwen/Qwen2.5-VL-72B-Instruct"
	}

	var part visionPart
	part.Type = "image_url"
	part.ImageURL.URL = fmt.Sprintf("data:%s;base64,%s", mime, imageBase64)
	body, _ := json.Marshal(visionReq{
		Model: model,
		Messages: []visionMsg{{
			Role: "user",
			Content: []visionPart{
				part,
				{Type: "text", Text: prompt},
			},
		}},
	})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)

	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var out chatResp
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return "", fmt.Errorf("decode vision response: %v", err)
	}
	if out.Error != nil {
		return "", fmt.Errorf("vision error: %s", out.Error.Message)
	}
	if len(out.Choices) == 0 {
		return "", fmt.Errorf("vision returned no choices")
	}
	return strings.TrimSpace(out.Choices[0].Message.Content), nil
}

// explainSkill GET /api/skills/:id/explain（需登录）
// 根据当前用户的 AI 熟练度 + 用户背景 + skill 内容，用 LLM 生成个性化介绍
func explainSkill(c *gin.Context) {
	uid := c.GetInt64("userID")
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid skill id"})
		return
	}

	skill, err := getSkillByID(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "skill not found"})
		return
	}
	// 从 DB 读取文件清单，让 LLM 了解 skill 包构成
	fileNames := []string{}
	rows, err := db.Query(`SELECT file_path FROM skill_files WHERE skill_id = ? ORDER BY file_path`, id)
	if err == nil {
		for rows.Next() {
			var p string
			if rows.Scan(&p) == nil {
				fileNames = append(fileNames, p)
			}
		}
		rows.Close()
	}

	user, err := getUserByID(uid)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
		return
	}

	// 是否已安装 Agent（影响介绍中是否包含安装引导）
	agentParsed, hasAgent := parseAgentState(user)
	agentFlag := "u" // u=未知 0=未装 1=已装
	if agentParsed {
		if hasAgent {
			agentFlag = "1"
		} else {
			agentFlag = "0"
		}
	}

	// 命中缓存直接返回（key 含 ai_level + agent 状态：同水平但环境不同视为不同内容）
	key := fmt.Sprintf("%d:%s:%s:%d", uid, user.AILevel, agentFlag, skill.ID)
	explainMu.Lock()
	if e, ok := explainCache[key]; ok {
		explainMu.Unlock()
		c.JSON(http.StatusOK, gin.H{
			"data":        e.Content,
			"ai_level":    user.AILevel,
			"level_label": levelLabel(user.AILevel),
			"cached":      true,
		})
		return
	}
	explainMu.Unlock()

	// 构建 LLM 输入
	userProfile := fmt.Sprintf("学校：%s；专业：%s；年级：%s", user.School, user.Major, user.Grade)
	if userProfile == "学校：；专业：；年级：" {
		userProfile = "未填写"
	}
	agentEnv := agentInstallGuide(agentParsed, hasAgent)
	skillInfo := fmt.Sprintf(
		"技能名称：%s\n分类：%s\n版本：%s\n标签：%s\n作者描述：%s\n包内主要文件：%s",
		skill.Name, skill.Category, skill.Version, skill.Tags, skill.Description,
		strings.Join(fileNames, "、"),
	)
	if len(fileNames) == 0 {
		skillInfo += "（该 skill 暂无文件包）"
	}

	prompt := fmt.Sprintf(`你是 SkillHub 平台的技能导览专家。请根据用户的 AI 使用水平，为一个技能生成个性化介绍。

【用户的 AI 水平】%s
【用户背景】%s
【用户 Agent 环境】%s
【技能信息】
%s

【要求】
%s

%s
介绍控制在 150-300 字，使用中文，用 markdown 格式输出（可用小标题、列表）。直接输出介绍内容，不要任何开场白。`, levelLabel(user.AILevel), userProfile, agentEnv, skillInfo, levelGuide(user.AILevel), commonGuide)

	content, err := callDeepSeek(context.Background(), []chatMsg{
		{Role: "system", Content: "你是一个专业的技能介绍助手，擅长根据读者水平调整讲解深度。"},
		{Role: "user", Content: prompt},
	})
	if err != nil {
		log.Printf("explain skill %d for user %d: %v", skill.ID, uid, err)
		c.JSON(http.StatusBadGateway, gin.H{"error": "AI 生成失败：" + err.Error()})
		return
	}

	explainMu.Lock()
	explainCache[key] = explainEntry{Content: content, CreatedAt: time.Now()}
	explainMu.Unlock()

	c.JSON(http.StatusOK, gin.H{
		"data":        content,
		"ai_level":    user.AILevel,
		"level_label": levelLabel(user.AILevel),
		"cached":      false,
	})
}
