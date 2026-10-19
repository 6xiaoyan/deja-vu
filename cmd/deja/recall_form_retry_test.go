package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// formRetryStore writes one claude session per text and returns the index dir.
func formRetryStore(t *testing.T, texts map[string]string) string {
	t.Helper()
	hermeticEnv(t)
	dir := filepath.Join(t.TempDir(), "idx")
	t.Setenv("DEJA_INDEX_DIR", dir)
	proj := filepath.Join(os.Getenv("DEJA_CLAUDE_ROOT"), "-work-app")
	if err := os.MkdirAll(proj, 0o700); err != nil {
		t.Fatal(err)
	}
	at := time.Now().Add(-48 * time.Hour).UTC()
	i := 0
	for sid, text := range texts {
		i++
		var body strings.Builder
		for j, line := range strings.Split(text, "\n") {
			b, err := json.Marshal(map[string]any{
				"type": "user", "sessionId": sid, "cwd": "/work/app",
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
