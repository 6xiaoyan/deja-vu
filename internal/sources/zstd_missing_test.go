package sources

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"
)

func TestZstdMissingBeforeRead(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	path := filepath.Join(t.TempDir(), "rollout-missing.jsonl.zst")
	_, err := ParseCodexRollout(path)
	if !errors.Is(err, errNoZstd) {
		t.Fatalf("ParseCodexRollout = %v, want errNoZstd before reading the missing file", err)
	}
	if !strings.Contains(err.Error(), "codex") || !strings.Contains(err.Error(), filepath.Base(path)) {
		t.Fatalf("missing-tool error must name the harness and file: %v", err)
	}
}
