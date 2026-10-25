//go:build !tray || (!linux && !darwin) || (darwin && !cgo)

package cmd

import (
	"strings"
	"testing"
)

func TestDefaultTrayExplainsOptionalBuild(t *testing.T) {
	root := newRootCmd()
	root.SetArgs([]string{"bar"})
	if err := root.Execute(); err == nil || !strings.Contains(err.Error(), "go build -tags tray") {
		t.Fatalf("err=%v", err)
	}
}
