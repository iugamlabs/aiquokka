package openrouter

import (
	"os"
	"strings"

	"github.com/star-plan/aiquokka/internal/usage"
)

// loadKey reads the Management API key. Regular OpenRouter API keys cannot
// access the account-credit endpoint.
func loadKey() (string, error) {
	if v := strings.TrimSpace(os.Getenv("OPENROUTER_MANAGEMENT_KEY")); v != "" {
		return v, nil
	}
	return "", usage.NotConfigured("no OpenRouter Management API key found — set OPENROUTER_MANAGEMENT_KEY")
}
