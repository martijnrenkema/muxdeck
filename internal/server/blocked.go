package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/josecarlosrivas/muxdeck/internal/push"
	"github.com/josecarlosrivas/muxdeck/internal/subagent"
	"github.com/josecarlosrivas/muxdeck/internal/tmux"
)

// When auto mode blocks a tool call, Claude Code tells nobody but the
// terminal: the session just looks "working" or "done". The PermissionDenied
// hook reports the block here, so it gets a card, a mark in the sidebar and
// a push. "Allow once" types an approval that names the exact action into
// the session: auto mode reads an approval stated in the conversation as
// clearing that one block (permission-modes docs, "Approvals you state in
// conversation"). It is typed, not sent to Claude's inbox, because it has to
// be the user's own message. A block stays until it is answered, the user
// sends the session anything else, or the agent exits.

type blockedAction struct {
	ID      string    `json:"id"`
	Agent   string    `json:"agent"`
	Tool    string    `json:"tool"`
	Summary string    `json:"summary"`
	Detail  string    `json:"detail,omitempty"`
	Reason  string    `json:"reason"`
	Created time.Time `json:"created"`

	session  string
	approval string
}

const (
	blockKeep       = time.Hour
	maxBlocks       = 5
	maxApprovalBody = 4000
)

type blockStore struct {
	mu sync.Mutex
	m  map[string]*blockedAction
}

func (bs *blockStore) add(b *blockedAction) {
	bs.mu.Lock()
	defer bs.mu.Unlock()
	if bs.m == nil {
		bs.m = map[string]*blockedAction{}
	}
	var same []*blockedAction
	for _, x := range bs.m {
		if x.session == b.session {
			same = append(same, x)
		}
	}
	for len(same) >= maxBlocks {
		oldest := 0
		for i := range same {
			if same[i].Created.Before(same[oldest].Created) {
				oldest = i
			}
		}
		delete(bs.m, same[oldest].ID)
		same = append(same[:oldest], same[oldest+1:]...)
	}
	bs.m[b.ID] = b
}

func (bs *blockStore) forSession(session string) []*blockedAction {
	bs.mu.Lock()
	defer bs.mu.Unlock()
	var out []*blockedAction
	for id, b := range bs.m {
		if time.Since(b.Created) > blockKeep {
			delete(bs.m, id)
			continue
		}
		if b.session == session {
			out = append(out, b)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Created.Before(out[j].Created) })
	return out
}

func (bs *blockStore) take(id string) *blockedAction {
	bs.mu.Lock()
	defer bs.mu.Unlock()
	b := bs.m[id]
	delete(bs.m, id)
	return b
}

func (bs *blockStore) put(b *blockedAction) {
	bs.mu.Lock()
	defer bs.mu.Unlock()
	bs.m[b.ID] = b
}

// clear drops a session's blocks; true if there were any.
func (bs *blockStore) clear(session string) bool {
	bs.mu.Lock()
	defer bs.mu.Unlock()
	n := 0
	for id, b := range bs.m {
		if b.session == session {
			delete(bs.m, id)
			n++
		}
	}
	return n > 0
}

// approvalText is what "Allow once" types: the action named with its
// specifics, so the approval covers exactly that.
func approvalText(tool, reason string, input json.RawMessage) string {
	var in map[string]any
	_ = json.Unmarshal(input, &in)
	str := func(k string) string { v, _ := in[k].(string); return v }
	because := ""
	if reason != "" {
		because = " (auto mode blocked it: " + reason + ")"
	}
	switch tool {
	case "Bash":
		return "Approved from muxdeck, once: you may run this exact command" + because + ":\n\n" +
			truncateRunes(str("command"), maxApprovalBody)
	case "Edit", "Write", "MultiEdit", "Read", "NotebookEdit":
		p := str("file_path")
		if p == "" {
			p = str("notebook_path")
		}
		return fmt.Sprintf("Approved from muxdeck, once: you may use %s on %s%s.", tool, p, because)
	case "WebFetch":
		return fmt.Sprintf("Approved from muxdeck, once: you may fetch %s%s.", str("url"), because)
	}
	raw, _ := json.Marshal(in)
	return fmt.Sprintf("Approved from muxdeck, once: you may make this %s call%s:\n\n%s", tool, because,
		truncateRunes(string(raw), maxApprovalBody))
}

func (s *Server) handleBlockedReport(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Session string          `json:"session"`
		Agent   string          `json:"agent"`
		Tool    string          `json:"tool_name"`
		Input   json.RawMessage `json:"tool_input"`
		Reason  string          `json:"reason"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&body); err != nil || body.Tool == "" {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	if !tmux.ValidName(body.Session) {
		writeErr(w, tmux.ErrBadName)
		return
	}
	summary, detail := summarizeTool(body.Tool, body.Input)
	reason := truncateRunes(strings.TrimSpace(body.Reason), 200)
	b := &blockedAction{
		ID: newID(), Agent: body.Agent, Tool: truncateRunes(body.Tool, 60),
		Summary: summary, Detail: detail, Reason: reason,
		Created: time.Now(), session: body.Session,
		approval: approvalText(body.Tool, reason, body.Input),
	}
	s.blocks.add(b)
	s.changes.bump()
	log.Printf("blocked: %s: %s %s", body.Session, b.Reason, b.Tool)
	st := s.settings.get()
	if st.Notify.Permission && !s.watched(body.Session) {
		text := b.Tool
		if b.Summary != "" {
			text += ": " + b.Summary
		}
		if b.Reason != "" {
			text = b.Reason + " · " + text
		}
		s.notify(push.Message{
			Title: body.Session + ": blocked by auto mode", Body: text,
			Session: body.Session, Tag: "muxdeck-agent-" + body.Session,
			Block: b.ID, Actions: []string{"block-allow", "block-dismiss"}, Titles: []string{"Allow once", "Leave it"},
		})
	}
	w.WriteHeader(http.StatusNoContent)
}

// handleBlockedAnswer: {"decision": "allow"} types the approval (queued
// when the session can't take it right now), "dismiss" just drops the card.
func (s *Server) handleBlockedAnswer(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Decision string `json:"decision"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1024)).Decode(&body); err != nil ||
		(body.Decision != "allow" && body.Decision != "dismiss") {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	b := s.blocks.take(r.PathValue("id"))
	if b == nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "that block was already handled"})
		return
	}
	s.changes.bump()
	if body.Decision == "dismiss" {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	log.Printf("blocked: %s: allowed once from muxdeck", b.session)
	if typing(b.session) || s.prompts.len(b.session) > 0 {
		if _, err := s.agentPane(b.session); err != nil {
			s.blocks.put(b)
			s.promptError(w, err)
			return
		}
		s.enqueuePrompt(b.session, b.approval, "")
		writeJSON(w, http.StatusAccepted, map[string]any{"queued": true, "reason": "sent once you stop typing there"})
		return
	}
	err := s.deliverPrompt(b.session, b.approval)
	var ref *promptRefusal
	if errors.As(err, &ref) && (ref.Code == refQuestion || ref.Code == refCopyMode) {
		s.enqueuePrompt(b.session, b.approval, "")
		writeJSON(w, http.StatusAccepted, map[string]any{"queued": true, "reason": ref.Msg})
		return
	}
	if err != nil {
		s.blocks.put(b)
		s.promptError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"sent": true})
}

// --- background task output ---

var taskOutRe = regexp.MustCompile(`^[A-Za-z0-9_-]{4,40}$`)
var ansiRe = regexp.MustCompile(`\x1b(\[[0-9;?]*[ -/]*[@-~]|\][^\x07\x1b]*(\x07|\x1b\\)|[@-Z\\-_])`)

const taskOutTail = 16 << 10

// handleTaskOutput returns a background task and the tail of its output.
func (s *Server) handleTaskOutput(w http.ResponseWriter, r *http.Request) {
	name, id := r.PathValue("name"), r.PathValue("id")
	if !tmux.ValidName(name) || !taskOutRe.MatchString(id) {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	task, path, ok := s.subs.TaskOutput(name, id)
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "no such task"})
		return
	}
	root := subagent.TaskOutputDir() + string(filepath.Separator)
	if path == "" {
		if m, _ := filepath.Glob(filepath.Join(root, "*", "*", "tasks", id+".output")); len(m) == 1 {
			path = m[0]
		}
	}
	path = filepath.Clean(path)
	out := map[string]any{"task": task}
	if !strings.HasPrefix(path, root) || filepath.Base(path) != id+".output" {
		out["missing"] = true
		writeJSON(w, http.StatusOK, out)
		return
	}
	f, err := os.Open(path)
	if err != nil {
		out["missing"] = true
		writeJSON(w, http.StatusOK, out)
		return
	}
	defer f.Close()
	fi, err := f.Stat()
	if err != nil || !fi.Mode().IsRegular() {
		out["missing"] = true
		writeJSON(w, http.StatusOK, out)
		return
	}
	start := max(fi.Size()-taskOutTail, 0)
	buf := make([]byte, fi.Size()-start)
	n, _ := f.ReadAt(buf, start)
	text := cleanOutput(string(buf[:n]))
	if start > 0 {
		if i := strings.IndexByte(text, '\n'); i >= 0 {
			text = text[i+1:]
		}
	}
	out["output"] = text
	out["truncated"] = start > 0
	writeJSON(w, http.StatusOK, out)
}

// cleanOutput drops terminal escapes and keeps what a carriage return left
// on each line (progress bars).
func cleanOutput(s string) string {
	s = ansiRe.ReplaceAllString(s, "")
	lines := strings.Split(s, "\n")
	for i, l := range lines {
		l = strings.TrimRight(l, "\r")
		if j := strings.LastIndexByte(l, '\r'); j >= 0 {
			l = l[j+1:]
		}
		lines[i] = l
	}
	return strings.ToValidUTF8(strings.Join(lines, "\n"), "")
}
