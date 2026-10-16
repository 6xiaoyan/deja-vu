package main

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// The help page says everything after `deja aider` is aider's own, --help
// included, and the same of goose — and both answered with deja's page, so the
// tool's own help was unreachable through the wrapper (#5060). The help goes to
// the tool, with the line as typed and none of the recall a real run adds.
func TestToolHelpGoesToTheTool(t *testing.T) {
	for _, tool := range []string{"aider", "goose"} {
		for _, flag := range []string{"--help", "-h"} {
			t.Run(tool+" "+flag, func(t *testing.T) {
				home := t.TempDir()
				t.Setenv("HOME", home)
				t.Setenv("USERPROFILE", home)
				t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
				t.Setenv("DEJA_INDEX_DIR", filepath.Join(home, "index"))
				t.Setenv("DEJA_WARMUP_SENTINEL", "1")
				bin := filepath.Join(home, "bin")
				if err := os.MkdirAll(bin, 0o755); err != nil {
					t.Fatal(err)
				}
				got := filepath.Join(home, "args")
				name, script := tool, "#!/bin/sh\nprintf '%s\\n' \"$@\" > '"+got+"'\n"
				if runtime.GOOS == "windows" {
					// LookPath finds it through PATHEXT, as it would aider.exe.
					name, script = tool+".cmd", "@echo %*> \""+got+"\"\r\n"
				}
				if err := os.WriteFile(filepath.Join(bin, name), []byte(script), 0o755); err != nil {
					t.Fatal(err)
				}
				t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))

				out := captureStdout(t, func() {
					if err := run([]string{tool, flag}); err != nil {
						t.Errorf("deja %s %s: %v", tool, flag, err)
					}
				})
				if strings.Contains(out, "Usage:") {
					t.Errorf("deja %s %s printed deja's own page:\n%s", tool, flag, out)
				}
				b, err := os.ReadFile(got)
				if err != nil {
					t.Fatalf("%s never ran: %v", tool, err)
				}
				if args := strings.TrimSpace(string(b)); args != flag {
					t.Errorf("%s got %q, want only %q", tool, args, flag)
				}
			})
		}
	}
}

// `deja help aider` stays deja's page: it is the one way to read what the
// wrapper does once --help belongs to the tool.
func TestDejaHelpStillDescribesTheWrappers(t *testing.T) {
	for _, tool := range []string{"aider", "goose"} {
		if !toolOwnsHelp(tool) {
			t.Errorf("%s is not marked as owning its --help", tool)
		}
		if h := helpForCommand(tool); !strings.Contains(h, "deja "+tool) {
			t.Errorf("deja help %s lost its page:\n%s", tool, h)
		}
	}
	if toolOwnsHelp("search") {
		t.Error("search's --help is deja's own")
	}
}

// With the tool missing, the error says so and where deja's own page is,
// rather than falling back to that page as if it had been asked for.
func TestToolHelpWithoutTheToolSaysSo(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	err := runToolHelp("aider", []string{"--help"})
	if err == nil || !strings.Contains(err.Error(), "not on PATH") || !strings.Contains(err.Error(), "deja help aider") {
		t.Errorf("err = %v", err)
	}
}
