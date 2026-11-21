package server

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestQuietHoursOverMidnight(t *testing.T) {
	st := defaultSettings()
	st.Notify.Quiet = true
	at := func(h, m int) time.Time { return time.Date(2026, 10, 28, h, m, 0, 0, time.Local) }
	for _, c := range []struct {
		h, m  int
		quiet bool
	}{{23, 0, true}, {2, 30, true}, {6, 59, true}, {7, 0, false}, {12, 0, false}, {22, 59, false}} {
		if got := st.quietNow(at(c.h, c.m)); got != c.quiet {
			t.Errorf("%02d:%02d quiet=%v, want %v", c.h, c.m, got, c.quiet)
		}
	}
	st.Notify.QuietFrom, st.Notify.QuietTo = "13:00", "14:00"
	if !st.quietNow(at(13, 30)) || st.quietNow(at(14, 0)) {
		t.Error("same-day window wrong")
	}
	st.Notify.Quiet = false
	if st.quietNow(at(13, 30)) {
		t.Error("quiet while switched off")
	}
}

func putSettings(s *Server, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPut, "/api/settings", strings.NewReader(body))
	rec := httptest.NewRecorder()
	s.ServeHTTP(rec, req)
	return rec
}

func TestSettingsValidateAndPersist(t *testing.T) {
	s := newTestServer(t)
	path := filepath.Join(t.TempDir(), "settings.json")
	s.SetSettingsPath(path)
	for _, bad := range []string{
		`{"perms":{"enabled":true,"wait":500}}`,
		`{"notify":{"quiet_from":"25:00"}}`,
		`{"new_session":{"dir":"/does/not/exist"}}`,
		`{"new_session":{"command":"a\nb"}}`,
		`{"dashboard":{"url":"javascript:alert(1)"}}`,
		`{"dashboard":{"url":"/relative"}}`,
	} {
		if rec := putSettings(s, bad); rec.Code != http.StatusBadRequest {
			t.Errorf("accepted %s: %d", bad, rec.Code)
		}
	}
	if rec := putSettings(s, `{"perms":{"enabled":false,"wait":60},"new_session":{"dir":"~","command":"claude"},"dashboard":{"url":" http://box:9000 ","label":"Status"}}`); rec.Code != http.StatusOK {
		t.Fatalf("good settings: %d %s", rec.Code, rec.Body)
	}
	fresh := newTestServer(t)
	fresh.SetSettingsPath(path)
	got := fresh.settings.get()
	if got.Perms.Enabled || got.Perms.Wait != 60 || got.NewSession.Command != "claude" || !got.Notify.Done || got.Dashboard.URL != "http://box:9000" {
		t.Fatalf("reloaded %+v", got)
	}
}

func TestPermissionsOffAnswersTerminalAtOnce(t *testing.T) {
	s := newTestServer(t)
	putSettings(s, `{"perms":{"enabled":false,"wait":60}}`)
	select {
	case rec := <-ask(s, `{"session":"w","tool_name":"Bash","tool_input":{}}`):
		if d := decisionOf(t, rec); d != "terminal" {
			t.Fatalf("decision %q", d)
		}
	case <-time.After(time.Second):
		t.Fatal("waited although permission cards are off")
	}
	if len(s.perms.forSession("w")) != 0 {
		t.Fatal("card shown while off")
	}
}

func TestDeviceLabel(t *testing.T) {
	ua := "Mozilla/5.0 (Linux; Android 15; SM-F956B) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/141.0 Mobile Safari/537.36"
	if got := deviceLabel(ua); got != "Android · Chrome" {
		t.Fatalf("label %q", got)
	}
}
