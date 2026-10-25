package claude

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestConfigDirCredentialReadAndWrite(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("CLAUDE_CONFIG_DIR", dir)
	path := filepath.Join(dir, ".credentials.json")
	original := `{"claudeAiOauth":{"accessToken":"old","refreshToken":"refresh"},"future":"keep"}`
	if err := os.WriteFile(path, []byte(original), 0600); err != nil {
		t.Fatal(err)
	}
	o, err := loadCredentials(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	o.AccessToken = "new"
	if err := persist(context.Background(), o); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `"new"`) || !strings.Contains(string(data), `"keep"`) {
		t.Fatalf("unexpected credential write: %s", data)
	}
}
