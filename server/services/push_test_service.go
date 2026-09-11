package services

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

const pushTestResponseLimit = 2 << 20

var pushTestHTTPClient = &http.Client{Timeout: 20 * time.Second}

type pushTestTargetRequest struct {
	BaseURL string `json:"base_url"`
	Path    string `json:"path"`
}

type pushTestPushRequest struct {
	BaseURL string                 `json:"base_url"`
	Path    string                 `json:"path"`
	Payload map[string]interface{} `json:"payload"`
}

type pushTestStatsRequest struct {
	BaseURL string `json:"base_url"`
	Path    string `json:"path"`
	TaskID  string `json:"task_id"`
}

type pushTestUpstreamResult struct {
	OK         bool        `json:"ok"`
	Status     int         `json:"status"`
	DurationMS int64       `json:"duration_ms"`
	Body       interface{} `json:"body"`
}

func normalizePushTestTarget(baseURL string, requestPath string) (string, error) {
	baseURL = strings.TrimSpace(baseURL)
	if baseURL == "" {
		return "", fmt.Errorf("API 服务地址不能为空")
	}
	parsed, err := url.Parse(baseURL)
	if err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return "", fmt.Errorf("API 服务地址必须是有效的 http/https 地址")
	}
	if parsed.User != nil {
		return "", fmt.Errorf("API 服务地址不能包含账号信息")
	}

	requestPath = strings.TrimSpace(requestPath)
	if requestPath == "" {
		return "", fmt.Errorf("接口路径不能为空")
	}
	if strings.Contains(requestPath, "://") {
		return "", fmt.Errorf("接口路径只能填写相对路径")
	}
	if !strings.HasPrefix(requestPath, "/") {
		requestPath = "/" + requestPath
	}
	return strings.TrimRight(baseURL, "/") + requestPath, nil
}

func executePushTestUpstream(ctx context.Context, method string, targetURL string, payload interface{}) (*pushTestUpstreamResult, error) {
	var bodyReader io.Reader
	if payload != nil {
		body, err := json.Marshal(payload)
		if err != nil {
			return nil, fmt.Errorf("序列化请求体失败: %w", err)
		}
		bodyReader = bytes.NewReader(body)
	}

	req, err := http.NewRequestWithContext(ctx, method, targetURL, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("创建上游请求失败: %w", err)
	}
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	startedAt := time.Now()
	resp, err := pushTestHTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("连接推送服务失败: %w", err)
	}
	defer resp.Body.Close()

	rawBody, err := io.ReadAll(io.LimitReader(resp.Body, pushTestResponseLimit))
	if err != nil {
		return nil, fmt.Errorf("读取推送服务响应失败: %w", err)
	}
	var responseBody interface{}
	if len(bytes.TrimSpace(rawBody)) == 0 {
		responseBody = nil
	} else if err := json.Unmarshal(rawBody, &responseBody); err != nil {
		responseBody = string(rawBody)
	}

	return &pushTestUpstreamResult{
		OK:         resp.StatusCode >= http.StatusOK && resp.StatusCode < http.StatusMultipleChoices,
		Status:     resp.StatusCode,
		DurationMS: time.Since(startedAt).Milliseconds(),
		Body:       responseBody,
	}, nil
}

func PushTestHealthHandler(c *gin.Context) {
	var input pushTestTargetRequest
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数格式错误"})
		return
	}
	targetURL, err := normalizePushTestTarget(input.BaseURL, input.Path)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	result, err := executePushTestUpstream(c.Request.Context(), http.MethodGet, targetURL, nil)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, result)
}

func PushTestExecuteHandler(c *gin.Context) {
	var input pushTestPushRequest
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数格式错误"})
		return
	}
	if input.Payload == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "推送请求体不能为空"})
		return
	}
	targetURL, err := normalizePushTestTarget(input.BaseURL, input.Path)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	result, err := executePushTestUpstream(c.Request.Context(), http.MethodPost, targetURL, input.Payload)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, result)
}

func PushTestStatsHandler(c *gin.Context) {
	var input pushTestStatsRequest
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数格式错误"})
		return
	}
	input.TaskID = strings.TrimSpace(input.TaskID)
	if input.TaskID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "task_id 不能为空"})
		return
	}
	statsPath := strings.TrimRight(strings.TrimSpace(input.Path), "/") + "/" + url.PathEscape(input.TaskID)
	targetURL, err := normalizePushTestTarget(input.BaseURL, statsPath)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	result, err := executePushTestUpstream(c.Request.Context(), http.MethodGet, targetURL, nil)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, result)
}
