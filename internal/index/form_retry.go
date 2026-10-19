package index

import (
	"sort"
	"strings"

	"github.com/vshulcz/deja-vu/internal/query"
)

// FormRetry is a spelling of the asked query that the store holds: one form
// per word, all of them in one message of Sessions sessions.
type FormRetry struct {
	Terms    []string
	Sessions int
	changed  int
}

// formRetryCombos bounds how many spellings of a query are tried; the walk
// prunes on an empty intersection, so the common shapes never reach it.
const formRetryCombos = 1024

// formRetryWords bounds the query length the walk takes on.
const formRetryWords = 6

// WordFormRetry lists, best first, up to three spellings of a Russian query
// that the store holds word for word in one message.
//
// The relevance tier folds Russian endings, so a query in the wrong case still
// ranks; what it cannot do on a store where everyone uses the same nouns is
// tell the one session that said all of them apart from the hundreds that said
// most. The exact AND can, but only in the form the store wrote, and the agent
// cannot know which form that was. The catalog does.
//
// The intersection is the search's own: per message, over the sessions o lets
// through, so a spelling found only in another project or in the live session
// is not offered. skip names the sessions ("harness:id") the caller already
// showed; a spelling that only finds those is no retry. names are the words
// the asker wrote as a name, which may be spelled in the other alphabet even
// where the store also writes them in this one. The caller still runs the
// candidates through the search before it claims anything.
func WordFormRetry(dir string, terms []string, o query.Options, skip map[string]bool, names map[string]bool) []FormRetry {
	if len(terms) < 2 || len(terms) > formRetryWords {
		return nil
	}
	if dir == "" {
		dir = DefaultDir()
	}
	catalog, err := tokenCatalogCached(dir)
	if err != nil {
		return nil
	}
	options := make([][]string, len(terms))
	changed := false
	for i, term := range terms {
		options[i] = retryOptions(term, catalog, names[term])
		if len(options[i]) > 1 {
			changed = true
		}
	}
	if !changed {
		return nil
	}
	m, err := readManifestCached(dir)
	if err != nil {
		return nil
	}
	servable := servableSids(dir)
	key := make(map[uint32]string, len(m.Sessions))
	for _, meta := range m.Sessions {
		if sessionMetaMatches(meta, o) && (servable == nil || servable[meta.Ord]) {
			key[meta.Ord] = meta.Harness + ":" + meta.ID
		}
	}
	recs := map[string]map[int64]uint32{}
	for _, opts := range options {
		for _, f := range opts {
			if _, ok := recs[f]; ok {
				continue
			}
			rec := map[int64]uint32{}
			if posts, err := postingsFor(dir, "t"+f); err == nil {
				for _, p := range posts {
					if _, ok := key[p.Sid]; ok {
						rec[p.Off] = p.Sid
					}
				}
			}
			recs[f] = rec
		}
	}
	for _, opts := range options {
		sort.SliceStable(opts, func(a, b int) bool { return len(recs[opts[a]]) < len(recs[opts[b]]) })
	}
	var found []FormRetry
	tried := 0
	pick := make([]string, len(terms))
	used := map[string]bool{}
	var walk func(i int, rin map[int64]uint32)
	walk = func(i int, rin map[int64]uint32) {
		if tried >= formRetryCombos {
			return
		}
		if i == len(terms) {
			tried++
			n := 0
			for j := range pick {
				if pick[j] != terms[j] {
					n++
				}
			}
			if n == 0 {
				return
			}
			sessions := map[uint32]bool{}
			fresh := false
			for _, sid := range rin {
				sessions[sid] = true
				if !skip[key[sid]] {
					fresh = true
				}
			}
			if fresh {
				found = append(found, FormRetry{Terms: append([]string(nil), pick...), Sessions: len(sessions), changed: n})
			}
			return
		}
		for _, f := range options[i] {
			// One word, one place: the same form twice is not a spelling of
			// two words.
			if used[f] {
				continue
			}
			next := recs[f]
			if rin != nil {
				next = intersectRecs(rin, next)
			}
			if len(next) == 0 {
				continue
			}
			pick[i] = f
			used[f] = true
			walk(i+1, next)
			used[f] = false
		}
	}
	walk(0, nil)
	// The narrowest first: a spelling that a handful of sessions share is a
	// session about the thing; one that hundreds share is the store's
	// vocabulary. Then the one closest to what was asked.
	sort.SliceStable(found, func(a, b int) bool {
		if found[a].Sessions != found[b].Sessions {
			return found[a].Sessions < found[b].Sessions
		}
		if found[a].changed != found[b].changed {
			return found[a].changed < found[b].changed
		}
		return strings.Join(found[a].Terms, " ") < strings.Join(found[b].Terms, " ")
	})
	if len(found) > 3 {
		found = found[:3]
	}
	return found
}

// retryOptions is a word and the spellings of it the catalog holds: its other
// cases and ё, and the word in the other alphabet only when the store holds
// none of those, or when the caller says the word is a name (asName). A
// Russian word the store writes in Russian is usually not a name, and its
// romanization is an English word more often than not: мост is most, план is
// plan, лист is list.
func retryOptions(term string, catalog map[string]bool, asName bool) []string {
	opts := []string{term}
	seen := map[string]bool{term: true}
	add := func(f string) {
		if !seen[f] && catalog[f] {
			seen[f] = true
			opts = append(opts, f)
		}
	}
	if isCyrToken(term) {
		for _, v := range yoSpellings(term) {
			add(v)
			for _, f := range stemMatches(v, catalog) {
				if wholeForm(term, f) {
					add(f)
				}
			}
			for _, f := range shortNounForms(v) {
				add(f)
			}
		}
	}
	if asName || (len(opts) == 1 && !catalog[term]) {
		for _, f := range otherScriptSpellings(term) {
			add(f)
		}
	}
	return opts
}

// wholeForm keeps the forms that are the word with another ending, not a
// stem the catalog happens to hold: упал does not mean упа, and данные does
// not mean данн. A form longer than the stem carries an ending and is a word;
// the bare stem is one only where Russian writes it bare — a noun's consonant
// stem (лимитами, лимит), not a verb's (упал, упа) and not a cut through a
// doubled letter (данные, данн).
func wholeForm(term, form string) bool {
	if len([]rune(form)) < 3 {
		return false
	}
	stem, end := term, ""
	for _, e := range cyrEndings {
		if strings.HasSuffix(term, e) && len([]rune(term))-len([]rune(e)) >= 3 {
			stem, end = strings.TrimSuffix(term, e), e
			break
		}
	}
	if form != stem {
		return true
	}
	if cyrVerbEndings[end] {
		return false
	}
	rs := []rune(stem)
	last := rs[len(rs)-1]
	return !strings.ContainsRune("аеёиоуыэюяйьъ", last) && rs[len(rs)-2] != last
}

func intersectRecs(a, b map[int64]uint32) map[int64]uint32 {
	if len(a) > len(b) {
		a, b = b, a
	}
	out := make(map[int64]uint32, len(a))
	for k, v := range a {
		if _, ok := b[k]; ok {
			out[k] = v
		}
	}
	return out
}

// cyrToLat is one common romanization per letter, with the alternatives a
// name is often written with. Letters, not words: there is no loanword list
// here, and a spelling only counts when the catalog holds it.
var cyrToLat = map[rune][]string{
	'а': {"a"}, 'б': {"b"}, 'в': {"v"}, 'г': {"g"}, 'д': {"d"}, 'е': {"e"},
	'ё': {"e", "yo"}, 'ж': {"zh"}, 'з': {"z"}, 'и': {"i"}, 'й': {"y", "i", "j"},
	'к': {"k"}, 'л': {"l"}, 'м': {"m"}, 'н': {"n"}, 'о': {"o"}, 'п': {"p"},
	'р': {"r"}, 'с': {"s"}, 'т': {"t"}, 'у': {"u"}, 'ф': {"f"}, 'х': {"kh", "h"},
	'ц': {"ts", "c"}, 'ч': {"ch"}, 'ш': {"sh"}, 'щ': {"shch", "sch"}, 'ъ': {""},
	'ы': {"y"}, 'ь': {""}, 'э': {"e"}, 'ю': {"yu", "ju"}, 'я': {"ya", "ja"},
}

// latToCyr reads a romanized word back, longest letter group first.
var latToCyr = []struct {
	lat string
	cyr []string
}{
	{"shch", []string{"щ"}}, {"sch", []string{"щ"}},
	{"zh", []string{"ж"}}, {"kh", []string{"х"}}, {"ts", []string{"ц", "тс"}},
	{"ch", []string{"ч"}}, {"sh", []string{"ш"}}, {"yu", []string{"ю"}},
	{"ya", []string{"я"}}, {"yo", []string{"ё", "йо"}}, {"ju", []string{"ю"}},
	{"ja", []string{"я"}},
	{"a", []string{"а"}}, {"b", []string{"б"}}, {"c", []string{"ц", "к"}},
	{"d", []string{"д"}}, {"e", []string{"е", "э"}}, {"f", []string{"ф"}},
	{"g", []string{"г"}}, {"h", []string{"х"}}, {"i", []string{"и", "й"}},
	{"j", []string{"й"}}, {"k", []string{"к"}}, {"l", []string{"л"}},
	{"m", []string{"м"}}, {"n", []string{"н"}}, {"o", []string{"о"}},
	{"p", []string{"п"}}, {"r", []string{"р"}}, {"s", []string{"с"}},
	{"t", []string{"т"}}, {"u", []string{"у"}}, {"v", []string{"в"}},
	{"w", []string{"в"}}, {"x", []string{"кс"}}, {"y", []string{"ы", "й"}},
	{"z", []string{"з"}},
}

// spellingCap bounds the spellings one word expands to.
const spellingCap = 32

// otherScriptSpellings is a word written in the other alphabet, in the few
// ways a romanization usually goes. Most of them are words nobody wrote; the
// caller keeps only the ones the catalog holds.
func otherScriptSpellings(term string) []string {
	if len([]rune(term)) < 4 {
		return nil
	}
	if isCyrToken(term) {
		out := []string{""}
		for _, r := range term {
			alts, ok := cyrToLat[r]
			if !ok {
				return nil
			}
			var next []string
			for _, s := range out {
				for _, a := range alts {
					if len(next) < spellingCap {
						next = append(next, s+a)
					}
				}
			}
			out = next
		}
		return out
	}
	for _, r := range term {
		if r < 'a' || r > 'z' {
			return nil
		}
	}
	var out []string
	var walk func(rest, acc string)
	walk = func(rest, acc string) {
		if len(out) >= spellingCap {
			return
		}
		if rest == "" {
			out = append(out, acc)
			return
		}
		for _, g := range latToCyr {
			if strings.HasPrefix(rest, g.lat) {
				for _, c := range g.cyr {
					walk(rest[len(g.lat):], acc+c)
				}
			}
		}
	}
	walk(term, "")
	return out
}

// yoSpellings is a word with and without ё. Most Russian text writes е for
// it and some writes ё, and the index keeps what was written: «отчёт» and
// «отчет» are two tokens that never meet.
func yoSpellings(term string) []string {
	out := []string{term}
	if strings.ContainsRune(term, 'ё') {
		return append(out, strings.ReplaceAll(term, "ё", "е"))
	}
	rs := []rune(term)
	var at []int
	for i, r := range rs {
		if r == 'е' {
			at = append(at, i)
		}
	}
	if len(at) == 0 || len(at) > 3 {
		return out
	}
	// One ё per word: Russian does not write two.
	for _, i := range at {
		v := append([]rune(nil), rs...)
		v[i] = 'ё'
		out = append(out, string(v))
	}
	return out
}

// shortNounEndings are the hard-stem noun cases.
var shortNounEndings = []string{"а", "у", "е", "ом", "ем", "и", "ы", "ов", "ей", "ам", "ами", "ах"}

// shortNounForms covers the three-letter nouns stemMatches leaves alone
// (кэш, лог, баг): its four-rune floor is there for the relevance tier, which
// has no catalog gate and would fan что and как out into other words. Here
// every form still has to be in the catalog and meet the query's other words,
// so a consonant-final word gets its noun cases.
func shortNounForms(term string) []string {
	rs := []rune(term)
	if len(rs) != 3 || strings.ContainsRune("аеёиоуыэюяйьъ", rs[2]) {
		return nil
	}
	out := make([]string, 0, len(shortNounEndings))
	for _, e := range shortNounEndings {
		out = append(out, term+e)
	}
	return out
}
