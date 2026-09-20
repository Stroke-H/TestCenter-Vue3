package services

import (
	"context"
	"encoding/json"
	"os"
	"testing"
)

func TestFeishuProjectOAuthDeviceAuthorizationIntegration(t *testing.T) {
	if os.Getenv("TESTCENTER_RUN_FEISHU_OAUTH_INTEGRATION") != "1" {
		t.Skip("set TESTCENTER_RUN_FEISHU_OAUTH_INTEGRATION=1 to run")
	}
	username := os.Getenv("TESTCENTER_FEISHU_OAUTH_TEST_USER")
	if username == "" {
		username = "jinhuayang"
	}
	user, err := findUserByUsername(username)
	if err != nil {
		t.Fatal(err)
	}
	if user == nil {
		t.Skipf("%s platform user does not exist", username)
	}
	db, _, err := DatabaseManager.DB(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	_, _ = db.Exec(`DELETE FROM feishu_project_oauth_sessions WHERE platform_user_id=? AND status IN ('pending','superseded')`, user.ID)
	status, err := startFeishuProjectDeviceAuthorization(context.Background(), user)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = db.Exec(`DELETE FROM feishu_project_oauth_sessions WHERE id=?`, status.SessionID)
	})
	if status.Status != "pending" || status.SessionID == "" || status.UserCode == "" || status.VerificationURI == "" {
		t.Fatalf("unexpected device authorization response: %+v", status)
	}
	pollStatus, err := pollFeishuProjectDeviceAuthorization(context.Background(), user, status.SessionID)
	if err != nil {
		t.Fatal(err)
	}
	if pollStatus.Status != "pending" {
		t.Fatalf("expected authorization_pending, got: %+v", pollStatus)
	}
}

func TestFeishuProjectOAuthEncryptionRoundTrip(t *testing.T) {
	const envKey = "TESTCENTER_FEISHU_OAUTH_ENCRYPTION_KEY"
	previous := os.Getenv(envKey)
	t.Cleanup(func() { _ = os.Setenv(envKey, previous) })
	if err := os.Setenv(envKey, "unit-test-encryption-key"); err != nil {
		t.Fatal(err)
	}

	encrypted, err := encryptFeishuProjectOAuthValue("secret-access-token")
	if err != nil {
		t.Fatal(err)
	}
	if encrypted == "secret-access-token" || encrypted == "" {
		t.Fatalf("token was not encrypted: %q", encrypted)
	}
	decrypted, err := decryptFeishuProjectOAuthValue(encrypted)
	if err != nil {
		t.Fatal(err)
	}
	if decrypted != "secret-access-token" {
		t.Fatalf("unexpected decrypted value: %q", decrypted)
	}
}

func TestDecodeFeishuProjectOAuthWrappedResponse(t *testing.T) {
	body := []byte(`{"code":0,"msg":"","data":{"device_code":"device","user_code":"ABCD","verification_uri":"https://example.com","expires_in":600,"interval":5}}`)
	var response feishuProjectDeviceResponse
	if err := decodeFeishuProjectOAuthAPIResponse(body, &response); err != nil {
		t.Fatal(err)
	}
	if response.DeviceCode != "device" || response.UserCode != "ABCD" {
		t.Fatalf("unexpected response: %+v", response)
	}
}

func TestDecodeFeishuProjectOAuthStandardResponse(t *testing.T) {
	body, _ := json.Marshal(feishuProjectTokenResponse{AccessToken: "access", ExpiresIn: 3600})
	var response feishuProjectTokenResponse
	if err := decodeFeishuProjectOAuthAPIResponse(body, &response); err != nil {
		t.Fatal(err)
	}
	if response.AccessToken != "access" || response.ExpiresIn != 3600 {
		t.Fatalf("unexpected response: %+v", response)
	}
}

func TestDecodeFeishuProjectOAuthWrappedError(t *testing.T) {
	body := []byte(`{"code":1000053834,"msg":"Oauth 客户端信息失效，请重新生成","data":{}}`)
	var response feishuProjectDeviceResponse
	if err := decodeFeishuProjectOAuthAPIResponse(body, &response); err == nil {
		t.Fatal("expected wrapped API error")
	}
}

func TestDefectFeishuOAuthTokenContext(t *testing.T) {
	ctx := context.WithValue(context.Background(), defectFeishuOAuthContextKey{}, "oauth-token")
	if got := defectFeishuOAuthTokenFromContext(ctx); got != "oauth-token" {
		t.Fatalf("unexpected token: %q", got)
	}
}
