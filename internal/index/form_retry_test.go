package index

import (
	"slices"
	"testing"
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
	if _, ok := WordFormRetry(t.TempDir(), []string{"миграция"}, nil); ok {
		t.Error("a single word got a retry")
	}
}
