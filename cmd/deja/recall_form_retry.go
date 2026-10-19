package main

import (
	"fmt"
	"strings"
	"unicode"

	"github.com/vshulcz/deja-vu/internal/index"
	"github.com/vshulcz/deja-vu/internal/query"
	"github.com/vshulcz/deja-vu/internal/search"
)

// formRetryThin is how few sessions an exact answer can hold and still get a
// retry line: below it the AND is the thin kind that missed (the bound the
// index uses for its relevance tail).
const formRetryThin = 10

// formRetryWanted is whether an answer missed in the way a respelling can
// fix: no session held every word, or only a few did.
func formRetryWanted(result index.SearchResult, total int) bool {
	switch {
	case result.Tier == search.TierError, result.Directory != "":
		return false
	case result.Tier == search.TierExact && !result.Stemmed && !result.Fuzzy && total >= formRetryThin:
		return false
	}
	return true
}

// formRetryLine hands the agent a Russian query back in the spelling the store
// holds, as a q it can pass straight to recall.
//
// Agents reword a recall that missed — a quarter of real MCP recalls are such
// retries — but in 103 transcripts where the CLI's word-forms line named the
// form to search, the next call used it zero times (#5367). A finished query
// is a different thing to read than advice about one (#4761). Only for a query
// with Cyrillic in it, so an English recall pays nothing.
func formRetryLine(dir, q string, wanted bool, served []search.Hit) string {
	if !wanted || !hasCyrillic(q) {
		return ""
	}
	terms, phrases := query.QueryParts(q)
	if len(phrases) > 0 {
		return ""
	}
	skip := make(map[string]bool, len(served))
	for _, h := range served {
		skip[h.Session.Harness+":"+h.Session.ID] = true
	}
	r, ok := index.WordFormRetry(dir, terms, skip)
	if !ok {
		return ""
	}
	next := strings.Join(r.Terms, " ")
	return fmt.Sprintf("\nIn the forms this store holds, %s %s every word — call recall again with q=%q.\n",
		pluralSessions(r.Sessions), holdOrHolds(r.Sessions), next)
}

func hasCyrillic(s string) bool {
	for _, r := range s {
		if unicode.Is(unicode.Cyrillic, r) {
			return true
		}
	}
	return false
}
