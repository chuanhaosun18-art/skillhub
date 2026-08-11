# SkillHub —— AI 技能分享与发布平台

一个面向「AI 技能（Skill）」的分享平台：用户可以浏览、搜索、下载别人封装的 Skill，也可以把自
己的 Skill 发布到平台。平台针对「有 AI 经验的用户」与「没有 AI 经验的新手」设计了两种发布路径，
并利用大模型为不同水平的用户提供个性化的技能解读。

## 核心功能

### 1. Skill 市场
- 技能列表：按 **最新 / 评分 / 下载量** 排序
- 搜索：支持按关键词（名称、描述、分类、标签）检索，可按分类筛选
- 技能详情：查看作者、版本、标签、文件清单，支持 **zip 下载**
- 数据统计：下载量、浏览量、评分

### 2. 用户体系
- 注册时填写 **AI 熟练度问卷**（5 题），系统自动推导用户水平（从未用过 / 初级 / 中级 / 高级）
- 补充学校、专业、年级等背景画像
- 登录 / 个人信息编辑 / 我的发布管理（支持删除自己的技能）

### 3. 双路径发布技能
平台为两种不同用户设计了完全不同的发布体验：

- **路径一：直接上传（面向有经验的用户）**
  填写技能名称、描述、分类、标签、版本，上传 zip 压缩包即可发布。后端自动解压、登记文件清单与 SHA256 校验。

- **路径二：AI 引导创建（面向没有 AI 经验的用户）**
  通过大模型对话一步步引导用户把自己想封装成 Skill 的流程、方法论或经验知识说清楚，最终自动生成符合 Claude 官方规范的 Skill 包并发布。支持 **四种交流方式**：
  - 打字输入
  - 语音输入（浏览器 Web Speech API 语音转文字）
  - 文件上传（文本文件内容自动嵌入对话上下文）
  - 图片上传（配置视觉模型后自动识别图片内容，未配置时降级为文字引导）

  引导过程中 LLM 每轮输出【进度】标签，前端据此展示信息完整度进度条；信息足够时用户可一键「生成 Skill 包」——两阶段生成（先提炼需求简报，再生成 `SKILL.md` + `references/` 参考文件并打包 zip），前端预览确认后发布。

### 4. AI 个性化技能解读
进入技能详情页时，后端调用 DeepSeek，根据当前用户的 **AI 水平 + 背景画像 + 是否已安装 AI 助手**，
为同一个技能生成不同深度、不同侧重的介绍：
- 从未用过的新手：最通俗的语言 + 从安装 AI 助手到运行的逐步指引
- 高级用户：重点讲技术构成、目录结构与扩展点
- 结果按 (用户, 技能) 缓存，避免重复调用产生费用

### 5. 评价与反馈
- 评分 + 文字评价（同一用户对同一技能只能评一次）
- Issue 反馈：类似 GitHub Issue，可对技能提出问题，作者可关闭

## 技术栈

| 端 | 技术 |
| --- | --- |
| 前端 | Vue 3 + Vite + Element Plus + Vue Router + Web Speech API |
| 后端 | Go + Gin + SQLite（modernc.org/sqlite 纯 Go 驱动，无需 CGO）|
| 认证 | JWT |
| LLM | DeepSeek Chat（对话解读 + 引导）+ 硅基流动 Qwen2.5-VL（图片理解，可选）|

## 项目结构

```
.
├── qianduan/                  # 前端（Vue 3 + Vite）
│   ├── src/
│   │   ├── api/               # 后端 API 封装（auth / skills）
│   │   ├── components/        # 公共组件（导航栏）
│   │   ├── router/            # 路由
│   │   └── views/             # 页面：首页、登录、发布、详情、搜索结果、个人中心
│   ├── index.html
│   ├── package.json
│   └── vite.config.js
└── backend/                   # 后端（Go + Gin）
    ├── main.go                # 入口 + 路由 + CORS
    ├── auth.go                # 注册/登录/JWT + AI 熟练度问卷
    ├── handlers.go            # 技能 CRUD、搜索、下载、解压
    ├── guide.go               # AI 引导对话 + 生成 skill 包（多模态）
    ├── llm.go                 # DeepSeek 调用 + 个性化解读 + 视觉模型
    ├── reviews.go             # 评分评价
    ├── zip.go                 # skill 包 zip 打包
    ├── db.go                  # SQLite 建表与迁移
    └── paths.go               # 数据目录配置
```

## 快速开始

### 后端（Go 1.26+）

```bash
cd backend
# 配置 API Key（必须）
#   Windows PowerShell:
$env:DEEPSEEK_API_KEY="sk-..."        # 技能解读 / 引导对话（两者共用，可分开配置）
$env:DEEPSEEK_GUIDE_API_KEY="sk-..."  # （可选）引导对话专用 key，未配置则回退用上面那个
$env:VISION_API_KEY="sk-..."          # （可选）图片理解，未配置则图片降级为文字引导
$env:VISION_BASE_URL="https://api.siliconflow.cn/v1"   # （可选）默认已填
$env:VISION_MODEL="Qwen/Qwen2.5-VL-72B-Instruct"        # （可选）默认已填

# 启动（默认端口 8080，可用 SKILLHUB_PORT 覆盖）
go run .
```

> 数据默认存放在 `D:\skillhub-data`，可用环境变量 `SKILLHUB_DATA` 覆盖。

### 前端（Node 18+）

```bash
cd qianduan
npm install
npm run dev    # 默认 http://localhost:5173
```

打开 http://localhost:5173 即可使用。

## 环境变量一览

| 变量 | 用途 | 必填 |
| --- | --- | --- |
| `DEEPSEEK_API_KEY` | 技能个性化解读 | 是 |
| `DEEPSEEK_GUIDE_API_KEY` | AI 引导对话专用 key | 否（回退 `DEEPSEEK_API_KEY`）|
| `VISION_API_KEY` | 引导对话中的图片理解 | 否（未配置时图片降级）|
| `VISION_BASE_URL` | 视觉模型 API 地址 | 否 |
| `VISION_MODEL` | 视觉模型名称 | 否 |
| `SKILLHUB_PORT` | 后端端口 | 否（默认 8080）|
| `SKILLHUB_DATA` | 数据存储目录 | 否（默认 `D:\skillhub-data`）|

## API 一览

| 方法 | 路径 | 说明 | 鉴权 |
| --- | --- | --- | --- |
| GET | `/api/health` | 健康检查 | 否 |
| POST | `/api/auth/register` | 注册（含 AI 问卷）| 否 |
| POST | `/api/auth/login` | 登录 | 否 |
| GET | `/api/auth/me` | 当前用户信息 | 是 |
| PUT | `/api/users/:id` | 更新个人资料 | 是 |
| GET | `/api/users/me/skills` | 我的发布 | 是 |
| GET | `/api/skills` | 技能列表（搜索/分类/排序）| 否 |
| POST | `/api/skills` | 发布技能（multipart，zip）| 是 |
| GET | `/api/skills/:id` | 技能详情（含文件清单）| 否 |
| DELETE | `/api/skills/:id` | 删除技能（仅属主）| 是 |
| GET | `/api/skills/:id/download` | 下载 zip | 否 |
| GET | `/api/skills/:id/explain` | AI 个性化解读 | 是 |
| POST | `/api/skills/guide/chat` | AI 引导对话（可携带附件）| 是 |
| POST | `/api/skills/guide/generate` | 生成 skill 包（zip）| 是 |
| POST | `/api/skills/:id/reviews` | 提交评分评价 | 是 |
| GET | `/api/skills/:id/reviews` | 评价列表 | 否 |
| POST | `/api/skills/:id/issues` | 提交 Issue | 是 |
| GET | `/api/skills/:id/issues` | Issue 列表 | 否 |
| PATCH | `/api/issues/:id` | 关闭 Issue | 是 |

## AI 引导创建 Skill 的工作流

```
用户描述想法（打字/语音/文件/图片）
        │
        ▼
┌─────────────────────────────────────────────┐
│ 引导对话（guide/chat）                        │
│ 教练每次只问 1-2 个问题，开放式引导（不限 SOP，  │
│ 支持经验知识型 Skill），每轮输出【进度】标签     │
└─────────────────────────────────────────────┘
        │  信息足够，点击「生成 Skill 包」
        ▼
┌─────────────────────────────────────────────┐
│ 生成（guide/generate，两阶段）                 │
│ ① 先提炼需求简报锁定主题                        │
│ ② 按 Claude 官方规范生成：                      │
│    SKILL.md（kebab-case 名称 + frontmatter     │
│    + 中文正文）+ references/ 参考文件            │
│    → 打包 zip，返回给前端预览                    │
└─────────────────────────────────────────────┘
        │  用户确认
        ▼
发布（POST /api/skills）→ 自动解压、登记文件清单
```

## 说明

- JWT 密钥为开发默认值（`auth.go` 中 `skillhub-dev-secret-change-me`），上线前请替换为环境变量注入的随机密钥。
- 各 API Key 均从环境变量读取，代码中不包含任何真实密钥。
- `backend/` 下的 `verify_*.py`、`e2e_guide.py` 为开发期验证脚本，用于接口与 AI 链路的回归测试。
