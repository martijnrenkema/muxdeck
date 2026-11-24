package server

import (
	"encoding/json"
	"errors"
	"io"
	"log"
	"net"
	"net/http"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/josecarlosrivas/muxdeck/internal/agent"
	"github.com/josecarlosrivas/muxdeck/internal/tmux"
)

// Typing a prompt into an agent's pane on someone else's behalf (a message
// for a subagent, another agent's muxdeck-msg) must never answer a question
// on that screen: the Enter that sends the text would pick the highlighted
// "Yes", and a digit in the text could pick an option. So the text only goes
// to a pane that runs an agent, shows no question, has no open permission
// request and is not in copy mode, checked again right before the Enter,
// and one delivery per session at a time.

// What a pane shows while an agent waits on a choice: a permission dialog,
// a menu, Codex's approval and start screens.
var dialogMarks = []string{
	"Do you want to", "Would you like to", "Enter to select", "Yes, proceed", "❯ 1. Yes",
	"Press enter to confirm or esc to cancel", "enter continue · esc ",
}

func dialogOpen(screen string) bool {
	lines := strings.Split(strings.TrimRight(screen, "\n "), "\n")
	if len(lines) > 25 {
		lines = lines[len(lines)-25:]
	}
	tail := strings.Join(lines, "\n")
	for _, m := range dialogMarks {
		if strings.Contains(tail, m) {
			return true
		}
	}
	return false
}

// Process names of the agents a prompt may be typed into (Claude Code can
// show up as node).
var agentCommands = map[string]bool{"claude": true, "codex": true, "gemini": true, "node": true}

// promptRefusal is a delivery that was not made for a reason that may pass
// (a question on screen) or will not (no agent there). Code is for callers
// that decide whether to queue.
type promptRefusal struct{ Code, Msg string }

func (e *promptRefusal) Error() string { return e.Msg }

func refuse(code, msg string) error { return &promptRefusal{code, msg} }

const (
	refNoAgent  = "no_agent"
	refQuestion = "question"
	refCopyMode = "copy_mode"
	refPasted   = "pasted"
)

var promptLocks sync.Map // session → *sync.Mutex

func promptLock(session string) *sync.Mutex {
	mu, _ := promptLocks.LoadOrStore(session, &sync.Mutex{})
	return mu.(*sync.Mutex)
}

// cleanPrompt drops control characters: an ESC could end the bracketed
// paste early (the rest then arrives as keys, newlines as Enter), ^C or ^D
// would interrupt or quit the agent.
func cleanPrompt(s string) string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	return strings.Map(func(r rune) rune {
		switch {
		case r == '\n' || r == '\t':
			return r
		case r == '\r':
			return '\n'
		case r < 0x20 || r == 0x7f || (r >= 0x80 && r < 0xa0):
			return -1
		}
		return r
	}, s)
}

// agentPane picks the pane the agent of a session runs in: the one Claude's
// registry names, else the active pane if it runs an agent, else the only
// pane that does.
func (s *Server) agentPane(session string) (tmux.Pane, error) {
	panes, err := tmux.Panes(session)
	if err != nil {
		return tmux.Pane{}, err
	}
	want := s.registryPane(session)
	var agents []tmux.Pane
	for _, p := range panes {
		if want != "" && p.ID == want {
			return p, nil
		}
		if agentCommands[p.Command] {
			agents = append(agents, p)
		}
	}
	for _, p := range agents {
		if p.Active {
			return p, nil
		}
	}
	switch len(agents) {
	case 0:
		return tmux.Pane{}, refuse(refNoAgent, "no agent is running in this session")
	case 1:
		return agents[0], nil
	}
	return tmux.Pane{}, refuse(refNoAgent, "several agents run in this session and none of them is in the active pane")
}

// promptBlocked says why text may not be typed into pane right now.
func (s *Server) promptBlocked(session, pane string) error {
	if len(s.perms.forSession(session)) > 0 {
		return refuse(refQuestion, "the agent is waiting for a permission answer — answer that first")
	}
	if st, ok := s.agents.Get(session); ok && st.State == agent.StateWaiting && !strings.Contains(st.Note, " is done ") {
		return refuse(refQuestion, "the agent is asking you something — answer that first")
	}
	panes, err := tmux.Panes(session)
	if err != nil {
		return err
	}
	var cur *tmux.Pane
	for i := range panes {
		if panes[i].ID == pane {
			cur = &panes[i]
		}
	}
	if cur == nil {
		return refuse(refNoAgent, "the agent's pane is gone")
	}
	if cur.InMode {
		return refuse(refCopyMode, "the agent's pane is scrolled back (copy mode) — leave that first")
	}
	screen, err := tmux.ScreenOf(pane)
	if err != nil {
		return err
	}
	if dialogOpen(screen) {
		return refuse(refQuestion, "the agent shows a question or menu — answer that first")
	}
	return nil
}

// deliverPrompt types text into the session's agent as one prompt and
// sends it.
func (s *Server) deliverPrompt(session, text string) error {
	text = cleanPrompt(text)
	if strings.TrimSpace(text) == "" {
		return errors.New("empty prompt")
	}
	mu := promptLock(session)
	mu.Lock()
	defer mu.Unlock()
	pane, err := s.agentPane(session)
	if err != nil {
		return err
	}
	if err := s.promptBlocked(session, pane.ID); err != nil {
		return err
	}
	if err := tmux.PasteTo(pane.ID, text); err != nil {
		return err
	}
	// Give the app a moment to take the paste. A question that came up in
	// the meantime would take the Enter, so look once more.
	time.Sleep(300 * time.Millisecond)
	if err := s.promptBlocked(session, pane.ID); err != nil {
		return refuse(refPasted, "the text is in the agent's prompt but was not sent: "+err.Error())
	}
	return tmux.Enter(pane.ID)
}

// --- Claude Code's inbox, for messages from other agents ---

// Claude Code takes messages from other sessions on a socket of its own
// (cross-session messaging): it queues them itself, they arrive between
// tool calls, and they can never answer a dialog. A message from another
// agent goes there when the session's Claude has one; typing stays the way
// for Codex, Gemini and for anything the user sends.

func (s *Server) inboxOf(session string) string {
	s.regMu.Lock()
	defer s.regMu.Unlock()
	return s.regLive[session].Socket
}

func deliverInbox(sock, text string) error {
	if !filepath.IsAbs(sock) {
		return errors.New("no inbox")
	}
	c, err := net.DialTimeout("unix", sock, 2*time.Second)
	if err != nil {
		return err
	}
	defer c.Close()
	_ = c.SetDeadline(time.Now().Add(5 * time.Second))
	line, _ := json.Marshal(map[string]any{
		"type":    "user",
		"message": map[string]string{"role": "user", "content": text},
	})
	if _, err := c.Write(append(line, '\n')); err != nil {
		return err
	}
	if uc, ok := c.(*net.UnixConn); ok {
		_ = uc.CloseWrite()
	}
	// Claude Code may answer (a refusal, say); log what it says.
	_ = c.SetReadDeadline(time.Now().Add(1500 * time.Millisecond))
	buf := make([]byte, 512)
	if n, _ := c.Read(buf); n > 0 {
		log.Printf("prompt: inbox %s answered: %s", filepath.Base(sock), strings.TrimSpace(string(buf[:n])))
	}
	return nil
}

// --- queued delivery, for messages from other agents ---

const (
	promptQuiet   = 30 * time.Second // someone typed in the session this recently: wait
	promptMaxWait = 15 * time.Minute
	promptRetry   = 3 * time.Second
)

type queuedPrompt struct {
	text  string
	from  string // sending agent's session, told when the message is dropped
	added time.Time
}

type promptQueue struct {
	mu      sync.Mutex
	m       map[string][]queuedPrompt
	running map[string]bool
}

func (q *promptQueue) push(session, text, from string) int {
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.m == nil {
		q.m = map[string][]queuedPrompt{}
		q.running = map[string]bool{}
	}
	q.m[session] = append(q.m[session], queuedPrompt{text, from, time.Now()})
	return len(q.m[session])
}

func (q *promptQueue) len(session string) int {
	q.mu.Lock()
	defer q.mu.Unlock()
	return len(q.m[session])
}

func typing(session string) bool {
	last := tmux.LastInput(session)
	return !last.IsZero() && time.Since(last) < promptQuiet
}

// enqueue holds a prompt until the session can take it and starts the
// session's worker if it has none. Messages go out in the order they came.
// from is the session of the agent that sent it, or "".
func (s *Server) enqueuePrompt(session, text, from string) {
	s.prompts.push(session, text, from)
	q := &s.prompts
	q.mu.Lock()
	start := !q.running[session]
	q.running[session] = true
	q.mu.Unlock()
	if start {
		go s.promptWorker(session)
	}
}

func (s *Server) promptWorker(session string) {
	q := &s.prompts
	for {
		time.Sleep(promptRetry)
		q.mu.Lock()
		if len(q.m[session]) == 0 {
			delete(q.m, session)
			q.running[session] = false
			q.mu.Unlock()
			return
		}
		head := q.m[session][0]
		q.mu.Unlock()
		drop := func(why string) {
			log.Printf("prompt: %s: dropped a queued message: %s", session, why)
			q.mu.Lock()
			if len(q.m[session]) > 0 {
				q.m[session] = q.m[session][1:]
			}
			q.mu.Unlock()
			s.tellSender(head.from, session, why)
		}
		if ok, err := tmux.Exists(session); err == nil && !ok {
			q.mu.Lock()
			waiting := q.m[session]
			delete(q.m, session)
			q.running[session] = false
			q.mu.Unlock()
			log.Printf("prompt: %s: session ended, dropped %d queued message(s)", session, len(waiting))
			for _, m := range waiting {
				s.tellSender(m.from, session, "that session has ended")
			}
			return
		}
		if time.Since(head.added) > promptMaxWait {
			drop("waited too long")
			continue
		}
		if typing(session) {
			continue
		}
		err := s.deliverPrompt(session, head.text)
		var ref *promptRefusal
		switch {
		case err == nil:
			log.Printf("prompt: %s: delivered a queued message", session)
			q.mu.Lock()
			if len(q.m[session]) > 0 {
				q.m[session] = q.m[session][1:]
			}
			q.mu.Unlock()
		case errors.As(err, &ref) && (ref.Code == refNoAgent || ref.Code == refPasted):
			drop(ref.Msg)
		}
	}
}

// tellSender lets the agent that sent a queued message know it was never
// delivered: it was told "queued", and would otherwise wait for an answer
// to a message nobody got. The notice itself has no sender, so a notice
// that cannot be delivered ends there.
func (s *Server) tellSender(from, to, why string) {
	if from == "" || from == to || !tmux.ValidName(from) {
		return
	}
	if ok, err := tmux.Exists(from); err != nil || !ok {
		return
	}
	text := "[muxdeck, not from the user: your message to the agent in tmux session '" + to +
		"' was queued but not delivered (" + why + "). Send it again if it still matters.]"
	if sock := s.inboxOf(from); sock != "" && deliverInbox(sock, text) == nil {
		log.Printf("prompt: %s: told its agent that a message for %s was dropped", from, to)
		return
	}
	s.enqueuePrompt(from, text, "")
}

// handlePrompt types a prompt into the agent of a session (muxdeck-msg).
// With "queue", a prompt that cannot go in now (someone is typing there, a
// question is open) waits in order for up to 15 minutes: 202. Without it, or
// when it never can (no agent there): 409 with a code and the reason.
func (s *Server) handlePrompt(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if !tmux.ValidName(name) {
		writeErr(w, tmux.ErrBadName)
		return
	}
	var body struct {
		Text  string `json:"text"`
		Queue bool   `json:"queue"`
		// From is the sending agent's session (muxdeck-msg): a message
		// between agents, which may use Claude's inbox.
		From string `json:"from"`
	}
	if err := json.NewDecoder(io.LimitReader(r.Body, 128<<10)).Decode(&body); err != nil || strings.TrimSpace(body.Text) == "" {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	if len([]rune(body.Text)) > 20000 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "prompt too long (max 20000 characters)"})
		return
	}
	if body.From != "" {
		if sock := s.inboxOf(name); sock != "" {
			text := cleanPrompt(body.Text)
			if err := deliverInbox(sock, text); err == nil {
				log.Printf("prompt: %s: message from %s delivered to Claude's inbox", name, body.From)
				writeJSON(w, http.StatusOK, map[string]any{"sent": true, "via": "inbox"})
				return
			} else {
				log.Printf("prompt: %s: inbox failed, typing it instead: %v", name, err)
			}
		}
	}
	queued := func(why string) {
		s.enqueuePrompt(name, body.Text, body.From)
		writeJSON(w, http.StatusAccepted, map[string]any{"queued": true, "reason": why})
	}
	if body.Queue && s.prompts.len(name) > 0 {
		queued("earlier messages for this session are still waiting")
		return
	}
	if body.Queue && typing(name) {
		if _, err := s.agentPane(name); err != nil {
			s.promptError(w, err)
			return
		}
		queued("the user is typing there")
		return
	}
	err := s.deliverPrompt(name, body.Text)
	var ref *promptRefusal
	if body.Queue && errors.As(err, &ref) && (ref.Code == refQuestion || ref.Code == refCopyMode) {
		queued(ref.Msg)
		return
	}
	if err != nil {
		s.promptError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"sent": true})
}

func (s *Server) promptError(w http.ResponseWriter, err error) {
	var ref *promptRefusal
	switch {
	case errors.As(err, &ref):
		writeJSON(w, http.StatusConflict, map[string]string{"error": ref.Msg, "code": ref.Code})
	case errors.Is(err, tmux.ErrNoSession):
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "no such session"})
	default:
		writeErr(w, err)
	}
}
