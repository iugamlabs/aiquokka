package cmd

import (
	"context"
	"errors"
	"github.com/iugamlabs/aiquokka/internal/provider"
	"github.com/iugamlabs/aiquokka/internal/usage"
	"testing"
	"time"
)

type testProvider struct {
	name  string
	fetch func(context.Context) (*usage.Report, error)
}

func (p testProvider) ID() string                                       { return p.name }
func (p testProvider) Name() string                                     { return p.name }
func (p testProvider) Description() string                              { return p.name }
func (p testProvider) Fetch(ctx context.Context) (*usage.Report, error) { return p.fetch(ctx) }

func TestFetchWithinPreservesAnswersAndCancelsLateProvider(t *testing.T) {
	stopped := make(chan struct{})
	ps := []provider.Provider{
		testProvider{"Slow", func(ctx context.Context) (*usage.Report, error) { <-ctx.Done(); close(stopped); return nil, ctx.Err() }},
		testProvider{"Quick", func(context.Context) (*usage.Report, error) { return &usage.Report{Provider: "Quick"}, nil }},
		testProvider{"Unused", func(context.Context) (*usage.Report, error) { return nil, usage.NotConfigured("unused") }},
	}
	got := fetchWithin(context.Background(), ps, 30*time.Millisecond)
	if got[0].name != "Slow" || got[0].err == nil || got[1].report == nil || !usage.IsNotConfigured(got[2].err) {
		t.Fatalf("results=%+v", got)
	}
	select {
	case <-stopped:
	case <-time.After(time.Second):
		t.Fatal("late work not cancelled")
	}
}

func TestFetchWithinParentCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	ps := []provider.Provider{testProvider{"Never", func(ctx context.Context) (*usage.Report, error) { <-ctx.Done(); return nil, ctx.Err() }}}
	if got := fetchWithin(ctx, ps, time.Hour); !errors.Is(got[0].err, context.Canceled) {
		t.Fatalf("result=%+v", got)
	}
}

func TestFetchAllWaitsForAnswers(t *testing.T) {
	ps := []provider.Provider{testProvider{"Slow", func(context.Context) (*usage.Report, error) {
		time.Sleep(30 * time.Millisecond)
		return &usage.Report{Provider: "Slow"}, nil
	}}}
	if got := fetchAll(context.Background(), ps); got[0].report == nil || got[0].err != nil {
		t.Fatalf("result=%+v", got)
	}
}
