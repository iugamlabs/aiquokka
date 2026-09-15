package deepseek

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/star-plan/aiquokka/internal/usage"
)

func TestReportFromResponseSingleCurrency(t *testing.T) {
	resp := &balanceResponse{
		IsAvailable: true,
		BalanceInfos: []balanceInfo{{
			Currency:        "CNY",
			TotalBalance:    "110.00",
			GrantedBalance:  "10.00",
			ToppedUpBalance: "100.00",
		}},
	}

	report := reportFromResponse(resp)

	if report.Provider != "DeepSeek" {
		t.Fatalf("Provider = %q, want DeepSeek", report.Provider)
	}
	if len(report.Windows) != 0 {
		t.Fatalf("Windows = %d, want 0", len(report.Windows))
	}
	if len(report.Extra) != 3 {
		t.Fatalf("Extra = %d, want 3", len(report.Extra))
	}
	if report.Extra[0].Label != "Balance" || report.Extra[0].Value != "¥110.00" {
		t.Fatalf("Extra[0] = %+v, want Balance ¥110.00", report.Extra[0])
	}
	if report.Extra[1].Label != "Granted" || report.Extra[1].Value != "¥10.00" {
		t.Fatalf("Extra[1] = %+v, want Granted ¥10.00", report.Extra[1])
	}
	if report.Extra[2].Label != "Topped up" || report.Extra[2].Value != "¥100.00" {
		t.Fatalf("Extra[2] = %+v, want Topped up ¥100.00", report.Extra[2])
	}

	var out bytes.Buffer
	usage.Render(&out, report, time.Time{})
	got := out.String()
	for _, want := range []string{"Balance:       ¥110.00", "Granted:       ¥10.00", "Topped up:     ¥100.00"} {
		if !strings.Contains(got, want) {
			t.Errorf("rendered output missing %q:\n%s", want, got)
		}
	}
	if strings.Contains(got, "[") || strings.Contains(got, "█") {
		t.Errorf("rendered balance must not contain a progress bar:\n%s", got)
	}
}

func TestReportFromResponseUnavailable(t *testing.T) {
	report := reportFromResponse(&balanceResponse{IsAvailable: false})
	if len(report.Windows) != 0 {
		t.Fatalf("Windows = %d, want 0", len(report.Windows))
	}
	if len(report.Extra) != 1 || report.Extra[0].Label != "Status" {
		t.Fatalf("Extra = %+v, want Status fact", report.Extra)
	}
}

func TestReportFromResponseMultipleCurrencies(t *testing.T) {
	resp := &balanceResponse{
		IsAvailable: true,
		BalanceInfos: []balanceInfo{
			{Currency: "USD", TotalBalance: "5.00", GrantedBalance: "5.00"},
			{Currency: "CNY", TotalBalance: "20.00", ToppedUpBalance: "20.00"},
		},
	}

	report := reportFromResponse(resp)

	if len(report.Windows) != 0 {
		t.Fatalf("Windows = %d, want 0", len(report.Windows))
	}
	if report.Extra[0].Label != "Balance (USD)" || report.Extra[0].Value != "$5.00" {
		t.Fatalf("Extra[0] = %+v, want Balance (USD) $5.00", report.Extra[0])
	}
	if report.Extra[2].Label != "Balance (CNY)" || report.Extra[2].Value != "¥20.00" {
		t.Fatalf("Extra[2] = %+v, want Balance (CNY) ¥20.00", report.Extra[2])
	}
}

func TestReportFromResponseZeroBalance(t *testing.T) {
	report := reportFromResponse(&balanceResponse{
		IsAvailable: true,
		BalanceInfos: []balanceInfo{{
			Currency:        "CNY",
			TotalBalance:    "0",
			GrantedBalance:  "0.00",
			ToppedUpBalance: "0",
		}},
	})

	if len(report.Windows) != 0 {
		t.Fatalf("Windows = %d, want 0", len(report.Windows))
	}
	if len(report.Extra) != 3 {
		t.Fatalf("Extra = %+v, want three money facts", report.Extra)
	}
	for _, fact := range report.Extra {
		if fact.Value != "¥0.00" {
			t.Errorf("%s = %q, want ¥0.00", fact.Label, fact.Value)
		}
	}
}

func TestParseMoney(t *testing.T) {
	if v, err := parseMoney(" 12.34 "); err != nil || v != 12.34 {
		t.Fatalf("parseMoney = %v, %v", v, err)
	}
	if _, err := parseMoney(""); err == nil {
		t.Fatal("parseMoney(\"\") should error")
	}
}
