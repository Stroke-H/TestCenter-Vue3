package services

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"
)

const (
	defaultPaymentRecordFeishuWikiToken = ""
	defaultPaymentRecordFeishuTableID   = ""
)

var paymentRecordFeishuSchemaMu sync.Mutex

type paymentRecordFeishuField struct {
	FieldID   string `json:"field_id"`
	FieldName string `json:"field_name"`
	Type      int    `json:"type"`
	IsPrimary bool   `json:"is_primary"`
}

type paymentRecordFeishuResult struct {
	RecordID string
}

func paymentRecordFeishuSetting(envName, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(envName)); value != "" {
		return value
	}
	return fallback
}

func paymentRecordFeishuRequest(ctx context.Context, method, apiURL, token string, payload interface{}) ([]byte, error) {
	var body io.Reader
	if payload != nil {
		encoded, err := json.Marshal(payload)
		if err != nil {
			return nil, err
		}
		body = bytes.NewReader(encoded)
	}
	req, err := http.NewRequestWithContext(ctx, method, apiURL, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json; charset=utf-8")
	resp, err := (&http.Client{Timeout: 8 * time.Second}).Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	responseBody, readErr := io.ReadAll(resp.Body)
	if readErr != nil {
		return nil, readErr
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return responseBody, fmt.Errorf("feishu api error: status %d, body: %s", resp.StatusCode, string(responseBody))
	}
	var envelope struct {
		Code int    `json:"code"`
		Msg  string `json:"msg"`
	}
	if err := json.Unmarshal(responseBody, &envelope); err != nil {
		return responseBody, fmt.Errorf("decode feishu response: %w", err)
	}
	if envelope.Code != 0 {
		return responseBody, fmt.Errorf("feishu api error: code=%d, msg=%s", envelope.Code, envelope.Msg)
	}
	return responseBody, nil
}

func resolvePaymentRecordFeishuAppToken(ctx context.Context, tenantToken string) (string, error) {
	wikiToken := paymentRecordFeishuSetting("PAYMENT_RECORD_FEISHU_WIKI_TOKEN", defaultPaymentRecordFeishuWikiToken)
	apiURL := "https://open.feishu.cn/open-apis/wiki/v2/spaces/get_node?token=" + url.QueryEscape(wikiToken)
	responseBody, err := paymentRecordFeishuRequest(ctx, http.MethodGet, apiURL, tenantToken, nil)
	if err != nil {
		return "", err
	}
	var result struct {
		Data struct {
			Node struct {
				ObjToken string `json:"obj_token"`
				ObjType  string `json:"obj_type"`
			} `json:"node"`
		} `json:"data"`
	}
	if err := json.Unmarshal(responseBody, &result); err != nil {
		return "", err
	}
	if strings.ToLower(strings.TrimSpace(result.Data.Node.ObjType)) != "bitable" || strings.TrimSpace(result.Data.Node.ObjToken) == "" {
		return "", fmt.Errorf("支付记录飞书链接不是有效的多维表格")
	}
	return result.Data.Node.ObjToken, nil
}

func listPaymentRecordFeishuFields(ctx context.Context, tenantToken, appToken, tableID string) ([]paymentRecordFeishuField, error) {
	apiURL := fmt.Sprintf("https://open.feishu.cn/open-apis/bitable/v1/apps/%s/tables/%s/fields?page_size=100",
		url.PathEscape(appToken), url.PathEscape(tableID))
	responseBody, err := paymentRecordFeishuRequest(ctx, http.MethodGet, apiURL, tenantToken, nil)
	if err != nil {
		return nil, err
	}
	var result struct {
		Data struct {
			Items []paymentRecordFeishuField `json:"items"`
		} `json:"data"`
	}
	if err := json.Unmarshal(responseBody, &result); err != nil {
		return nil, err
	}
	return result.Data.Items, nil
}

func createPaymentRecordFeishuField(ctx context.Context, tenantToken, appToken, tableID, name string, fieldType int) error {
	apiURL := fmt.Sprintf("https://open.feishu.cn/open-apis/bitable/v1/apps/%s/tables/%s/fields",
		url.PathEscape(appToken), url.PathEscape(tableID))
	_, err := paymentRecordFeishuRequest(ctx, http.MethodPost, apiURL, tenantToken, map[string]interface{}{
		"field_name": name,
		"type":       fieldType,
	})
	return err
}

func renamePaymentRecordFeishuPrimaryField(ctx context.Context, tenantToken, appToken, tableID string, field paymentRecordFeishuField) error {
	apiURL := fmt.Sprintf("https://open.feishu.cn/open-apis/bitable/v1/apps/%s/tables/%s/fields/%s",
		url.PathEscape(appToken), url.PathEscape(tableID), url.PathEscape(field.FieldID))
	_, err := paymentRecordFeishuRequest(ctx, http.MethodPut, apiURL, tenantToken, map[string]interface{}{
		"field_name": "支付记录",
		"type":       field.Type,
	})
	return err
}

func ensurePaymentRecordFeishuSchema(ctx context.Context, tenantToken, appToken, tableID string) (string, error) {
	paymentRecordFeishuSchemaMu.Lock()
	defer paymentRecordFeishuSchemaMu.Unlock()

	fields, err := listPaymentRecordFeishuFields(ctx, tenantToken, appToken, tableID)
	if err != nil {
		return "", err
	}
	primaryName := ""
	existing := make(map[string]struct{}, len(fields))
	for _, field := range fields {
		existing[field.FieldName] = struct{}{}
		if field.IsPrimary {
			primaryName = field.FieldName
			if field.FieldName != "支付记录" {
				if err := renamePaymentRecordFeishuPrimaryField(ctx, tenantToken, appToken, tableID, field); err != nil {
					return "", fmt.Errorf("重命名飞书主字段失败: %w", err)
				}
				primaryName = "支付记录"
			}
		}
	}
	if primaryName == "" {
		return "", fmt.Errorf("飞书多维表格缺少主字段")
	}
	definitions := []struct {
		Name string
		Type int
	}{
		{Name: "支付时间", Type: 5},
		{Name: "支付人", Type: 1},
		{Name: "支付金额", Type: 2},
		{Name: "币种", Type: 1},
		{Name: "支付方式", Type: 1},
		{Name: "消费应用", Type: 1},
		{Name: "退款状态", Type: 1},
		{Name: "对公支付申报状态", Type: 1},
		{Name: "备注", Type: 1},
		{Name: "平台创建人", Type: 1},
		{Name: "平台记录ID", Type: 1},
		{Name: "更新时间", Type: 5},
	}
	for _, definition := range definitions {
		if _, ok := existing[definition.Name]; ok {
			continue
		}
		if err := createPaymentRecordFeishuField(ctx, tenantToken, appToken, tableID, definition.Name, definition.Type); err != nil {
			return "", fmt.Errorf("创建飞书字段“%s”失败: %w", definition.Name, err)
		}
	}
	return primaryName, nil
}

func paymentRecordFeishuTitle(record PaymentRecord) string {
	parts := []string{record.PaymentTime.In(paymentRecordLocation()).Format("2006-01-02"), record.ConsumerApp, fmt.Sprintf("$%.2f", record.Amount), record.Payer}
	return strings.Join(parts, " · ")
}

func paymentRecordFeishuFields(record PaymentRecord, primaryName string) map[string]interface{} {
	return map[string]interface{}{
		primaryName: paymentRecordFeishuTitle(record),
		"支付时间":      record.PaymentTime.UnixMilli(),
		"支付人":       record.Payer,
		"支付金额":      record.Amount,
		"币种":        record.Currency,
		"支付方式":      record.PaymentMethod,
		"消费应用":      record.ConsumerApp,
		"退款状态":      record.RefundStatus,
		"对公支付申报状态":  record.DeclarationStatus,
		"备注":        record.Remark,
		"平台创建人":     record.CreatedByName,
		"平台记录ID":    record.ID,
		"更新时间":      record.UpdatedAt.UnixMilli(),
	}
}

func syncPaymentRecordToFeishu(ctx context.Context, record PaymentRecord) (paymentRecordFeishuResult, error) {
	tenantToken, err := getFeishuTenantAccessToken()
	if err != nil {
		return paymentRecordFeishuResult{}, err
	}
	appToken, err := resolvePaymentRecordFeishuAppToken(ctx, tenantToken)
	if err != nil {
		return paymentRecordFeishuResult{}, err
	}
	tableID := paymentRecordFeishuSetting("PAYMENT_RECORD_FEISHU_TABLE_ID", defaultPaymentRecordFeishuTableID)
	primaryName, err := ensurePaymentRecordFeishuSchema(ctx, tenantToken, appToken, tableID)
	if err != nil {
		return paymentRecordFeishuResult{}, err
	}
	payload := map[string]interface{}{"fields": paymentRecordFeishuFields(record, primaryName)}
	method := http.MethodPost
	apiURL := fmt.Sprintf("https://open.feishu.cn/open-apis/bitable/v1/apps/%s/tables/%s/records?client_token=%s",
		url.PathEscape(appToken), url.PathEscape(tableID), url.QueryEscape(record.ID))
	if strings.TrimSpace(record.FeishuRecordID) != "" {
		method = http.MethodPut
		apiURL = fmt.Sprintf("https://open.feishu.cn/open-apis/bitable/v1/apps/%s/tables/%s/records/%s",
			url.PathEscape(appToken), url.PathEscape(tableID), url.PathEscape(record.FeishuRecordID))
	}
	var responseBody []byte
	for attempt := 1; attempt <= 3; attempt++ {
		responseBody, err = paymentRecordFeishuRequest(ctx, method, apiURL, tenantToken, payload)
		if err == nil {
			break
		}
		if attempt < 3 {
			select {
			case <-ctx.Done():
				return paymentRecordFeishuResult{}, ctx.Err()
			case <-time.After(time.Duration(attempt) * 200 * time.Millisecond):
			}
		}
	}
	if err != nil {
		return paymentRecordFeishuResult{}, err
	}
	var response struct {
		Data struct {
			Record struct {
				RecordID string `json:"record_id"`
			} `json:"record"`
		} `json:"data"`
	}
	if err := json.Unmarshal(responseBody, &response); err != nil {
		return paymentRecordFeishuResult{}, err
	}
	recordID := strings.TrimSpace(response.Data.Record.RecordID)
	if recordID == "" {
		recordID = record.FeishuRecordID
	}
	if recordID == "" {
		return paymentRecordFeishuResult{}, fmt.Errorf("飞书同步成功但未返回记录 ID")
	}
	return paymentRecordFeishuResult{RecordID: recordID}, nil
}
