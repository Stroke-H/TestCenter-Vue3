package feishu

import (
	"context"
	"github.com/gin-gonic/gin"
	larkws "github.com/larksuite/oapi-sdk-go/v3/ws"
	"log"
	"net/http"
	"testcenter-server/feishu/client"
	"testcenter-server/feishu/controller"
	"testcenter-server/feishu/model"
	"testcenter-server/feishu/service"
	"testcenter-server/services"
)

// InitFeishuBridge initializes the Feishu Bot module
func InitFeishuBridge(r *gin.Engine) {
	// 1. Load Config
	config, err := model.LoadConfig("data/feishu_config.json")
	model.LoadAIConfig() // Load DeepSeek configuration
	if err != nil {
		log.Printf("[Feishu] Failed to load config: %v. Module skipped.\n", err)
		return
	}
	model.GlobalFeishuConfig = config
	model.LoadTestPhones() // Initialize migrated phone data

	if config.AppID == "" || config.AppSecret == "" {
		log.Println("[Feishu] AppID or AppSecret is empty. Please check data/feishu_config.json")
		return
	}

	// 2. Initialize Client
	client.InitClient(config.AppID, config.AppSecret)
	service.InitFeishuClient(config.AppID, config.AppSecret) // [NEW] 为 Wiki 读写能力初始化 Client

	// 3. Start according to mode
	if config.Mode == "long_conn" {
		startLongConnection(config)
	} else {
		log.Printf("[Feishu] Mode %s not implemented or supported in Phase 1\n", config.Mode)
	}

	// 4. Register Dashboard AI Routes (Protected)
	api := r.Group("/api/ai")
	api.Use(func(c *gin.Context) {
		token := c.GetHeader("Authorization")
		if token == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized: No token provided"})
			c.Abort()
			return
		}
		// We can now use the central GetUserByID
		_, err := services.GetUserByID(token)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized: Invalid token"})
			c.Abort()
			return
		}
		c.Next()
	})

	{
		api.GET("/logs/operations", controller.GetOperationLogsHandler)
		api.POST("/web-chat", controller.WebChatHandler)
		api.POST("/web-chat/end", controller.EndWebChatHandler)
	}
}

func startLongConnection(config *model.FeishuConfig) {
	// a. Create Dispatcher (no tokens needed for long connection)
	dispatcher := service.NewEventDispatcher("", "")

	// b. Create WS Client
	cli := larkws.NewClient(config.AppID, config.AppSecret,
		larkws.WithEventHandler(dispatcher),
	)

	// c. Start in a background goroutine
	go func() {
		log.Println("[Feishu] Starting Long Connection (WebSocket)...")
		err := cli.Start(context.Background())
		if err != nil {
			log.Printf("[Feishu] Long Connection error: %v\n", err)
		}
	}()
}
