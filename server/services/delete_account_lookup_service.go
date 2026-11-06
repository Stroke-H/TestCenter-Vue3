package services

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

type deleteAccountLookupRequest struct {
	UID         string `json:"uid"`
	App         string `json:"app"`
	Project     string `json:"project"`
	Environment string `json:"environment"`
}

// Only server-configured admin origins are used. Never send admin credentials
// to a client-supplied custom domain. This endpoint cannot delete an account.
func DeleteAccountLookupHandler(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	if _, err := CurrentUserFromRequest(c); err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "请先登录平台"})
		return
	}
	var req deleteAccountLookupRequest
	if c.ShouldBindJSON(&req) != nil || strings.TrimSpace(req.UID) == "" || strings.TrimSpace(req.App) == "" || strings.TrimSpace(req.Project) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请填写 UID、app 并选择项目"})
		return
	}
	if req.Environment != "test" && req.Environment != "prod" && req.Environment != "gray" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请选择后台查询环境：测试、正式或灰度"})
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 45*time.Second)
	defer cancel()
	headers, err := lookupDeleteAccountUser(ctx, getDramaProfile(req.Environment), strings.TrimSpace(req.App), strings.TrimSpace(req.UID), time.Now())
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}
	c.Header("Cache-Control", "no-store")
	c.JSON(http.StatusOK, gin.H{"headers": headers, "uid": strings.TrimSpace(req.UID), "start": "2025-01-01", "end": time.Now().In(time.FixedZone("Asia/Shanghai", 8*3600)).Format("2006-01-02")})
}

func lookupDeleteAccountUser(ctx context.Context, profile dramaProfile, app, uid string, now time.Time) (map[string]string, error) {
	if profile.Email == "" || profile.Password == "" {
		return nil, errors.New("对应环境未配置运营后台账号密码")
	}
	u, err := url.Parse(profile.LoginURL)
	if err != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") {
		return nil, errors.New("运营后台登录地址配置无效")
	}
	client := &http.Client{Timeout: 40 * time.Second, CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse }}
	body, _ := json.Marshal(map[string]string{"app": "com.shorts.wave.drama", "email": profile.Email, "password": profile.Password})
	r, _ := http.NewRequestWithContext(ctx, http.MethodPost, u.String(), bytes.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(r)
	if err != nil {
		return nil, errors.New("运营后台认证请求失败或超时")
	}
	io.Copy(io.Discard, io.LimitReader(resp.Body, 1<<20))
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("运营后台认证失败（HTTP %d）", resp.StatusCode)
	}
	var token string
	for _, cookie := range resp.Cookies() {
		if cookie.Name == "x-token" {
			token = cookie.Value
		}
	}
	if token == "" {
		return nil, errors.New("运营后台未返回 x-token")
	}
	u.Path = "/api/management/user/list"
	u.RawQuery = url.Values{"app": {app}, "user_id": {uid}, "page": {"1"}, "page_size": {"20"}, "start": {"2025-01-01"}, "end": {now.In(time.FixedZone("Asia/Shanghai", 8*3600)).Format("2006-01-02")}}.Encode()
	u.Fragment = ""
	r, _ = http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	r.Header.Set("Cookie", "x-token="+token)
	resp, err = client.Do(r)
	if err != nil {
		return nil, errors.New("后台用户查询失败或超时")
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("后台用户查询失败（HTTP %d）", resp.StatusCode)
	}
	var payload struct {
		Code int             `json:"code"`
		Data json.RawMessage `json:"data"`
	}
	if json.NewDecoder(io.LimitReader(resp.Body, 8<<20)).Decode(&payload) != nil {
		return nil, errors.New("后台用户响应格式无效")
	}
	if payload.Code != 0 && payload.Code != 200 {
		return nil, errors.New("后台用户查询返回业务错误")
	}
	return mapDeleteAccountUser(payload.Data, app, uid)
}

func mapDeleteAccountUser(data json.RawMessage, app, uid string) (map[string]string, error) {
	var rows []map[string]interface{}
	if json.Unmarshal(data, &rows) != nil {
		return nil, errors.New("后台用户 data 不是用户列表")
	}
	var selected map[string]interface{}
	for _, row := range rows {
		matches := false
		for _, key := range []string{"uid", "user_id", "id"} {
			if value, ok := row[key].(string); ok && value == uid {
				matches = true
			}
		}
		if !matches {
			continue
		}
		if value, _ := row["app"].(string); value != app {
			continue
		}
		if selected != nil {
			return nil, errors.New("查询到多个同 UID 用户，无法安全选择")
		}
		selected = row
	}
	if selected == nil {
		return nil, errors.New("未找到 app 和 UID 完全匹配的用户，不会执行删号")
	}
	mapping := map[string][]string{
		"device-uuid": {"device_uuid", "device-uuid", "device_id"}, "advertising-id": {"advertising_id", "advertising-id"},
		"app": {"app"}, "version": {"app_version", "version"}, "User-Agent": {"user_agent", "User-Agent"},
		"mobile-brand": {"mobile_brand", "mobile-brand"}, "timezone": {"time_zone", "timezone"},
		"afid": {"afid"}, "idfv": {"idfv"}, "lang": {"lang"}, "region": {"region"},
		"attribution_id": {"attribution_id"}, "attribution_type": {"attribution_type"},
		"os_ver": {"os_ver"}, "os": {"os"}, "model": {"model"}, "build": {"build"},
		"height": {"height"}, "width": {"width"}, "rule-ab": {"rule_ab", "rule-ab"},
	}
	headers := map[string]string{}
	for header, keys := range mapping {
		for _, key := range keys {
			var value string
			switch v := selected[key].(type) {
			case string:
				value = v
			case float64:
				value = fmt.Sprintf("%v", v)
			}
			if strings.TrimSpace(value) != "" && !strings.ContainsAny(value, "\r\n") {
				headers[header] = value
				break
			}
		}
	}
	if headers["device-uuid"] == "" {
		return nil, errors.New("后台用户缺少 device_id/device_uuid，不能安全自动匿名登录")
	}
	// Never reuse an old user's session/refresh token from the admin response.
	return headers, nil
}
