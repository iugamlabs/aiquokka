package claude

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestMergeCredentialsPreservesEnvelopeAndOAuthFields(t *testing.T) {
	raw := []byte(`{"claudeAiOauth":{"accessToken":"old","refreshToken":"old-refresh","expiresAt":1,"futureOAuthField":"keep"},"futureTopLevelField":"keep"}`)
	merged, err := mergeCredentials(raw, false, &oauth{AccessToken: "new", RefreshToken: "new-refresh", ExpiresAt: 2})
	if err != nil {
		t.Fatal(err)
	}
	assertJSONField(t, merged, "futureTopLevelField", "keep")
	assertJSONField(t, mustObject(t, merged, "claudeAiOauth"), "futureOAuthField", "keep")
	assertJSONField(t, mustObject(t, merged, "claudeAiOauth"), "accessToken", "new")
}

func TestMergeCredentialsPreservesBareOAuthFields(t *testing.T) {
	raw := []byte(`{"accessToken":"old","refreshToken":"old-refresh","expiresAt":1,"futureOAuthField":"keep"}`)
	merged, err := mergeCredentials(raw, true, &oauth{AccessToken: "new", RefreshToken: "new-refresh", ExpiresAt: 2})
	if err != nil {
		t.Fatal(err)
	}
	assertJSONField(t, merged, "futureOAuthField", "keep")
	assertJSONField(t, merged, "accessToken", "new")
}

func TestLoadCredentialsPrefersFileOverKeychain(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("CLAUDE_CONFIG_DIR", "")
	if err := os.MkdirAll(filepath.Join(home, ".claude"), 0o700); err != nil {
		t.Fatal(err)
	}
	raw := []byte(`{"claudeAiOauth":{"accessToken":"file-token","refreshToken":"file-refresh"}}`)
	if err := os.WriteFile(filepath.Join(home, ".claude", ".credentials.json"), raw, 0o600); err != nil {
		t.Fatal(err)
	}

	original := readKeychainCredentials
	t.Cleanup(func() { readKeychainCredentials = original })
	readKeychainCredentials = func(context.Context) ([]byte, []byte, error) {
		return []byte(`{"accessToken":"keychain-token"}`), []byte("item"), nil
	}

	o, err := loadCredentials(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if o.AccessToken != "file-token" {
		t.Fatalf("AccessToken = %q, want file-token", o.AccessToken)
	}
	if o.source.isKeychain() {
		t.Fatal("expected file credentials, not a Keychain source")
	}
}

func TestLoadCredentialsPrefersLiveKeychainOverExpiredFile(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("CLAUDE_CONFIG_DIR", "")
	if err := os.MkdirAll(filepath.Join(home, ".claude"), 0o700); err != nil {
		t.Fatal(err)
	}
	raw := []byte(`{"claudeAiOauth":{"accessToken":"file-token","refreshToken":"file-refresh","expiresAt":1}}`)
	if err := os.WriteFile(filepath.Join(home, ".claude", ".credentials.json"), raw, 0o600); err != nil {
		t.Fatal(err)
	}

	original := readKeychainCredentials
	t.Cleanup(func() { readKeychainCredentials = original })
	future := time.Now().Add(time.Hour).UnixMilli()
	readKeychainCredentials = func(context.Context) ([]byte, []byte, error) {
		return []byte(fmt.Sprintf(`{"accessToken":"keychain-token","expiresAt":%d}`, future)), []byte("item"), nil
	}

	o, err := loadCredentials(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if o.AccessToken != "keychain-token" {
		t.Fatalf("AccessToken = %q, want keychain-token", o.AccessToken)
	}
	if !o.source.isKeychain() || string(o.source.keychainItem) != "item" {
		t.Fatalf("source = %+v, want keychain item", o.source)
	}
}

// When the file is expired and the Keychain cannot supply a live credential
// (expired, unparseable, or unavailable), the file keeps its place so its own
// refresh path still works — except on context cancellation.
func TestLoadCredentialsExpiredFileFallback(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("CLAUDE_CONFIG_DIR", "")
	if err := os.MkdirAll(filepath.Join(home, ".claude"), 0o700); err != nil {
		t.Fatal(err)
	}
	raw := []byte(`{"claudeAiOauth":{"accessToken":"file-token","refreshToken":"file-refresh","expiresAt":1}}`)
	if err := os.WriteFile(filepath.Join(home, ".claude", ".credentials.json"), raw, 0o600); err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		name    string
		data    []byte
		err     error
		wantErr error
	}{
		{name: "expired keychain", data: []byte(`{"accessToken":"keychain-token","refreshToken":"keychain-refresh","expiresAt":1}`)},
		{name: "unparseable keychain", data: []byte(`not-json`)},
		{name: "keychain locked or timed out", err: errKeychainTimeout},
		{name: "keychain canceled", err: context.Canceled, wantErr: context.Canceled},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			original := readKeychainCredentials
			t.Cleanup(func() { readKeychainCredentials = original })
			readKeychainCredentials = func(context.Context) ([]byte, []byte, error) {
				if tc.err != nil {
					return nil, nil, tc.err
				}
				return tc.data, []byte("item"), nil
			}

			o, err := loadCredentials(context.Background())
			if tc.wantErr != nil {
				if !errors.Is(err, tc.wantErr) {
					t.Fatalf("err = %v, want %v", err, tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if o.AccessToken != "file-token" {
				t.Fatalf("AccessToken = %q, want file-token", o.AccessToken)
			}
			if o.source.isKeychain() {
				t.Fatal("expected file credentials, not a Keychain source")
			}
		})
	}
}

func TestLoadCredentialsFallsBackToKeychainWhenFileHasNoToken(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("CLAUDE_CONFIG_DIR", "")
	if err := os.MkdirAll(filepath.Join(home, ".claude"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(home, ".claude", ".credentials.json"), []byte(`{}`), 0o600); err != nil {
		t.Fatal(err)
	}

	original := readKeychainCredentials
	t.Cleanup(func() { readKeychainCredentials = original })
	readKeychainCredentials = func(context.Context) ([]byte, []byte, error) {
		return []byte(`{"accessToken":"keychain-token"}`), []byte("item"), nil
	}

	o, err := loadCredentials(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if o.AccessToken != "keychain-token" {
		t.Fatalf("AccessToken = %q, want keychain-token", o.AccessToken)
	}
	if !o.source.isKeychain() || string(o.source.keychainItem) != "item" {
		t.Fatalf("source = %+v, want keychain item", o.source)
	}
}

func TestLoadCredentialsFallsBackToKeychain(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("CLAUDE_CONFIG_DIR", "")

	original := readKeychainCredentials
	t.Cleanup(func() { readKeychainCredentials = original })
	readKeychainCredentials = func(context.Context) ([]byte, []byte, error) {
		return []byte(`{"accessToken":"keychain-token"}`), []byte("item"), nil
	}

	o, err := loadCredentials(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if o.AccessToken != "keychain-token" {
		t.Fatalf("AccessToken = %q, want keychain-token", o.AccessToken)
	}
	if !o.source.isKeychain() || string(o.source.keychainItem) != "item" {
		t.Fatalf("source = %+v, want keychain item", o.source)
	}
}

func TestPersistRefusesKeychainCredentials(t *testing.T) {
	originalUpdate := updateKeychainItem
	t.Cleanup(func() { updateKeychainItem = originalUpdate })
	writes := 0
	updateKeychainItem = func(context.Context, []byte, []byte) error {
		writes++
		return nil
	}

	for _, source := range []credentialSource{
		{keychain: true},
		{keychainItem: []byte("item")},
	} {
		err := persist(context.Background(), &oauth{
			AccessToken:  "new",
			RefreshToken: "new-refresh",
			ExpiresAt:    2,
			source:       source,
		})
		if err == nil {
			t.Fatalf("source=%+v: persist succeeded, want read-only error", source)
		}
	}
	if writes != 0 {
		t.Fatalf("Keychain writes = %d, want 0", writes)
	}
}

func mustObject(t *testing.T, data []byte, field string) []byte {
	t.Helper()
	var object map[string]json.RawMessage
	if err := json.Unmarshal(data, &object); err != nil {
		t.Fatal(err)
	}
	return object[field]
}

func assertJSONField(t *testing.T, data []byte, field, want string) {
	t.Helper()
	var object map[string]json.RawMessage
	if err := json.Unmarshal(data, &object); err != nil {
		t.Fatal(err)
	}
	var got string
	if err := json.Unmarshal(object[field], &got); err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("%s = %q, want %q", field, got, want)
	}
}
