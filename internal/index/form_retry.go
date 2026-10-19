package index

import (
	"sort"
	"strings"
)

// FormRetry is a query the store can answer in its own spelling: one form per
// word of the asked query, picked so that the fewest sessions hold them all.
type FormRetry struct {
	Terms    []string
	Sessions int
}

// formRetryCombos bounds how many spellings of a query are tried. Four words
// of eight forms each is 4096; the cap keeps the walk well under a
// millisecond and leaves the common shapes (two or three words, two to five
// forms each) untouched.
const formRetryCombos = 1024

// WordFormRetry looks for the spelling of a Russian query that the store
// holds word for word.
//
// The relevance tier folds Russian endings, so a query in the wrong case
// still ranks; what it cannot do on a store where every session uses the same
// few nouns is tell the one session that said all of them apart from the
// hundreds that said most. The exact AND can, but only in the form the store
// wrote, and the agent has no way to know which form that was. The catalog
// does: every word gets its other forms (and a Latin or Cyrillic spelling of
// it, when the store holds one), and the combination with the narrowest
// non-empty intersection is the query to hand back.
//
// skip names the sessions ("harness:id") the caller already showed; a
// spelling that only finds those is no retry. Nothing is returned for fewer
// than two words: a single word has no intersection to narrow, and its
// thinnest form is just its rarest one.
func WordFormRetry(dir string, terms []string, skip map[string]bool) (FormRetry, bool) {
	if len(terms) < 2 || len(terms) > 4 {
		return FormRetry{}, false
	}
	if dir == "" {
		dir = DefaultDir()
	}
	catalog, err := tokenCatalogCached(dir)
	if err != nil {
		return FormRetry{}, false
	}
	options := make([][]string, len(terms))
	changed := false
	for i, term := range terms {
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
					add(f)
				}
				for _, f := range shortNounForms(v) {
					add(f)
				}
			}
		}
		for _, f := range otherScriptSpellings(term) {
			add(f)
		}
		if len(opts) > 1 {
			changed = true
		}
		options[i] = opts
	}
	if !changed {
		return FormRetry{}, false
	}
	m, err := readManifestCached(dir)
	if err != nil {
		return FormRetry{}, false
	}
	servable := servableSids(dir)
	key := make(map[uint32]string, len(m.Sessions))
	for _, meta := range m.Sessions {
		key[meta.Ord] = meta.Harness + ":" + meta.ID
	}
	// Spoken sessions only, for the reason TermSessionCountsSpoken gives: a
	// form deja printed into a transcript is not a word anyone wrote (#3820).
	sets := map[string]map[uint32]bool{}
	recs := map[string]map[int64]uint32{}
	for _, opts := range options {
		for _, f := range opts {
			if _, ok := sets[f]; ok {
				continue
			}
			set := map[uint32]bool{}
			rec := map[int64]uint32{}
			if posts, err := postingsFor(dir, "t"+f); err == nil {
				for _, p := range posts {
					if p.Tool || (servable != nil && !servable[p.Sid]) {
						continue
					}
					set[p.Sid] = true
					rec[p.Off] = p.Sid
				}
			}
			sets[f], recs[f] = set, rec
		}
	}
	// Narrowest first so the walk prunes early: an empty partial intersection
	// ends that branch.
	for _, opts := range options {
		sort.SliceStable(opts, func(a, b int) bool { return len(sets[opts[a]]) < len(sets[opts[b]]) })
	}
	asked := strings.Join(terms, " ")
	// One message saying every word is what a session about the thing looks
	// like; the same words scattered over a long session is what a store
	// where everyone uses them looks like. A spelling that meets in one
	// message wins over one that only meets in one session, then the
	// narrowest wins.
	type scored struct {
		FormRetry
		together int
	}
	var best scored
	better := func(c scored) bool {
		if best.Terms == nil {
			return true
		}
		if (c.together > 0) != (best.together > 0) {
			return c.together > 0
		}
		if c.Sessions != best.Sessions {
			return c.Sessions < best.Sessions
		}
		return c.together > best.together
	}
	tried := 0
	pick := make([]string, len(terms))
	var walk func(i int, in map[uint32]bool, rin map[int64]uint32)
	walk = func(i int, in map[uint32]bool, rin map[int64]uint32) {
		if tried >= formRetryCombos {
			return
		}
		if i == len(terms) {
			tried++
			if strings.Join(pick, " ") == asked {
				return
			}
			fresh := 0
			for sid := range in {
				if k, ok := key[sid]; ok && !skip[k] {
					fresh++
				}
			}
			if fresh == 0 {
				return
			}
			together := 0
			for _, sid := range rin {
				if k, ok := key[sid]; ok && !skip[k] {
					together++
				}
			}
			c := scored{FormRetry{Terms: append([]string(nil), pick...), Sessions: len(in)}, together}
			if better(c) {
				best = c
			}
			return
		}
		for _, f := range options[i] {
			next, rnext := sets[f], recs[f]
			if in != nil {
				next = intersectSids(in, next)
				rnext = intersectRecs(rin, rnext)
			}
			if len(next) == 0 {
				continue
			}
			pick[i] = f
			walk(i+1, next, rnext)
		}
	}
	walk(0, nil, nil)
	return best.FormRetry, best.Terms != nil
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

func intersectSids(a, b map[uint32]bool) map[uint32]bool {
	if len(a) > len(b) {
		a, b = b, a
	}
	out := make(map[uint32]bool, len(a))
	for k := range a {
		if b[k] {
			out[k] = true
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
