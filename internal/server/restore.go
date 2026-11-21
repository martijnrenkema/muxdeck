package server

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/josecarlosrivas/muxdeck/internal/push"
	"github.com/josecarlosrivas/muxdeck/internal/tmux"
)

// tmux sessions do not survive a reboot of the machine (or a tmux server
// that dies). The daemon keeps a snapshot of what runs where: each
// session's name, directory and, when an agent runs in it, the agent's
// conversation id. When the tmux server it was taken from is gone, the
// sessions it had become "lost" and the sidebar offers them back: the same
// name and directory, with `claude --resume <id>` or `codex resume <id>`
// typed into the new shell.
//
// A session that ends while its server keeps running was ended on purpose
// and is simply forgotten. So is a session closed from muxdeck, and the last
// session of a server that stops by itself once it has none left (tmux's
// exit-empty, on by default): only a reboot, or a server that dies with
// sessions in it, takes sessions away.

type savedSession struct {
	Name   string    `json:"name"`
	Dir    string    `json:"dir"`
	Agent  string    `json:"agent,omitempty"`  // claude-code or codex
	Resume string    `json:"resume,omitempty"` // conversation id
	Quiet  bool      `json:"quiet,omitempty"`  // quiet alert was on
	Seen   time.Time `json:"seen"`
}

type restoreState struct {
	Server string `json:"server"` // tmux server the snapshot is from
	// Boot is the machine boot the snapshot was taken in (Linux boot_id);
	// empty where the system has none.
	Boot string `json:"boot,omitempty"`
	// KeepsEmpty: the server had exit-empty off, so it never stops by
	// itself and its end always takes the sessions it had.
	KeepsEmpty bool           `json:"keeps_empty,omitempty"`
	Sessions   []savedSession `json:"sessions"`
	Lost       []savedSession `json:"lost"`
	LostAt     time.Time      `json:"lost_at,omitzero"`
}

type restoreStore struct {
	mu     sync.Mutex
	path   string
	st     restoreState
	loaded bool
}

// SetRestorePath makes the session snapshot persistent and starts keeping it.
func (s *Server) SetRestorePath(path string) {
	s.restore.path = path
	stateDir = filepath.Dir(path)
	go s.watchRestore()
}

var resumeRe = regexp.MustCompile(`^[0-9a-fA-F-]{36}$`)

func (rs *restoreStore) load() {
	if rs.loaded {
		return
	}
	rs.loaded = true
	data, err := os.ReadFile(rs.path)
	if err != nil {
		return
	}
	if err := json.Unmarshal(data, &rs.st); err != nil {
		log.Printf("restore: %s unreadable, starting over: %v", rs.path, err)
		rs.st = restoreState{}
	}
}

func (rs *restoreStore) save() {
	data, err := json.MarshalIndent(rs.st, "", "  ")
	if err != nil {
		return
	}
	if err := os.MkdirAll(filepath.Dir(rs.path), 0o700); err != nil {
		log.Printf("restore: %v", err)
		return
	}
	if err := push.WriteFileSync(rs.path, data); err != nil {
		log.Printf("restore: %v", err)
	}
}

// snapshotEvery is short so a snapshot rarely holds a session that ended on
// its own just before the server did.
const snapshotEvery = 3 * time.Second

func (s *Server) watchRestore() {
	for {
		if tmux.Found {
			s.snapshotSessions()
		}
		time.Sleep(snapshotEvery)
	}
}

const bootIDPath = "/proc/sys/kernel/random/boot_id"

// bootID names the current boot of the machine, or "" where it cannot tell.
func bootID() string {
	b, err := os.ReadFile(bootIDPath)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(b))
}

// takenBy says which sessions of the old snapshot the end of its tmux server
// took: those the current server lacks. After a reboot that is all of them.
// Without one, a single session missing is the last session ending, after
// which tmux stops by itself (exit-empty); the server dying with more than
// one session in it, or with exit-empty off, is a loss.
func takenBy(old restoreState, alive map[string]bool, boot string) (lost []savedSession, rebooted bool) {
	for _, x := range old.Sessions {
		if alive[x.Name] || !tmux.ValidName(x.Name) {
			continue
		}
		lost = append(lost, x)
	}
	rebooted = old.Boot != "" && boot != "" && old.Boot != boot
	if !rebooted && !old.KeepsEmpty && len(lost) == 1 {
		return nil, false
	}
	return lost, rebooted
}

// forgetSession drops a session that was closed on purpose from the
// snapshot, so the server stopping right after cannot offer it back.
func (s *Server) forgetSession(name string) {
	rs := &s.restore
	rs.mu.Lock()
	defer rs.mu.Unlock()
	rs.load()
	n, l := len(rs.st.Sessions), len(rs.st.Lost)
	rs.st.Sessions = dropSaved(rs.st.Sessions, name)
	rs.st.Lost = dropSaved(rs.st.Lost, name)
	if len(rs.st.Sessions) != n || len(rs.st.Lost) != l {
		rs.save()
	}
}

type convInfo struct{ Agent, ID, Dir string }

// noteConversation keeps the conversation an agent's hooks reported for a
// session; an idle agent (it exited) has none.
func (s *Server) noteConversation(session, agent, id, dir, state string) {
	s.convMu.Lock()
	defer s.convMu.Unlock()
	if s.convs == nil {
		s.convs = map[string]convInfo{}
	}
	switch {
	case state == "idle":
		delete(s.convs, session)
	case resumeRe.MatchString(id):
		s.convs[session] = convInfo{agent, id, dir}
	}
}

// conversation says which agent conversation runs in a session, if any.
func (s *Server) conversation(name string) (agent, id, dir string) {
	st, ok := s.agents.Get(name)
	if ok && st.State == "idle" {
		return "", "", "" // the agent has exited: a shell comes back
	}
	s.regMu.Lock()
	l, ok := s.regLive[name]
	s.regMu.Unlock()
	if ok && resumeRe.MatchString(l.SessionID) {
		return "claude-code", l.SessionID, l.Cwd
	}
	s.convMu.Lock()
	c, ok := s.convs[name]
	s.convMu.Unlock()
	if ok && (st.Agent == "" || st.Agent == c.Agent) {
		return c.Agent, c.ID, c.Dir
	}
	kind, path := s.subs.Transcript(name)
	base := strings.TrimSuffix(filepath.Base(path), ".jsonl")
	switch kind {
	case "claude-code":
		if resumeRe.MatchString(base) {
			return kind, base, ""
		}
	case "codex":
		// rollout-2026-11-12T10-00-00-<uuid>.jsonl
		if len(base) > 36 && resumeRe.MatchString(base[len(base)-36:]) {
			return kind, base[len(base)-36:], ""
		}
	}
	return "", "", ""
}

func (s *Server) snapshotSessions() {
	id, err := tmux.ServerID()
	if err != nil {
		return // tmux did not answer: judge nothing
	}
	var sessions []tmux.Session
	keepsEmpty := false
	if id != "" {
		if sessions, err = tmux.List(); err != nil {
			return
		}
		keepsEmpty = !tmux.ExitEmpty()
	}
	boot := bootID()
	now := time.Now()
	cur := make([]savedSession, 0, len(sessions))
	alive := map[string]bool{}
	for _, sess := range sessions {
		alive[sess.Name] = true
		sv := savedSession{Name: sess.Name, Dir: sess.Path, Quiet: sess.Quiet, Seen: now}
		if agent, conv, dir := s.conversation(sess.Name); conv != "" {
			sv.Agent, sv.Resume = agent, conv
			if dir != "" {
				sv.Dir = dir
			}
		}
		cur = append(cur, sv)
	}

	rs := &s.restore
	rs.mu.Lock()
	defer rs.mu.Unlock()
	rs.load()
	before, _ := json.Marshal(rs.st)
	if rs.st.Server != "" && rs.st.Server != id {
		// Another tmux server (or none): what the old one had and this one
		// lacks may have been taken with it.
		lost, rebooted := takenBy(rs.st, alive, boot)
		for _, x := range lost {
			rs.st.Lost = append(dropSaved(rs.st.Lost, x.Name), x)
		}
		switch {
		case len(lost) > 0:
			rs.st.LostAt = now
			why := "tmux server changed"
			if rebooted {
				why = "machine restarted"
			}
			log.Printf("restore: %s (%s -> %q), %d session(s) can be restored", why, rs.st.Server, id, len(lost))
		case len(rs.st.Sessions) > 0:
			log.Printf("restore: tmux server %s stopped after its last session ended, nothing to restore", rs.st.Server)
		}
	}
	rs.st.Boot = boot
	rs.st.KeepsEmpty = keepsEmpty
	if id == "" {
		// No server yet (just booted): keep the old snapshot's sessions
		// counted as lost, and wait for the next server.
		rs.st.Server = ""
		rs.st.Sessions = nil
	} else {
		rs.st.Server = id
		// Seen timestamps change every pass; only a real change is saved.
		if !sameSessions(rs.st.Sessions, cur) {
			rs.st.Sessions = cur
		}
	}
	for name := range alive {
		rs.st.Lost = dropSaved(rs.st.Lost, name)
	}
	if len(rs.st.Lost) == 0 {
		rs.st.LostAt = time.Time{}
	}
	after, _ := json.Marshal(rs.st)
	if string(before) != string(after) {
		rs.save()
		s.changes.bump()
	}
}

func sameSessions(a, b []savedSession) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i].Name != b[i].Name || a[i].Dir != b[i].Dir || a[i].Agent != b[i].Agent || a[i].Resume != b[i].Resume || a[i].Quiet != b[i].Quiet {
			return false
		}
	}
	return true
}

func dropSaved(list []savedSession, name string) []savedSession {
	out := list[:0:0]
	for _, x := range list {
		if x.Name != name {
			out = append(out, x)
		}
	}
	return out
}

func (s *Server) handleRestoreList(w http.ResponseWriter, r *http.Request) {
	rs := &s.restore
	rs.mu.Lock()
	rs.load()
	lost := append([]savedSession{}, rs.st.Lost...)
	at := rs.st.LostAt
	rs.mu.Unlock()
	writeJSON(w, http.StatusOK, map[string]any{"lost": lost, "lost_at": at})
}

// handleRestore brings lost sessions back: {"names": [...]}, or all of them
// when names is empty.
func (s *Server) handleRestore(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Names []string `json:"names"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16<<10)).Decode(&body); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	want := map[string]bool{}
	for _, n := range body.Names {
		want[n] = true
	}
	rs := &s.restore
	rs.mu.Lock()
	rs.load()
	var todo []savedSession
	for _, x := range rs.st.Lost {
		if len(want) == 0 || want[x.Name] {
			todo = append(todo, x)
		}
	}
	rs.mu.Unlock()

	type result struct {
		Name  string `json:"name"`
		Error string `json:"error,omitempty"`
	}
	var results []result
	for _, x := range todo {
		err := restoreSession(x)
		res := result{Name: x.Name}
		if err != nil {
			res.Error = err.Error()
			log.Printf("restore: %s: %v", x.Name, err)
		} else {
			log.Printf("restore: %s restored in %s%s", x.Name, x.Dir, map[bool]string{true: " with " + x.Agent + " " + x.Resume}[x.Resume != ""])
			rs.mu.Lock()
			rs.st.Lost = dropSaved(rs.st.Lost, x.Name)
			rs.save()
			rs.mu.Unlock()
		}
		results = append(results, res)
	}
	s.changes.bump()
	writeJSON(w, http.StatusOK, map[string]any{"results": results})
}

func restoreSession(x savedSession) error {
	if !tmux.ValidName(x.Name) {
		return tmux.ErrBadName
	}
	if ok, err := tmux.Exists(x.Name); err == nil && ok {
		return errors.New("a session with this name exists already")
	}
	dir := x.Dir
	if fi, err := os.Stat(dir); dir == "" || !filepath.IsAbs(dir) || err != nil || !fi.IsDir() {
		dir = ""
	}
	if err := tmux.NewIn(x.Name, dir); err != nil {
		return err
	}
	if x.Quiet {
		_ = tmux.SetQuiet(x.Name, true)
	}
	var cmd string
	if resumeRe.MatchString(x.Resume) {
		switch x.Agent {
		case "claude-code":
			cmd = "claude --resume " + x.Resume
		case "codex":
			cmd = "codex resume " + x.Resume
		case "gemini":
			cmd = "gemini --resume " + x.Resume
		}
	}
	if cmd == "" {
		return nil
	}
	// The new shell needs a moment before it reads its first line.
	time.Sleep(300 * time.Millisecond)
	return tmux.SendKeys(x.Name, cmd, true)
}

// handleRestoreDismiss forgets lost sessions: {"names": [...]}, or all.
func (s *Server) handleRestoreDismiss(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Names []string `json:"names"`
	}
	_ = json.NewDecoder(http.MaxBytesReader(w, r.Body, 16<<10)).Decode(&body)
	rs := &s.restore
	rs.mu.Lock()
	rs.load()
	if len(body.Names) == 0 {
		rs.st.Lost = nil
	}
	for _, n := range body.Names {
		rs.st.Lost = dropSaved(rs.st.Lost, n)
	}
	if len(rs.st.Lost) == 0 {
		rs.st.LostAt = time.Time{}
	}
	rs.save()
	rs.mu.Unlock()
	s.changes.bump()
	w.WriteHeader(http.StatusNoContent)
}
