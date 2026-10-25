package cmd

import (
	"context"
	"encoding/json"
	"github.com/iugamlabs/aiquokka/internal/httpx"
	"github.com/iugamlabs/aiquokka/internal/provider"
	"github.com/iugamlabs/aiquokka/internal/usage"
	"io"
	"os"
	"strings"
	"testing"
	"time"
)

func captureBatchOutput(t *testing.T, fn func() error) string {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	original := os.Stdout
	os.Stdout = w
	result := make(chan string, 1)
	go func() { b, _ := io.ReadAll(r); _ = r.Close(); result <- string(b) }()
	err = fn()
	os.Stdout = original
	_ = w.Close()
	out := <-result
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func TestBatchAndJSONKeepAnswersWhenProviderTimesOut(t *testing.T) {
	original := httpx.Client.Timeout
	httpx.Client.Timeout = 30 * time.Millisecond
	t.Cleanup(func() { httpx.Client.Timeout = original; jsonOut = false })
	ps := []provider.Provider{
		testProvider{"Quick", func(context.Context) (*usage.Report, error) { return &usage.Report{Provider: "Quick"}, nil }},
		testProvider{"Silent", func(ctx context.Context) (*usage.Report, error) { <-ctx.Done(); return nil, ctx.Err() }},
		testProvider{"Unused", func(context.Context) (*usage.Report, error) { return nil, usage.NotConfigured("unused") }},
	}
	out := captureBatchOutput(t, func() error { return runAllBatch(context.Background(), ps) })
	if !strings.Contains(out, "Quick") || !strings.Contains(out, "Silent") || strings.Contains(out, "Unused") {
		t.Fatalf("batch=%s", out)
	}
	jsonOut = true
	out = captureBatchOutput(t, func() error { return runAllStructured(context.Background(), ps) })
	var got map[string]map[string]any
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got["quick"]["provider"] != "Quick" || got["silent"]["error"] == nil {
		t.Fatalf("JSON=%s", out)
	}
}
