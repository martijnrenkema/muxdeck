package server

import (
	"net/http/httptest"
	"testing"
)

func TestCleanPrompt(t *testing.T) {
	in := "a\r\nb\x1b[201~c\x03d\te\x7f\u0085f"
	if got, want := cleanPrompt(in), "a\nb[201~cd\tef"; got != want {
		t.Fatalf("cleanPrompt = %q, want %q", got, want)
	}
}

func TestDialogOpenCodex(t *testing.T) {
	if !dialogOpen("…\n  Would you like to run the following command?\n  $ rm -rf x\n Press enter to confirm or esc to cancel\n") {
		t.Fatal("Codex approval not seen as a dialog")
	}
	if dialogOpen("❯ \n  ? for shortcuts\n") {
		t.Fatal("an empty prompt is no dialog")
	}
}

func TestForeignWrite(t *testing.T) {
	cases := []struct {
		method, site, origin string
		want                 bool
	}{
		{"POST", "same-origin", "", false},
		{"POST", "same-site", "", true},
		{"POST", "cross-site", "", true},
		{"GET", "cross-site", "", false},
		{"POST", "", "https://other.example.ts.net", true},
		{"POST", "", "https://muxdeck.test", false},
		{"POST", "", "", false}, // a script with the bearer token
	}
	for _, c := range cases {
		r := httptest.NewRequest(c.method, "https://muxdeck.test/api/push", nil)
		if c.site != "" {
			r.Header.Set("Sec-Fetch-Site", c.site)
		}
		if c.origin != "" {
			r.Header.Set("Origin", c.origin)
		}
		if got := foreignWrite(r); got != c.want {
			t.Errorf("%s site=%q origin=%q: foreignWrite = %v, want %v", c.method, c.site, c.origin, got, c.want)
		}
	}
}

func TestPermAnswerAfterFinish(t *testing.T) {
	var ps permStore
	p := &permRequest{ID: "x", session: "s", answer: make(chan permAnswer, 1)}
	ps.add(p)
	ps.finish("x")
	if exists, ok := ps.answer("x", permAnswer{Decision: "allow"}); exists || ok {
		t.Fatalf("answer after finish: exists=%v ok=%v, want false/false", exists, ok)
	}
	q := &permRequest{ID: "y", session: "s", answer: make(chan permAnswer, 1)}
	ps.add(q)
	if _, ok := ps.answer("y", permAnswer{Decision: "deny"}); !ok {
		t.Fatal("first answer refused")
	}
	if exists, ok := ps.answer("y", permAnswer{Decision: "allow"}); !exists || ok {
		t.Fatalf("second answer: exists=%v ok=%v, want true/false", exists, ok)
	}
	if d := <-q.answer; d.Decision != "deny" {
		t.Fatalf("decision = %q, want deny", d.Decision)
	}
}
