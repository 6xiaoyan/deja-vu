package main

import "testing"

// log and view had no case in any script, so Tab offered the search flags,
// which both refuse, and never their own (#5070).
func TestLogViewCompletionDispatch(t *testing.T) {
	checkCompletionDispatch(t, []completionDispatchCase{
		{[]string{"deja", "log", "--"}, []string{"--last", "--json"}, []string{"--re", "--harness"}},
		{[]string{"deja", "view", "--"}, []string{"--no-open", "--out"}, []string{"--re", "--json"}},
	})
}
