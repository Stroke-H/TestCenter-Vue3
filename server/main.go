package main

import (
	"log"
	"net/http"
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
	r.Use(cors.New(config))

	// Serve static files for K6 reports
	// We will map the /reports route to the local ../k6-scripts/reports directory
	r.StaticFS("/reports", http.Dir("../k6-scripts/reports"))

	// API Routes
	api := r.Group("/api")
	{
		// WebSocket endpoint for streaming K6 execution
		api.GET("/ws/k6", services.RunK6TestHandler)
		// Generic HTTP proxy to avoid CORS for external APIs
		api.POST("/proxy", services.ProxyHandler)
	}

	log.Println("TestCenter Go Backend starting on :8080")
	if err := r.Run(":8080"); err != nil {
		log.Fatal("Failed to start server: ", err)
	}
}
