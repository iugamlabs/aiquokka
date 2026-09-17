package openrouter

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/star-plan/aiquokka/internal/httpx"
	"github.com/star-plan/aiquokka/internal/usage"
)

// baseURL returns the OpenRouter API host (override via OPENROUTER_BASE_URL).
func baseURL() string {
	if v := os.Getenv("OPENROUTER_BASE_URL"); v != "" {
		return strings.TrimRight(v, "/")
	}
	return "https://openrouter.ai"
}

// creditsResponse mirrors GET /api/v1/credits.
type creditsResponse struct {
	Data *credits `json:"data"`
}

type credits struct {
	TotalCredits float64 `json:"total_credits"`
	TotalUsage   float64 `json:"total_usage"`
}

// Fetch reports the remaining OpenRouter account credits for the configured
// Management API key.
func Fetch(ctx context.Context) (*usage.Report, error) {
	key, err := loadKey()
	if err != nil {
		return nil, err
	}

	credits, err := getCredits(ctx, key)
	if err != nil {
		return nil, err
	}
	return reportFromCredits(credits), nil
}

// getCredits performs the authenticated account-credit request.
func getCredits(ctx context.Context, key string) (*credits, error) {
	url := baseURL() + "/api/v1/credits"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+key)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "aiquokka")

	resp, err := httpx.Client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		return nil, fmt.Errorf("OpenRouter Management API key was rejected (%s) — check OPENROUTER_MANAGEMENT_KEY", resp.Status)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("%s: %s: %s", url, resp.Status, strings.TrimSpace(string(body)))
	}

	var out creditsResponse
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, fmt.Errorf("decoding OpenRouter credits: %w", err)
	}
	if out.Data == nil {
		return nil, fmt.Errorf("decoding OpenRouter credits: missing data")
	}
	return out.Data, nil
}

// reportFromCredits converts OpenRouter's purchased-credit and usage totals
// into monetary facts. The API does not return the remainder directly.
func reportFromCredits(credits *credits) *usage.Report {
	report := &usage.Report{Provider: "OpenRouter"}
	if credits == nil {
		report.Extra = append(report.Extra, usage.Fact{Label: "Balance", Value: "unknown"})
		return report
	}

	balance := credits.TotalCredits - credits.TotalUsage
	report.Extra = append(report.Extra,
		usage.Fact{Label: "Balance", Value: usage.FormatMoney(balance, "USD")},
		usage.Fact{Label: "Credits", Value: usage.FormatMoney(credits.TotalCredits, "USD")},
		usage.Fact{Label: "Spent", Value: usage.FormatMoney(credits.TotalUsage, "USD")},
	)
	return report
}
