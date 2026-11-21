package server

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/josecarlosrivas/muxdeck/internal/push"
	"github.com/josecarlosrivas/muxdeck/internal/tmux"
)

// A session can ask to be told when it goes quiet: a build, a deploy, a
// long job over SSH — anything without an agent to say "done". Once it has
// printed something by itself (not the echo of typing) and then stays
// silent for quietAfter, a push goes out with the last line on its screen.
// The flag is a tmux option on the session (@muxdeck-quiet), so it lasts as
// long as the session and survives a daemon restart.

const (
	quietAfter = 20 * time.Second
	// Output this soon after a key press is taken for the echo of typing.
	quietEcho = 3 * time.Second
)

type quietState struct {
	output int64 // last output seen (Unix s)
	busy   bool  // printed by itself since the last alert
}

type quietWatch struct {
	mu sync.Mutex
	m  map[string]*quietState
}

func (s *Server) StartQuietWatch() {
	go func() {
		for {
			time.Sleep(5 * time.Second)
			if tmux.Found {
				s.quietTick(time.Now())
			}
		}
	}()
}

func (s *Server) quietTick(now time.Time) {
	sessions, err := tmux.List()
	if err != nil {
		return
	}
	q := &s.quiet
	q.mu.Lock()
	if q.m == nil {
		q.m = map[string]*quietState{}
	}
	var alert []tmux.Session
	seen := map[string]bool{}
	for _, sess := range sessions {
		if !sess.Quiet {
			continue
		}
		seen[sess.Name] = true
		st := q.m[sess.Name]
		if st == nil {
			// Armed just now: count from here, not from old output.
			q.m[sess.Name] = &quietState{output: sess.Output}
			continue
		}
		if sess.Output > st.output {
			st.output = sess.Output
			if sess.Output-sess.Activity >= int64(quietEcho/time.Second) {
				st.busy = true
			}
		}
		if st.busy && now.Sub(time.Unix(st.output, 0)) >= quietAfter {
			st.busy = false
			alert = append(alert, sess)
		}
	}
	for name := range q.m {
		if !seen[name] {
			delete(q.m, name)
		}
	}
	q.mu.Unlock()
	for _, sess := range alert {
		s.notifyQuiet(sess.Name, now)
	}
}

func (s *Server) notifyQuiet(name string, now time.Time) {
	if s.watched(name) || s.settings.get().quietNow(now) {
		return
	}
	body := "No new output for a while"
	if screen, err := tmux.Screen(name); err == nil {
		if line := lastLine(screen); line != "" {
			body = line
		}
	}
	log.Printf("notify: %s quiet: %s", name, body)
	s.notify(push.Message{Title: name + ": quiet now", Body: body, Session: name, Tag: "muxdeck-quiet-" + name})
}

// lastLine is the last line of a screen with text on it, trimmed to what
// fits a notification. An empty shell prompt at the bottom says nothing, so
// the line above it is taken instead.
func lastLine(screen string) string {
	var lines []string
	for _, l := range strings.Split(screen, "\n") {
		if l = strings.TrimSpace(l); l != "" {
			lines = append(lines, l)
		}
	}
	if n := len(lines); n > 1 && strings.ContainsAny(lines[n-1][len(lines[n-1])-1:], "$#%>") {
		lines = lines[:n-1]
	}
	if len(lines) == 0 {
		return ""
	}
	return truncateRunes(lines[len(lines)-1], 160)
}

func (s *Server) handleQuiet(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Enabled bool `json:"enabled"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1024)).Decode(&body); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	if err := tmux.SetQuiet(r.PathValue("name"), body.Enabled); err != nil {
		writeErr(w, err)
		return
	}
	s.changes.bump()
	writeJSON(w, http.StatusOK, map[string]bool{"enabled": body.Enabled})
}
