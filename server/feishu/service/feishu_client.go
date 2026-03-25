package service

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"
)

type FeishuClient struct {
	AppID     string
	AppSecret string
	token     string
	expireAt  time.Time
	mu        sync.Mutex
}

var FeishuClientInstance *FeishuClient

func InitFeishuClient(appID, appSecret string) {
	FeishuClientInstance = &FeishuClient{
		AppID:     appID,
		AppSecret: appSecret,
	}
}

// GetToken returns a valid tenant_access_token
func (c *FeishuClient) GetToken() (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.token != "" && time.Now().Before(c.expireAt) {
		return c.token, nil
	}

	url := "https://open.feishu.cn/open-apis/auth/v3/tenant_access_token/internal"
	body, _ := json.Marshal(map[string]string{
		"app_id":     c.AppID,
		"app_secret": c.AppSecret,
	})

	resp, err := http.Post(url, "application/json", bytes.NewBuffer(body))
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var res struct {
		Code              int    `json:"code"`
		Msg               string `json:"msg"`
		TenantAccessToken string `json:"tenant_access_token"`
		Expire            int    `json:"expire"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return "", err
	}

	if res.Code != 0 {
		return "", fmt.Errorf("feishu auth failed: %s", res.Msg)
	}

	c.token = res.TenantAccessToken
	c.expireAt = time.Now().Add(time.Duration(res.Expire-60) * time.Second) // Buffer 1 min
	return c.token, nil
}

// DoRequest makes an authorized request to Feishu API
func (c *FeishuClient) DoRequest(method, url string, body interface{}) ([]byte, error) {
	token, err := c.GetToken()
	if err != nil {
		return nil, err
	}

	var bodyReader io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		bodyReader = bytes.NewBuffer(b)
	}

	req, err := http.NewRequest(method, url, bodyReader)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	resBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return resBody, fmt.Errorf("feishu api error: status %d, body: %s", resp.StatusCode, string(resBody))
	}

	return resBody, nil
}

// GetWikiNode maps a Wiki node token to its object token and type
func (c *FeishuClient) GetWikiNode(nodeToken string) (string, string, error) {
	url := fmt.Sprintf("https://open.feishu.cn/open-apis/wiki/v2/spaces/nodes/%s", nodeToken)
	resBody, err := c.DoRequest("GET", url, nil)
	if err != nil {
		return "", "", err
	}

	var res struct {
		Code int    `json:"code"`
		Msg  string `json:"msg"`
		Data struct {
			Node struct {
				ObjToken string `json:"obj_token"`
				ObjType  string `json:"obj_type"`
			} `json:"node"`
		} `json:"data"`
	}
	if err := json.NewDecoder(bytes.NewBuffer(resBody)).Decode(&res); err != nil {
		return "", "", err
	}
	if res.Code != 0 {
		return "", "", fmt.Errorf("wiki node mapping failed: %s", res.Msg)
	}
	return res.Data.Node.ObjToken, res.Data.Node.ObjType, nil
}

// PrependToDocx inserts content at the top of a Docx document
func (c *FeishuClient) PrependToDocx(docID string, text string) error {
	// 1. Get document kids to ensure it's a valid doc and maybe find insertion point
	// Actually for "prepend", we can just add to index 0 of the root block.
	// The root block ID for a docx is the docID itself.
	
	url := fmt.Sprintf("https://open.feishu.cn/open-apis/docx/v1/documents/%s/blocks/%s/children", docID, docID)
	payload := map[string]interface{}{
		"index": 0,
		"children": []map[string]interface{}{
			{
				"block_type": 2, // Text block
				"text": map[string]interface{}{
					"content": text + "\n",
					"style": map[string]interface{}{},
				},
			},
		},
	}
	_, err := c.DoRequest("POST", url, payload)
	return err
}

// GetDocxRawContent returns the plain text content of a Docx document
func (c *FeishuClient) GetDocxRawContent(docID string) (string, error) {
	url := fmt.Sprintf("https://open.feishu.cn/open-apis/docx/v1/documents/%s/raw_content", docID)
	resBody, err := c.DoRequest("GET", url, nil)
	if err != nil {
		return "", err
	}

	var res struct {
		Code int    `json:"code"`
		Msg  string `json:"msg"`
		Data struct {
			Content string `json:"content"`
		} `json:"data"`
	}
	if err := json.NewDecoder(bytes.NewBuffer(resBody)).Decode(&res); err != nil {
		return "", err
	}
	if res.Code != 0 {
		return "", fmt.Errorf("read docx failed: %s", res.Msg)
	}
	return res.Data.Content, nil
}
