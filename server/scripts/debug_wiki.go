package main

import (
	"fmt"
	"log"
	"strings"
	"testcenter-server/feishu/model"
	"testcenter-server/feishu/service"
)

func main() {
	config, err := model.LoadConfig("data/feishu_config.json")
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	service.InitFeishuClient(config.AppID, config.AppSecret)

	wikiURL := "https://webeye.feishu.cn/wiki/CSmZwtv2UiHmz9kYqg2cHjIgnkU?from=from_copylink"
	parts := strings.Split(wikiURL, "/")
	nodeToken := ""
	for i, p := range parts {
		if p == "wiki" && i+1 < len(parts) {
			nodeToken = strings.Split(parts[i+1], "?")[0]
			break
		}
	}
	fmt.Println("nodeToken:", nodeToken)

	objToken, objType, err := service.FeishuClientInstance.GetWikiNode(nodeToken)
	fmt.Printf("GetWikiNode: objToken=%v, objType=%v, err=%v\n", objToken, objType, err)

	if err == nil && objType == "docx" {
		cleanedReport := "Test report body for 1168"
		displayVersion := "V2.58.0"

		h2Block := map[string]interface{}{
			"block_type": 4, // Heading 2
			"heading2": map[string]interface{}{
				"style": map[string]interface{}{},
				"elements": []map[string]interface{}{
					{
						"text_run": map[string]interface{}{
							"content": strings.ToUpper(displayVersion), // V2.58.0
						},
					},
				},
			},
		}
		textBlock := map[string]interface{}{
			"block_type": 2, // Text
			"text": map[string]interface{}{
				"style": map[string]interface{}{},
				"elements": []map[string]interface{}{
					{
						"text_run": map[string]interface{}{
							"content": cleanedReport + "\n",
						},
					},
				},
			},
		}

		err = service.FeishuClientInstance.AddBlocksToDocx(objToken, 0, []map[string]interface{}{h2Block, textBlock})
		fmt.Println("AddBlocksToDocx err:", err)
	}
}
