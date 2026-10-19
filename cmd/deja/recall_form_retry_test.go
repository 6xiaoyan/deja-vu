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
	return formRetryStoreAt(t, "/work", texts)
}

// formRetryStoreAt is formRetryStore with the projects under root.
func formRetryStoreAt(t *testing.T, root string, texts map[string]string) string {
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
		cwd := filepath.Join(root, name)
		proj := filepath.Join(os.Getenv("DEJA_CLAUDE_ROOT"), strings.ReplaceAll(filepath.ToSlash(cwd), "/", "-"))
		if err := os.MkdirAll(proj, 0o700); err != nil {
			t.Fatal(err)
		}
		var body strings.Builder
		for j, line := range strings.Split(text, "\n") {
			b, err := json.Marshal(map[string]any{
				"type": "user", "sessionId": sid, "cwd": cwd,
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

var retryLine = regexp.MustCompile(`This store writes (.+) — call recall again with q="([^"]+)"`)

// noisyNames is six sessions about a migration that outrank, on wording, the
// one session that names the person.
func noisyNames(texts map[string]string) map[string]string {
	for n := 0; n < 6; n++ {
		texts[fmt.Sprintf("bbbbbbbb-0000-4000-8000-%012d", n)] = "миграция базы, миграция таблиц\nмиграция упала, миграция снова"
	}
	return texts
}

// A Russian recall that missed hands the agent a q in the spelling the store
// wrote, rather than prose about it: in 103 transcripts the CLI's word-forms
// line was acted on zero times (#5367). Here a name, which the store keeps in
// Latin letters.
func TestARussianRecallThatMissedHandsBackTheStoresSpelling(t *testing.T) {
	dir := formRetryStore(t, noisyNames(map[string]string{
		"aaaaaaaa-0000-4000-8000-00000000000a": "Kuznetsov просил перенести миграцию на пятницу",
	}))
	rec := mcpRecall(t, dir, map[string]any{"q": "Кузнецов миграция", "limit": 2})
	if strings.Contains(rec, "aaaaaaaa") {
		t.Fatalf("the fixture is meant to miss on the first call:\n%s", rec)
	}
	m := retryLine.FindStringSubmatch(rec)
	if m == nil || m[2] != "kuznetsov миграция" {
		t.Fatalf("want q=\"kuznetsov миграция\":\n%s", rec)
	}
	// Followed at the same limit, it finds the session.
	if again := mcpRecall(t, dir, map[string]any{"q": m[2], "limit": 2}); !strings.Contains(again, "aaaaaaaa") {
		t.Errorf("the handed-back q does not find the session:\n%s", again)
	}
}

// ё and е are two tokens to the index; the store wrote one, the agent the
// other. limit 1 is the case where the page holds nothing but the noise.
func TestARussianRecallHandsBackTheYoTheStoreWrote(t *testing.T) {
	texts := map[string]string{
		"aaaaaaaa-0000-4000-8000-00000000000a": "отчёт по счетам перенесли на понедельник",
	}
	for n := 0; n < 6; n++ {
		texts[fmt.Sprintf("bbbbbbbb-0000-4000-8000-%012d", n)] = "счетам и счетам, отчеты по счетам\nсчетам снова"
	}
	dir := formRetryStore(t, texts)
	rec := mcpRecall(t, dir, map[string]any{"q": "отчет по счетам", "limit": 1})
	if strings.Contains(rec, "aaaaaaaa") {
		t.Fatalf("the fixture is meant to miss on the first call:\n%s", rec)
	}
	m := retryLine.FindStringSubmatch(rec)
	if m == nil || m[2] != "отчёт по счетам" {
		t.Fatalf("want q=\"отчёт по счетам\":\n%s", rec)
	}
	if again := mcpRecall(t, dir, map[string]any{"q": m[2], "limit": 1}); !strings.Contains(again, "aaaaaaaa") {
		t.Errorf("the handed-back q does not find the session at limit 1:\n%s", again)
	}
}

// From inside a project, the recall the agent makes next answers that
// project first; the q still has to reach the session (#5367 review: from a
// project cwd most lines were false).
func TestTheRetryHoldsFromAProjectDirectory(t *testing.T) {
	root := t.TempDir()
	for _, p := range []string{"app", "other"} {
		if err := os.MkdirAll(filepath.Join(root, p), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	dir := formRetryStoreAt(t, root, noisyNames(map[string]string{
		"aaaaaaaa-0000-4000-8000-00000000000a": "Kuznetsov просил перенести миграцию на пятницу",
	}))
	t.Chdir(filepath.Join(root, "app"))
	rec := mcpRecall(t, dir, map[string]any{"q": "Кузнецов миграция", "limit": 2})
	m := retryLine.FindStringSubmatch(rec)
	if m == nil {
		t.Fatalf("no retry line from the project directory:\n%s", rec)
	}
	if again := mcpRecall(t, dir, map[string]any{"q": m[2], "limit": 2}); !strings.Contains(again, "aaaaaaaa") {
		t.Errorf("from the project directory the q does not find the session:\n%s", again)
	}
}

// An English recall pays nothing and says nothing new.
func TestAnEnglishRecallGetsNoRespelling(t *testing.T) {
	texts := map[string]string{
		"aaaaaaaa-0000-4000-8000-00000000000a": "Kuznetsov raised the request limits on the gateway",
	}
	for n := 0; n < 6; n++ {
		texts[fmt.Sprintf("bbbbbbbb-0000-4000-8000-%012d", n)] = "limit request gateway, limit request again"
	}
	dir := formRetryStore(t, texts)
	rec := mcpRecall(t, dir, map[string]any{"q": "Kuznetsov limits gateways", "limit": 1})
	if strings.Contains(rec, "call recall again with q=") {
		t.Errorf("an English query got a respelling:\n%s", rec)
	}
}

// «Мост упал» starts with a capital because it starts a sentence. Its
// romanization is an English word: most, in every English session (#5367
// review: q="most упал" pulled one in). Neither a store that writes мост in
// Russian nor one that never does gets it.
func TestASentenceInitialWordIsNotRomanized(t *testing.T) {
	for _, withRussian := range []bool{true, false} {
		texts := map[string]string{}
		for n := 0; n < 8; n++ {
			texts[fmt.Sprintf("eeeeeeee-0000-4000-8000-%012d", n)] = "most of the build steps fail, упал опять"
		}
		for n := 0; n < 6; n++ {
			texts[fmt.Sprintf("bbbbbbbb-0000-4000-8000-%012d", n)] = "сервер упал\nупал снова"
		}
		if withRussian {
			texts["cccccccc-0000-4000-8000-00000000000c"] = "мосты чинили, мостом занимались"
		}
		dir := formRetryStore(t, texts)
		rec := mcpRecall(t, dir, map[string]any{"q": "Мост упал", "limit": 1})
		if strings.Contains(rec, `"most`) {
			t.Errorf("russian мост in store %v: romanized a sentence-initial word:\n%s", withRussian, rec)
		}
	}
}

// An answer that already holds every word as asked gets no respelling: the
// first call found what was written (#5367 review).
func TestAnExactAnswerGetsNoRespelling(t *testing.T) {
	texts := map[string]string{
		"aaaaaaaa-0000-4000-8000-00000000000a": "отчёт падает трафик",
	}
	for n := 0; n < 3; n++ {
		texts[fmt.Sprintf("bbbbbbbb-0000-4000-8000-%012d", n)] = "отчет падает каждую ночь"
	}
	dir := formRetryStore(t, texts)
	rec := mcpRecall(t, dir, map[string]any{"q": "отчет падает"})
	if strings.Contains(rec, "call recall again with q=") {
		t.Errorf("an exact answer got a respelling:\n%s", rec)
	}
}

// Only a whole word goes into the q: the review found "описание репозитори"
// and "лог ошибок хук", stems the catalog happens to hold.
func TestARetryHoldsNoCutStem(t *testing.T) {
	texts := map[string]string{
		"aaaaaaaa-0000-4000-8000-00000000000a": "описание репозитория обновили\nлоги ошибок хуков чистим",
		"cccccccc-0000-4000-8000-00000000000c": "репозитори хук лог",
	}
	for n := 0; n < 6; n++ {
		texts[fmt.Sprintf("bbbbbbbb-0000-4000-8000-%012d", n)] = "описания много\nошибок много"
	}
	dir := formRetryStore(t, texts)
	for _, q := range []string{"описанию репозиториев", "логов ошибка хуком"} {
		rec := mcpRecall(t, dir, map[string]any{"q": q, "limit": 1})
		if m := retryLine.FindStringSubmatch(rec); m != nil {
			for _, w := range strings.Fields(m[2]) {
				if w == "репозитори" || w == "хук" || w == "лог" {
					t.Errorf("%q: q holds the stem %q:\n%s", q, w, rec)
				}
			}
		}
	}
}

// The project filter holds for the respelling too: a name only another
// project holds is no retry for this one (#5367 review).
func TestTheRetryKeepsTheProjectFilter(t *testing.T) {
	texts := map[string]string{
		"other/aaaaaaaa-0000-4000-8000-00000000000a": "Ivanov просил выкатить релиз в пятницу",
	}
	for n := 0; n < 4; n++ {
		texts[fmt.Sprintf("bbbbbbbb-0000-4000-8000-%012d", n)] = "релиз собрали, релиз выкатили"
	}
	dir := formRetryStore(t, texts)
	rec := mcpRecall(t, dir, map[string]any{"q": "релиз Иванов", "project": "app"})
	if strings.Contains(rec, "call recall again with q=") {
		t.Errorf("handed back a q only another project answers:\n%s", rec)
	}
	// The control: without the filter the same q is the right one.
	rec = mcpRecall(t, dir, map[string]any{"q": "релиз Иванов", "limit": 1})
	if !strings.Contains(rec, `q="релиз ivanov"`) {
		t.Errorf("no retry without the filter either, so the test proves nothing:\n%s", rec)
	}
}
