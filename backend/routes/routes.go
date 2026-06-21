package routes

import (
	"devsync/backend/controllers"
	"devsync/backend/middleware"
	"devsync/backend/websocket"
	"github.com/gin-gonic/gin"
)

func SetupRouter(
	authCtrl *controllers.AuthController,
	snippetCtrl *controllers.SnippetController,
	fileCtrl *controllers.FileController,
	deviceCtrl *controllers.DeviceController,
	dashCtrl *controllers.DashboardController,
) *gin.Engine {
	r := gin.Default()

	// Middlewares
	r.Use(middleware.CORSMiddleware())

	// Serve Frontend Static SPA
	r.Static("/assets", "./frontend/assets")
	r.StaticFile("/", "./frontend/index.html")

	// WebSocket Connection Handler
	r.GET("/ws", func(c *gin.Context) {
		websocket.ServeWs(c.Writer, c.Request)
	})

	// Health Check API
	r.GET("/health", dashCtrl.HealthCheck)

	api := r.Group("/api")
	{
		// Authentication Module
		auth := api.Group("/auth")
		{
			auth.POST("/register", authCtrl.Register)
			auth.POST("/login", authCtrl.Login)
		}

		// Device Discovery
		api.POST("/devices/ping", deviceCtrl.RegisterOrPing)
		api.GET("/devices", deviceCtrl.List)

		// System Monitor & Stats
		api.GET("/dashboard/metrics", dashCtrl.GetMetrics)

		// Public/CLI File Share endpoints
		api.GET("/files/download/:id", fileCtrl.Download)
		api.POST("/files/upload", fileCtrl.UploadSingle) // Direct cURL upload

		// Protected Workspace Operations (Requires JWT)
		protected := api.Group("")
		protected.Use(middleware.AuthMiddleware())
		{
			// Code Snippets Vault
			snippets := protected.Group("/snippets")
			{
				snippets.POST("", snippetCtrl.Create)
				snippets.GET("", snippetCtrl.List)
				snippets.GET("/:id", snippetCtrl.GetByID)
				snippets.PUT("/:id", snippetCtrl.Update)
				snippets.DELETE("/:id", snippetCtrl.Delete)
				snippets.POST("/:id/favorite", snippetCtrl.ToggleFavorite)
			}

			// Managed File sharing (chunk transfers, deletions, lists)
			files := protected.Group("/files")
			{
				files.GET("", fileCtrl.List)
				files.POST("/upload-chunk", fileCtrl.UploadChunk)
				files.POST("/merge-chunks", fileCtrl.MergeChunks)
				files.DELETE("/:id", fileCtrl.Delete)
			}
		}
	}

	return r
}
