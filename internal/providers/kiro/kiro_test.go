package kiro

import (
	"context"
	"math"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/iugamlabs/aiquokka/internal/credential"
	"github.com/iugamlabs/aiquokka/internal/usage"
)

func TestParseUsage(t *testing.T) {
	now := time.Date(2026, time.August, 12, 13, 25, 0, 0, time.FixedZone("CEST", 2*60*60))
	raw := "\x1b[38;5;244mModel: \x1b[0mauto | Plan: KIRO PRO (/usage for more detail)\n" +
		"\x1b[1mEstimated Usage\x1b[0m | resets on 09/01 | \x1b[38;5;141mKIRO PRO\x1b[0m\n" +
		"\x1b[1mCredits\x1b[0m (2.94 of 1000 covered in plan)\n" +
		"████ 0%\n\n" +
		"Overages: \x1b[1mDisabled\x1b[0m \x1b[38;5;244m(managed by your organization)\x1b[0m\n"

	report, err := parseUsage(raw, now)
	if err != nil {
		t.Fatal(err)
	}
	if report.Provider != "Kiro" || report.Plan != "KIRO PRO" {
		t.Fatalf("unexpected report identity: %#v", report)
	}
	if len(report.Windows) != 1 {
		t.Fatalf("got %d windows, want 1", len(report.Windows))
	}
	window := report.Windows[0]
	if window.UsedPercent == nil || math.Abs(*window.UsedPercent-0.294) > 0.000001 {
		t.Fatalf("used percent = %v, want 0.294", window.UsedPercent)
	}
	wantReset := time.Date(2026, time.September, 1, 0, 0, 0, 0, now.Location())
	if !window.ResetsAt.Equal(wantReset) {
		t.Fatalf("reset = %s, want %s", window.ResetsAt, wantReset)
	}
	if len(report.Extra) != 2 || report.Extra[0].Value != "2.94 / 1000 covered in plan" ||
		report.Extra[1].Value != "Disabled (managed by your organization)" {
		t.Fatalf("unexpected facts: %#v", report.Extra)
	}
}

func TestParseUsageUsesNextYearForPastReset(t *testing.T) {
	now := time.Date(2026, time.December, 15, 12, 0, 0, 0, time.UTC)
	report, err := parseUsage("Estimated Usage | resets on 01/01 | KIRO FREE\nCredits (10 of 50 included)\n", now)
	if err != nil {
		t.Fatal(err)
	}
	want := time.Date(2027, time.January, 1, 0, 0, 0, 0, time.UTC)
	if !report.Windows[0].ResetsAt.Equal(want) {
		t.Fatalf("reset = %s, want %s", report.Windows[0].ResetsAt, want)
	}
}

func TestParseUsageRejectsChangedOutput(t *testing.T) {
	if _, err := parseUsage("Estimated Usage | KIRO PRO", time.Now()); err == nil {
		t.Fatal("expected missing credits error")
	}
}

func TestKiroKeepsOfficialCLICredentialOwnership(t *testing.T) {
	caps := New().Capabilities()
	if !caps.OfficialCLIRefresh {
		t.Fatal("Kiro should refresh only via its official CLI")
	}
	if caps.RefreshInMemory || caps.RefreshAndPersist {
		t.Fatal("Kiro must not refresh or persist credentials itself")
	}
	if err := caps.Allows(credential.ReadOnly); err != nil {
		t.Fatalf("ReadOnly should be allowed: %v", err)
	}
	if err := caps.Allows(credential.RefreshAndPersist); err == nil {
		t.Fatal("persist must be rejected — Kiro CLI owns the credential store")
	}
}

func TestParseUsageAcceptsISOResetDate(t *testing.T) {
	now := time.Date(2026, time.September, 17, 12, 0, 0, 0, time.UTC)
	report, err := parseUsage("Estimated Usage | resets on 2026-10-01 | KIRO FREE\nCredits (0.04 of 50 covered in plan)\n", now)
	if err != nil {
		t.Fatal(err)
	}
	want := time.Date(2026, time.October, 1, 0, 0, 0, 0, time.UTC)
	if !report.Windows[0].ResetsAt.Equal(want) {
		t.Fatalf("reset = %s, want %s", report.Windows[0].ResetsAt, want)
	}
}

func TestChatBinaryFallback(t *testing.T) {
	dir := t.TempDir()
	cli := filepath.Join(dir, "kiro-cli")
	chat := filepath.Join(dir, "kiro-cli-chat")
	for _, p := range []string{cli, chat} {
		if err := os.WriteFile(p, []byte("#!/bin/sh\n"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	link := filepath.Join(t.TempDir(), "kiro-cli")
	if err := os.Symlink(cli, link); err != nil {
		t.Fatal(err)
	}
	got, ok := chatBinary(link)
	want, _ := filepath.EvalSymlinks(chat)
	if !ok || got != want {
		t.Fatalf("chatBinary = %q, %v; want %q", got, ok, want)
	}
	if _, ok := chatBinary(filepath.Join(t.TempDir(), "kiro-cli")); ok {
		t.Fatal("expected no chat binary next to a missing kiro-cli")
	}
}

func TestIsLoginMessage(t *testing.T) {
	cases := []struct {
		text string
		want bool
	}{
		{"Not logged in", true},
		{"You are not logged in. Login now?", true},
		{"Welcome to Kiro CLI, let's get you signed in!\nPress enter to continue to the browser or esc to cancel", true},
		{"Login is required to use chat", true},
		{"Please log in with kiro-cli login", true},
		{"Logged in with Builder ID", false},
		{"network timeout", false},
	}
	for _, tc := range cases {
		if got := isLoginMessage(tc.text); got != tc.want {
			t.Errorf("isLoginMessage(%q) = %v, want %v", tc.text, got, tc.want)
		}
	}
}

func writeFakeKiro(t *testing.T, dir, name, script string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte("#!/bin/sh\n"+script), 0o755); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestFetchSkipsChatWhenLoggedOut(t *testing.T) {
	dir := t.TempDir()
	marker := filepath.Join(dir, "chat-called")
	writeFakeKiro(t, dir, "kiro-cli", `
if [ "$1" = "whoami" ]; then
  echo "Not logged in"
  exit 1
fi
touch "`+marker+`"
echo "Welcome to Kiro CLI, let's get you signed in!"
exit 1
`)
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("KIRO_API_KEY", "")

	_, err := Fetch(context.Background())
	if !usage.IsNotConfigured(err) {
		t.Fatalf("err = %v, want NotConfigured", err)
	}
	if _, statErr := os.Stat(marker); statErr == nil {
		t.Fatal("kiro-cli chat was invoked even though whoami reported not logged in")
	}
}

func TestFetchRunsUsageWhenLoggedIn(t *testing.T) {
	dir := t.TempDir()
	writeFakeKiro(t, dir, "kiro-cli", `
if [ "$1" = "whoami" ]; then
  echo "Logged in with Builder ID"
  exit 0
fi
if [ "$Q_FAKE_IS_REMOTE" != "1" ] || [ "$BROWSER" != "false" ]; then
  echo "missing browser-disable env"
  exit 1
fi
echo "Estimated Usage | resets on 09/01 | KIRO PRO"
echo "Credits (2.5 of 100 covered in plan)"
`)
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("KIRO_API_KEY", "")

	report, err := Fetch(context.Background())
	if err != nil {
		t.Fatalf("Fetch failed: %v", err)
	}
	if report.Plan != "KIRO PRO" || len(report.Windows) != 1 {
		t.Fatalf("unexpected report: %#v", report)
	}
}

func TestFetchSkipsWhoamiWhenAPIKeySet(t *testing.T) {
	dir := t.TempDir()
	writeFakeKiro(t, dir, "kiro-cli", `
if [ "$1" = "whoami" ]; then
  echo "Not logged in"
  exit 1
fi
echo "Estimated Usage | resets on 09/01 | KIRO PRO"
echo "Credits (1 of 50 included)"
`)
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("KIRO_API_KEY", "ksk_test")

	report, err := Fetch(context.Background())
	if err != nil {
		t.Fatalf("Fetch failed with KIRO_API_KEY: %v", err)
	}
	if report.Plan != "KIRO PRO" {
		t.Fatalf("unexpected report: %#v", report)
	}
}

func TestFetchTreatsBrowserLoginFallbackAsNotConfigured(t *testing.T) {
	dir := t.TempDir()
	writeFakeKiro(t, dir, "kiro-cli", `
if [ "$1" = "whoami" ]; then
  echo "Logged in with Builder ID"
  exit 0
fi
echo "Welcome to Kiro CLI, let's get you signed in!"
echo "Press enter to continue to the browser or esc to cancel"
echo "error: No such device or address (os error 6)"
exit 1
`)
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("KIRO_API_KEY", "")

	_, err := Fetch(context.Background())
	if !usage.IsNotConfigured(err) {
		t.Fatalf("err = %v, want NotConfigured", err)
	}
}
