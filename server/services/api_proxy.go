package services

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// ProxyRequest 定义了前端发送的代理请求结构
type ProxyRequest struct {
	Method       string                 `json:"method"` // GET, POST, etc.
	URL          string                 `json:"url"`
	Headers      map[string]interface{} `json:"headers,omitempty"`
	Body         json.RawMessage        `json:"body,omitempty"`
	Data         map[string]interface{} `json:"data,omitempty"` // 兼容旧版：同时作为请求头和请求体
	ResponseMode string                 `json:"response_mode,omitempty"`
}

type ProxyResponseEnvelope struct {
	Status  int                 `json:"status"`
	Headers map[string][]string `json:"headers"`
	Cookies map[string]string   `json:"cookies"`
	Body    interface{}         `json:"body"`
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

	// 1. 准备目标请求的 Body（非 GET/DELETE/HEAD 请求才包含 Body）。
	// 新版请求明确区分 headers/body；未提供 body 时继续兼容旧版 data。
	var bodyReader io.Reader
	if method != "GET" && method != "DELETE" && method != "HEAD" {
		bodyBytes := []byte(req.Body)
		if len(bytes.TrimSpace(bodyBytes)) == 0 {
			var err error
			bodyBytes, err = json.Marshal(req.Data)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to serialize body"})
				return
			}
		}
		bodyReader = bytes.NewBuffer(bodyBytes)
	}

	// 2. 创建 HTTP 客户端请求
	httpReq, err := http.NewRequest(method, req.URL, bodyReader)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create upstream request"})
		return
	}

	// 3. 设置 Headers。新版优先使用独立 headers；未提供时回退到旧版 data。
	requestHeaders := req.Headers
	if requestHeaders == nil {
		requestHeaders = req.Data
	}
	for k, v := range requestHeaders {
		// 跳过可能导致压缩或长度冲突的 Header
		lowerK := strings.ToLower(k)
		if lowerK == "accept-encoding" || lowerK == "content-length" || lowerK == "connection" {
			continue
		}

		var headerValue string
		switch value := v.(type) {
		case string:
			headerValue = value
		case float64, bool, json.Number:
			headerValue = fmt.Sprint(value)
		case nil:
			continue
		default:
			c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("Invalid header value for %s", k)})
			return
		}
		httpReq.Header.Set(k, headerValue)
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
	var responseBody interface{}
	if err := json.Unmarshal(respBytes, &responseBody); err != nil {
		responseBody = string(respBytes)
	}

	// envelope 模式用于接口链路提取响应头、Cookie 和响应体变量。
	// 未开启时保持旧响应格式，避免影响现有代理调用。
	if strings.EqualFold(req.ResponseMode, "envelope") {
		cookies := make(map[string]string)
		for _, cookie := range resp.Cookies() {
			cookies[cookie.Name] = cookie.Value
		}
		c.JSON(resp.StatusCode, ProxyResponseEnvelope{
			Status:  resp.StatusCode,
			Headers: resp.Header,
			Cookies: cookies,
			Body:    responseBody,
		})
		return
	}

	if _, ok := responseBody.(string); ok {
		c.Data(resp.StatusCode, "text/plain; charset=utf-8", respBytes)
		return
	}
	c.JSON(resp.StatusCode, responseBody)
}
