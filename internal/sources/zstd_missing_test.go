package sources

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
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

// A pass asks for zstd once, not once per rollout, and a rebuild nested in a
// pass must not unpin the outer pass's answer when it ends (#5105).
func TestPinZstdHoldsForThePass(t *testing.T) {
	bin := t.TempDir()
	t.Setenv("PATH", bin)
	unpin := PinZstd()
	name := "zstd"
	if runtime.GOOS == "windows" {
		name = "zstd.bat"
	}
	if err := os.WriteFile(filepath.Join(bin, name), []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if ZstdAvailable() {
		t.Fatal("zstd installed mid-pass changed the pinned answer")
	}
	PinZstd()()
	if ZstdAvailable() {
		t.Fatal("a nested pin unpinned the outer pass")
	}
	unpin()
	if !ZstdAvailable() {
		t.Fatal("after the pass, ZstdAvailable did not see zstd")
	}
}

// The index learns which rollouts wait for zstd from this side channel.
func TestZstdMissingIsRecorded(t *testing.T) {
	DiagSnapshot()
	t.Cleanup(func() { DiagSnapshot() })
	t.Setenv("PATH", t.TempDir())
	path := filepath.Join(t.TempDir(), "rollout-missing.jsonl.zst")
	if _, err := ParseCodexRollout(path); err == nil {
		t.Fatal("rollout parsed without zstd")
	}
	if !DiagNoZstdPaths()[path] {
		t.Fatal("rollout left unread for want of zstd was not recorded")
	}
	DiagSnapshot()
	if len(DiagNoZstdPaths()) != 0 {
		t.Fatal("DiagSnapshot did not start the next pass's record over")
	}
}
