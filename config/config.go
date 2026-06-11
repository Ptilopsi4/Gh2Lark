package config

import (
	"fmt"
	"os"
	"strconv"
)

// Config holds all configuration for the Gh2Lark relay service.
type Config struct {
	// LarkWebhookURL is the full URL of the Lark custom bot webhook.
	// Must start with https://open.feishu.cn/open-apis/bot/v2/hook/
	LarkWebhookURL string

	// GitHubWebhookSecret is the optional secret token configured on the GitHub
	// webhook for HMAC-SHA256 payload signature validation.
	// When empty, signature validation is skipped.
	GitHubWebhookSecret string

	// Port is the HTTP listen port. Defaults to "8080".
	Port string

	// MaxPayloadSize is the maximum request body size in bytes.
	// Defaults to 25 MB (matching GitHub's webhook payload cap).
	MaxPayloadSize int64
}

// Load reads configuration from environment variables and returns a Config.
// Returns an error if LarkWebhookURL is empty.
func Load() (*Config, error) {
	cfg := &Config{
		LarkWebhookURL:       os.Getenv("LARK_WEBHOOK_URL"),
		GitHubWebhookSecret:  os.Getenv("GITHUB_WEBHOOK_SECRET"),
		Port:                 envOrDefault("PORT", "8080"),
		MaxPayloadSize:       envOrDefaultInt64("MAX_PAYLOAD_SIZE", 25*1024*1024),
	}

	if cfg.LarkWebhookURL == "" {
		return nil, fmt.Errorf("LARK_WEBHOOK_URL environment variable is required")
	}

	return cfg, nil
}

func envOrDefault(key, defaultVal string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultVal
}

func envOrDefaultInt64(key string, defaultVal int64) int64 {
	raw := os.Getenv(key)
	if raw == "" {
		return defaultVal
	}
	v, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return defaultVal
	}
	return v
}
