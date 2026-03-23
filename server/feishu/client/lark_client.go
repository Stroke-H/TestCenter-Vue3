package client

import (
	"sync"
	lark "github.com/larksuite/oapi-sdk-go/v3"
	larkcore "github.com/larksuite/oapi-sdk-go/v3/core"
)

var (
	larkClient *lark.Client
	once       sync.Once
)

// InitClient initializes the global Lark client singleton
func InitClient(appID, appSecret string) {
	once.Do(func() {
		larkClient = lark.NewClient(appID, appSecret, 
			lark.WithLogLevel(larkcore.LogLevelInfo),
			lark.WithLogReqAtDebug(true),
		)
	})
}

// GetClient returns the initialized Lark client
func GetClient() *lark.Client {
	return larkClient
}
