package feishu

import (
	"context"
	"log"
	"testcenter-server/feishu/client"
	"testcenter-server/feishu/model"
	"testcenter-server/feishu/service"
	larkws "github.com/larksuite/oapi-sdk-go/v3/ws"
	"github.com/gin-gonic/gin"
)

// InitFeishuBridge initializes the Feishu Bot module
func InitFeishuBridge(r *gin.Engine) {
	// 1. Load Config
	config, err := model.LoadConfig("data/feishu_config.json")
	if err != nil {
		log.Printf("[Feishu] Failed to load config: %v. Module skipped.\n", err)
		return
	}

	if config.AppID == "" || config.AppSecret == "" {
		log.Println("[Feishu] AppID or AppSecret is empty. Please check data/feishu_config.json")
		return
	}

	// 2. Initialize Client
	client.InitClient(config.AppID, config.AppSecret)

	// 3. Start according to mode
	if config.Mode == "long_conn" {
		startLongConnection(config)
	} else {
		log.Printf("[Feishu] Mode %s not implemented or supported in Phase 1\n", config.Mode)
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
