package muse

import (
	"context"
	"github.com/iugamlabs/aiquokka/internal/httpx/httpxtest"
	"net/http"
	"sync/atomic"
	"testing"
)

func TestMintCacheDoesNotCrossLogins(t *testing.T) {
	resetMintCache()
	t.Cleanup(resetMintCache)
	var calls atomic.Int32
	httpxtest.Stub(t, func(r *http.Request) (int, string) {
		calls.Add(1)
		switch r.Header.Get("Authorization") {
		case "Bearer first":
			return http.StatusOK, `{"subs_tier_name":"first"}`
		case "Bearer second":
			return http.StatusOK, `{"subs_tier_name":"second"}`
		default:
			t.Errorf("unexpected login")
			return http.StatusUnauthorized, `{}`
		}
	})
	for _, token := range []string{"first", "first", "second", "second"} {
		got, err := mint(context.Background(), token)
		if err != nil {
			t.Fatal(err)
		}
		if got.SubsTierName != token {
			t.Fatalf("cache crossed accounts: %s", got.SubsTierName)
		}
	}
	if calls.Load() != 2 {
		t.Fatalf("network calls=%d, want 2", calls.Load())
	}
}
