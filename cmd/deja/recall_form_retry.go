package main

import (
	"fmt"
	"strings"
	"unicode"

	"github.com/vshulcz/deja-vu/internal/index"
	"github.com/vshulcz/deja-vu/internal/search"
)

// formRetryWanted is whether an answer missed in the way a respelling can
// fix: nothing came back, or what came back was ranked and none of it on the
// page holds every word. An exact answer already found the words as written.
func formRetryWanted(result index.SearchResult, strictShown int) bool {
	switch {
	case result.Tier == search.TierError, result.Directory != "":
		return false
	case result.Tier == search.TierExact && !result.Stemmed && !result.Fuzzy:
		return false
	}
	return strictShown == 0
}

// formRetryLine hands the agent a Russian query back in the spelling the store
// holds, as a q it can pass straight to recall.
//
// Agents reword a recall that missed — a quarter of real MCP recalls are such
// retries — but in 103 transcripts where the CLI's word-forms line named the
// form to search, the next call used it zero times (#5367). A finished query
// is a different thing to read than advice about one (#4761).
//
// Only the two respellings the catalog settles without a search: ё for е, and
// a name in the other alphabet. Case forms were tried and left out — the
// relevance tier already folds Russian endings, and picking the case the store
// wrote took a search per candidate to get right. No count either: the recall
// the agent makes next ranks project-first and cuts to its limit, so any
// number said here would describe a different answer. A query with no
// Cyrillic in it pays nothing.
func formRetryLine(dir, q string, o search.Options, served []search.Hit) string {
	if !hasCyrillic(q) || strings.Contains(q, `"`) {
		return ""
	}
	words := strings.FieldsFunc(q, notWordRune)
	if len(words) == 0 {
		return ""
	}
	swaps := index.StoreSpellings(dir, words, o)
	if len(swaps) == 0 {
		return ""
	}
	to := make(map[string]string, len(swaps))
	said := make([]string, 0, len(swaps))
	for _, s := range swaps {
		// The page already quotes a session in that spelling.
		if sessionsHolding(served, s.To) > 0 {
			continue
		}
		to[strings.ToLower(s.From)] = s.To
		said = append(said, fmt.Sprintf("%q as %q", s.From, s.To))
	}
	if len(said) == 0 {
		return ""
	}
	return fmt.Sprintf("\nThis store writes %s — call recall again with q=%q.\n",
		strings.Join(said, ", "), respell(q, to))
}

// respell swaps whole words of q, leaving the rest as the agent wrote it: the
// whole sentence ranks better than its bare terms (#2134).
func respell(q string, to map[string]string) string {
	var b strings.Builder
	var word []rune
	flush := func() {
		if len(word) == 0 {
			return
		}
		w := string(word)
		if r, ok := to[strings.ToLower(w)]; ok {
			w = r
		}
		b.WriteString(w)
		word = word[:0]
	}
	for _, r := range q {
		if notWordRune(r) {
			flush()
			b.WriteRune(r)
			continue
		}
		word = append(word, r)
	}
	flush()
	return b.String()
}

func notWordRune(r rune) bool { return !unicode.IsLetter(r) && !unicode.IsDigit(r) }

func hasCyrillic(s string) bool {
	for _, r := range s {
		if unicode.Is(unicode.Cyrillic, r) {
			return true
		}
	}
	return false
}
