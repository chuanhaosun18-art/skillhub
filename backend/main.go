package main

import (
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
)

func main() {
	initDB()

	r := gin.Default()

	// CORS 中间件（允许前端开发服务器跨域调用）
	r.Use(corsMiddleware())

	// 静态文件：评估指标证明图片
	r.Static("/uploads/proofs", ProofsDir)

	// API 路由
	api := r.Group("/api")
	{
		api.GET("/health", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{"status": "ok"})
		})

	// 认证
	api.POST("/auth/register", register)
	api.POST("/auth/login", login)
	api.GET("/auth/me", authMiddleware(), me)

	// 用户
	api.PUT("/users/:id", authMiddleware(), updateUser)
	api.GET("/users/me/skills", authMiddleware(), mySkills)

	// 技能
	api.GET("/skills", listSkills)                 // 搜索/列表（游客可用）
	api.POST("/skills", authMiddleware(), createSkill) // 发布（需登录）
	api.GET("/skills/:id", getSkill)               // 详情（游客可用）
	api.DELETE("/skills/:id", authMiddleware(), deleteSkill) // 删除（仅属主）
	api.GET("/skills/:id/download", downloadSkill) // 下载 zip（游客可用）
	api.GET("/skills/:id/explain", authMiddleware(), explainSkill) // AI 个性化解读（需登录）

	// AI 引导创建 Skill（需登录）：多模态对话引导 + 生成 skill 包
	api.POST("/skills/guide/chat", authMiddleware(), guideChat)
	api.POST("/skills/guide/generate", authMiddleware(), guideGenerate)

	// 评分 / 评价（需登录提交，游客可看列表）
	api.POST("/skills/:id/reviews", authMiddleware(), submitReview)
	api.GET("/skills/:id/reviews", optionalAuth(), listReviews)

	// Issue 反馈（类 GitHub issue）
	api.POST("/skills/:id/issues", authMiddleware(), createIssue)
	api.GET("/skills/:id/issues", listIssues)
	api.PATCH("/issues/:id", authMiddleware(), closeIssue)
	}

	// ---------- 成长闭环（PRD P0）----------
	// 主张：做事发生在平台内 → 供给是执行的副产品 → 信任来自证据而非评分
	growth := r.Group("/api/growth")
	{
		// F1 目标识别与四筛判定（伪需求在这里被拦住，不进任务流）
		growth.POST("/goals/interpret", authMiddleware(), interpretGoal)

		// F4 任务工作台：所有执行必须落 execution_steps
		growth.POST("/executions", authMiddleware(), createExecution)
		growth.GET("/executions", authMiddleware(), listMyExecutions)
		growth.GET("/executions/:id", authMiddleware(), getExecution)
		growth.POST("/executions/:id/advance", authMiddleware(), advanceExecution)
		growth.POST("/executions/:id/decide", authMiddleware(), recordDecision)
		growth.POST("/executions/:id/edit", authMiddleware(), recordEdit)
		growth.POST("/executions/:id/complete", authMiddleware(), completeExecution)
		growth.POST("/executions/:id/abandon", authMiddleware(), abandonExecution)

		// F5 Skill Creator：轨迹 → 四槽 → 蒸馏度 → 六 slot 文件夹
		growth.POST("/executions/:id/distill", authMiddleware(), distillExecution)
		growth.GET("/drafts/:versionID", authMiddleware(), getDraft)
		growth.PATCH("/drafts/:versionID", authMiddleware(), updateDraft)
		growth.POST("/drafts/:versionID/decisions", authMiddleware(), upsertDecision)
		growth.DELETE("/decisions/:id", authMiddleware(), deleteDecision)
		growth.POST("/drafts/:versionID/downgrade", authMiddleware(), downgradeToInsight)
		growth.POST("/drafts/:versionID/generate-folder", authMiddleware(), generateFolder)

		// F6 发布前四问与门禁
		growth.POST("/skills/:id/evals/run", authMiddleware(), runEvals)
		growth.GET("/skills/:id/gate", getGateStatus)
		growth.POST("/skills/:id/publish", authMiddleware(), publishSkill)

		// F7/F8 准入四层与两段式路由
		growth.POST("/route", authMiddleware(), routeSkills)
		growth.POST("/admin/recompute-scores", authMiddleware(), recomputeAllScores)

		// F10 Trust Card 与判断级溯源
		growth.GET("/skills/:id/trust-card", getTrustCard)
		growth.GET("/decisions/:id/trace", getDecisionTrace)

		// F5.3b 轨迹补录：承认用户会在平台外做事，但蒸馏度封顶 0.85
		growth.POST("/backfill", authMiddleware(), backfillExecution)

		// F17 编排态：长周期方向性需求。只承诺编排，不承诺结果。
		// probe 与 interview 单独命名，避免与 /orchestrations/:id 在同级产生静态段与参数段冲突
		growth.POST("/orch-probe", authMiddleware(), probeOrchestration)
		growth.POST("/orch-interview", authMiddleware(), interviewOrchestration)
		growth.POST("/orchestrations", authMiddleware(), createOrchestration)
		growth.GET("/orchestrations", authMiddleware(), listMyOrchestrations)
		growth.GET("/orchestrations/:id", authMiddleware(), getOrchestration)
		growth.POST("/orchestrations/:id/adopt", authMiddleware(), adoptOrchestration)
		growth.PATCH("/orchestrations/:id/items/:itemId", authMiddleware(), updateOrchItem)
		growth.POST("/orchestrations/:id/reviews", authMiddleware(), reviewOrchestration)

		// F13 个人成长主页与成长身份（成长路径从真实执行派生）
		// 注意：my-profile 与 profile/:id 分开命名，避免静态段与参数段在同级冲突
		growth.GET("/my-profile", authMiddleware(), getMyGrowthProfile)
		growth.PATCH("/my-profile/visibility", authMiddleware(), updateVisibility)
		growth.GET("/profile/:id", optionalAuth(), getUserGrowthProfile)

		// F12 反馈闭环与版本升级
		growth.POST("/executions/:id/feedback", authMiddleware(), submitExecFeedback)
		growth.GET("/skills/:id/version-candidates", listVersionCandidates)
		growth.POST("/version-candidates/:id/accept", authMiddleware(), acceptVersionCandidate)
	}

	port := os.Getenv("SKILLHUB_PORT")
	if port == "" {
		port = "8080"
	}

	r.Run(":" + port)
}

// corsMiddleware 允许前端跨域请求
func corsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		origin := c.GetHeader("Origin")
		if origin != "" {
			c.Header("Access-Control-Allow-Origin", origin)
			c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
			c.Header("Access-Control-Allow-Headers", "Content-Type, Authorization")
			c.Header("Access-Control-Allow-Credentials", "true")
		}
		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}
