//go:build unix

package kiro

import (
	"context"
	"os/exec"
	"strings"
	"testing"
	"time"
)

func TestRunCommandKillsChildrenOnTimeout(t *testing.T) {
	const marker = "37.4242"
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()

	start := time.Now()
	_, err := runCommand(ctx, "sh", "-c", "sleep "+marker+" & sleep "+marker)
	if err == nil {
		t.Fatal("expected the timed-out command to fail")
	}
	if elapsed := time.Since(start); elapsed > commandWaitDelay+time.Second {
		t.Fatalf("runCommand waited %s for a child that holds the output pipe", elapsed)
	}

	time.Sleep(200 * time.Millisecond)
	out, _ := exec.Command("pgrep", "-f", "sleep "+marker).Output()
	if left := strings.TrimSpace(string(out)); left != "" {
		_ = exec.Command("pkill", "-f", "sleep "+marker).Run()
		t.Fatalf("child processes left running after timeout: %s", left)
	}
}

func TestRunCommandDetachesControllingTTYAndBlocksBrowser(t *testing.T) {
	out, err := runCommand(context.Background(), "sh", "-c",
		`printf '%s|%s|' "$Q_FAKE_IS_REMOTE" "$BROWSER"; if (: < /dev/tty) 2>/dev/null; then printf 'tty'; else printf 'no-tty'; fi`)
	if err != nil {
		t.Fatalf("runCommand failed: %v (%s)", err, out)
	}
	if got := string(out); got != "1|false|no-tty" {
		t.Fatalf("got %q, want %q", got, "1|false|no-tty")
	}
}
