package muse

import (
	"context"
	"github.com/iugamlabs/aiquokka/internal/credential"
	"github.com/iugamlabs/aiquokka/internal/usage"
)

type Provider struct{}

func New() *Provider                                               { return &Provider{} }
func (*Provider) ID() string                                       { return "muse" }
func (*Provider) Name() string                                     { return "Muse" }
func (*Provider) Description() string                              { return "Muse 5-hour and weekly subscription limits" }
func (*Provider) Fetch(ctx context.Context) (*usage.Report, error) { return Fetch(ctx) }
func (*Provider) Capabilities() credential.Capabilities {
	return credential.Capabilities{OfficialCLIRefresh: true}
}
