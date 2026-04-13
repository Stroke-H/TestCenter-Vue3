package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"testcenter-server/feishu/model"

	"github.com/sashabaranov/go-openai/jsonschema"
)

type MCPRequest struct {
	JsonRPC string      `json:"jsonrpc"` // "2.0"
	Method  string      `json:"method"`  // "tools/call" or "tools/list"
	Params  interface{} `json:"params,omitempty"`
	ID      int         `json:"id"`
}

type MCPResponse struct {
	JsonRPC string          `json:"jsonrpc"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *MCPError       `json:"error,omitempty"`
	ID      int             `json:"id"`
}

type MCPError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type MCPToolDef struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	InputSchema json.RawMessage `json:"inputSchema"`
}

type MCPToolsListResult struct {
	Tools []MCPToolDef `json:"tools"`
}

type MCPToolCallParams struct {
	Name      string                 `json:"name"`
	Arguments map[string]interface{} `json:"arguments"`
}

type MCPToolCallResult struct {
	Content []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"content"`
	IsError bool `json:"isError"`
}

type MCPClient struct {
	ServerURL string
	Token     string
	Client    *http.Client
}

var GlobalMCPClient *MCPClient

func InitMCPClient() {
	if model.GlobalFeishuConfig != nil && model.GlobalFeishuConfig.MCP != nil && model.GlobalFeishuConfig.MCP.Enabled {
		GlobalMCPClient = &MCPClient{
			ServerURL: model.GlobalFeishuConfig.MCP.ServerURL,
			Token:     model.GlobalFeishuConfig.MCP.Token,
			Client:    &http.Client{},
		}
		
		// Attempt to register tools asynchronously
		go func() {
			err := GlobalMCPClient.RegisterMCPTools()
			if err != nil {
				log.Printf("[Feishu MCP] Failed to register tools: %v", err)
			} else {
				log.Printf("[Feishu MCP] Successfully registered MCP tools")
			}
		}()
	}
}

func (m *MCPClient) RegisterMCPTools() error {
	ctx := context.Background()
	tools, err := m.ListTools(ctx)
	if err != nil {
		return err
	}

	for _, t := range tools {
		// Needs to translate raw json schema to jsonschema.Definition
		// An easy way is unmarshalling it
		var params jsonschema.Definition
		if err := json.Unmarshal(t.InputSchema, &params); err != nil {
			log.Printf("[Feishu MCP] Failed to parse inputSchema for tool %s: %v", t.Name, err)
			continue
		}

		toolName := t.Name // Capture variable
		
		RegisterTool(ToolDef{
			Name:        toolName,
			Description: "[飞书项目] " + t.Description,
			Parameters:  &params,
			NeedConfirm: false, // For safety you might want true for create/update, but typically false for read
			Execute: func(ctx context.Context, chatID string, senderID string, args string) (string, error) {
				var parsedArgs map[string]interface{}
				if err := json.Unmarshal([]byte(args), &parsedArgs); err != nil {
					return "", fmt.Errorf("failed to parse JSON arguments: %v", err)
				}
				log.Printf("[Feishu MCP] Calling Tool '%s' with args: %v", toolName, parsedArgs)
				return m.CallTool(ctx, toolName, parsedArgs)
			},
		})
	}

	return nil
}

func (m *MCPClient) sendRequest(ctx context.Context, reqBody interface{}) (*MCPResponse, error) {
	if m.Token == "" {
		return nil, errors.New("MCP Token is not set")
	}

	b, err := json.Marshal(reqBody)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", m.ServerURL, bytes.NewBuffer(b))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Mcp-Token", m.Token)

	resp, err := m.Client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("MCP Server returned status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var mcpResp MCPResponse
	if err := json.Unmarshal(bodyBytes, &mcpResp); err != nil {
		return nil, fmt.Errorf("failed to parse MCP response: %v, body: %s", err, string(bodyBytes))
	}

	if mcpResp.Error != nil {
		return nil, fmt.Errorf("MCP Error %d: %s", mcpResp.Error.Code, mcpResp.Error.Message)
	}

	return &mcpResp, nil
}

func (m *MCPClient) ListTools(ctx context.Context) ([]MCPToolDef, error) {
	req := MCPRequest{
		JsonRPC: "2.0",
		Method:  "tools/list",
		ID:      1,
	}

	resp, err := m.sendRequest(ctx, req)
	if err != nil {
		return nil, err
	}

	var result MCPToolsListResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal tools/list result: %v", err)
	}

	return result.Tools, nil
}

func (m *MCPClient) CallTool(ctx context.Context, name string, args map[string]interface{}) (string, error) {
	req := MCPRequest{
		JsonRPC: "2.0",
		Method:  "tools/call",
		ID:      2,
		Params: MCPToolCallParams{
			Name:      name,
			Arguments: args,
		},
	}

	resp, err := m.sendRequest(ctx, req)
	if err != nil {
		return "", err
	}

	var result MCPToolCallResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return "", fmt.Errorf("failed to unmarshal tools/call result: %v", err)
	}

	if result.IsError {
		errMsg := "Unknown error"
		if len(result.Content) > 0 {
			errMsg = result.Content[0].Text
		}
		return "", fmt.Errorf("tool execution failed: %s", errMsg)
	}

	if len(result.Content) > 0 {
		return result.Content[0].Text, nil
	}

	return "{}", nil
}
