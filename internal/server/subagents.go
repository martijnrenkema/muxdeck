package server

import (
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/josecarlosrivas/muxdeck/internal/agent"
	"github.com/josecarlosrivas/muxdeck/internal/subagent"
	"github.com/josecarlosrivas/muxdeck/internal/tmux"
)

func newSubagentTracker() *subagent.Tracker {
	home, _ := os.UserHomeDir()
	return subagent.New(home)
}

// watchSubagents rescans the tracked transcripts and wakes /api/changes
// when a subagent appears, finishes or drops off, so the sidebar follows
// without waiting for its next poll.
func (s *Server) watchSubagents() {
	t := time.NewTicker(3 * time.Second)
	defer t.Stop()
	for range t.C {
		if tmux.Found {
			s.statusFromRegistry(s.subs.DiscoverClaude(tmux.PanePIDs()))
		}
		if s.subs.Scan() {
			s.changes.bump()
		}
	}
}

// statusFromRegistry gives a Claude Code session whose hooks have not
// reported yet (the daemon restarted while it sat idle or worked in the
// background) its state from Claude's own registry, so its row shows the
// agent instead of a bare shell. A report from the agent's hooks takes over
// for good; this never notifies.
func (s *Server) statusFromRegistry(live []subagent.Live) {
	s.regMu.Lock()
	defer s.regMu.Unlock()
	if s.regOwned == nil {
		s.regOwned = map[string]bool{}
	}
	s.regPanes = map[string]string{}
	s.regLive = map[string]subagent.Live{}
	for _, l := range live {
		if l.Pane != "" {
			s.regPanes[l.Session] = l.Pane
		}
		s.regLive[l.Session] = l
	}
	changed := false
	for _, l := range live {
		prev, had := s.agents.Get(l.Session)
		if had && !s.regOwned[l.Session] {
			continue
		}
		st := agent.Status{Agent: "claude-code", State: agent.StateWaiting, Note: "Claude is done and waiting for you"}
		if l.Busy {
			st = agent.Status{Agent: "claude-code", State: agent.StateWorking}
		}
		if had && prev.State == st.State {
			continue
		}
		s.agents.Set(l.Session, st)
		s.regOwned[l.Session] = true
		changed = true
	}
	if changed {
		s.changes.bump()
	}
}

// registryPane is the pane Claude's registry says the session's Claude
// runs in, or "".
func (s *Server) registryPane(session string) string {
	s.regMu.Lock()
	defer s.regMu.Unlock()
	return s.regPanes[session]
}

// reportedByAgent marks a session's status as the agent's own from now on.
func (s *Server) reportedByAgent(session string) {
	s.regMu.Lock()
	delete(s.regOwned, session)
	s.regMu.Unlock()
}

// handleSubagent returns a subagent's transcript from ?after= on, as
// display events. The client polls with the returned next offset.
func (s *Server) handleSubagent(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if !tmux.ValidName(name) {
		writeErr(w, tmux.ErrBadName)
		return
	}
	after, _ := strconv.ParseInt(r.URL.Query().Get("after"), 10, 64)
	tx, err := s.subs.Events(name, r.PathValue("id"), after)
	if errors.Is(err, subagent.ErrNotFound) {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "subagent not found"})
		return
	}
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, tx)
}

// handleSubagentMessage passes a message from the user to a subagent. It
// goes to the agent that spawned it, as a prompt asking it to forward the
// text with its own tool — the only route either agent offers. A busy
// parent reads it as soon as its running tool calls finish.
func (s *Server) handleSubagentMessage(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if !tmux.ValidName(name) {
		writeErr(w, tmux.ErrBadName)
		return
	}
	var body struct {
		Text string `json:"text"`
	}
	if err := json.NewDecoder(io.LimitReader(r.Body, 64<<10)).Decode(&body); err != nil || strings.TrimSpace(body.Text) == "" {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	if len([]rune(body.Text)) > 8000 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "message too long (max 8000 characters)"})
		return
	}
	sub, ok := s.subs.Get(name, r.PathValue("id"))
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "subagent not found"})
		return
	}
	// A parent that stopped can still be running (after /clear, say): what
	// counts is whether an agent is there to take the prompt.
	if err := s.deliverPrompt(name, subagent.RelayPrompt(sub, body.Text)); err != nil {
		s.promptError(w, err)
		return
	}
	log.Printf("subagent message: %s/%s (%d chars)", name, sub.ID, len(body.Text))
	w.WriteHeader(http.StatusNoContent)
}
