package main

import (
	"strings"
	"testing"
)

// --quiet belongs to index. bash and zsh offered it to search, which refuses
// it, and fish and PowerShell left it off index, where it works (#5073).
func TestIndexQuietCompletionDispatch(t *testing.T) {
	checkCompletionDispatch(t, []completionDispatchCase{
		{[]string{"deja", "search", "foo", "--"}, []string{"--limit"}, []string{"--quiet", "-quiet"}},
		{[]string{"deja", "index", "--"}, []string{"--rebuild", "--quiet"}, []string{"--json", "--limit"}},
	})
}

// The dispatch test above cannot reach bash's first-word list, which offered
// --quiet to `deja --<Tab>` and turned it into the query "--quiet ...". So each
// script names the flag exactly once, on its index line.
func TestQuietIsOfferedOnlyByIndex(t *testing.T) {
	for _, tc := range []struct{ shell, flag, line string }{
		{"bash", "--quiet", `compgen -W "--rebuild -rebuild --quiet -quiet"`},
		{"zsh", "'--quiet[", "'--rebuild[force a full rebuild]' '-rebuild[force a full rebuild]' '--quiet["},
		{"fish", "-l quiet", "__fish_seen_subcommand_from index' -l quiet"},
		{"powershell", "'--quiet'", "'index' { @('--rebuild', '-rebuild', '--quiet', '-quiet') }"},
	} {
		script := emittedCompletion(t, tc.shell)
		if n := strings.Count(script, tc.flag); n != 1 {
			t.Errorf("%s completion offers %s in %d places, want only index", tc.shell, tc.flag, n)
		}
		if !strings.Contains(script, tc.line) {
			t.Errorf("%s completion does not offer %s to index", tc.shell, tc.flag)
		}
	}
}
