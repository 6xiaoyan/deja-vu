package main

import "testing"

// files and how take the same four flags, but neither had a case, so Tab
// offered the search flags: two by accident, the rest refused, and never
// --all-projects, the one each help page points at (#5067).
func TestFilesHowCompletionDispatch(t *testing.T) {
	checkCompletionDispatch(t, []completionDispatchCase{
		{[]string{"deja", "files", "x", "--"}, []string{"--all-projects", "--limit", "--project", "--json"}, []string{"--harness", "--re"}},
		{[]string{"deja", "how", "x", "--"}, []string{"--all-projects", "--limit", "--project", "--json"}, []string{"--harness", "--re"}},
	})
}
