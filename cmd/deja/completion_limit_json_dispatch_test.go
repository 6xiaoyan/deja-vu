package main

import "testing"

func TestLimitJSONCompletionDispatch(t *testing.T) {
	checkCompletionDispatch(t, []completionDispatchCase{
		{[]string{"deja", "fix", "--"}, []string{"--limit", "--json"}, []string{"--since", "--re"}},
		{[]string{"deja", "friction", "--"}, []string{"--limit", "--json"}, []string{"--since", "--re"}},
		{[]string{"deja", "tests", "--"}, []string{"--limit", "--json"}, []string{"--since", "--re"}},
	})
}
