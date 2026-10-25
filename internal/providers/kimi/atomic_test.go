package kimi

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestAtomicPersistenceFailureLeavesExistingLoginIntact(t *testing.T) {
	if runtime.GOOS == "windows" || os.Geteuid() == 0 {
		t.Skip("requires non-root Unix directory permissions")
	}
	stored := `{"access_token":"old","refresh_token":"old-refresh","future":"keep"}`
	path := storeLogin(t, stored)
	dir := filepath.Dir(path)
	if err := os.Chmod(dir, 0500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0700) })
	c := &credentials{file: path, bearer: "new", refreshToken: "rotated"}
	if err := c.persist(oauthFile{}); err == nil {
		t.Fatal("atomic write unexpectedly succeeded in unwritable directory")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(data, []byte(stored)) {
		t.Fatal("failed atomic write damaged original login")
	}
}
