package main

import (
	"fmt"
	"strings"
	"unicode"

	"github.com/vshulcz/deja-vu/internal/index"
	"github.com/vshulcz/deja-vu/internal/query"
	"github.com/vshulcz/deja-vu/internal/search"
)

// formRetryWanted is whether an answer missed in the way a respelling can
// fix: nothing came back, or what came back was ranked and none of it on the
// page holds every word. An exact answer already found the words as written.
func formRetryWanted(result index.SearchResult, strictShown int) bool {
	switch {
	case result.Tier == search.TierError, result.Directory != "":
		return false
	case strictlyMatched(result):
		return false
	}
	return strictShown == 0
}

// strictlyMatched is an answer every session of which holds every word.
func strictlyMatched(result index.SearchResult) bool {
	return result.Tier == search.TierExact && !result.Stemmed && !result.Fuzzy && result.Directory == ""
}

// formRetryLine hands the agent a Russian query back in the spelling the store
// holds, as a q it can pass straight to recall.
//
// Agents reword a recall that missed — a quarter of real MCP recalls are such
// retries — but in 103 transcripts where the CLI's word-forms line named the
// form to search, the next call used it zero times (#5367). A finished query
// is a different thing to read than advice about one (#4761). Only for a query
// with Cyrillic in it, so an English recall pays nothing.
//
// o is the search the first answer ran — its project, harness and live-session
// cuts — and every candidate is run through it before the line says anything:
// the count is what that recall will find, and a spelling that finds nothing
// new is not offered.
func formRetryLine(dir, q string, o search.Options, pathScope []string, served []search.Hit) string {
	if !hasCyrillic(q) {
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
	// A word written capitalized may be a name, and a name is the one word the
	// store may hold in both alphabets; it gets the other alphabet once no
	// spelling in this one found anything. Lowercase words get it only where
	// the store has no form of them at all: мост romanizes to most, an English
	// word, not the same one.
	tried := map[string]bool{}
	for _, names := range []map[string]bool{nil, capitalizedWords(q)} {
		if names != nil && len(names) == 0 {
			break
		}
		for _, c := range index.WordFormRetry(dir, terms, o, skip, names) {
			next := strings.Join(c.Terms, " ")
			if tried[next] {
				continue
			}
			tried[next] = true
			if n := formRetryFinds(dir, next, o, pathScope, skip); n > 0 {
				return fmt.Sprintf("\nIn the forms this store holds, %s %s every word — call recall again with q=%q.\n",
					pluralSessions(n), holdOrHolds(n), next)
			}
		}
	}
	return ""
}

// formRetryFinds runs a respelling the way recall will and counts the sessions
// that hold every word of it; zero when none of them is new to the page.
func formRetryFinds(dir, next string, o search.Options, pathScope []string, skip map[string]bool) int {
	vo := o
	vo.Query = next
	result, _, hits, _, err := recallRank(dir, next, vo)
	if err != nil {
		return 0
	}
	if len(pathScope) > 0 {
		hits = inProjectHits(hits, pathScope)
	}
	hits = dropCopiedHits(dir, hits)
	all := strictlyMatched(result)
	n, fresh := 0, false
	for _, h := range hits {
		if all || h.Strict {
			n++
			if !skip[h.Session.Harness+":"+h.Session.ID] {
				fresh = true
			}
		}
	}
	if !fresh {
		return 0
	}
	return n
}

// capitalizedWords is the words of q written with a capital, lowercased the
// way the query terms are.
func capitalizedWords(q string) map[string]bool {
	out := map[string]bool{}
	for _, w := range strings.FieldsFunc(q, func(r rune) bool { return !unicode.IsLetter(r) && !unicode.IsDigit(r) }) {
		if r := []rune(w); unicode.IsUpper(r[0]) {
			out[strings.ToLower(w)] = true
		}
	}
	return out
}

func hasCyrillic(s string) bool {
	for _, r := range s {
		if unicode.Is(unicode.Cyrillic, r) {
			return true
		}
	}
	return false
}
