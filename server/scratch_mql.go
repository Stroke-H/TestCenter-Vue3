package main

import (
	"context"
	"encoding/json"
	"fmt"
	"testcenter-server/feishu/model"
	"testcenter-server/feishu/service"
)

func main() {
	model.GlobalFeishuConfig = &model.FeishuConfig{
		MCP: &model.MCPConfig{
			Enabled:   true,
			ServerURL: "http://127.0.0.1:8081/mcp", // The local proxy or whatever is set in config
			Token:     "test", // Need the real token
		},
	}
	
	// Read real config
	var conf model.FeishuConfig
	data, _ := ioutil.ReadFile("/Users/apple/TestCenter_Vue3/server/data/feishu_config.json")
	json.Unmarshal(data, &conf)
	service.GlobalMCPClient = &service.MCPClient{
		ServerURL: conf.MCP.ServerURL,
		Token:     conf.MCP.Token,
		Client:    &http.Client{},
	}
}
