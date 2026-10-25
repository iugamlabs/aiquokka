package cmd

import (
	"context"
	"errors"
	"github.com/iugamlabs/aiquokka/internal/credential"
	"github.com/iugamlabs/aiquokka/internal/usage"
	"testing"
)

func TestNoRefreshFlagPolicy(t *testing.T) {
	t.Cleanup(func() { noRefresh = false; credentialPolicy = credential.DefaultPolicy })
	for _, tc := range []struct {
		args    []string
		wantErr bool
		want    credential.Policy
	}{
		{[]string{"--no-refresh", "--list"}, false, credential.ReadOnly},
		{[]string{"--no-refresh", "--credential-policy", "readonly", "list"}, false, credential.ReadOnly},
		{[]string{"--no-refresh", "--credential-policy", "auto", "list"}, true, credential.Auto},
		{[]string{"--no-refresh", "--credential-policy", "persist", "list"}, true, credential.Auto},
		{[]string{"--no-refresh=false", "--list"}, false, credential.Auto},
	} {
		credentialPolicy = credential.DefaultPolicy
		root := newRootCmd()
		root.SetArgs(tc.args)
		err := root.Execute()
		if (err != nil) != tc.wantErr {
			t.Fatalf("%v: err=%v", tc.args, err)
		}
		if err == nil && credential.PolicyFrom(withCredentialPolicy(context.Background())) != tc.want {
			t.Fatalf("%v: wrong fetch policy", tc.args)
		}
	}
}

func TestNoRefreshInheritedByProviders(t *testing.T) {
	root := newRootCmd()
	for _, p := range registry.All() {
		sub, _, err := root.Find([]string{p.ID()})
		if err != nil || sub.InheritedFlags().Lookup("no-refresh") == nil {
			t.Fatalf("%s lacks --no-refresh", p.ID())
		}
	}
}

func TestRegistryCapabilitiesRejectPolicyBeforeFetch(t *testing.T) {
	called := false
	p := testProvider{"ReadOnly", func(context.Context) (*usage.Report, error) { called = true; return nil, nil }}
	ctx := credential.WithPolicy(context.Background(), credential.RefreshAndPersist)
	_, err := fetchProvider(ctx, p)
	if !errors.Is(err, credential.ErrPolicyNotSupported) || called {
		t.Fatalf("err=%v called=%v", err, called)
	}
}
