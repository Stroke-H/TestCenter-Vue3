package main

import (
	"log"
	"net/http"
	"testcenter-server/feishu"
	"testcenter-server/services"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

func main() {
	r := gin.Default()

	// Disable "You trusted all proxies" warning
	r.SetTrustedProxies(nil)

	// Configure CORS to allow the Vue frontend to connect
	config := cors.DefaultConfig()
	config.AllowAllOrigins = true
	config.AllowMethods = []string{"GET", "POST", "PUT", "PATCH", "DELETE", "HEAD", "OPTIONS"}
	config.AllowHeaders = []string{"Origin", "Content-Length", "Content-Type", "Authorization", "Accept"}
	config.AllowCredentials = true
	r.Use(cors.New(config))

	// Serve static files for K6 reports
	r.StaticFS("/reports", http.Dir("../k6-scripts/reports"))

	// API Routes
	api := r.Group("/api")
	{
		// WebSocket endpoint for streaming K6 execution
		api.GET("/ws/k6", services.RunK6TestHandler)
		// Generic HTTP proxy to avoid CORS for external APIs
		api.POST("/proxy", services.ProxyHandler)

		// System Configuration
		services.InitConfigService("data/projects.jsonl", "data/test_phones.jsonl")
		configGroup := api.Group("/config")
		{
			configGroup.GET("/projects", services.GetProjectsHandler)
			configGroup.POST("/projects", services.SaveProjectHandler)
			configGroup.DELETE("/projects/:id", services.DeleteProjectHandler)

			configGroup.GET("/devices", services.GetDevicesHandler)
			configGroup.POST("/devices", services.SaveDeviceHandler)
			configGroup.DELETE("/devices/:id", services.DeleteDeviceHandler)
		}

		// Mind Map Processes
		services.EnsureDataDir()
		processes := api.Group("/processes")
		{
			processes.GET("", services.ListProcessesHandler)
			processes.GET("/:id", services.GetProcessHandler)
			processes.POST("", services.SaveProcessHandler)
			processes.DELETE("/:id", services.DeleteProcessHandler)
		}

		// Core Authentication (Public)
		auth := api.Group("/auth")
		{
			auth.POST("/register", services.RegisterHandler)
			auth.POST("/login", services.LoginHandler)
			auth.GET("/me", services.GetUserMeHandler)
		}

		// Acceptance Reports (Protected)
		reports := api.Group("/acceptance-reports")
		reports.Use(func(c *gin.Context) {
			token := c.GetHeader("Authorization")
			if token == "" {
				c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized: No token provided"})
				c.Abort()
				return
			}
			_, err := services.GetUserByID(token)
			if err != nil {
				c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized: Invalid token"})
				c.Abort()
				return
			}
			c.Next()
		})
		{
			reports.GET("/list", services.GetAcceptanceReportsHandler)
			reports.POST("/save", services.SaveAcceptanceReportHandler)
		}
	}

	// Initialize Feishu Bot Bridge
	feishu.InitFeishuBridge(r)

	log.Println("TestCenter Go Backend starting on :8080")
	if err := r.Run(":8080"); err != nil {
		log.Fatal("Failed to start server: ", err)
	}
}
