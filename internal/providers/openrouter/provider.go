// Package openrouter reports the OpenRouter credit balance.
package openrouter

import (
	"context"

	"github.com/star-plan/aiquokka/internal/usage"
)

// Provider adapts the OpenRouter fetcher to the shared provider.Provider contract.
type Provider struct{}

// New returns the OpenRouter provider.
func New() *Provider { return &Provider{} }

func (*Provider) ID() string          { return "openrouter" }
func (*Provider) Name() string        { return "OpenRouter" }
func (*Provider) Description() string { return "OpenRouter account credit balance" }
func (*Provider) Fetch(ctx context.Context) (*usage.Report, error) {
	return Fetch(ctx)
}
