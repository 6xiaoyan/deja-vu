package index

import (
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/vshulcz/deja-vu/internal/query"
)

// Respelling is one word of a query and the spelling the store holds for it.
type Respelling struct {
	From, To string
}

// nameRareMax is how many sessions the other-alphabet spelling of a query's
// first word may sit in and still read as a name. A first word is capitalized
// because it starts the sentence as often as because it is a name, and the
// romanization of an ordinary word is usually a common English one: мост is
// most, in hundreds of sessions; Кузнецов is kuznetsov, in a few.
const nameRareMax = 5

// StoreSpellings finds the words of a Russian query the store writes another
// way, from the catalog alone — no search, so it costs a lookup per word.
//
// Two moves, both of a single word, neither a guess the catalog has not seen:
// ё for е or е for ё, which the index keeps apart («отчёт» and «отчет» never
// meet); and a name in the other alphabet («Кузнецов» for kuznetsov,
// «Shevchenko» for шевченко). A name is a word the asker capitalized whose
// spelling in its own alphabet the store holds in no form at all — a Russian
// word the store writes in Russian is not a name, and romanizing it hands
// back an English word (мост, most). The first word of the query is held to
// more: its other spelling must be rare.
//
// words are the query's words as written, in order; o scopes the sessions a
// spelling has to be found in, so a name only another project holds is not
// offered to a recall filtered to this one.
func StoreSpellings(dir string, words []string, o query.Options) []Respelling {
	if dir == "" {
		dir = DefaultDir()
	}
	catalog, err := tokenCatalogCached(dir)
	if err != nil {
		return nil
	}
	var metaByOrd map[uint32]SessionMeta
	var servable map[uint32]bool
	loaded := false
	recs := map[string]map[int64]uint32{}
	// The messages a form is said in, inside the scope o sets.
	records := func(form string) map[int64]uint32 {
		if r, ok := recs[form]; ok {
			return r
		}
		if !loaded {
			loaded = true
			if m, err := readManifestCached(dir); err == nil {
				metaByOrd = sessionMetaByOrd(m)
			}
			servable = servableSids(dir)
		}
		out := map[int64]uint32{}
		if posts, err := postingsFor(dir, "t"+form); err == nil {
			for _, p := range posts {
				if servable != nil && !servable[p.Sid] {
					continue
				}
				if meta, ok := metaByOrd[p.Sid]; ok && sessionMetaMatches(meta, o) {
					out[p.Off] = p.Sid
				}
			}
		}
		recs[form] = out
		return out
	}
	var terms []string
	for _, w := range words {
		if t := strings.ToLower(w); len(t) >= 2 && !query.IsStopWord(t) {
			terms = append(terms, t)
		}
	}
	// meets is whether the respelling shares a message with another word of
	// the query, in any case form the store holds. A spelling that never sits
	// next to the rest brings back the same ranked page: the review's «отчёт по
	// счетам» was held, and never beside счетам.
	meets := func(form, term string) bool {
		mine := records(form)
		if len(mine) == 0 {
			return false
		}
		others := 0
		for _, t := range terms {
			if t == term {
				continue
			}
			others++
			forms := []string{t}
			if isCyrToken(t) {
				forms = append(yoSpellings(t), stemMatches(t, catalog)...)
			}
			for _, f := range forms {
				if !catalog[f] {
					continue
				}
				for off := range records(f) {
					if _, ok := mine[off]; ok {
						return true
					}
				}
			}
		}
		return others == 0
	}
	sessions := func(form string) int {
		seen := map[uint32]bool{}
		for _, sid := range records(form) {
			seen[sid] = true
		}
		return len(seen)
	}
	var out []Respelling
	done := map[string]bool{}
	for i, w := range words {
		term := strings.ToLower(w)
		if done[term] || query.IsStopWord(term) {
			continue
		}
		done[term] = true
		if isCyrToken(term) {
			if to := yoRespelling(term, catalog); to != "" && meets(to, term) {
				out = append(out, Respelling{w, to})
				continue
			}
		}
		if !capitalized(w) || len([]rune(term)) < 4 || catalog[term] || heldInAnyForm(term, catalog) {
			continue
		}
		for _, to := range otherScriptSpellings(term) {
			if !catalog[to] {
				continue
			}
			if i == 0 && sessions(to) > nameRareMax {
				continue
			}
			if !meets(to, term) {
				continue
			}
			out = append(out, Respelling{w, to})
			break
		}
	}
	return out
}

// yoRespelling is the word with ё swapped for е or back, when the catalog
// holds that spelling.
func yoRespelling(term string, catalog map[string]bool) string {
	for _, v := range yoSpellings(term)[1:] {
		if catalog[v] {
			return v
		}
	}
	return ""
}

// heldInAnyForm is whether the store writes the word in its own alphabet at
// all: as it is, with ё or е, or in another case.
func heldInAnyForm(term string, catalog map[string]bool) bool {
	if !isCyrToken(term) {
		return false
	}
	for _, v := range yoSpellings(term) {
		if catalog[v] || len(stemMatches(v, catalog)) > 0 {
			return true
		}
	}
	return false
}

func capitalized(w string) bool {
	r, _ := utf8.DecodeRuneInString(w)
	return unicode.IsUpper(r)
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
		out := romanize(term)
		// A surname's -ий and -ый are usually written -y: Шиманский, Shimansky.
		for _, end := range []string{"ий", "ый"} {
			if stem, ok := strings.CutSuffix(term, end); ok {
				for _, s := range romanize(stem) {
					if len(out) < 2*spellingCap {
						out = append(out, s+"y")
					}
				}
			}
		}
		return out
	}
	for _, r := range term {
		if r < 'a' || r > 'z' {
			return nil
		}
	}
	return cyrillicReadings(term)
}

// romanize spells a Cyrillic word in Latin letters, in the few ways each
// letter usually goes.
func romanize(term string) []string {
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

// cyrillicReadings reads a romanized word back into Cyrillic.
func cyrillicReadings(term string) []string {
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
