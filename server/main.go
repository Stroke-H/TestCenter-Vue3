package main

import (
	"log"
	"net/http"
	"os"
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
	config.AllowOriginFunc = func(origin string) bool { return true }
	config.AllowMethods = []string{"GET", "POST", "PUT", "PATCH", "DELETE", "HEAD", "OPTIONS"}
	config.AllowHeaders = []string{"Origin", "Content-Length", "Content-Type", "Authorization", "Accept"}
	config.AllowCredentials = true
	r.Use(cors.New(config))

	// Serve static files for K6 reports
	r.StaticFS("/reports", http.Dir("../k6-scripts/reports"))
	// Serve static files for Lighthouse performance reports
	r.StaticFS("/performance-reports", http.Dir("../report"))

	// API Routes
	services.StartMatchmaker() // Start the matching worker
	api := r.Group("/api")
	{
		// WebSocket endpoint for streaming K6 execution
		api.GET("/ws/k6", services.RunK6TestHandler)
		api.GET("/ws/drama-run", services.SubscribeDramaRunHandler)
		api.GET("/drama-runs/current", services.CurrentDramaRunHandler)
		api.POST("/drama-runs/start", services.StartDramaRunHandler)
		api.POST("/drama-runs/stop", services.StopDramaRunHandler)
		// WebSocket endpoint for Lighthouse execution
		api.GET("/ws/lighthouse", services.RunLighthouseHandler)
		// WebSocket endpoint for Playwright execution
		api.GET("/ws/playwright", services.PlaywrightWSHandler)
		// WebSocket endpoint for UI Inspector
		api.GET("/ws/inspector", services.ServeInspectorWS)
		// WebSocket endpoint for Jungle Chess online matching
		api.GET("/ws/jungle", services.JungleChessWSHandler)
		// Generic HTTP proxy to avoid CORS for external APIs
		api.POST("/proxy", services.ProxyHandler)

		// System Configuration
		services.InitConfigService()
		configGroup := api.Group("/config")
		{
			configGroup.GET("/projects", services.GetProjectsHandler)
			configGroup.POST("/projects", services.SaveProjectHandler)
			configGroup.DELETE("/projects/:id", services.DeleteProjectHandler)

			configGroup.GET("/devices", services.GetDevicesHandler)
			configGroup.POST("/devices", services.SaveDeviceHandler)
			configGroup.DELETE("/devices/:id", services.DeleteDeviceHandler)

			configGroup.GET("/accounts", services.GetAccountsHandler)
			configGroup.POST("/accounts", services.SaveAccountHandler)
			configGroup.GET("/permissions", services.GetPermissionListHandler)
			configGroup.GET("/permissions/me", services.GetCurrentUserPermissionsHandler)
			configGroup.POST("/permissions", services.SaveUserPermissionHandler)
			configGroup.DELETE("/permissions/:user_id", services.ResetUserPermissionHandler)
			configGroup.GET("/sandbox-accounts", services.GetSandboxAccountsHandler)
			configGroup.POST("/sandbox-accounts", services.SaveSandboxAccountHandler)
			configGroup.DELETE("/sandbox-accounts/:id", services.DeleteSandboxAccountHandler)
			configGroup.PUT("/sandbox-accounts/:id", services.UpdateSandboxAccountHandler)
		}

		// Database Infrastructure
		databaseGroup := api.Group("/database")
		{
			databaseGroup.GET("/config", services.GetDatabaseConfigHandler)
			databaseGroup.GET("/health", services.GetDatabaseHealthHandler)
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
			auth.POST("/logout", services.LogoutHandler)
			auth.GET("/me", services.GetUserMeHandler)
			auth.PUT("/profile", services.UpdateCurrentUserProfileHandler)
		}

		// Acceptance Reports (Protected)
		reports := api.Group("/acceptance-reports")
		reports.Use(func(c *gin.Context) {
			_, err := services.CurrentUserFromRequest(c)
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
			reports.POST("/send-feishu", services.SendAcceptanceReportToFeishuHandler)
			reports.POST("/sync-cloud-doc", services.SyncAcceptanceReportToCloudDocHandler)
			reports.POST("/fetch-project-items", services.FetchAcceptanceReportProjectItemsHandler)
		}

		// Performance Monitoring Routes
		performance := api.Group("/performance")
		{
			performance.GET("/reports", services.ListLighthouseReportsHandler)
			performance.POST("/analyze", services.AnalyzeLighthouseHandler)
		}

		// Execution Reports (Persistent)
		execReports := api.Group("/execution-reports")
		{
			execReports.GET("", services.GetExecutionReportsHandler)
			execReports.POST("", services.AddExecutionReportHandler)
			execReports.DELETE("", services.ClearExecutionReportsHandler)
		}

		// Scheduled Tasks
		scheduledTasks := api.Group("/scheduled-tasks")
		{
			scheduledTasks.GET("", services.ListScheduledTasksHandler)
			scheduledTasks.POST("", services.CreateScheduledTaskHandler)
			scheduledTasks.PUT("/:id", services.UpdateScheduledTaskHandler)
		}

		// Playwright UI Automation
		playwright := api.Group("/playwright")
		{
			playwright.GET("/suites", services.ListPlaywrightSuitesHandler)
			playwright.GET("/suites/:id", services.GetPlaywrightSuiteHandler)
			playwright.POST("/suites", services.SavePlaywrightSuiteHandler)
			playwright.DELETE("/suites/:id", services.DeletePlaywrightSuiteHandler)

			playwright.GET("/cases", services.ListPlaywrightCasesHandler)
			playwright.GET("/cases/:id", services.GetPlaywrightCaseHandler)
			playwright.POST("/cases", services.SavePlaywrightCaseHandler)
			playwright.DELETE("/cases/:id", services.DeletePlaywrightCaseHandler)

			playwright.GET("/keywords", services.ListUserKeywordsHandler)
			playwright.POST("/keywords", services.SaveUserKeywordHandler)
			playwright.DELETE("/keywords/:id", services.DeleteUserKeywordHandler)
			playwright.PUT("/keywords/suite/:name", services.RenameUserKeywordSuiteHandler)
			playwright.DELETE("/keywords/suite/:name", services.DeleteUserKeywordSuiteHandler)

			playwright.GET("/builtin-keywords", services.GetBuiltinKeywordsHandler)
		}

		// Node Skillify Tool
		skillify := api.Group("/skillify")
		{
			skillify.POST("/scan", services.SkillifyScanHandler)
			skillify.POST("/generate", services.SkillifyGenerateHandler)
			skillify.POST("/save", services.SkillifySaveHandler)
		}

		// Test Case Generation
		testcaseGen := api.Group("/testcase-gen")
		{
			testcaseGen.GET("/models", services.GetTestcaseAIModelsHandler)
			testcaseGen.GET("/review/roles", services.GetTestcaseReviewRolesHandler)
			testcaseGen.POST("/decompose", services.DecomposeRequirementHandler)
			testcaseGen.POST("/smart-decompose", services.SmartDecomposeHandler)
			testcaseGen.POST("/generate", services.GenerateTestCasesHandler)
			testcaseGen.POST("/review", services.ReviewTestCasesHandler)
			testcaseGen.POST("/review/optimize", services.OptimizeReviewedCasesHandler)
			testcaseGen.POST("/export", services.ExportTestCasesExcelHandler)

			// History Management
			testcaseGen.GET("/records", services.ListRecordsHandler)
			testcaseGen.GET("/records/:id", services.GetRecordHandler)
			testcaseGen.GET("/records/:id/download", services.DownloadRecordHandler)
			testcaseGen.POST("/records", services.SaveRecordHandler)
			testcaseGen.PUT("/records/:id", services.UpdateRecordHandler)
			testcaseGen.DELETE("/records/:id", services.DeleteRecordHandler)
		}
	}

	// Initialize Feishu Bot Bridge
	feishu.InitFeishuBridge(r)
	services.InitScheduledTaskService()

	host := os.Getenv("TESTCENTER_BACKEND_HOST")
	if host == "" {
		host = "0.0.0.0"
	}
	port := os.Getenv("TESTCENTER_BACKEND_PORT")
	if port == "" {
		port = "8080"
	}
	addr := host + ":" + port

	log.Println("TestCenter Go Backend starting on " + addr)
	if err := r.Run(addr); err != nil {
		log.Fatal("Failed to start server: ", err)
	}
}
