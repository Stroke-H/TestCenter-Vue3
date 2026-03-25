package services

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// ProxyRequest 定义了前端发送的代理请求结构
type ProxyRequest struct {
	Method string                 `json:"method"` // GET, POST, etc.
	URL    string                 `json:"url"`
	Data   map[string]interface{} `json:"data"`
}

// ProxyHandler 处理通用的代理请求
func ProxyHandler(c *gin.Context) {
	var req ProxyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	method := strings.ToUpper(req.Method)
	if method == "" {
		method = "POST"
	}

	// 1. 准备目标请求的 Body (非 GET/DELETE 请求才包含 Body)
	var bodyReader io.Reader
	if method != "GET" && method != "DELETE" && method != "HEAD" {
		bodyBytes, err := json.Marshal(req.Data)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to serialize body"})
			return
		}
		bodyReader = bytes.NewBuffer(bodyBytes)
	}

	// 2. 创建 HTTP 客户端请求
	httpReq, err := http.NewRequest(method, req.URL, bodyReader)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create upstream request"})
		return
	}

	// 3. 设置 Headers (将 Data 中的所有键值对都作为 Header 传入)
	for k, v := range req.Data {
		// 跳过可能导致压缩或长度冲突的 Header
		lowerK := strings.ToLower(k)
		if lowerK == "accept-encoding" || lowerK == "content-length" || lowerK == "connection" {
			continue
		}

		switch value := v.(type) {
		case string:
			httpReq.Header.Set(k, value)
		}
	}

	// 针对非 GET 请求设置 Content-Type
	if method != "GET" {
		if httpReq.Header.Get("Content-Type") == "" {
			httpReq.Header.Set("Content-Type", "application/json")
		}
	}

	// 4. 执行请求
	client := &http.Client{}
	resp, err := client.Do(httpReq)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Proxy request failed: " + err.Error()})
		return
	}
	defer resp.Body.Close()

	// 5. 处理响应结果
	respBytes, _ := io.ReadAll(resp.Body)
	var jsonResult interface{}
	// 尝试解析为 JSON
	if err := json.Unmarshal(respBytes, &jsonResult); err == nil {
		c.JSON(resp.StatusCode, jsonResult)
	} else {
		// 如果不是 JSON，直接返回原始字节流
		c.Data(resp.StatusCode, "text/plain; charset=utf-8", respBytes)
	}
}
