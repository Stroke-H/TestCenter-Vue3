package services

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"testcenter-server/database"
	feishumodel "testcenter-server/feishu/model"
)

const (
	feishuProjectOAuthRegistrationEndpoint = "https://project.feishu.cn/mcp_server/oauth/register"
	feishuProjectOAuthDeviceEndpoint       = "https://project.feishu.cn/goapi/v5/mcp_server/oauth/device_code"
	feishuProjectOAuthTokenEndpoint        = "https://project.feishu.cn/mcp_server/oauth/token"
	feishuProjectOAuthResource             = "https://project.feishu.cn/mcp_server/v1"
	feishuProjectDeviceGrant               = "urn:ietf:params:oauth:grant-type:device_code"
)

var (
	feishuProjectOAuthClientMu sync.Mutex
	feishuProjectOAuthTokenMu  sync.Mutex
	feishuProjectOAuthKeyMu    sync.Mutex
	feishuProjectOAuthOnce     sync.Once
	feishuProjectOAuthFileKey  []byte
	errFeishuProjectNoOAuth    = errors.New("未完成飞书项目授权")
)

type feishuProjectOAuthClient struct {
	ClientID     string
	ClientSecret string
}

type FeishuProjectOAuthStatus struct {
	Status             string `json:"status"`
	Authorized         bool   `json:"authorized"`
	AutoRefresh        bool   `json:"auto_refresh"`
	FeishuUserKey      string `json:"feishu_user_key,omitempty"`
	FeishuName         string `json:"feishu_name,omitempty"`
	FeishuEmail        string `json:"feishu_email,omitempty"`
	ExpiresAt          string `json:"expires_at,omitempty"`
	LastError          string `json:"last_error,omitempty"`
	SessionID          string `json:"session_id,omitempty"`
	UserCode           string `json:"user_code,omitempty"`
	VerificationURI    string `json:"verification_uri,omitempty"`
	VerificationURIAll string `json:"verification_uri_complete,omitempty"`
	PollInterval       int    `json:"poll_interval,omitempty"`
}

type feishuProjectDeviceResponse struct {
	DeviceCode              string `json:"device_code"`
	UserCode                string `json:"user_code"`
	VerificationURI         string `json:"verification_uri"`
	VerificationURL         string `json:"verification_url"`
	VerificationURIComplete string `json:"verification_uri_complete"`
	ExpiresIn               int    `json:"expires_in"`
	Interval                int    `json:"interval"`
}

type feishuProjectTokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"`
	Scope        string `json:"scope"`
	Error        string `json:"error"`
	Description  string `json:"error_description"`
}

type feishuProjectOAuthAPIEnvelope struct {
	Code  int             `json:"code"`
	Msg   string          `json:"msg"`
	Data  json.RawMessage `json:"data"`
	Error *struct {
		ID               int `json:"id"`
		LocalizedMessage struct {
			Message string `json:"message"`
		} `json:"localizedMessage"`
	} `json:"error"`
}

type feishuProjectOAuthSession struct {
	ID                      string
	UserID                  string
	DeviceCodeEncrypted     string
	ClientID                string
	ClientSecretEncrypted   string
	UserCode                string
	VerificationURI         string
	VerificationURIComplete string
	Status                  string
	LastError               string
	IntervalSeconds         int
	NextPollAt              time.Time
	ExpiresAt               time.Time
}

type defectFeishuOAuthContextKey struct{}

func InitFeishuProjectOAuthService() error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := ensureFeishuProjectOAuthSchema(ctx); err != nil {
		return err
	}
	feishuProjectOAuthOnce.Do(func() { go runFeishuProjectOAuthCredentialKeeper() })
	return nil
}

func ensureFeishuProjectOAuthSchema(ctx context.Context) error {
	db, _, err := DatabaseManager.DB(ctx)
	if err != nil {
		return err
	}
	statements := []string{
		`CREATE TABLE IF NOT EXISTS feishu_project_oauth_clients (
			id VARCHAR(32) PRIMARY KEY,
			client_id TEXT NOT NULL,
			client_secret_encrypted TEXT NOT NULL,
			created_at DATETIME(6) NOT NULL,
			updated_at DATETIME(6) NOT NULL
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci`,
		`CREATE TABLE IF NOT EXISTS feishu_project_oauth_credentials (
			platform_user_id VARCHAR(128) PRIMARY KEY,
			feishu_user_key VARCHAR(128) NOT NULL,
			feishu_name VARCHAR(255) NOT NULL,
			feishu_email VARCHAR(255) NOT NULL,
			oauth_client_id VARCHAR(255) NOT NULL DEFAULT '',
			oauth_client_secret_encrypted TEXT NOT NULL,
			access_token_encrypted TEXT NOT NULL,
			refresh_token_encrypted TEXT NOT NULL,
			token_type VARCHAR(32) NOT NULL,
			scope_text TEXT NOT NULL,
			expires_at DATETIME(6) NOT NULL,
			status VARCHAR(32) NOT NULL,
			last_error TEXT NOT NULL,
			authorized_at DATETIME(6) NOT NULL,
			updated_at DATETIME(6) NOT NULL,
			INDEX idx_feishu_project_oauth_status (status, expires_at)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci`,
		`CREATE TABLE IF NOT EXISTS feishu_project_oauth_sessions (
			id VARCHAR(36) PRIMARY KEY,
			platform_user_id VARCHAR(128) NOT NULL,
			device_code_encrypted TEXT NOT NULL,
			oauth_client_id VARCHAR(255) NOT NULL DEFAULT '',
			oauth_client_secret_encrypted TEXT NOT NULL,
			user_code VARCHAR(255) NOT NULL,
			verification_uri TEXT NOT NULL,
			verification_uri_complete TEXT NOT NULL,
			status VARCHAR(32) NOT NULL,
			last_error TEXT NOT NULL,
			interval_seconds INT NOT NULL,
			next_poll_at DATETIME(6) NOT NULL,
			expires_at DATETIME(6) NOT NULL,
			created_at DATETIME(6) NOT NULL,
			updated_at DATETIME(6) NOT NULL,
			INDEX idx_feishu_project_oauth_session_user (platform_user_id, status, expires_at)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci`,
	}
	for _, statement := range statements {
		if _, err := db.ExecContext(ctx, statement); err != nil {
			return err
		}
	}
	columns := []struct {
		table      string
		column     string
		definition string
	}{
		{"feishu_project_oauth_credentials", "oauth_client_id", "VARCHAR(255) NOT NULL DEFAULT ''"},
		{"feishu_project_oauth_credentials", "oauth_client_secret_encrypted", "TEXT NOT NULL"},
		{"feishu_project_oauth_sessions", "oauth_client_id", "VARCHAR(255) NOT NULL DEFAULT ''"},
		{"feishu_project_oauth_sessions", "oauth_client_secret_encrypted", "TEXT NOT NULL"},
	}
	for _, column := range columns {
		if err := ensureFeishuProjectOAuthColumn(ctx, db, column.table, column.column, column.definition); err != nil {
			return err
		}
	}
	// Migrate the one legacy shared client only to credentials that were
	// actually authorized with it. New authorizations never reuse this row.
	_, _ = db.ExecContext(ctx, `UPDATE feishu_project_oauth_credentials c
		JOIN feishu_project_oauth_clients legacy ON legacy.id='default'
		SET c.oauth_client_id=legacy.client_id,
			c.oauth_client_secret_encrypted=legacy.client_secret_encrypted
		WHERE c.oauth_client_id=''`)
	if err := ensureFeishuProjectOAuthUniqueIndex(ctx, db, "feishu_project_oauth_credentials", "uniq_feishu_project_oauth_user_key", "feishu_user_key"); err != nil {
		return err
	}
	return nil
}

func ensureFeishuProjectOAuthColumn(ctx context.Context, db *sql.DB, table, column, definition string) error {
	var count int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM information_schema.COLUMNS
		WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME=? AND COLUMN_NAME=?`, table, column).Scan(&count); err != nil {
		return err
	}
	if count > 0 {
		return nil
	}
	_, err := db.ExecContext(ctx, fmt.Sprintf("ALTER TABLE `%s` ADD COLUMN `%s` %s", table, column, definition))
	return err
}

func ensureFeishuProjectOAuthUniqueIndex(ctx context.Context, db *sql.DB, table, index, column string) error {
	var count int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM information_schema.STATISTICS
		WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME=? AND INDEX_NAME=?`, table, index).Scan(&count); err != nil {
		return err
	}
	if count > 0 {
		return nil
	}
	_, err := db.ExecContext(ctx, fmt.Sprintf("CREATE UNIQUE INDEX `%s` ON `%s` (`%s`)", index, table, column))
	return err
}

func feishuProjectOAuthEndpoint(envKey, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(envKey)); value != "" {
		return value
	}
	return fallback
}

func feishuProjectOAuthEncryptionKey() ([]byte, error) {
	material := strings.TrimSpace(os.Getenv("TESTCENTER_FEISHU_OAUTH_ENCRYPTION_KEY"))
	if material != "" {
		sum := sha256.Sum256([]byte("testcenter-feishu-project-oauth|" + material))
		return sum[:], nil
	}

	feishuProjectOAuthKeyMu.Lock()
	defer feishuProjectOAuthKeyMu.Unlock()
	if len(feishuProjectOAuthFileKey) == 32 {
		return append([]byte(nil), feishuProjectOAuthFileKey...), nil
	}
	keyPath := strings.TrimSpace(os.Getenv("TESTCENTER_FEISHU_OAUTH_KEY_FILE"))
	if keyPath == "" {
		databaseConfigPath := strings.TrimSpace(os.Getenv("TESTCENTER_DB_CONFIG_FILE"))
		if databaseConfigPath == "" {
			databaseConfigPath = "data/database_config.json"
		}
		keyPath = filepath.Join(filepath.Dir(databaseConfigPath), "feishu_project_oauth.key")
	}
	if encoded, err := os.ReadFile(keyPath); err == nil {
		key, decodeErr := base64.RawStdEncoding.DecodeString(strings.TrimSpace(string(encoded)))
		if decodeErr != nil || len(key) != 32 {
			return nil, errors.New("飞书项目 OAuth 本机密钥文件格式不正确")
		}
		feishuProjectOAuthFileKey = append([]byte(nil), key...)
		return append([]byte(nil), key...), nil
	} else if !os.IsNotExist(err) {
		return nil, fmt.Errorf("读取飞书项目 OAuth 本机密钥失败：%w", err)
	}
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Dir(keyPath), 0700); err != nil {
		return nil, err
	}
	encoded := base64.RawStdEncoding.EncodeToString(key)
	if err := os.WriteFile(keyPath, []byte(encoded+"\n"), 0600); err != nil {
		return nil, fmt.Errorf("写入飞书项目 OAuth 本机密钥失败：%w", err)
	}
	feishuProjectOAuthFileKey = append([]byte(nil), key...)
	return append([]byte(nil), key...), nil
}

func legacyFeishuProjectOAuthEncryptionKey() ([]byte, error) {
	config, _ := feishumodel.LoadConfig("data/feishu_config.json")
	parts := []string{}
	if config != nil {
		parts = append(parts, config.AppSecret)
		if config.MCP != nil {
			parts = append(parts, config.MCP.Token)
		}
	}
	parts = append(parts, database.LoadConfigFromEnv().Password)
	material := strings.Join(parts, "|")
	if strings.Trim(material, "|") == "" {
		return nil, errors.New("缺少旧版飞书项目 OAuth 加密密钥材料")
	}
	sum := sha256.Sum256([]byte("testcenter-feishu-project-oauth|" + material))
	return sum[:], nil
}

func encryptFeishuProjectOAuthValue(value string) (string, error) {
	if value == "" {
		return "", nil
	}
	key, err := feishuProjectOAuthEncryptionKey()
	if err != nil {
		return "", err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", err
	}
	sealed := gcm.Seal(nonce, nonce, []byte(value), nil)
	return base64.RawStdEncoding.EncodeToString(sealed), nil
}

func decryptFeishuProjectOAuthValue(value string) (string, error) {
	plain, _, err := decryptFeishuProjectOAuthValueWithMigration(value)
	return plain, err
}

func decryptFeishuProjectOAuthValueWithMigration(value string) (string, bool, error) {
	if value == "" {
		return "", false, nil
	}
	key, err := feishuProjectOAuthEncryptionKey()
	if err != nil {
		return "", false, err
	}
	data, err := base64.RawStdEncoding.DecodeString(value)
	if err != nil {
		return "", false, err
	}
	plain, err := decryptFeishuProjectOAuthBytes(data, key)
	if err == nil {
		return plain, false, nil
	}
	legacyKey, legacyKeyErr := legacyFeishuProjectOAuthEncryptionKey()
	if legacyKeyErr != nil {
		return "", false, err
	}
	legacyPlain, legacyErr := decryptFeishuProjectOAuthBytes(data, legacyKey)
	if legacyErr != nil {
		return "", false, err
	}
	return legacyPlain, true, nil
}

func decryptFeishuProjectOAuthBytes(data, key []byte) (string, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	if len(data) < gcm.NonceSize() {
		return "", errors.New("OAuth 密文长度不正确")
	}
	plain, err := gcm.Open(nil, data[:gcm.NonceSize()], data[gcm.NonceSize():], nil)
	if err != nil {
		return "", err
	}
	return string(plain), nil
}

func registerFeishuProjectOAuthClient(ctx context.Context, user *User) (feishuProjectOAuthClient, error) {
	feishuProjectOAuthClientMu.Lock()
	defer feishuProjectOAuthClientMu.Unlock()
	if err := ensureFeishuProjectOAuthSchema(ctx); err != nil {
		return feishuProjectOAuthClient{}, err
	}
	clientName := "TestCenter"
	if user != nil {
		identity := strings.TrimSpace(user.Username)
		if identity == "" {
			identity = strings.TrimSpace(user.ID)
		}
		if len(identity) > 48 {
			identity = identity[:48]
		}
		clientName = "TestCenter - " + identity
	}
	payload, _ := json.Marshal(map[string]any{
		"client_name":                clientName,
		"redirect_uris":              []string{"http://127.0.0.1"},
		"grant_types":                []string{feishuProjectDeviceGrant, "refresh_token"},
		"response_types":             []string{"code"},
		"token_endpoint_auth_method": "client_secret_post",
	})
	request, err := http.NewRequestWithContext(ctx, http.MethodPost,
		feishuProjectOAuthEndpoint("FEISHU_PROJECT_OAUTH_REGISTRATION_ENDPOINT", feishuProjectOAuthRegistrationEndpoint), bytes.NewReader(payload))
	if err != nil {
		return feishuProjectOAuthClient{}, err
	}
	request.Header.Set("Content-Type", "application/json")
	responseBody, status, err := doFeishuProjectOAuthRequest(request)
	if err != nil {
		return feishuProjectOAuthClient{}, err
	}
	if status < 200 || status >= 300 {
		return feishuProjectOAuthClient{}, fmt.Errorf("注册飞书项目 OAuth 客户端失败（HTTP %d）：%s", status, truncateDefectFeishuText(string(responseBody), 400))
	}
	var result struct {
		ClientID     string `json:"client_id"`
		ClientSecret string `json:"client_secret"`
	}
	if err := decodeFeishuProjectOAuthAPIResponse(responseBody, &result); err != nil {
		return feishuProjectOAuthClient{}, err
	}
	if strings.TrimSpace(result.ClientID) == "" {
		return feishuProjectOAuthClient{}, errors.New("飞书项目 OAuth 客户端注册未返回 client_id")
	}
	return feishuProjectOAuthClient{ClientID: result.ClientID, ClientSecret: result.ClientSecret}, nil
}

func startFeishuProjectDeviceAuthorization(ctx context.Context, user *User) (FeishuProjectOAuthStatus, error) {
	client, err := registerFeishuProjectOAuthClient(ctx, user)
	if err != nil {
		return FeishuProjectOAuthStatus{}, err
	}
	form := url.Values{
		"client_id": {client.ClientID},
		"resource":  {feishuProjectOAuthResource},
	}
	if client.ClientSecret != "" {
		form.Set("client_secret", client.ClientSecret)
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost,
		feishuProjectOAuthEndpoint("FEISHU_PROJECT_OAUTH_DEVICE_ENDPOINT", feishuProjectOAuthDeviceEndpoint), strings.NewReader(form.Encode()))
	if err != nil {
		return FeishuProjectOAuthStatus{}, err
	}
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	body, status, err := doFeishuProjectOAuthRequest(request)
	if err != nil {
		return FeishuProjectOAuthStatus{}, err
	}
	if status < 200 || status >= 300 {
		return FeishuProjectOAuthStatus{}, fmt.Errorf("发起飞书项目授权失败（HTTP %d）：%s", status, truncateDefectFeishuText(string(body), 400))
	}
	var device feishuProjectDeviceResponse
	if err := decodeFeishuProjectOAuthAPIResponse(body, &device); err != nil {
		return FeishuProjectOAuthStatus{}, err
	}
	if device.VerificationURI == "" {
		device.VerificationURI = device.VerificationURL
	}
	if device.DeviceCode == "" || device.UserCode == "" || device.VerificationURI == "" {
		return FeishuProjectOAuthStatus{}, errors.New("飞书项目设备授权响应不完整")
	}
	if device.ExpiresIn <= 0 {
		device.ExpiresIn = 600
	}
	if device.Interval < 2 {
		device.Interval = 5
	}
	encryptedCode, err := encryptFeishuProjectOAuthValue(device.DeviceCode)
	if err != nil {
		return FeishuProjectOAuthStatus{}, err
	}
	encryptedClientSecret, err := encryptFeishuProjectOAuthValue(client.ClientSecret)
	if err != nil {
		return FeishuProjectOAuthStatus{}, err
	}
	db, _, err := DatabaseManager.DB(ctx)
	if err != nil {
		return FeishuProjectOAuthStatus{}, err
	}
	now := time.Now()
	_, _ = db.ExecContext(ctx, `UPDATE feishu_project_oauth_sessions SET status='superseded', updated_at=?
		WHERE platform_user_id=? AND status='pending'`, now, user.ID)
	sessionID := uuid.NewString()
	_, err = db.ExecContext(ctx, `INSERT INTO feishu_project_oauth_sessions
		(id, platform_user_id, device_code_encrypted, oauth_client_id, oauth_client_secret_encrypted, user_code, verification_uri,
		 verification_uri_complete, status, last_error, interval_seconds, next_poll_at,
		 expires_at, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, 'pending', '', ?, ?, ?, ?, ?)`, sessionID, user.ID,
		encryptedCode, client.ClientID, encryptedClientSecret, device.UserCode, device.VerificationURI, device.VerificationURIComplete,
		device.Interval, now, now.Add(time.Duration(device.ExpiresIn)*time.Second), now, now)
	if err != nil {
		return FeishuProjectOAuthStatus{}, err
	}
	return FeishuProjectOAuthStatus{
		Status:             "pending",
		SessionID:          sessionID,
		UserCode:           device.UserCode,
		VerificationURI:    device.VerificationURI,
		VerificationURIAll: device.VerificationURIComplete,
		PollInterval:       device.Interval,
	}, nil
}

func loadFeishuProjectOAuthSession(ctx context.Context, sessionID, userID string) (*feishuProjectOAuthSession, error) {
	db, _, err := DatabaseManager.DB(ctx)
	if err != nil {
		return nil, err
	}
	var session feishuProjectOAuthSession
	err = db.QueryRowContext(ctx, `SELECT id, platform_user_id, device_code_encrypted, oauth_client_id,
		oauth_client_secret_encrypted, user_code,
		verification_uri, verification_uri_complete, status, last_error, interval_seconds,
		next_poll_at, expires_at FROM feishu_project_oauth_sessions WHERE id=? AND platform_user_id=?`, sessionID, userID).Scan(
		&session.ID, &session.UserID, &session.DeviceCodeEncrypted, &session.ClientID, &session.ClientSecretEncrypted, &session.UserCode,
		&session.VerificationURI, &session.VerificationURIComplete, &session.Status, &session.LastError,
		&session.IntervalSeconds, &session.NextPollAt, &session.ExpiresAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	return &session, err
}

func pollFeishuProjectDeviceAuthorization(ctx context.Context, user *User, sessionID string) (FeishuProjectOAuthStatus, error) {
	session, err := loadFeishuProjectOAuthSession(ctx, sessionID, user.ID)
	if err != nil {
		return FeishuProjectOAuthStatus{}, err
	}
	if session == nil {
		return FeishuProjectOAuthStatus{}, errors.New("授权会话不存在")
	}
	baseStatus := FeishuProjectOAuthStatus{
		Status:             session.Status,
		SessionID:          session.ID,
		UserCode:           session.UserCode,
		VerificationURI:    session.VerificationURI,
		VerificationURIAll: session.VerificationURIComplete,
		PollInterval:       session.IntervalSeconds,
		LastError:          session.LastError,
	}
	if session.Status != "pending" {
		if session.Status == "authorized" {
			return getFeishuProjectOAuthStatus(ctx, user.ID)
		}
		return baseStatus, nil
	}
	if time.Now().After(session.ExpiresAt) {
		return failFeishuProjectOAuthSession(ctx, session, "授权码已过期，请重新发起授权")
	}
	if time.Now().Before(session.NextPollAt) {
		return baseStatus, nil
	}

	deviceCode, err := decryptFeishuProjectOAuthValue(session.DeviceCodeEncrypted)
	if err != nil {
		return FeishuProjectOAuthStatus{}, err
	}
	clientSecret, err := decryptFeishuProjectOAuthValue(session.ClientSecretEncrypted)
	if err != nil {
		return FeishuProjectOAuthStatus{}, err
	}
	client := feishuProjectOAuthClient{ClientID: session.ClientID, ClientSecret: clientSecret}
	if strings.TrimSpace(client.ClientID) == "" {
		return failFeishuProjectOAuthSession(ctx, session, "授权会话缺少独立 OAuth Client，请重新发起授权")
	}
	form := url.Values{
		"grant_type":  {feishuProjectDeviceGrant},
		"device_code": {deviceCode},
		"client_id":   {client.ClientID},
		"resource":    {feishuProjectOAuthResource},
	}
	if client.ClientSecret != "" {
		form.Set("client_secret", client.ClientSecret)
	}
	token, statusCode, rawBody, err := requestFeishuProjectToken(ctx, form)
	if err != nil {
		return FeishuProjectOAuthStatus{}, err
	}
	db, _, err := DatabaseManager.DB(ctx)
	if err != nil {
		return FeishuProjectOAuthStatus{}, err
	}
	now := time.Now()
	if token.Error != "" || statusCode < 200 || statusCode >= 300 {
		switch token.Error {
		case "authorization_pending":
			_, _ = db.ExecContext(ctx, `UPDATE feishu_project_oauth_sessions SET next_poll_at=?, updated_at=? WHERE id=?`,
				now.Add(time.Duration(session.IntervalSeconds)*time.Second), now, session.ID)
			return baseStatus, nil
		case "slow_down":
			session.IntervalSeconds += 5
			_, _ = db.ExecContext(ctx, `UPDATE feishu_project_oauth_sessions SET interval_seconds=?, next_poll_at=?, updated_at=? WHERE id=?`,
				session.IntervalSeconds, now.Add(time.Duration(session.IntervalSeconds)*time.Second), now, session.ID)
			baseStatus.PollInterval = session.IntervalSeconds
			return baseStatus, nil
		case "access_denied":
			return failFeishuProjectOAuthSession(ctx, session, "用户拒绝了飞书项目授权")
		case "expired_token":
			return failFeishuProjectOAuthSession(ctx, session, "授权码已过期，请重新发起授权")
		default:
			message := strings.TrimSpace(token.Description)
			if message == "" {
				message = truncateDefectFeishuText(string(rawBody), 300)
			}
			return failFeishuProjectOAuthSession(ctx, session, "飞书项目授权失败："+message)
		}
	}
	if token.AccessToken == "" {
		return failFeishuProjectOAuthSession(ctx, session, "飞书项目授权未返回 access_token")
	}
	identity, err := verifyFeishuProjectOAuthIdentity(ctx, token.AccessToken)
	if err != nil {
		return failFeishuProjectOAuthSession(ctx, session, "授权身份验证失败："+err.Error())
	}
	if err := saveFeishuProjectOAuthCredential(ctx, user.ID, identity, token, client); err != nil {
		return FeishuProjectOAuthStatus{}, err
	}
	_, err = db.ExecContext(ctx, `UPDATE feishu_project_oauth_sessions SET status='authorized', last_error='', updated_at=? WHERE id=?`, now, session.ID)
	if err != nil {
		return FeishuProjectOAuthStatus{}, err
	}
	return getFeishuProjectOAuthStatus(ctx, user.ID)
}

func failFeishuProjectOAuthSession(ctx context.Context, session *feishuProjectOAuthSession, message string) (FeishuProjectOAuthStatus, error) {
	db, _, err := DatabaseManager.DB(ctx)
	if err != nil {
		return FeishuProjectOAuthStatus{}, err
	}
	now := time.Now()
	_, err = db.ExecContext(ctx, `UPDATE feishu_project_oauth_sessions SET status='failed', last_error=?, updated_at=? WHERE id=?`, message, now, session.ID)
	if err != nil {
		return FeishuProjectOAuthStatus{}, err
	}
	return FeishuProjectOAuthStatus{Status: "failed", SessionID: session.ID, LastError: message}, nil
}

func requestFeishuProjectToken(ctx context.Context, form url.Values) (feishuProjectTokenResponse, int, []byte, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodPost,
		feishuProjectOAuthEndpoint("FEISHU_PROJECT_OAUTH_TOKEN_ENDPOINT", feishuProjectOAuthTokenEndpoint), strings.NewReader(form.Encode()))
	if err != nil {
		return feishuProjectTokenResponse{}, 0, nil, err
	}
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	body, status, err := doFeishuProjectOAuthRequest(request)
	if err != nil {
		return feishuProjectTokenResponse{}, status, body, err
	}
	var token feishuProjectTokenResponse
	if err := json.Unmarshal(body, &token); err != nil {
		return token, status, body, err
	}
	return token, status, body, nil
}

func doFeishuProjectOAuthRequest(request *http.Request) ([]byte, int, error) {
	client := &http.Client{Timeout: 25 * time.Second}
	response, err := client.Do(request)
	if err != nil {
		return nil, 0, err
	}
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, 2<<20))
	return body, response.StatusCode, err
}

func decodeFeishuProjectOAuthAPIResponse(body []byte, target any) error {
	var envelope feishuProjectOAuthAPIEnvelope
	if err := json.Unmarshal(body, &envelope); err == nil && envelope.Data != nil {
		if envelope.Code != 0 {
			message := strings.TrimSpace(envelope.Msg)
			if message == "" && envelope.Error != nil {
				message = strings.TrimSpace(envelope.Error.LocalizedMessage.Message)
			}
			if message == "" {
				message = fmt.Sprintf("飞书项目 OAuth 接口返回错误码 %d", envelope.Code)
			}
			return errors.New(message)
		}
		if len(envelope.Data) == 0 || string(envelope.Data) == "null" {
			return errors.New("飞书项目 OAuth 接口未返回数据")
		}
		return json.Unmarshal(envelope.Data, target)
	}
	return json.Unmarshal(body, target)
}

func verifyFeishuProjectOAuthIdentity(ctx context.Context, accessToken string) (defectFeishuUserCandidate, error) {
	result, err := callDefectFeishuMCPWithBearer(ctx, accessToken, "search_user_info", map[string]any{
		"user_keys": []string{"current_login_user()"},
	})
	if err != nil {
		return defectFeishuUserCandidate{}, err
	}
	var decoded any
	if err := json.Unmarshal([]byte(result), &decoded); err != nil {
		return defectFeishuUserCandidate{}, err
	}
	users := collectDefectFeishuUsers(decoded)
	if len(users) != 1 || strings.TrimSpace(users[0].UserKey) == "" {
		return defectFeishuUserCandidate{}, errors.New("无法唯一确定飞书项目授权用户")
	}
	return users[0], nil
}

func saveFeishuProjectOAuthCredential(ctx context.Context, userID string, identity defectFeishuUserCandidate, token feishuProjectTokenResponse, client feishuProjectOAuthClient) error {
	if strings.TrimSpace(identity.UserKey) == "" {
		return errors.New("飞书项目授权身份缺少 user_key")
	}
	db, _, err := DatabaseManager.DB(ctx)
	if err != nil {
		return err
	}
	var boundPlatformUserID string
	err = db.QueryRowContext(ctx, `SELECT platform_user_id FROM feishu_project_oauth_credentials
		WHERE feishu_user_key=? AND platform_user_id<>? LIMIT 1`, identity.UserKey, userID).Scan(&boundPlatformUserID)
	if err == nil {
		boundName := boundPlatformUserID
		if boundUser, lookupErr := GetUserByID(boundPlatformUserID); lookupErr == nil && boundUser != nil {
			boundName = boundUser.Username
		}
		return fmt.Errorf("该飞书账号已由平台用户“%s”授权，不能重复绑定给其他用户", boundName)
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	accessEncrypted, err := encryptFeishuProjectOAuthValue(token.AccessToken)
	if err != nil {
		return err
	}
	refreshEncrypted, err := encryptFeishuProjectOAuthValue(token.RefreshToken)
	if err != nil {
		return err
	}
	clientSecretEncrypted, err := encryptFeishuProjectOAuthValue(client.ClientSecret)
	if err != nil {
		return err
	}
	if strings.TrimSpace(client.ClientID) == "" {
		return errors.New("独立 OAuth Client ID 为空")
	}
	if token.ExpiresIn <= 0 {
		token.ExpiresIn = 3600
	}
	if token.TokenType == "" {
		token.TokenType = "Bearer"
	}
	now := time.Now()
	name := strings.TrimSpace(identity.NameCN)
	if name == "" {
		name = strings.TrimSpace(identity.NameEN)
	}
	_, err = db.ExecContext(ctx, `INSERT INTO feishu_project_oauth_credentials
		(platform_user_id, feishu_user_key, feishu_name, feishu_email, oauth_client_id,
		 oauth_client_secret_encrypted, access_token_encrypted,
		 refresh_token_encrypted, token_type, scope_text, expires_at, status, last_error, authorized_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 'authorized', '', ?, ?)
		ON DUPLICATE KEY UPDATE feishu_user_key=VALUES(feishu_user_key), feishu_name=VALUES(feishu_name),
		feishu_email=VALUES(feishu_email), oauth_client_id=VALUES(oauth_client_id),
		oauth_client_secret_encrypted=VALUES(oauth_client_secret_encrypted), access_token_encrypted=VALUES(access_token_encrypted),
		refresh_token_encrypted=VALUES(refresh_token_encrypted), token_type=VALUES(token_type),
		scope_text=VALUES(scope_text), expires_at=VALUES(expires_at), status='authorized', last_error='',
		authorized_at=VALUES(authorized_at), updated_at=VALUES(updated_at)`, userID, identity.UserKey, name,
		identity.Email, client.ClientID, clientSecretEncrypted, accessEncrypted, refreshEncrypted, token.TokenType, token.Scope,
		now.Add(time.Duration(token.ExpiresIn)*time.Second), now, now)
	return err
}

func getFeishuProjectOAuthStatus(ctx context.Context, userID string) (FeishuProjectOAuthStatus, error) {
	if err := ensureFeishuProjectOAuthSchema(ctx); err != nil {
		return FeishuProjectOAuthStatus{}, err
	}
	db, _, err := DatabaseManager.DB(ctx)
	if err != nil {
		return FeishuProjectOAuthStatus{}, err
	}
	var status FeishuProjectOAuthStatus
	var expiresAt time.Time
	err = db.QueryRowContext(ctx, `SELECT status, feishu_user_key, feishu_name, feishu_email, expires_at, last_error
		FROM feishu_project_oauth_credentials WHERE platform_user_id=?`, userID).Scan(
		&status.Status, &status.FeishuUserKey, &status.FeishuName, &status.FeishuEmail, &expiresAt, &status.LastError,
	)
	if errors.Is(err, sql.ErrNoRows) {
		var session feishuProjectOAuthSession
		sessionErr := db.QueryRowContext(ctx, `SELECT id, platform_user_id, device_code_encrypted,
			oauth_client_id, oauth_client_secret_encrypted, user_code,
			verification_uri, verification_uri_complete, status, last_error, interval_seconds,
			next_poll_at, expires_at FROM feishu_project_oauth_sessions
			WHERE platform_user_id=? AND status IN ('pending','failed')
			ORDER BY created_at DESC LIMIT 1`, userID).Scan(
			&session.ID, &session.UserID, &session.DeviceCodeEncrypted, &session.ClientID, &session.ClientSecretEncrypted, &session.UserCode,
			&session.VerificationURI, &session.VerificationURIComplete, &session.Status, &session.LastError,
			&session.IntervalSeconds, &session.NextPollAt, &session.ExpiresAt,
		)
		if sessionErr == nil && session.Status == "pending" && time.Now().Before(session.ExpiresAt) {
			return FeishuProjectOAuthStatus{
				Status:             session.Status,
				SessionID:          session.ID,
				UserCode:           session.UserCode,
				VerificationURI:    session.VerificationURI,
				VerificationURIAll: session.VerificationURIComplete,
				PollInterval:       session.IntervalSeconds,
			}, nil
		}
		if sessionErr == nil && session.Status == "failed" {
			return FeishuProjectOAuthStatus{Status: "failed", SessionID: session.ID, LastError: session.LastError}, nil
		}
		if sessionErr != nil && !errors.Is(sessionErr, sql.ErrNoRows) {
			return FeishuProjectOAuthStatus{}, sessionErr
		}
		status.Status = "not_authorized"
		return status, nil
	}
	if err != nil {
		return FeishuProjectOAuthStatus{}, err
	}
	status.Authorized = status.Status == "authorized"
	status.AutoRefresh = status.Authorized
	status.ExpiresAt = expiresAt.Format(time.RFC3339)
	return status, nil
}

func deleteFeishuProjectOAuthCredential(ctx context.Context, userID string) error {
	db, _, err := DatabaseManager.DB(ctx)
	if err != nil {
		return err
	}
	_, err = db.ExecContext(ctx, `DELETE FROM feishu_project_oauth_credentials WHERE platform_user_id=?`, userID)
	if err == nil {
		_, _ = db.ExecContext(ctx, `DELETE FROM feishu_project_oauth_sessions WHERE platform_user_id=?`, userID)
	}
	return err
}

func getFeishuProjectOAuthAccessToken(ctx context.Context, userID string) (string, error) {
	if strings.TrimSpace(userID) == "" {
		return "", errFeishuProjectNoOAuth
	}
	if err := ensureFeishuProjectOAuthSchema(ctx); err != nil {
		return "", err
	}
	feishuProjectOAuthTokenMu.Lock()
	defer feishuProjectOAuthTokenMu.Unlock()
	db, _, err := DatabaseManager.DB(ctx)
	if err != nil {
		return "", err
	}
	var accessEncrypted, refreshEncrypted, status, clientID, clientSecretEncrypted string
	var expiresAt time.Time
	err = db.QueryRowContext(ctx, `SELECT access_token_encrypted, refresh_token_encrypted, status, expires_at,
		oauth_client_id, oauth_client_secret_encrypted
		FROM feishu_project_oauth_credentials WHERE platform_user_id=?`, userID).Scan(
		&accessEncrypted, &refreshEncrypted, &status, &expiresAt, &clientID, &clientSecretEncrypted,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return "", errFeishuProjectNoOAuth
	}
	if err != nil {
		return "", err
	}
	if status != "authorized" {
		return "", fmt.Errorf("飞书项目授权状态异常：%s", status)
	}
	accessToken, err := decryptFeishuProjectOAuthValue(accessEncrypted)
	if err != nil {
		return "", err
	}
	if time.Now().Add(30 * time.Minute).Before(expiresAt) {
		return accessToken, nil
	}
	refreshToken, err := decryptFeishuProjectOAuthValue(refreshEncrypted)
	if err != nil {
		return "", err
	}
	if refreshToken == "" {
		return "", markFeishuProjectOAuthCredentialExpired(ctx, userID, "refresh_token 为空，请重新授权")
	}
	clientSecret, err := decryptFeishuProjectOAuthValue(clientSecretEncrypted)
	if err != nil {
		return "", err
	}
	client := feishuProjectOAuthClient{ClientID: clientID, ClientSecret: clientSecret}
	if strings.TrimSpace(client.ClientID) == "" {
		return "", markFeishuProjectOAuthCredentialExpired(ctx, userID, "授权记录缺少独立 OAuth Client，请重新授权")
	}
	form := url.Values{
		"grant_type":    {"refresh_token"},
		"refresh_token": {refreshToken},
		"client_id":     {client.ClientID},
		"resource":      {feishuProjectOAuthResource},
	}
	if client.ClientSecret != "" {
		form.Set("client_secret", client.ClientSecret)
	}
	token, statusCode, body, err := requestFeishuProjectToken(ctx, form)
	if err != nil {
		return "", err
	}
	if statusCode < 200 || statusCode >= 300 || token.Error != "" || token.AccessToken == "" {
		message := token.Description
		if message == "" {
			message = truncateDefectFeishuText(string(body), 300)
		}
		switch token.Error {
		case "invalid_grant", "invalid_client", "expired_token", "access_denied":
			return "", markFeishuProjectOAuthCredentialExpired(ctx, userID, "授权刷新失败，需要重新授权："+message)
		default:
			return "", fmt.Errorf("授权刷新暂时失败：%s", message)
		}
	}
	if token.RefreshToken == "" {
		token.RefreshToken = refreshToken
	}
	var identity defectFeishuUserCandidate
	_ = db.QueryRowContext(ctx, `SELECT feishu_user_key, feishu_name, feishu_email FROM feishu_project_oauth_credentials WHERE platform_user_id=?`, userID).Scan(
		&identity.UserKey, &identity.NameCN, &identity.Email,
	)
	if err := saveFeishuProjectOAuthCredential(ctx, userID, identity, token, client); err != nil {
		return "", err
	}
	return token.AccessToken, nil
}

func markFeishuProjectOAuthCredentialExpired(ctx context.Context, userID, message string) error {
	db, _, err := DatabaseManager.DB(ctx)
	if err == nil {
		_, _ = db.ExecContext(ctx, `UPDATE feishu_project_oauth_credentials SET status='expired', last_error=?, updated_at=? WHERE platform_user_id=?`,
			message, time.Now(), userID)
	}
	return errors.New(message)
}

func runFeishuProjectOAuthCredentialKeeper() {
	refresh := func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()
		db, _, err := DatabaseManager.DB(ctx)
		if err != nil {
			log.Printf("[FeishuProjectOAuth] credential keeper database unavailable: %v", err)
			return
		}
		rows, err := db.QueryContext(ctx, `SELECT platform_user_id FROM feishu_project_oauth_credentials
			WHERE status='authorized' ORDER BY updated_at ASC`)
		if err != nil {
			log.Printf("[FeishuProjectOAuth] credential keeper query failed: %v", err)
			return
		}
		userIDs := []string{}
		for rows.Next() {
			var userID string
			if rows.Scan(&userID) == nil {
				userIDs = append(userIDs, userID)
			}
		}
		rows.Close()
		for _, userID := range userIDs {
			if _, err := getFeishuProjectOAuthAccessToken(ctx, userID); err != nil {
				log.Printf("[FeishuProjectOAuth] credential refresh for user %s failed: %v", userID, err)
			}
		}
	}

	refresh()
	ticker := time.NewTicker(15 * time.Minute)
	defer ticker.Stop()
	for range ticker.C {
		refresh()
	}
}

func withDefectFeishuOAuthForReporter(ctx context.Context, defect Defect) (context.Context, error) {
	token, err := getFeishuProjectOAuthAccessToken(ctx, defect.ReporterID)
	if err == nil {
		return context.WithValue(ctx, defectFeishuOAuthContextKey{}, token), nil
	}
	if errors.Is(err, errFeishuProjectNoOAuth) {
		user, userErr := GetUserByID(defect.ReporterID)
		if userErr == nil && strings.EqualFold(strings.TrimSpace(user.Username), "minghong") {
			return ctx, nil
		}
		return nil, fmt.Errorf("提交人“%s”尚未完成飞书项目独立授权", fallbackDefectFeishuText(defect.ReporterName))
	}
	return nil, fmt.Errorf("提交人“%s”的飞书项目授权不可用：%w", fallbackDefectFeishuText(defect.ReporterName), err)
}

func withDefectFeishuOAuthForUser(ctx context.Context, user *User) (context.Context, error) {
	if user == nil {
		return nil, errors.New("未登录")
	}
	token, err := getFeishuProjectOAuthAccessToken(ctx, user.ID)
	if err == nil {
		return context.WithValue(ctx, defectFeishuOAuthContextKey{}, token), nil
	}
	// Keep the historical service-token fallback for the original administrator.
	// Every other account must use its own OAuth identity.
	if errors.Is(err, errFeishuProjectNoOAuth) && strings.EqualFold(strings.TrimSpace(user.Username), "minghong") {
		return ctx, nil
	}
	if errors.Is(err, errFeishuProjectNoOAuth) {
		return nil, fmt.Errorf("当前账号尚未完成飞书项目独立授权")
	}
	return nil, fmt.Errorf("当前账号的飞书项目授权不可用：%w", err)
}

func defectFeishuOAuthTokenFromContext(ctx context.Context) string {
	value, _ := ctx.Value(defectFeishuOAuthContextKey{}).(string)
	return strings.TrimSpace(value)
}

func callDefectFeishuMCPWithBearer(ctx context.Context, accessToken, toolName string, arguments map[string]any) (string, error) {
	config, err := feishumodel.LoadConfig("data/feishu_config.json")
	if err != nil {
		return "", err
	}
	serverURL := feishuProjectOAuthResource
	if config.MCP != nil && strings.TrimSpace(config.MCP.ServerURL) != "" {
		serverURL = config.MCP.ServerURL
	}
	return callDefectFeishuMCPRequest(ctx, serverURL, "Bearer "+accessToken, "", toolName, arguments)
}

func StartFeishuProjectOAuthHandler(c *gin.Context) {
	user, err := CurrentUserFromRequest(c)
	if err != nil || user == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "未登录"})
		return
	}
	status, err := startFeishuProjectDeviceAuthorization(c.Request.Context(), user)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, status)
}

func PollFeishuProjectOAuthHandler(c *gin.Context) {
	user, err := CurrentUserFromRequest(c)
	if err != nil || user == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "未登录"})
		return
	}
	sessionID := strings.TrimSpace(c.Query("session_id"))
	if sessionID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "缺少 session_id"})
		return
	}
	status, err := pollFeishuProjectDeviceAuthorization(c.Request.Context(), user, sessionID)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, status)
}

func GetFeishuProjectOAuthStatusHandler(c *gin.Context) {
	user, err := CurrentUserFromRequest(c)
	if err != nil || user == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "未登录"})
		return
	}
	status, err := getFeishuProjectOAuthStatus(c.Request.Context(), user.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, status)
}

func DeleteFeishuProjectOAuthHandler(c *gin.Context) {
	user, err := CurrentUserFromRequest(c)
	if err != nil || user == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "未登录"})
		return
	}
	if err := deleteFeishuProjectOAuthCredential(c.Request.Context(), user.ID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "not_authorized"})
}

func parseFeishuProjectOAuthExpiresIn(value any) int {
	switch typed := value.(type) {
	case float64:
		return int(typed)
	case string:
		parsed, _ := strconv.Atoi(typed)
		return parsed
	default:
		return 0
	}
}
