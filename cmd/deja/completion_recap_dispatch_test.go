package main

import "testing"

// recap had no case in any script, so Tab offered the search flags: --since
// only because search has it too, and --project, which recap refuses (#5069).
func TestRecapCompletionDispatch(t *testing.T) {
	checkCompletionDispatch(t, []completionDispatchCase{
		{[]string{"deja", "recap", "--"}, []string{"--since", "--limit", "--json"}, []string{"--project", "--re", "--harness"}},
	})
}
