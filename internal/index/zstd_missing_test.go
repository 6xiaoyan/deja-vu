package index

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// Without zstd a compressed rollout stays out of the manifest, pass after pass,
// so the first pass that has zstd reads it. Once it is held, an unchanged
// rollout does not run the decoder again (#5061).
func TestZstdInstalledReadsRolloutOnce(t *testing.T) {
	tmp := hermeticIndexEnv(t)
	bin := filepath.Join(tmp, "bin")
	if err := os.MkdirAll(bin, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin)

	dir := filepath.Join(os.Getenv("DEJA_CODEX_ROOT"), "sessions", "2026", "07", "31")
	plain := filepath.Join(dir, "rollout-2026-07-31T00-00-00-cx-1.jsonl")
	zst := filepath.Join(dir, "rollout-2026-07-20T00-00-00-zstd-installed-thread.jsonl.zst")
	write(t, plain, codexRolloutHead)
	write(t, zst, string([]byte{0x28, 0xB5, 0x2F, 0xFD, 0x00, 0x01, 0x02}))

	idx := filepath.Join(tmp, "idx")
	for pass := 1; pass <= 2; pass++ {
		if err := Ensure(idx, "", false, nil); err != nil {
			t.Fatal(err)
		}
		m, err := readManifest(idx)
		if err != nil {
			t.Fatal(err)
		}
		if _, held := m.Files[zst]; held {
			t.Fatalf("pass %d: compressed rollout held without zstd", pass)
		}
		if _, held := m.Files[plain]; !held {
			t.Fatalf("pass %d: plain rollout missing from manifest", pass)
		}
	}

	counter := filepath.Join(tmp, "zstd-calls.txt")
	rollout := filepath.Join(tmp, "decoded.jsonl")
	write(t, rollout, strings.ReplaceAll(codexRolloutHead, "cx-1", "zstd-installed-thread"))
	t.Setenv("DEJA_TEST_ZSTD_COUNTER", counter)
	t.Setenv("DEJA_TEST_ZSTD_ROLLOUT", rollout)
	if runtime.GOOS == "windows" {
		write(t, filepath.Join(bin, "zstd.bat"), "@echo off\r\n@echo x>>\"%DEJA_TEST_ZSTD_COUNTER%\"\r\n@type \"%DEJA_TEST_ZSTD_ROLLOUT%\"\r\n@exit /b 0\r\n")
	} else {
		write(t, filepath.Join(bin, "zstd"), "#!/bin/sh\nprintf 'x\\n' >>\"$DEJA_TEST_ZSTD_COUNTER\"\nwhile IFS= read -r line; do\n  printf '%s\\n' \"$line\"\ndone <\"$DEJA_TEST_ZSTD_ROLLOUT\"\n")
		if err := os.Chmod(filepath.Join(bin, "zstd"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	calls := func() int {
		b, err := os.ReadFile(counter)
		if os.IsNotExist(err) {
			return 0
		}
		if err != nil {
			t.Fatal(err)
		}
		return bytes.Count(b, []byte("\n"))
	}
	if err := Ensure(idx, "", false, nil); err != nil {
		t.Fatal(err)
	}
	m, err := readManifest(idx)
	if err != nil {
		t.Fatal(err)
	}
	if _, held := m.Files[zst]; !held {
		t.Fatal("first pass with zstd did not hold the compressed rollout")
	}
	if _, held := m.Sessions["codex:zstd-installed-thread"]; !held {
		t.Fatal("decoded rollout session missing from manifest")
	}
	before := calls()
	if before == 0 {
		t.Fatal("first pass with zstd did not run the decoder")
	}
	if err := Ensure(idx, "", false, nil); err != nil {
		t.Fatal(err)
	}
	if got := calls(); got != before {
		t.Errorf("unchanged pass ran zstd %d more times", got-before)
	}
}
