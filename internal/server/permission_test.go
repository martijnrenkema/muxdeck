package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func ask(s *Server, body string) chan *httptest.ResponseRecorder {
	done := make(chan *httptest.ResponseRecorder, 1)
	go func() {
		req := httptest.NewRequest(http.MethodPost, "/api/agent/permission", strings.NewReader(body))
		rec := httptest.NewRecorder()
		s.ServeHTTP(rec, req)
		done <- rec
	}()
	return done
}

func pending(t *testing.T, s *Server, session string) *permRequest {
	t.Helper()
	for i := 0; i < 100; i++ {
		if ps := s.perms.forSession(session); len(ps) > 0 {
			return ps[0]
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("request never registered")
	return nil
}

func answer(s *Server, id, decision string) int {
	req := httptest.NewRequest(http.MethodPost, "/api/agent/permission/"+id, strings.NewReader(`{"decision":"`+decision+`"}`))
	rec := httptest.NewRecorder()
	s.ServeHTTP(rec, req)
	return rec.Code
}

func decisionOf(t *testing.T, rec *httptest.ResponseRecorder) string {
	t.Helper()
	var out map[string]string
	json.Unmarshal(rec.Body.Bytes(), &out)
	return out["decision"]
}

func TestPermissionAnsweredFromUI(t *testing.T) {
	s := newTestServer(t)
	done := ask(s, `{"session":"work","tool_name":"Bash","tool_input":{"command":"rm -rf build","description":"Clean build"},"can_always":true}`)
	p := pending(t, s, "work")
	if p.Summary != "Clean build" || p.Detail != "rm -rf build" || !p.CanAlways {
		t.Fatalf("card %+v", p)
	}
	if code := answer(s, p.ID, "always"); code != http.StatusNoContent {
		t.Fatalf("answer: %d", code)
	}
	rec := <-done
	if d := decisionOf(t, rec); d != "always" {
		t.Fatalf("decision %q", d)
	}
	if len(s.perms.forSession("work")) != 0 {
		t.Fatal("answered request still listed")
	}
	if code := answer(s, p.ID, "allow"); code != http.StatusNotFound {
		t.Fatalf("late answer: %d", code)
	}
}

func TestPermissionTimesOutToTerminal(t *testing.T) {
	s := newTestServer(t)
	done := ask(s, `{"session":"work","tool_name":"Write","tool_input":{"file_path":"/tmp/x"},"wait":1}`)
	if p := pending(t, s, "work"); p.Summary != "/tmp/x" {
		t.Fatalf("summary %q", p.Summary)
	}
	select {
	case rec := <-done:
		if d := decisionOf(t, rec); d != "terminal" {
			t.Fatalf("decision %q", d)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("wait did not run out")
	}
}

func TestPermissionRejectsBadInput(t *testing.T) {
	s := newTestServer(t)
	for _, body := range []string{`{}`, `{"session":"a b","tool_name":"Bash"}`, `{"session":"x"}`} {
		rec := <-ask(s, body)
		if rec.Code == http.StatusOK {
			t.Fatalf("accepted %s", body)
		}
	}
	if code := answer(s, "nope", "yes"); code != http.StatusBadRequest {
		t.Fatalf("bad decision: %d", code)
	}
}

func TestChangesWakesOnPermission(t *testing.T) {
	s := newTestServer(t)
	v, _ := s.changes.wait(^uint64(0))
	woke := make(chan struct{})
	go func() {
		req := httptest.NewRequest(http.MethodGet, "/api/changes?since="+itoa(v), nil)
		s.ServeHTTP(httptest.NewRecorder(), req)
		close(woke)
	}()
	time.Sleep(50 * time.Millisecond)
	ask(s, `{"session":"w","tool_name":"Bash","tool_input":{},"wait":1}`)
	select {
	case <-woke:
	case <-time.After(2 * time.Second):
		t.Fatal("long poll did not wake")
	}
}

func TestWatchedSessionExpires(t *testing.T) {
	s := newTestServer(t)
	s.watch.set("w", "v1", true)
	if !s.watched("w") {
		t.Fatal("not watched")
	}
	s.watch.set("w", "v1", false)
	if s.watched("w") {
		t.Fatal("still watched after inactive")
	}
}

func itoa(v uint64) string { b, _ := json.Marshal(v); return string(b) }
