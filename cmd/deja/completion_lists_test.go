package main

import (
	"strings"
	"testing"

	"github.com/vshulcz/deja-vu/internal/sources"
)

var completionTestShells = []string{"bash", "zsh", "fish", "powershell"}

func emittedCompletion(t *testing.T, shell string) string {
	t.Helper()
	out, err := captureRun(t, "completion", shell)
	if err != nil {
		t.Fatalf("completion %s: %v", shell, err)
	}
	return out
}

// Tab completion offered eleven harness names while deja read eighteen, in six
// hand-maintained copies of one list — hermes, goose, kimi, cline, roo,
// openclaw and zed were all missing. The lists are substituted from the
// registry now, and this is what keeps the next harness from repeating it.
func TestCompletionOffersEveryHarness(t *testing.T) {
	names := sources.HarnessNames()
	if len(names) < 10 {
		t.Fatalf("registry returned %d names, so this test proves nothing", len(names))
	}
	// The whole list, in registry order, not each name somewhere in the script:
	// every harness name also appears among the install targets, so looking for
	// them one at a time passes on a completion that offers none of them.
	want := strings.Join(names, " ")
	for _, shell := range completionTestShells {
		if script := emittedCompletion(t, shell); !strings.Contains(script, want) {
			t.Errorf("%s completion does not offer the registry's harnesses", shell)
		}
	}
}

// The same for the agents `handoff --to` accepts: that list is a function, and
// the shells held a copy of it from when it was shorter.
func TestCompletionOffersEveryHandoffTarget(t *testing.T) {
	targets := handoffTargets()
	if len(targets) < 5 {
		t.Fatalf("handoffTargets returned %d, so this test proves nothing", len(targets))
	}
	want := strings.Join(targets, " ")
	for _, shell := range completionTestShells {
		if script := emittedCompletion(t, shell); !strings.Contains(script, want) {
			t.Errorf("%s completion does not offer every handoff target", shell)
		}
	}
}

// And nothing is left holding a placeholder: a substitution that stops matching
// would otherwise ship the marker to the reader's shell.
func TestCompletionSubstitutesEveryPlaceholder(t *testing.T) {
	for _, shell := range completionTestShells {
		if script := emittedCompletion(t, shell); strings.Contains(script, "%HARNESSES%") ||
			strings.Contains(script, "%HANDOFF_TARGETS%") || strings.Contains(script, "%INSTALL_TARGETS%") ||
			strings.Contains(script, "%NOTE_STATES%") {
			t.Errorf("%s completion still carries a placeholder", shell)
		}
	}
}

// The roles list was the one hand-maintained copy left: seven of them across
// the three shells, all reading "user assistant tool" while `--role` accepted
// four more — tool-output, files, command and edit, the channels that record
// which files a session touched, which commands it ran and which edits it made
// (#1658). Substituted from knownRoles now, like the harnesses above.
// PowerShell kept two copies of the old list after that, because this test
// never looked at it. (#5017)
func TestCompletionOffersEveryRole(t *testing.T) {
	if len(knownRoles) < 5 {
		t.Fatalf("knownRoles holds %d entries, so this test proves nothing", len(knownRoles))
	}
	want := strings.Join(knownRoles, " ")
	for _, shell := range completionTestShells {
		script := emittedCompletion(t, shell)
		if !strings.Contains(script, want) {
			t.Errorf("%s completion does not offer every role deja accepts", shell)
		}
		// And no copy of the old short list may survive alongside it.
		if strings.Contains(script, "user assistant tool)") ||
			strings.Contains(script, "'user assistant tool'") ||
			strings.Contains(script, "'user', 'assistant', 'tool'") {
			t.Errorf("%s completion still holds a hand-written role list", shell)
		}
	}
}

// The command list was the copy that outlived every other: `deja recap` and
// `deja tests` shipped in 0.21.0 and neither bash nor fish had ever heard of
// them, while the bash and fish scripts each carried the same list twice over
// and the PowerShell array carried it three times — merges landing beside each
// other. A command nobody can tab-complete is a command most people never find.
func TestEveryCommandIsOfferedByEveryShell(t *testing.T) {
	offered := map[string]bool{}
	for _, name := range completionCommands() {
		offered[name] = true
	}
	for name := range commands {
		// Flags the table answers to, hooks a harness calls, and the internals
		// nobody types: none of them is a command to complete.
		if strings.HasPrefix(name, "-") || strings.HasPrefix(name, "hook-") || completionHiddenCommands[name] {
			continue
		}
		if !offered[name] {
			t.Errorf("`deja %s` is in the command table and in no completion script", name)
		}
	}
	// The reverse, minus the four run() answers before the map is consulted.
	handledEarly := map[string]bool{"search": true, "show": true, "last": true, "help": true}
	for name := range offered {
		if _, ok := commands[name]; !ok && !handledEarly[name] {
			t.Errorf("completion offers %q, which is not a command", name)
		}
	}
	want := strings.Join(completionCommands(), " ")
	for _, shell := range []string{"bash", "fish"} {
		if script := emittedCompletion(t, shell); !strings.Contains(script, want) {
			t.Errorf("%s completion does not offer the command list", shell)
		}
	}
}

// One list per shell, not three. The duplicates were harmless to the shell and
// fatal to the list: each copy drifted on its own.
func TestNoShellCarriesTheCommandListTwice(t *testing.T) {
	first := completionCommands()[0]
	for _, shell := range completionTestShells {
		script := emittedCompletion(t, shell)
		if n := strings.Count(script, first+" "); n > 2 {
			t.Errorf("%s completion mentions the head of the command list %d times", shell, n)
		}
	}
	ps := emittedCompletion(t, "powershell")
	if n := strings.Count(ps, "$commands = @("); n != 1 {
		t.Errorf("the powershell script declares $commands %d times", n)
	}
	// Counted in the array alone: recap has a case of its own now, which names
	// it once more further down (#5069).
	_, decl, _ := strings.Cut(ps, "$commands = @(")
	decl, _, _ = strings.Cut(decl, "\n")
	if n := strings.Count(decl, "'recap'"); n != 1 {
		t.Errorf("'recap' appears %d times in the powershell command list", n)
	}
}

// No shell had a case for restore, so bash, zsh and PowerShell fell through to
// the search flags — the first of which, --re, restore refuses — and fish
// offered nothing at all.
func TestCompletionHasARestoreBlock(t *testing.T) {
	for _, tc := range []struct{ shell, start, end string }{
		{"bash", "\n        restore)\n", "\n            ;;\n"},
		{"zsh", "\n    restore)\n", "\n      ;;\n"},
		{"fish", "__fish_seen_subcommand_from restore'", "\ncomplete -c deja -n '__fish_seen_subcommand_from secrets'"},
		{"powershell", "\n            'restore' {\n", "\n            }\n"},
	} {
		script := emittedCompletion(t, tc.shell)
		_, block, ok := strings.Cut(script, tc.start)
		if !ok {
			t.Errorf("%s completion has no restore block", tc.shell)
			continue
		}
		block, _, _ = strings.Cut(block, tc.end)
		// fish names a long flag "-l span" where the others spell "--span".
		block = strings.ReplaceAll(block, "-l ", "--")
		for _, flag := range []string{"--span", "--force", "--all", "out"} {
			if !strings.Contains(block, flag) {
				t.Errorf("%s restore block does not offer %s:\n%s", tc.shell, flag, block)
			}
		}
	}
}

// zsh keeps its command list by hand, with a description beside each name, so
// the joined-list check above cannot see it — and `deja recall` was missing
// from it while bash and fish offered it.
func TestZshOffersEveryCommand(t *testing.T) {
	script := emittedCompletion(t, "zsh")
	_, block, ok := strings.Cut(script, "\n  commands=(\n")
	if !ok {
		t.Fatal("zsh completion has no commands=( block")
	}
	block, _, _ = strings.Cut(block, "\n  )\n")
	for _, name := range completionCommands() {
		if !strings.Contains(block, "'"+name+":") {
			t.Errorf("zsh completion does not offer `deja %s`", name)
		}
	}
}

// recall fell through to the search flags in bash, zsh and PowerShell, most of
// which it refuses, and fish offered nothing for it.
func TestCompletionHasARecallBlock(t *testing.T) {
	for _, tc := range []struct{ shell, start, end string }{
		{"bash", "\n        recall)\n", "\n            ;;\n"},
		{"zsh", "\n    recall)\n", "\n      ;;\n"},
		{"fish", "__fish_seen_subcommand_from recall;", "\ncomplete -c deja -n '__fish_seen_subcommand_from remember'"},
		{"powershell", "\n            'recall' {\n", "\n            }\n"},
	} {
		script := emittedCompletion(t, tc.shell)
		_, block, ok := strings.Cut(script, tc.start)
		if !ok {
			t.Errorf("%s completion has no recall block", tc.shell)
			continue
		}
		block, _, _ = strings.Cut(block, tc.end)
		block = strings.ReplaceAll(block, "-l ", "--")
		for _, flag := range recallFlags {
			if !strings.Contains(block, flag) {
				t.Errorf("%s recall block does not offer %s:\n%s", tc.shell, flag, block)
			}
		}
		for _, flag := range []string{"--re", "--role", "--json", "--since"} {
			if strings.Contains(block, flag+" ") || strings.Contains(block, flag+"'") || strings.Contains(block, flag+"=") || strings.Contains(block, flag+"[") {
				t.Errorf("%s recall block offers %s, which recall refuses:\n%s", tc.shell, flag, block)
			}
		}
		if !strings.Contains(block, sources.HarnessNames()[0]) && !strings.Contains(block, "harnesses") {
			t.Errorf("%s recall block does not complete --harness:\n%s", tc.shell, block)
		}
	}
}
