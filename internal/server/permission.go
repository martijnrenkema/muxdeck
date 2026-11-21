package server

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/josecarlosrivas/muxdeck/internal/agent"
	"github.com/josecarlosrivas/muxdeck/internal/push"
	"github.com/josecarlosrivas/muxdeck/internal/tmux"
)

// Permission requests come from an agent hook (Claude Code's
// PermissionRequest) that holds its POST open until someone answers in the
// UI or a notification. The hook owns the fallback: when the wait runs out
// or muxdeck goes away, it returns no decision and the agent asks in the
// terminal as usual. Nothing here can approve anything on its own.

const (
	permWaitDefault = 120 * time.Second
	maxPermSummary  = 300
	maxPermDetail   = 4000
)

var permDecisions = map[string]bool{"allow": true, "always": true, "deny": true, "terminal": true, "answer": true}

// askQuestion is one question of Claude's AskUserQuestion tool. The hook
// answers it with updatedInput.answers: question text → chosen label(s),
// several joined with ", " (or text of the user's own).
type askQuestion struct {
	Question    string `json:"question"`
	Header      string `json:"header,omitempty"`
	MultiSelect bool   `json:"multiSelect,omitempty"`
	Options     []struct {
		Label       string `json:"label"`
		Description string `json:"description,omitempty"`
	} `json:"options"`
}

type permAnswer struct {
	Decision string
	Answers  map[string]string
}

const maxAnswer = 2000

// checkAnswers accepts answers only for the questions asked, all of them.
func checkAnswers(qs []askQuestion, answers map[string]string) bool {
	if len(qs) == 0 || len(answers) != len(qs) {
		return false
	}
	for _, q := range qs {
		a := strings.TrimSpace(answers[q.Question])
		if a == "" || len([]rune(a)) > maxAnswer {
			return false
		}
	}
	return true
}

type permRequest struct {
	ID        string    `json:"id"`
	Agent     string    `json:"agent"`
	Tool      string    `json:"tool"`
	Summary   string    `json:"summary"`
	Detail    string    `json:"detail,omitempty"`
	CanAlways bool      `json:"can_always,omitempty"`
	Created   time.Time `json:"created"`
	// Questions is set for AskUserQuestion: the card offers their options
	// instead of Allow/Deny.
	Questions []askQuestion `json:"questions,omitempty"`

	session string
	answer  chan permAnswer
	closed  bool // answered or no longer waited on; guarded by permStore.mu
}

type permStore struct {
	mu sync.Mutex
	m  map[string]*permRequest
}

func (ps *permStore) add(p *permRequest) {
	ps.mu.Lock()
	if ps.m == nil {
		ps.m = map[string]*permRequest{}
	}
	ps.m[p.ID] = p
	ps.mu.Unlock()
}

// peek returns an open request without changing it.
func (ps *permStore) peek(id string) *permRequest {
	ps.mu.Lock()
	defer ps.mu.Unlock()
	return ps.m[id]
}

// finish ends a request: nothing can answer it from now on.
func (ps *permStore) finish(id string) {
	ps.mu.Lock()
	if p := ps.m[id]; p != nil {
		p.closed = true
	}
	delete(ps.m, id)
	ps.mu.Unlock()
}

// answer gives a request its decision. exists is false for an unknown
// request; ok is false when it already has a decision or stopped waiting,
// so a tap that comes too late is told so instead of seeming to work.
func (ps *permStore) answer(id string, a permAnswer) (exists, ok bool) {
	ps.mu.Lock()
	defer ps.mu.Unlock()
	p := ps.m[id]
	if p == nil {
		return false, false
	}
	if p.closed {
		return true, false
	}
	p.closed = true
	p.answer <- a // buffered, and closed makes this the only send
	return true, true
}

// forSession lists a session's open requests, oldest first.
func (ps *permStore) forSession(name string) []*permRequest {
	ps.mu.Lock()
	var out []*permRequest
	for _, p := range ps.m {
		if p.session == name {
			out = append(out, p)
		}
	}
	ps.mu.Unlock()
	sort.Slice(out, func(i, j int) bool { return out[i].Created.Before(out[j].Created) })
	return out
}

// changes is a version counter clients long-poll on, so a permission card
// shows up in a sidebar that is already open without waiting for its 10 s
// refresh tick.
type changes struct {
	mu sync.Mutex
	v  uint64
	ch chan struct{}
}

func (c *changes) bump() {
	c.mu.Lock()
	c.v++
	if c.ch != nil {
		close(c.ch)
		c.ch = nil
	}
	c.mu.Unlock()
}

func (c *changes) wait(since uint64) (uint64, <-chan struct{}) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.v != since {
		return c.v, nil
	}
	if c.ch == nil {
		c.ch = make(chan struct{})
	}
	return c.v, c.ch
}

func newID() string {
	b := make([]byte, 9)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// summarizeTool turns a tool call into one readable line plus a longer body
// for the card. Known tools get their meaningful field; anything else shows
// its input as JSON.
func summarizeTool(tool string, input json.RawMessage) (summary, detail string) {
	var in map[string]any
	json.Unmarshal(input, &in)
	str := func(k string) string {
		switch v := in[k].(type) {
		case string:
			return v
		case []any: // Codex can send a command as its argv
			parts := make([]string, 0, len(v))
			for _, p := range v {
				if s, ok := p.(string); ok {
					parts = append(parts, s)
				}
			}
			return strings.Join(parts, " ")
		}
		return ""
	}
	switch tool {
	case "Bash":
		summary, detail = str("description"), str("command")
		if summary == "" {
			summary = firstLine(detail)
		}
	case "Edit", "Write", "MultiEdit", "Read", "NotebookEdit":
		summary = str("file_path")
		if summary == "" {
			summary = str("notebook_path")
		}
	case "WebFetch":
		summary = str("url")
	case "WebSearch":
		summary = str("query")
	case "ExitPlanMode":
		summary, detail = "Approve plan", str("plan")
	case "apply_patch": // Codex: the patch is the command
		detail = str("command")
		summary = str("description")
		if summary == "" {
			summary = "Apply changes to files"
			for _, l := range strings.Split(detail, "\n") {
				if f, ok := strings.CutPrefix(l, "*** Update File: "); ok {
					summary = "Edit " + f
					break
				} else if f, ok := strings.CutPrefix(l, "*** Add File: "); ok {
					summary = "Add " + f
					break
				}
			}
		}
	case "AskUserQuestion":
		var qs struct {
			Questions []askQuestion `json:"questions"`
		}
		json.Unmarshal(input, &qs)
		if len(qs.Questions) > 0 {
			summary = qs.Questions[0].Question
			if n := len(qs.Questions); n > 1 {
				summary += fmt.Sprintf(" (+%d)", n-1)
			}
		}
	}
	if summary == "" && detail == "" {
		raw, _ := json.MarshalIndent(in, "", "  ")
		detail = string(raw)
		summary = firstLine(detail)
	}
	return truncateRunes(summary, maxPermSummary), truncateRunes(detail, maxPermDetail)
}

func firstLine(s string) string {
	s = strings.TrimSpace(s)
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[:i] + " …"
	}
	return s
}

func truncateRunes(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}

// handlePermissionAsk is the hook's side: register the request, alert, and
// block until an answer, the wait running out, or the hook going away.
// The response is always 200 with a decision; "terminal" means: no answer
// here, ask in the terminal.
func (s *Server) handlePermissionAsk(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Session   string          `json:"session"`
		Agent     string          `json:"agent"`
		Tool      string          `json:"tool_name"`
		Input     json.RawMessage `json:"tool_input"`
		CanAlways bool            `json:"can_always"`
		Wait      int             `json:"wait"`
	}
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Session == "" || body.Tool == "" {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	if !tmux.ValidName(body.Session) {
		writeErr(w, tmux.ErrBadName)
		return
	}
	if body.Agent == "" {
		body.Agent = "claude-code"
	}
	st := s.settings.get()
	if !st.Perms.Enabled {
		writeJSON(w, http.StatusOK, map[string]string{"decision": "terminal"})
		return
	}
	// The hook's curl gives up at 145 s and Claude Code at 150: a longer
	// wait would only be cut off.
	wait := time.Duration(st.Perms.Wait) * time.Second
	if body.Wait > 0 {
		wait = time.Duration(min(body.Wait, permWaitCap)) * time.Second
	}
	summary, detail := summarizeTool(body.Tool, body.Input)
	p := &permRequest{
		ID: newID(), Agent: body.Agent, Tool: truncateRunes(body.Tool, 60),
		Summary: summary, Detail: detail, CanAlways: body.CanAlways,
		Created: time.Now(), session: body.Session, answer: make(chan permAnswer, 1),
	}
	if body.Tool == "AskUserQuestion" {
		var qs struct {
			Questions []askQuestion `json:"questions"`
		}
		if json.Unmarshal(body.Input, &qs) != nil || len(qs.Questions) == 0 || len(qs.Questions) > 4 {
			writeJSON(w, http.StatusOK, map[string]string{"decision": "terminal"})
			return
		}
		p.Questions = qs.Questions
		p.Detail = ""
	}
	s.perms.add(p)
	s.changes.bump()
	defer func() {
		s.perms.finish(p.ID)
		s.changes.bump()
	}()

	if st.Notify.Permission && !s.watched(body.Session) {
		title, text := body.Session+": permission needed", p.Tool
		if p.Summary != "" {
			text += ": " + p.Summary
		}
		// Phones show two notification buttons; the third is for those that show more.
		actions := []string{"allow", "deny"}
		if p.CanAlways {
			actions = append(actions, "always")
		}
		var titles []string
		if len(p.Questions) > 0 {
			title, text, actions = body.Session+": question", p.Summary, nil
			// One plain question with few options can be answered from the
			// notification; anything else opens the card.
			if q := p.Questions[0]; len(p.Questions) == 1 && !q.MultiSelect && len(q.Options) <= 3 {
				for i, o := range q.Options {
					actions = append(actions, fmt.Sprintf("opt:%d", i))
					titles = append(titles, truncateRunes(o.Label, 40))
				}
			}
		}
		s.notify(push.Message{
			Title: title, Body: text,
			Session: body.Session, Tag: "muxdeck-agent-" + body.Session,
			Perm: p.ID, Actions: actions, Titles: titles,
			// Undelivered by then, it is dropped; delivered, the service
			// worker clears it once a later push or the app sees it expired.
			TTL: wait, Expires: time.Now().Add(wait).UnixMilli(),
		})
	}

	ans := permAnswer{Decision: "terminal"}
	timer := time.NewTimer(wait)
	defer timer.Stop()
	select {
	case ans = <-p.answer:
	case <-timer.C:
		// An answer that made it in just before this one still counts: its
		// tap was told it worked.
		s.perms.finish(p.ID)
		select {
		case ans = <-p.answer:
		default:
		}
	case <-r.Context().Done():
		return
	}
	out := map[string]any{"decision": ans.Decision}
	if ans.Decision == "answer" {
		out["answers"] = ans.Answers
	}
	writeJSON(w, http.StatusOK, out)
}

// handlePermissionAnswer is the UI's side. First answer wins; a request
// already answered or given up on is a 404, so a stale card or a late tap
// on a notification can't land on anything.
func (s *Server) handlePermissionAnswer(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Decision string            `json:"decision"`
		Answers  map[string]string `json:"answers"`
		// Pick is an option index of a single question, from a
		// notification button.
		Pick *int `json:"pick"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64<<10)).Decode(&body); err != nil || !permDecisions[body.Decision] {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	id := r.PathValue("id")
	a := permAnswer{Decision: body.Decision}
	if p := s.perms.peek(id); p != nil {
		asked := len(p.Questions) > 0
		switch {
		case asked && body.Decision != "answer" && body.Decision != "terminal",
			!asked && body.Decision == "answer":
			http.Error(w, "that answer does not fit this request", http.StatusBadRequest)
			return
		case body.Decision == "answer":
			if body.Pick != nil && len(p.Questions) == 1 {
				q := p.Questions[0]
				if *body.Pick < 0 || *body.Pick >= len(q.Options) {
					http.Error(w, "no such option", http.StatusBadRequest)
					return
				}
				body.Answers = map[string]string{q.Question: q.Options[*body.Pick].Label}
			}
			if !checkAnswers(p.Questions, body.Answers) {
				http.Error(w, "every question needs an answer", http.StatusBadRequest)
				return
			}
			a.Answers = body.Answers
		}
	}
	exists, ok := s.perms.answer(id, a)
	switch {
	case !exists:
		http.Error(w, "no such request", http.StatusNotFound)
	case !ok:
		http.Error(w, "already answered", http.StatusConflict)
	default:
		w.WriteHeader(http.StatusNoContent)
	}
}

// handleChanges long-polls the version counter for up to 25 s.
func (s *Server) handleChanges(w http.ResponseWriter, r *http.Request) {
	since, _ := strconv.ParseUint(r.URL.Query().Get("since"), 10, 64)
	v, ch := s.changes.wait(since)
	if ch != nil {
		t := time.NewTimer(25 * time.Second)
		defer t.Stop()
		select {
		case <-ch:
			v, _ = s.changes.wait(^uint64(0))
		case <-t.C:
		case <-r.Context().Done():
			return
		}
	}
	writeJSON(w, http.StatusOK, map[string]uint64{"v": v})
}

// --- presence per session, for not pushing what is already on screen ---

const watchTTL = 40 * time.Second

type watchers struct {
	mu sync.Mutex
	m  map[string]map[string]time.Time // session → viewer → last "active"
}

func (ws *watchers) set(session, viewer string, active bool) {
	ws.mu.Lock()
	defer ws.mu.Unlock()
	if ws.m == nil {
		ws.m = map[string]map[string]time.Time{}
	}
	if !active {
		delete(ws.m[session], viewer)
		return
	}
	if ws.m[session] == nil {
		ws.m[session] = map[string]time.Time{}
	}
	ws.m[session][viewer] = time.Now()
}

func (s *Server) watched(session string) bool {
	s.watch.mu.Lock()
	defer s.watch.mu.Unlock()
	for _, t := range s.watch.m[session] {
		if time.Since(t) < watchTTL {
			return true
		}
	}
	return false
}

// --- push ---

func (s *Server) notify(msg push.Message) {
	if s.push != nil {
		s.push.Send(msg)
	}
}

// notifyAgentWaiting pushes the transition into waiting, never the state
// repeating: statuslines re-post "waiting" on every redraw.
func (s *Server) notifyAgentWaiting(session string, prev agent.Status, had bool, cur agent.Status, reply string) {
	// A "done" note ("… is done and waiting for you") is a finished turn;
	// any other waiting note is a question the work is blocked on.
	question := !strings.Contains(cur.Note, " is done ")
	if cur.State != agent.StateWaiting || (had && prev.State == agent.StateWaiting) || s.watched(session) {
		return
	}
	// An open permission card already alerted, with buttons; this would
	// replace that notification (same tag) with one without.
	if len(s.perms.forSession(session)) > 0 || len(s.blocks.forSession(session)) > 0 {
		return
	}
	// The first status a session ever reports only counts when it is a
	// question (a fresh agent stopping at a trust or approval screen); a
	// first "done" is usually just the daemon restarting under an idle agent.
	if !had && !question {
		return
	}
	// Quiet hours hold back "done" alerts only: a question is something work
	// is actually blocked on, and follows the permission setting instead.
	st := s.settings.get()
	if question && !st.Notify.Permission {
		return
	}
	if !question && (!st.Notify.Done || st.quietNow(time.Now())) {
		return
	}
	log.Printf("notify: %s waiting: %s", session, cur.Note)
	title, body := session+": agent needs input", cur.Note
	if body == "" {
		body = cur.Agent + " is waiting for you"
	}
	// A finished turn says what it ended with, so a glance at the phone
	// tells "done" from "has a question at the end" without opening it.
	if r := replyPreview(reply); !question && r != "" && st.Notify.Preview {
		title, body = session+": done", r
		log.Printf("notify: %s: with the start of the reply (%d chars)", session, len([]rune(r)))
	}
	s.notify(push.Message{
		Title: title, Body: body,
		Session: session, Tag: "muxdeck-agent-" + session,
	})
}

// replyPreview is the start of an agent's answer as one line: markdown
// emphasis, headings and list marks dropped, at most 180 characters.
func replyPreview(reply string) string {
	var parts []string
	for _, l := range strings.Split(reply, "\n") {
		l = strings.TrimSpace(l)
		l = strings.TrimLeft(l, "#>*-• ")
		l = strings.NewReplacer("**", "", "__", "", "`", "").Replace(l)
		if l != "" {
			parts = append(parts, l)
		}
		if len(parts) >= 3 {
			break
		}
	}
	return truncateRunes(strings.Join(parts, " "), 180)
}

func (s *Server) handlePushKey(w http.ResponseWriter, r *http.Request) {
	if s.push == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "push unavailable"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"key": s.push.PublicKey(), "subscriptions": s.push.Count(), "devices": s.push.List()})
}

func (s *Server) handlePushSubscribe(w http.ResponseWriter, r *http.Request) {
	if s.push == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "push unavailable"})
		return
	}
	var body struct {
		Endpoint string `json:"endpoint"`
		Keys     struct {
			P256dh string `json:"p256dh"`
			Auth   string `json:"auth"`
		} `json:"keys"`
	}
	r.Body = http.MaxBytesReader(w, r.Body, 8<<10)
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	if r.Method == http.MethodDelete {
		s.push.Unsubscribe(body.Endpoint)
		w.WriteHeader(http.StatusNoContent)
		return
	}
	sub := push.Subscription{Endpoint: body.Endpoint, P256dh: body.Keys.P256dh, Auth: body.Keys.Auth, Label: deviceLabel(r.UserAgent())}
	if err := s.push.Subscribe(sub); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handlePushTest(w http.ResponseWriter, r *http.Request) {
	if s.push == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "push unavailable"})
		return
	}
	s.push.Send(push.Message{Title: "muxdeck", Body: "Push notifications work", Tag: "muxdeck-test"})
	w.WriteHeader(http.StatusNoContent)
}

// deviceLabel names a subscribing browser for the device list, from its
// user agent: enough to tell the phone from the laptop.
func deviceLabel(ua string) string {
	osName := "Unknown"
	for _, c := range []struct{ needle, name string }{
		{"Android", "Android"}, {"iPhone", "iPhone"}, {"iPad", "iPad"},
		{"Mac OS X", "Mac"}, {"Windows", "Windows"}, {"CrOS", "ChromeOS"}, {"Linux", "Linux"},
	} {
		if strings.Contains(ua, c.needle) {
			osName = c.name
			break
		}
	}
	browser := "browser"
	for _, c := range []struct{ needle, name string }{
		{"Edg/", "Edge"}, {"SamsungBrowser", "Samsung Internet"}, {"Firefox/", "Firefox"},
		{"Chrome/", "Chrome"}, {"Safari/", "Safari"},
	} {
		if strings.Contains(ua, c.needle) {
			browser = c.name
			break
		}
	}
	return osName + " · " + browser
}
