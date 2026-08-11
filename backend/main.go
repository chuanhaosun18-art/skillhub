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
