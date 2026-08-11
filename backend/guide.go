// AI 引导创建 Skill：多模态对话引导（打字/语音转文字/文件/图片）+ 生成符合 Claude 规范的 skill 包 zip
package main

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"regexp"
	"strings"

	"github.com/gin-gonic/gin"
)

// ---------- 环境变量 ----------
const (
	// DEEPSEEK_GUIDE_API_KEY 引导对话专用 key（与解读功能解耦）
	envGuideKey  = "DEEPSEEK_GUIDE_API_KEY"
	envVisionKey = "VISION_API_KEY" // 硅基流动等视觉模型 key，未配置时图片降级为附件
	envVisionURL = "VISION_BASE_URL"
	envVisionMod = "VISION_MODEL"
)

const (
	defaultVisionURL   = "https://api.siliconflow.cn/v1"
	defaultVisionModel = "Qwen/Qwen2.5-VL-72B-Instruct"
)

// ---------- 引导对话 ----------

// guideAttachment 一条消息中携带的附件
type guideAttachment struct {
	Type string `json:"type"` // "image" | "file"
	Name string `json:"name"`
	Mime string `json:"mime,omitempty"`
	Data string `json:"data,omitempty"` // base64 编码（图片字节 / 文本文件字节）
}

type guideChatRequest struct {
	Messages   []chatMsg        `json:"messages"`
	Attachment *guideAttachment `json:"attachment,omitempty"`
}

// 引导教练 system prompt：开放式引导，skill 不限于固定 SOP（可以是经验知识型）
const guideSystemPrompt = `你是 SkillHub 平台的「Skill 设计教练」。你的任务是像朋友一样，用通俗的中文对话，一步步引导用户把他想封装成 Skill 的流程、方法论或经验知识说清楚，最终产出一个可以发布到平台上的完整 Skill 包。

【Skill 是什么】
Skill 不只是「写论文、画图」这类有固定 SOP 的自动化流程，也可以是知识/经验型的，比如「保研经验」「面试方法论」「课题选题技巧」。只要用户觉得「这套东西值得整理成可复用的指南」，就可以做成 Skill。

【引导目标：帮用户说清以下信息】
1. 用途：这个 Skill 帮用户解决什么问题、适合谁用。
2. 输入：用户使用时会提供什么（材料、资料、描述、模板……）。
3. 输出：用户最终希望得到什么（文档、代码、分析结果、清单……）。
4. 核心内容：具体怎么做——分步骤的流程，或分主题的经验/知识框架，或必须遵守的规则与注意事项。
5. 细节与坑：关键细节、常见错误、注意事项、参考资料。

【引导策略】
- 每次只问 1-2 个问题，不要一次性抛出所有问题让用户回答。
- 用户说的内容模糊时，追问具体细节；用户不知道从何说起时，给出一个贴近他描述的示例帮他展开。
- 如果用户上传了图片或文件，先询问/确认这些材料的作用，并引导用户用文字补充关键信息。
- 当用户已经提供了足够的信息（用途、输入、输出、核心步骤/框架基本齐全）时，主动告诉他：可以点击「生成 Skill 包」。
- 全程使用简体中文，语气亲切、耐心、鼓励；回复保持简洁。
- 重要：每轮回复的末尾，用单独一行输出信息完整度标签，格式为【进度】N%（N 为 0-100 的整数）：信息很少时给 10-30，逐步补充后 40-80，信息基本齐全时 90，完全齐全时 100。该行之后不要再输出任何内容。`

// guideChat POST /api/skills/guide/chat（需登录）
// 前端每次携带完整对话历史 + 可选附件；后端处理附件（图片→视觉模型描述或降级提示；文本文件→嵌入内容）后调用引导 LLM
func guideChat(c *gin.Context) {
	var req guideChatRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request: " + err.Error()})
		return
	}
	if len(req.Messages) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "messages is required"})
		return
	}

	messages := make([]chatMsg, 0, len(req.Messages)+2)
	messages = append(messages, chatMsg{Role: "system", Content: guideSystemPrompt})
	messages = append(messages, req.Messages...)

	// 附件处理：注入一条"系统观察"说明
	if req.Attachment != nil && req.Attachment.Name != "" {
		note := processGuideAttachment(req.Attachment)
		if note != "" {
			messages = append(messages, chatMsg{Role: "system", Content: note})
		}
	}

	content, err := callGuideDeepSeek(context.Background(), messages)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "AI 生成失败：" + err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": content})
}

// processGuideAttachment 处理附件，返回注入上下文的说明文本
func processGuideAttachment(att *guideAttachment) string {
	if att.Type == "image" {
		return processImageAttachment(att)
	}
	return processFileAttachment(att)
}

// processImageAttachment 图片：配置了视觉模型则调用理解；否则降级为参考附件
func processImageAttachment(att *guideAttachment) string {
	key := os.Getenv(envVisionKey)
	if key == "" || att.Data == "" {
		return fmt.Sprintf("用户上传了一张图片【%s】作为参考。当前未接入视觉模型，无法直接识别图片内容。请引导用户用文字描述这张图片里重要的信息（如果图片只是装饰/示意，可忽略）。", att.Name)
	}
	mime := att.Mime
	if mime == "" {
		mime = "image/png"
	}
	desc, err := callVisionLLM(context.Background(), mime, att.Data, "请用中文详细描述这张图片的内容，包括其中所有文字、结构、步骤、关键信息。如果图片模糊或内容不明，请如实说明。")
	if err != nil {
		return fmt.Sprintf("用户上传了一张图片【%s】作为参考，但图片理解失败（%v）。请引导用户用文字描述图片内容。", att.Name, err)
	}
	return fmt.Sprintf("用户上传了一张图片【%s】作为参考，以下是 AI 对图片内容的识别结果：\n%s\n（如果识别有误，请结合用户后续的文字说明修正）", att.Name, desc)
}

// processFileAttachment 文本文件直接嵌入内容；二进制仅附文件名
func processFileAttachment(att *guideAttachment) string {
	name := att.Name
	dot := strings.LastIndex(name, ".")
	ext := ""
	if dot >= 0 {
		ext = strings.ToLower(name[dot+1:])
	}
	if isTextExt(ext) && att.Data != "" {
		raw, err := base64.StdEncoding.DecodeString(att.Data)
		if err != nil {
			return fmt.Sprintf("用户上传了文件【%s】，但文件内容解析失败。请引导用户用文字说明该文件的作用和关键内容。", name)
		}
		if len(raw) > 200*1024 {
			return fmt.Sprintf("用户上传了文件【%s】，内容较大（%.0f KB），仅截取前 200KB 供参考：\n%s", name, float64(len(raw))/1024, string(raw[:200*1024]))
		}
		return fmt.Sprintf("用户上传了文本文件【%s】，内容如下：\n---文件内容开始---\n%s\n---文件内容结束---", name, string(raw))
	}
	return fmt.Sprintf("用户上传了文件【%s】（类型：%s）。无法读取其内部内容，请引导用户用文字说明该文件的作用、包含的关键信息，以及希望 Skill 如何处理它。", name, extOrMime(att))
}

func extOrMime(att *guideAttachment) string {
	if att.Mime != "" {
		return att.Mime
	}
	return "未知类型"
}

var textExts = map[string]bool{
	"md": true, "markdown": true, "txt": true, "yaml": true, "yml": true,
	"json": true, "py": true, "go": true, "ts": true, "tsx": true, "js": true, "jsx": true,
	"vue": true, "tex": true, "sql": true, "sh": true, "bash": true, "toml": true,
	"ini": true, "cfg": true, "conf": true, "csv": true, "xml": true, "html": true,
	"css": true, "c": true, "cpp": true, "h": true, "java": true, "kt": true,
	"swift": true, "rs": true, "rb": true, "php": true, "r": true, "ipynb": true,
	"env": true, "dockerfile": true, "makefile": true, "license": true, "gitignore": true,
}

func isTextExt(ext string) bool {
	return textExts[ext]
}

// ---------- 生成 skill 包 ----------

type guideGenerateRequest struct {
	Messages []chatMsg `json:"messages"`
}

// generatedFile LLM 生成的一个文件
type generatedFile struct {
	Path    string `json:"path"`
	Content string `json:"content"`
}

// generatedSkill LLM 生成的 skill 包（JSON 结构）
type generatedSkill struct {
	Name        string          `json:"name"` // kebab-case 目录名
	Title       string          `json:"title"`
	Description string          `json:"description"`
	Category    string          `json:"category"`
	Tags        []string        `json:"tags"`
	Version     string          `json:"version"`
	Files       []generatedFile `json:"files"`
}

// 生成器 system prompt：遵循 Claude 官方 Agent Skill 规范，只输出 JSON
const generateSystemPrompt = `你是 SkillHub 平台的「Skill 包生成器」。请严格按下面的规则执行。

【第一步：确定技能主题（最重要）】
技能主题只能来自用户消息里描述的需求（例如：保研经验整理、写实验报告、论文写作流程）。用户消息没有提到的能力，一律不要编造。
如果消息中包含「用户技能需求简报」，则以简报中的 topic 为唯一主题，坚决围绕它生成。
本提示词只是生成规则说明，绝不是技能主题。禁止把「生成技能」「生成 skill」「skill 生成器」「Skill 包生成器」等本提示词相关的内容当作技能主题。

【第二步：按 Claude Skill 规范生成】
- Skill 是一个文件夹，至少包含 SKILL.md（必须），可选 scripts/、references/、assets/ 子目录。
- SKILL.md 以 YAML frontmatter 开头（--- 包裹），frontmatter 必须含 name（kebab-case：小写字母数字连字符，≤64 字符，禁止 claude/anthropic）和 description（做什么+什么时候用+触发词，≤1024 字符）；frontmatter 之后是 Markdown 正文，写成 AI 可直接照做的指令（步骤、规则、模板、注意事项），正文用中文。
- 不要放 README.md。
- 依据用户提供的全部信息生成；信息不足处用合理内容补全并注明。

【输出要求】
只输出一个 JSON 对象，不要输出任何其他文字、不要用 markdown 代码块包裹。JSON 结构：
{"name":"kebab-case 目录名","title":"中文展示标题","description":"一句话介绍(10-60字)","category":"论文写作或编程开发或数据科学或设计创作或效率工具或语言学习或其他","tags":["标签1"],"version":"1.0.0","files":[{"path":"SKILL.md","content":"完整内容"},{"path":"references/xx.md","content":"参考内容"}]}`

var jsonBlockRe = regexp.MustCompile("(?s)```(?:json)?\\s*(\\{.*?\\})\\s*```")

// extractJSONObject 从任意文本中提取第一个 `{` 到最后一个 `}` 之间的内容（容忍 LLM 开场白/结尾语）
func extractJSONObject(s string) string {
	start := strings.Index(s, "{")
	end := strings.LastIndex(s, "}")
	if start < 0 || end <= start {
		return ""
	}
	return s[start : end+1]
}

// 阶段一 prompt：从对话中提炼技能需求简报（简单任务，弱模型也能保持主题）
const briefSystemPrompt = `你是 SkillHub 的需求分析师。请仔细阅读用户与 AI 的对话内容，提炼出用户想创建的 Skill 的完整需求，只输出一个 JSON 对象（不要输出任何其他文字、不要用 markdown 代码块包裹）：
{"topic":"技能主题，一句话说清（如：帮本科生整理保研经验与行动指南）","who":"适合谁用","input":"用户使用时会输入什么","output":"用户希望得到什么","core":"核心内容：分步骤流程或分主题经验框架（要点式）","details":"注意事项、常见错误、关键细节"}`

// extractSkillBrief 阶段一：调用 LLM 提炼需求简报
func extractSkillBrief(ctx context.Context, msgs []chatMsg) (string, error) {
	messages := make([]chatMsg, 0, len(msgs)+2)
	messages = append(messages, chatMsg{Role: "system", Content: briefSystemPrompt})
	messages = append(messages, msgs...)
	return callGuideDeepSeek(ctx, messages)
}

// guideGenerate POST /api/skills/guide/generate（需登录）
// 两阶段生成：先提炼需求简报（锚定主题），再按 Claude 规范生成完整 skill 包（JSON + zip base64），前端确认后走 createSkill 发布
func guideGenerate(c *gin.Context) {
	var req guideGenerateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request: " + err.Error()})
		return
	}

	// 阶段一：提炼需求简报，作为生成的主题锚点（失败则直接基于原始对话生成）
	brief, err := extractSkillBrief(context.Background(), req.Messages)
	if err != nil || !strings.Contains(brief, "{") {
		brief = ""
	}

	// 阶段二：按简报生成
	messages := make([]chatMsg, 0, len(req.Messages)+3)
	messages = append(messages, chatMsg{Role: "system", Content: generateSystemPrompt})
	if brief != "" {
		messages = append(messages, chatMsg{Role: "user", Content: "以下是用户技能需求简报，请严格依据它确定主题并生成 Skill 包：\n" + brief})
	}
	messages = append(messages, req.Messages...)

	content, err := callGuideDeepSeek(context.Background(), messages)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "AI 生成失败：" + err.Error()})
		return
	}

	// 解析 JSON：优先直接解析 -> 容忍 ```json 代码块 -> 提取首个 {...} 对象（容忍开场白/结尾语）
	jsonStr := content
	if m := jsonBlockRe.FindStringSubmatch(content); m != nil {
		jsonStr = m[1]
	}
	var gen generatedSkill
	if err := json.Unmarshal([]byte(jsonStr), &gen); err != nil {
		if s := extractJSONObject(jsonStr); s != "" {
			jsonStr = s
		}
		if err := json.Unmarshal([]byte(jsonStr), &gen); err != nil {
			c.JSON(http.StatusBadGateway, gin.H{"error": "AI 输出解析失败，请重试：" + err.Error()})
			return
		}
	}
	if gen.Name == "" || len(gen.Files) == 0 {
		c.JSON(http.StatusBadGateway, gin.H{"error": "AI 输出缺少必要字段（name / files），请重试"})
		return
	}

	// 校验/修正 name 为 kebab-case
	gen.Name = sanitizeKebab(gen.Name)
	if gen.Version == "" {
		gen.Version = "1.0.0"
	}

	// 打包 zip（内存）
	zipBytes, err := buildSkillZip(gen.Files)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "打包 zip 失败：" + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data": gin.H{
			"name":        gen.Name,
			"title":       gen.Title,
			"description": gen.Description,
			"category":    gen.Category,
			"tags":        gen.Tags,
			"version":     gen.Version,
			"files":       gen.Files,
			"zip_base64":  base64.StdEncoding.EncodeToString(zipBytes),
		},
	})
}

// buildSkillZip 将生成的文件列表打包为 zip（根目录为 skill 名称的 kebab-case 目录）
func buildSkillZip(files []generatedFile) ([]byte, error) {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for _, f := range files {
		p := strings.TrimPrefix(f.Path, "/")
		if p == "" {
			continue
		}
		w, err := zw.Create(p)
		if err != nil {
			return nil, err
		}
		if _, err := w.Write([]byte(f.Content)); err != nil {
			return nil, err
		}
	}
	if err := zw.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// sanitizeKebab 修正 name 为 kebab-case（小写字母数字连字符）
func sanitizeKebab(name string) string {
	var sb strings.Builder
	lastDash := false
	for _, r := range strings.ToLower(strings.TrimSpace(name)) {
		ok := (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9')
		if ok {
			sb.WriteRune(r)
			lastDash = false
		} else if !lastDash && sb.Len() > 0 {
			sb.WriteRune('-')
			lastDash = true
		}
	}
	out := strings.Trim(sb.String(), "-")
	if out == "" {
		out = "my-skill"
	}
	if len(out) > 64 {
		out = out[:64]
	}
	return out
}
