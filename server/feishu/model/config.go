package model

import (
	"encoding/json"
	"os"
)

// FeishuConfig holds app credentials and mode
type FeishuConfig struct {
	AppID             string `json:"app_id"`
	AppSecret          string `json:"app_secret"`
	VerificationToken string `json:"verification_token"`
	EncryptKey        string `json:"encrypt_key"`
	Mode              string `json:"mode"` // "long_conn" or "webhook"
}

// LoadConfig loads configuration from file or environment variables
func LoadConfig(filePath string) (*FeishuConfig, error) {
	config := &FeishuConfig{
		Mode: "long_conn", // Default mode
	}

	// 1. Try loading from file
	if _, err := os.Stat(filePath); err == nil {
		file, err := os.Open(filePath)
		if err == nil {
			defer file.Close()
			if err := json.NewDecoder(file).Decode(config); err != nil {
				return nil, err
			}
		}
	}

	// 2. Override with Env vars if present
	if id := os.Getenv("FEISHU_APP_ID"); id != "" {
		config.AppID = id
	}
	if secret := os.Getenv("FEISHU_APP_SECRET"); secret != "" {
		config.AppSecret = secret
	}
	if token := os.Getenv("FEISHU_VERIFY_TOKEN"); token != "" {
		config.VerificationToken = token
	}
	if key := os.Getenv("FEISHU_ENCRYPT_KEY"); key != "" {
		config.EncryptKey = key
	}

	return config, nil
}
