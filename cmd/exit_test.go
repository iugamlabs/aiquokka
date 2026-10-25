package cmd

import (
	"errors"
	"fmt"

	"testing"

	"github.com/iugamlabs/aiquokka/internal/credential"
	"github.com/iugamlabs/aiquokka/internal/usage"
)

func TestExitCode(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want int
	}{
		{"success", nil, 0},
		{"not logged in", usage.NotConfigured("no Claude credentials"), ExitNotLoggedIn},
		{"not logged in, wrapped", fmt.Errorf("claude: %w", usage.NotConfigured("no login")), ExitNotLoggedIn},
		{"expired and not refreshed", credential.ErrRefreshRequired, ExitFailure},
		{"any other failure", errors.New("503 Service Unavailable"), ExitFailure},
	}
	for _, c := range cases {
		if got := ExitCode(c.err); got != c.want {
			t.Errorf("%s: ExitCode = %d, want %d", c.name, got, c.want)
		}
	}
	if ExitNotLoggedIn == ExitFailure || ExitNotLoggedIn == 0 || ExitNotLoggedIn == 2 {
		t.Fatalf("ExitNotLoggedIn = %d must differ from success, failure and a wrong command line", ExitNotLoggedIn)
	}
}
