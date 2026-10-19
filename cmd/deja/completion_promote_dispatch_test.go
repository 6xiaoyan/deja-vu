package main

import (
	"sort"
	"strings"
	"testing"

	"github.com/vshulcz/deja-vu/internal/sources"
)

// promote had no case in any script, so bash, zsh and PowerShell offered the
// search flags, every one refused, and fish offered nothing — not even the four
// states --state accepts (#5066).
func TestPromoteCompletionDispatch(t *testing.T) {
	checkCompletionDispatch(t, []completionDispatchCase{
		{[]string{"deja", "promote", "01a0", "--"}, []string{"--state", "--note", "--tag", "--to"}, []string{"--re", "--json"}},
		{[]string{"deja", "promote", "01a0", "--state", ""}, []string{"accepted", "rejected", "superseded", "stale"}, []string{"--re", "--note"}},
	})
}

// The states are substituted from the map promote checks against, like the
// roles, so a fifth state cannot leave the scripts behind.
func TestCompletionOffersEveryNoteState(t *testing.T) {
	states := make([]string, 0, len(sources.NoteStates))
	for s := range sources.NoteStates {
		states = append(states, s)
	}
	sort.Strings(states)
	want := strings.Join(states, " ")
	for _, shell := range completionTestShells {
		if script := emittedCompletion(t, shell); !strings.Contains(script, want) {
			t.Errorf("%s completion does not offer every promote state", shell)
		}
	}
}
