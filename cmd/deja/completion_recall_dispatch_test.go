package main

import "testing"

func TestRecallCompletionDispatch(t *testing.T) {
	checkCompletionDispatch(t, []completionDispatchCase{
		{[]string{"deja", "recall", "--"}, []string{"--project", "--harness", "--limit"}, []string{"--re", "--role", "--json"}},
		{[]string{"deja", "search", "--"}, []string{"--re", "--role"}, nil},
	})
}
