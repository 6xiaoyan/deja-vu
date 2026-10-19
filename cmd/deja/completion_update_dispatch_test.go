package main

import "testing"

// update sat in the no-argument list in every script, so Tab offered nothing
// for the one flag it takes (#5072).
func TestUpdateCompletionDispatch(t *testing.T) {
	checkCompletionDispatch(t, []completionDispatchCase{
		{[]string{"deja", "update", "--"}, []string{"--force"}, []string{"--re", "--json"}},
	})
}
