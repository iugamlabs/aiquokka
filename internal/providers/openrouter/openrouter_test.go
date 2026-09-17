package openrouter

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/star-plan/aiquokka/internal/usage"
)

func TestReportFromCredits(t *testing.T) {
	report := reportFromCredits(&credits{TotalCredits: 100.5, TotalUsage: 25.75})

	if report.Provider != "OpenRouter" {
		t.Fatalf("Provider = %q, want OpenRouter", report.Provider)
	}
	if len(report.Windows) != 0 {
		t.Fatalf("Windows = %d, want 0", len(report.Windows))
	}
	if got, want := report.Extra, []usage.Fact{
		{Label: "Balance", Value: "$74.75"},
		{Label: "Credits", Value: "$100.50"},
		{Label: "Spent", Value: "$25.75"},
	}; len(got) != len(want) || got[0] != want[0] || got[1] != want[1] || got[2] != want[2] {
		t.Fatalf("Extra = %#v, want %#v", got, want)
	}

	var out bytes.Buffer
	usage.Render(&out, report, time.Time{})
	if got := out.String(); !strings.Contains(got, "Balance:       $74.75") || strings.Contains(got, "[") {
		t.Fatalf("rendered balance = %q, want monetary facts without a bar", got)
	}
}

func TestFetch(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/credits" {
			t.Errorf("path = %q, want /api/v1/credits", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer management-key" {
			t.Errorf("Authorization = %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"data":{"total_credits":10,"total_usage":3.25}}`))
	}))
	defer server.Close()
	t.Setenv("OPENROUTER_BASE_URL", server.URL)
	t.Setenv("OPENROUTER_MANAGEMENT_KEY", "management-key")

	report, err := Fetch(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if got := report.Extra[0]; got != (usage.Fact{Label: "Balance", Value: "$6.75"}) {
		t.Fatalf("Balance = %#v", got)
	}
}

func TestFetchRejectsNonManagementKey(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "Management key required", http.StatusForbidden)
	}))
	defer server.Close()
	t.Setenv("OPENROUTER_BASE_URL", server.URL)
	t.Setenv("OPENROUTER_MANAGEMENT_KEY", "regular-key")

	_, err := Fetch(context.Background())
	if err == nil || !strings.Contains(err.Error(), "Management API key was rejected") {
		t.Fatalf("Fetch() error = %v, want Management API key rejection", err)
	}
}

func TestLoadKey(t *testing.T) {
	t.Setenv("OPENROUTER_MANAGEMENT_KEY", "")
	if _, err := loadKey(); !usage.IsNotConfigured(err) {
		t.Fatalf("loadKey() error = %v, want not configured", err)
	}
}
