package index

import (
	"slices"
	"testing"

	"github.com/vshulcz/deja-vu/internal/query"
)

func TestOtherScriptSpellingsReachTheUsualRomanization(t *testing.T) {
	for _, tc := range []struct{ in, want string }{
		{"кузнецов", "kuznetsov"},
		{"деплой", "deploy"},
		{"shevchenko", "шевченко"},
		{"deploy", "деплой"},
	} {
		if got := otherScriptSpellings(tc.in); !slices.Contains(got, tc.want) {
			t.Errorf("%s: %v lacks %s", tc.in, got, tc.want)
		}
	}
	// Short words and anything outside the two alphabets stay as they are.
	for _, in := range []string{"кот", "api", "go-build", "日本語です"} {
		if got := otherScriptSpellings(in); len(got) != 0 {
			t.Errorf("%s: want nothing, got %v", in, got)
		}
	}
}

func TestYoSpellingsGoBothWays(t *testing.T) {
	if got := yoSpellings("отчёт"); !slices.Contains(got, "отчет") {
		t.Errorf("отчёт: %v", got)
	}
	if got := yoSpellings("платежный"); !slices.Contains(got, "платёжный") {
		t.Errorf("платежный: %v", got)
	}
}

func TestShortNounFormsOnlyForConsonantStems(t *testing.T) {
	if got := shortNounForms("кэш"); !slices.Contains(got, "кэшем") {
		t.Errorf("кэш: %v", got)
	}
	for _, in := range []string{"что", "мой", "соль", "кэши"} {
		if got := shortNounForms(in); got != nil {
			t.Errorf("%s: want nothing, got %v", in, got)
		}
	}
}

func TestWordFormRetryNeedsTwoWords(t *testing.T) {
	if got := WordFormRetry(t.TempDir(), []string{"миграция"}, query.Options{}, nil, nil); len(got) > 0 {
		t.Error("a single word got a retry")
	}
}

// A stem the catalog holds is not a word form: упал is not упа, данные is not
// данн (#5367 review). A noun's bare stem is.
func TestWholeFormDropsCutStems(t *testing.T) {
	for _, tc := range []struct {
		term, form string
		want       bool
	}{
		{"упал", "упа", false},
		{"данные", "данн", false},
		{"лимитами", "лимит", true},
		{"запроса", "запрос", true},
		{"лимит", "лимиты", true},
		{"упал", "упали", true},
	} {
		if got := wholeForm(tc.term, tc.form); got != tc.want {
			t.Errorf("wholeForm(%s, %s) = %v, want %v", tc.term, tc.form, got, tc.want)
		}
	}
}
