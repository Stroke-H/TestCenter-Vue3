package model

import (
	"encoding/json"
	"log"
	"os"
)

type AIConfig struct {
	APIKey         string                  `json:"deepseek_api_key"`
	BaseURL        string                  `json:"base_url"`
	Model          string                  `json:"model"`
	ProviderGroups []AIProviderGroupConfig `json:"provider_groups"`
	Providers      []AIProviderConfig      `json:"providers"`
}

type AIProviderGroupConfig struct {
	Name    string                  `json:"name"`
	APIKey  string                  `json:"api_key"`
	BaseURL string                  `json:"base_url"`
	Models  []AIProviderModelConfig `json:"models"`
}

type AIProviderModelConfig struct {
	Name       string `json:"name"`
	Model      string `json:"model"`
	Capability string `json:"capability,omitempty"`
}

type AIProviderConfig struct {
	Name       string `json:"name"`
	APIKey     string `json:"api_key"`
	BaseURL    string `json:"base_url"`
	Model      string `json:"model"`
	Capability string `json:"capability,omitempty"`
}

var GlobalAIConfig *AIConfig

func (c *AIConfig) AllEffectiveProviders() []AIProviderConfig {
	if c == nil {
		return nil
	}

	providers := make([]AIProviderConfig, 0, len(c.Providers)+1)
	for _, group := range c.ProviderGroups {
		if group.APIKey == "" || group.BaseURL == "" {
			continue
		}
		groupName := group.Name
		if groupName == "" {
			groupName = "group"
		}
		for _, modelConfig := range group.Models {
			if modelConfig.Model == "" {
				continue
			}
			name := modelConfig.Name
			if name == "" {
				name = groupName
			}
			providers = append(providers, AIProviderConfig{
				Name:       name,
				APIKey:     group.APIKey,
				BaseURL:    group.BaseURL,
				Model:      modelConfig.Model,
				Capability: normalizeAICapability(modelConfig.Capability),
			})
		}
	}

	for _, provider := range c.Providers {
		if provider.APIKey == "" || provider.BaseURL == "" || provider.Model == "" {
			continue
		}
		if provider.Name == "" {
			provider.Name = "custom"
		}
		provider.Capability = normalizeAICapability(provider.Capability)
		providers = append(providers, provider)
	}

	if len(providers) == 0 && c.APIKey != "" {
		providers = append(providers, AIProviderConfig{
			Name:       "deepseek",
			APIKey:     c.APIKey,
			BaseURL:    c.BaseURL,
			Model:      c.Model,
			Capability: "chat",
		})
	}

	return providers
}

func (c *AIConfig) EffectiveProviders() []AIProviderConfig {
	return c.EffectiveProvidersByCapability("chat")
}

func (c *AIConfig) EffectiveProvidersByCapability(capability string) []AIProviderConfig {
	target := normalizeAICapability(capability)
	providers := c.AllEffectiveProviders()
	filtered := make([]AIProviderConfig, 0, len(providers))
	for _, provider := range providers {
		if normalizeAICapability(provider.Capability) == target {
			filtered = append(filtered, provider)
		}
	}
	return filtered
}

func (c *AIConfig) HasConfiguredProvider() bool {
	return len(c.EffectiveProviders()) > 0
}

func normalizeAICapability(capability string) string {
	if capability == "" {
		return "chat"
	}
	return capability
}

func LoadAIConfig() *AIConfig {
	path := "data/ai_config.json"
	file, err := os.Open(path)
	if err != nil {
		log.Printf("[Bot Brain] Warning: Failed to open %s. Using default empty config: %v", path, err)
		GlobalAIConfig = &AIConfig{Model: "deepseek-chat", BaseURL: "https://api.deepseek.com/v1"}
		return GlobalAIConfig
	}
	defer file.Close()

	config := &AIConfig{}
	if err := json.NewDecoder(file).Decode(config); err != nil {
		log.Printf("[Bot Brain] Error decoding %s: %v", path, err)
	}

	GlobalAIConfig = config
	log.Printf("[Bot Brain] Loaded AI configuration (Providers: %d, Fallback Model: %s)", len(config.EffectiveProviders()), config.Model)
	return config
}
