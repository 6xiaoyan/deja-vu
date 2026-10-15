package main

import "testing"

func TestRestoreCompletionDispatch(t *testing.T) {
	checkCompletionDispatch(t, []completionDispatchCase{
		{[]string{"deja", "restore", "src/a.go", "--"}, []string{"--span", "--force", "--all"}, []string{"--re", "--json"}},
		{[]string{"deja", "last", "--"}, []string{"--json"}, []string{"--span", "--force"}},
	})
}
