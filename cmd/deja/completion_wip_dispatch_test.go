package main

import "testing"

// wip takes --json alone and brief takes nothing, but both fell through to the
// search flags in every script but fish, and refused each one (#5071).
func TestWipBriefCompletionDispatch(t *testing.T) {
	checkCompletionDispatch(t, []completionDispatchCase{
		{[]string{"deja", "wip", "--"}, []string{"--json"}, []string{"--re", "--harness", "--limit"}},
		{[]string{"deja", "brief", "--"}, nil, []string{"--json", "--re", "--harness", "--limit"}},
	})
}
