package model

import (
	"encoding/json"
	"log"
	"os"
)

type AIConfig struct {
	APIKey  string `json:"deepseek_api_key"`
	BaseURL string `json:"base_url"`
	Model   string `json:"model"`
}

var GlobalAIConfig *AIConfig

func LoadAIConfig() *AIConfig {
	path := "data/ai_config.json"
	file, err := os.Open(path)
	if err != nil {
		log.Printf("[Bot Brain] Warning: Failed to open %s. Using default empty config: %v", path, err)
		return &AIConfig{Model: "deepseek-chat", BaseURL: "https://api.deepseek.com/v1"}
	}
	defer file.Close()

	config := &AIConfig{}
	if err := json.NewDecoder(file).Decode(config); err != nil {
		log.Printf("[Bot Brain] Error decoding %s: %v", path, err)
	}

	GlobalAIConfig = config
	log.Printf("[Bot Brain] Loaded AI configuration (Model: %s)", config.Model)
	return config
}
