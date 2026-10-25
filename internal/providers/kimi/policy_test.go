package kimi

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/iugamlabs/aiquokka/internal/credential"
	"github.com/iugamlabs/aiquokka/internal/httpx/httpxtest"
)

// storeLogin writes an OAuth credential file into a temporary share dir.
func storeLogin(t *testing.T, body string) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("KIMI_SHARE_DIR", dir)
	t.Setenv("KIMI_API_KEY", "")
	t.Setenv("KIMI_CODING_API_KEY", "")
	path := filepath.Join(dir, "credentials", "kimi-code.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func assertUntouched(t *testing.T, path, stored string) {
	t.Helper()
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(after, []byte(stored)) {
		t.Fatalf("stored login was rewritten:\n%s", after)
	}
}

func TestFetchWithoutRefreshLeavesExpiredTokenAlone(t *testing.T) {
	stored := `{"access_token":"old","refresh_token":"old-refresh","expires_at":1}`
	path := storeLogin(t, stored)
	httpxtest.Refuse(t)

	_, err := Fetch(credential.WithPolicy(context.Background(), credential.ReadOnly))
	if !errors.Is(err, credential.ErrRefreshRequired) {
		t.Fatalf("err = %v, want ErrNoRefresh", err)
	}
	assertUntouched(t, path, stored)
}

// A token the file calls valid can still be rejected; the 401 retry must not
// refresh either.
func TestFetchWithoutRefreshDoesNotRefreshOnRejection(t *testing.T) {
	stored := `{"access_token":"old","refresh_token":"old-refresh","expires_at":0}`
	path := storeLogin(t, stored)
	httpxtest.Stub(t, func(r *http.Request) (int, string) {
		if r.Method != http.MethodGet {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL)
		}
		return http.StatusUnauthorized, `{}`
	})

	_, err := Fetch(credential.WithPolicy(context.Background(), credential.ReadOnly))
	if !errors.Is(err, credential.ErrRefreshRequired) {
		t.Fatalf("err = %v, want ErrNoRefresh", err)
	}
	assertUntouched(t, path, stored)
}

func TestPersistPreservesFieldsAndPermissions(t *testing.T) {
	path := storeLogin(t, `{"access_token":"old","refresh_token":"old-refresh","future":"keep"}`)
	c := &credentials{file: path, bearer: "new", refreshToken: "new-refresh", expiresAt: 123}
	if err := c.persist(oauthFile{}); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(data, []byte(`"future": "keep"`)) || !bytes.Contains(data, []byte(`"access_token": "new"`)) {
		t.Fatalf("bad persisted data: %s", data)
	}
	st, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if st.Mode().Perm() != 0600 {
		t.Fatalf("mode=%v", st.Mode())
	}
}

func TestKimiRejectsMemoryPolicy(t *testing.T) {
	httpxtest.Refuse(t)
	_, err := Fetch(credential.WithPolicy(context.Background(), credential.RefreshInMemory))
	if !errors.Is(err, credential.ErrPolicyNotSupported) {
		t.Fatalf("err=%v", err)
	}
}

func TestRefreshPropagatesPersistenceFailure(t *testing.T) {
	path := storeLogin(t, `{"access_token":"old","refresh_token":"refresh","expires_at":1}`)
	httpxtest.Stub(t, func(r *http.Request) (int, string) {
		if r.Method != http.MethodPost {
			t.Errorf("unexpected request: %s", r.Method)
		}
		if err := os.Remove(path); err != nil {
			t.Fatal(err)
		}
		return http.StatusOK, `{"access_token":"new","refresh_token":"rotated","expires_in":3600}`
	})
	_, err := Fetch(context.Background())
	if err == nil {
		t.Fatal("persistence failure must fail refresh")
	}
}
