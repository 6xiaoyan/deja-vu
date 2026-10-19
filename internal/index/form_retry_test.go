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

func TestASurnameEndingRomanizesAsY(t *testing.T) {
	if got := otherScriptSpellings("шиманский"); !slices.Contains(got, "shimansky") {
		t.Errorf("шиманский: %v", got)
	}
}
