package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"
)

// formRetryStore writes one claude session per text and returns the index
// dir. A key "other/<id>" files the session under the project "other"; the
// rest go under "app". Each line of a text is its own message.
func formRetryStore(t *testing.T, texts map[string]string) string {
	t.Helper()
	hermeticEnv(t)
	dir := filepath.Join(t.TempDir(), "idx")
	t.Setenv("DEJA_INDEX_DIR", dir)
	at := time.Now().Add(-48 * time.Hour).UTC()
	i := 0
	for key, text := range texts {
		i++
		name, sid := "app", key
		if p, rest, ok := strings.Cut(key, "/"); ok {
			name, sid = p, rest
		}
		proj := filepath.Join(os.Getenv("DEJA_CLAUDE_ROOT"), "-work-"+name)
		if err := os.MkdirAll(proj, 0o700); err != nil {
			t.Fatal(err)
		}
		var body strings.Builder
		for j, line := range strings.Split(text, "\n") {
			b, err := json.Marshal(map[string]any{
				"type": "user", "sessionId": sid, "cwd": "/work/" + name,
				"timestamp": at.Add(time.Duration(i)*time.Hour + time.Duration(j)*time.Minute).Format(time.RFC3339),
				"message":   map[string]any{"role": "user", "content": line},
			})
			if err != nil {
				t.Fatal(err)
			}
			body.Write(b)
			body.WriteByte('\n')
		}
		writeFile(t, filepath.Join(proj, sid+".jsonl"), body.String())
	}
	return dir
}

// mcpRecall calls recall the way an agent does.
func mcpRecall(t *testing.T, dir string, args map[string]any) string {
	t.Helper()
	args["mode"] = "recall"
	raw, err := json.Marshal(args)
	if err != nil {
		t.Fatal(err)
	}
	out, err := callMCPTool(dir, "deja", raw)
	if err != nil {
		t.Fatal(err)
	}
	return out
}

var retryLine = regexp.MustCompile(`(\d+) sessions? holds? every word — call recall again with q="([^"]+)"`)

// A Russian recall that missed hands the agent a q in the spelling the store
// wrote — another case, the other alphabet — rather than nothing: in 103
// transcripts the CLI's prose about word forms was acted on zero times
// (#5367).
func TestARussianRecallThatMissedHandsBackTheStoresSpelling(t *testing.T) {
	texts := map[string]string{
		"aaaaaaaa-0000-4000-8000-00000000000a": "Kuznetsov просил перенести миграцию на пятницу",
	}
	// Sessions about миграция that outrank the one above on wording alone.
	for n := 0; n < 6; n++ {
		texts[fmt.Sprintf("bbbbbbbb-0000-4000-8000-%012d", n)] = "миграция базы, миграция таблиц\nмиграция упала, миграция снова"
	}
	dir := formRetryStore(t, texts)
	rec, err := callMCPTool(dir, "deja", json.RawMessage(`{"mode":"recall","q":"Кузнецов миграция","limit":2}`))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(rec, "aaaaaaaa") {
		t.Fatalf("the fixture is meant to miss on the first call:\n%s", rec)
	}
	want := `call recall again with q="kuznetsov миграцию"`
	if !strings.Contains(rec, want) {
		t.Fatalf("no ready query in the store's spelling, want %s:\n%s", want, rec)
	}
	// And the q it hands back is one recall answers with the session.
	again, err := callMCPTool(dir, "deja", json.RawMessage(`{"mode":"recall","q":"kuznetsov миграцию","limit":2}`))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(again, "aaaaaaaa") {
		t.Errorf("the handed-back q does not find the session:\n%s", again)
	}
}

// The other case of a word is a spelling too: «лимиты запросов к шлюзу» is
// what the store wrote, «лимит запроса шлюз» is what an agent asks.
func TestARussianRecallHandsBackTheCaseTheStoreWrote(t *testing.T) {
	texts := map[string]string{
		"aaaaaaaa-0000-4000-8000-00000000000a": "лимиты запросов к шлюзу подняли до 500 в секунду",
	}
	for n := 0; n < 6; n++ {
		texts[fmt.Sprintf("bbbbbbbb-0000-4000-8000-%012d", n)] = "лимит запроса и шлюз, лимит запроса снова\nшлюз лимит запроса"
	}
	dir := formRetryStore(t, texts)
	for _, tc := range []struct{ q, want string }{
		{"лимиты запроса шлюзов", `q="лимиты запросов шлюзу"`},
	} {
		raw, _ := json.Marshal(map[string]any{"mode": "recall", "q": tc.q, "limit": 1})
		rec, err := callMCPTool(dir, "deja", raw)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(rec, "aaaaaaaa") {
			t.Fatalf("%q: the fixture is meant to miss on the first call:\n%s", tc.q, rec)
		}
		if !strings.Contains(rec, tc.want) {
			t.Errorf("%q: want %s:\n%s", tc.q, tc.want, rec)
		}
	}
}

// An English recall pays nothing and says nothing new.
func TestAnEnglishRecallGetsNoRespelling(t *testing.T) {
	texts := map[string]string{
		"aaaaaaaa-0000-4000-8000-00000000000a": "raised the request limits on the gateway to 500",
	}
	for n := 0; n < 6; n++ {
		texts[fmt.Sprintf("bbbbbbbb-0000-4000-8000-%012d", n)] = "limit request gateway, limit request again"
	}
	dir := formRetryStore(t, texts)
	rec, err := callMCPTool(dir, "deja", json.RawMessage(`{"mode":"recall","q":"limit requests gateways","limit":1}`))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(rec, "call recall again with q=") {
		t.Errorf("an English query got a respelling:\n%s", rec)
	}
}

// A Russian word the store writes in Russian is not a name: its romanization
// is an English word (мост is most), and handing the agent "most падает"
// pulled English sessions into a Russian question (#5367 review).
func TestARussianWordTheStoreHoldsIsNotRomanized(t *testing.T) {
	texts := map[string]string{
		"aaaaaaaa-0000-4000-8000-00000000000a": "most of the build steps fail, падает сборка",
		"cccccccc-0000-4000-8000-00000000000c": "мосты падают ночью под нагрузкой",
		"dddddddd-0000-4000-8000-00000000000d": "мосты падают снова, мосты чинили",
	}
	for n := 0; n < 6; n++ {
		texts[fmt.Sprintf("bbbbbbbb-0000-4000-8000-%012d", n)] = "мост\nпадает"
	}
	dir := formRetryStore(t, texts)
	rec := mcpRecall(t, dir, map[string]any{"q": "мост падает", "limit": 1})
	if strings.Contains(rec, `q="most`) {
		t.Errorf("romanized a word the store writes in Russian:\n%s", rec)
	}
}

// The count on the line is what recall finds with that q: words in one
// message, not anywhere in the session (#5367 review: 11 claimed, 2 found).
func TestTheRetryCountIsWhatTheSearchFinds(t *testing.T) {
	texts := map[string]string{
		"aaaaaaaa-0000-4000-8000-00000000000a": "лимиты токенов подняли вдвое",
	}
	// Both words, never in one message.
	for n := 0; n < 4; n++ {
		texts[fmt.Sprintf("cccccccc-0000-4000-8000-%012d", n)] = "лимиты обсуждали\nтокенов не хватило"
	}
	for n := 0; n < 6; n++ {
		texts[fmt.Sprintf("bbbbbbbb-0000-4000-8000-%012d", n)] = "лимит токен лимит\nлимит"
	}
	dir := formRetryStore(t, texts)
	rec := mcpRecall(t, dir, map[string]any{"q": "лимитами токена", "limit": 1})
	m := retryLine.FindStringSubmatch(rec)
	if m == nil {
		t.Fatalf("no retry line:\n%s", rec)
	}
	if m[1] != "1" || m[2] != "лимиты токенов" {
		t.Errorf("claims %s sessions for %q, the search finds 1 for лимиты токенов:\n%s", m[1], m[2], rec)
	}
	// And recall with that q says the same.
	again := mcpRecall(t, dir, map[string]any{"q": m[2], "limit": 20})
	if !strings.Contains(again, `deja recall for "лимиты токенов" (1 match)`) && !strings.Contains(again, "1 session below holds every word") {
		t.Errorf("the line claimed %s, recall with q=%q says:\n%s", m[1], m[2], again)
	}
}

// An answer that already holds every word as asked gets no respelling: the
// first call found what was written (#5367 review: мост падает with three
// exact sessions was sent to мостом падает).
func TestAnExactAnswerGetsNoRespelling(t *testing.T) {
	texts := map[string]string{
		"aaaaaaaa-0000-4000-8000-00000000000a": "мостом падает трафик",
	}
	for n := 0; n < 3; n++ {
		texts[fmt.Sprintf("bbbbbbbb-0000-4000-8000-%012d", n)] = "мост падает каждую ночь"
	}
	dir := formRetryStore(t, texts)
	rec := mcpRecall(t, dir, map[string]any{"q": "мост падает"})
	if strings.Contains(rec, "call recall again with q=") {
		t.Errorf("an exact answer got a respelling:\n%s", rec)
	}
}

// No word twice: тестов тестов is not a spelling of two words (#5367
// review: "тесту тесту тестом тестов").
func TestARetryNeverRepeatsAWord(t *testing.T) {
	texts := map[string]string{
		"aaaaaaaa-0000-4000-8000-00000000000a": "тестов много",
	}
	for n := 0; n < 6; n++ {
		texts[fmt.Sprintf("bbbbbbbb-0000-4000-8000-%012d", n)] = "тест\nтесты"
	}
	dir := formRetryStore(t, texts)
	rec := mcpRecall(t, dir, map[string]any{"q": "тест тесты", "limit": 1})
	if m := retryLine.FindStringSubmatch(rec); m != nil {
		seen := map[string]bool{}
		for _, w := range strings.Fields(m[2]) {
			if seen[w] {
				t.Errorf("q repeats %q:\n%s", w, rec)
			}
			seen[w] = true
		}
	}
}

// The project filter holds for the respelling too: a q that only another
// project answers is no retry for this one (#5367 review: Иванов релиз,
// project=app, got ivanov релиз from elsewhere).
func TestTheRetryKeepsTheProjectFilter(t *testing.T) {
	texts := map[string]string{
		"other/aaaaaaaa-0000-4000-8000-00000000000a": "Ivanov просил выкатить релиз в пятницу",
	}
	for n := 0; n < 4; n++ {
		texts[fmt.Sprintf("bbbbbbbb-0000-4000-8000-%012d", n)] = "релиз собрали, релиз выкатили"
	}
	dir := formRetryStore(t, texts)
	rec := mcpRecall(t, dir, map[string]any{"q": "Иванов релиз", "project": "app"})
	if strings.Contains(rec, "call recall again with q=") {
		t.Errorf("handed back a q only another project answers:\n%s", rec)
	}
	// The control: without the filter the same q is the right one.
	rec = mcpRecall(t, dir, map[string]any{"q": "Иванов релиз", "limit": 1})
	if !strings.Contains(rec, `q="ivanov релиз"`) {
		t.Errorf("no retry without the filter either, so the test proves nothing:\n%s", rec)
	}
}
