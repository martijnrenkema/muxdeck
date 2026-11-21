// Package subagent finds the helpers an agent spawns — Claude Code's
// subagents, Codex's spawned threads — next to the transcript of the agent
// that spawned them, and turns their transcripts into a stream a pane can
// show. Both agents write those transcripts as JSONL files of their own:
//
//   - Claude Code: <session>.jsonl has a sibling directory
//     <session>/subagents/ with agent-<id>.jsonl and agent-<id>.meta.json.
//   - Codex: every thread is its own rollout file under
//     ~/.codex/sessions/YYYY/MM/DD/; a spawned thread names its parent in
//     the session_meta line that opens the file.
//
// The transcript path comes from the agent's reporter, so it is untrusted
// input: only paths under the two known roots are followed.
package subagent

import (
	"bufio"
	"bytes"
	"cmp"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

const (
	StateRunning = "running"
	StateDone    = "done"
	StateStopped = "stopped"
)

// A running subagent whose transcript has not moved in this long is taken
// to be dead (killed with its parent, or interrupted without a trace).
const staleAfter = 30 * time.Minute

// How long a finished subagent stays in the sidebar.
const keepFinished = 30 * time.Minute

// At most this many rows per session; running ones first.
const maxListed = 6

// How often a session's subagent directory is rescanned at most.
const rescanEvery = 2 * time.Second

var ErrNotFound = errors.New("subagent not found")

type Info struct {
	ID    string `json:"id"`
	Agent string `json:"agent"`
	Label string `json:"label"`
	Kind  string `json:"kind,omitempty"`
	State string `json:"state"`
	Depth int    `json:"depth,omitempty"`
	// Address is what the parent agent's own tools take to reach this
	// helper: Claude's agent id, Codex's task path (or thread id).
	Address string    `json:"address,omitempty"`
	Started time.Time `json:"started"`
	Updated time.Time `json:"updated"`
	// Now is the helper's latest tool call ("Bash: run the tests"), Tools
	// how many it made: what it is up to, without opening it.
	Now   string `json:"now,omitempty"`
	Tools int    `json:"tools,omitempty"`
	path  string
}

type tracked struct {
	agent      string
	transcript string
	parentIdle bool
	all        []Info
	tasks      []Task
	scanned    time.Time
}

// fileState is what an incremental read of one transcript has learned so
// far; the offset lets the next read pick up where this one stopped.
type fileState struct {
	offset  int64
	size    int64
	mtime   time.Time
	started time.Time
	state   string
	task    string
	now     string
	tools   int
}

// codexHead is the session_meta line that opens a Codex rollout.
type codexHead struct {
	ok       bool
	id       string
	parent   string
	kind     string // thread_spawn, review, other, ...
	nickname string
	path     string
	role     string
	depth    int
	started  time.Time
}

type Tracker struct {
	claudeRoot string
	codexRoot  string

	mu        sync.Mutex
	sessions  map[string]*tracked
	files     map[string]*fileState
	taskFiles map[string]*taskFile
	heads     map[string]codexHead
	idPaths   map[string]string
	lastSig   string
}

func New(home string) *Tracker {
	return &Tracker{
		claudeRoot: filepath.Join(home, ".claude", "projects"),
		codexRoot:  filepath.Join(home, ".codex", "sessions"),
		sessions:   map[string]*tracked{},
		files:      map[string]*fileState{},
		taskFiles:  map[string]*taskFile{},
		heads:      map[string]codexHead{},
		idPaths:    map[string]string{},
	}
}

func under(root, p string) bool {
	return strings.HasPrefix(p, root+string(filepath.Separator))
}

// Track records the transcript of the agent in a tmux session. A path
// outside the known roots, or one that is not a regular .jsonl file, is
// ignored. parentIdle says the agent itself has stopped, which settles any
// subagent still marked running.
func (t *Tracker) Track(session, agent, transcript, parentState string) {
	tr := strings.TrimSpace(transcript)
	t.mu.Lock()
	defer t.mu.Unlock()
	cur := t.sessions[session]
	if tr == "" {
		if cur != nil {
			cur.parentIdle = parentState == "idle"
		}
		return
	}
	tr = filepath.Clean(tr)
	if !strings.HasSuffix(tr, ".jsonl") || !(under(t.claudeRoot, tr) || under(t.codexRoot, tr)) {
		return
	}
	if fi, err := os.Stat(tr); err != nil || !fi.Mode().IsRegular() {
		return
	}
	if cur == nil || cur.transcript != tr {
		cur = &tracked{agent: agent, transcript: tr}
		t.sessions[session] = cur
	}
	cur.agent = agent
	cur.parentIdle = parentState == "idle"
}

// Transcript returns the agent and transcript tracked for a session.
func (t *Tracker) Transcript(session string) (agent, path string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if tr := t.sessions[session]; tr != nil {
		return tr.agent, tr.transcript
	}
	return "", ""
}

// Prune forgets sessions that no longer exist.
func (t *Tracker) Prune(live []string) {
	alive := make(map[string]bool, len(live))
	for _, n := range live {
		alive[n] = true
	}
	t.mu.Lock()
	for n := range t.sessions {
		if !alive[n] {
			delete(t.sessions, n)
		}
	}
	t.mu.Unlock()
}

// List returns the subagents worth showing for a session: everything still
// running, plus what finished recently.
func (t *Tracker) List(session string) []Info {
	t.mu.Lock()
	defer t.mu.Unlock()
	tr := t.sessions[session]
	if tr == nil {
		return nil
	}
	t.refresh(tr, false)
	return visible(tr.all, time.Now())
}

func visible(all []Info, now time.Time) []Info {
	var out []Info
	for _, s := range all {
		if s.State == StateRunning || now.Sub(s.Updated) < keepFinished {
			out = append(out, s)
		}
	}
	// Running ones oldest first, so a helper a subagent spawned lands under
	// the one that spawned it; then what finished, most recent first.
	sort.SliceStable(out, func(i, j int) bool {
		ri, rj := out[i].State == StateRunning, out[j].State == StateRunning
		if ri != rj {
			return ri
		}
		if ri {
			return out[i].Started.Before(out[j].Started)
		}
		return out[i].Updated.After(out[j].Updated)
	})
	if len(out) > maxListed {
		out = out[:maxListed]
	}
	return out
}

// Scan rescans every tracked session and reports whether what the sidebar
// would show changed since the previous Scan.
func (t *Tracker) Scan() bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	now := time.Now()
	names := make([]string, 0, len(t.sessions))
	for n := range t.sessions {
		names = append(names, n)
	}
	sort.Strings(names)
	var sig strings.Builder
	for _, n := range names {
		tr := t.sessions[n]
		t.refresh(tr, true)
		for _, s := range visible(tr.all, now) {
			sig.WriteString(n + "/" + s.ID + "=" + s.State + ";")
		}
		for _, k := range tr.tasks {
			sig.WriteString(n + "/task/" + k.ID + "=" + k.State + ";")
		}
	}
	changed := sig.String() != t.lastSig
	t.lastSig = sig.String()
	return changed
}

func (t *Tracker) refresh(tr *tracked, force bool) {
	if !force && time.Since(tr.scanned) < rescanEvery {
		return
	}
	tr.scanned = time.Now()
	if under(t.claudeRoot, tr.transcript) {
		tr.all = t.scanClaude(tr)
	} else {
		tr.all = t.scanCodex(tr)
	}
	t.scanTasks(tr)
	// Forget incremental state for files no longer in any list.
	keep := map[string]bool{}
	for _, s := range t.sessions {
		for _, i := range s.all {
			keep[i.path] = true
		}
	}
	for p := range t.files {
		if !keep[p] {
			delete(t.files, p)
		}
	}
	mains := map[string]bool{}
	for _, s := range t.sessions {
		mains[s.transcript] = true
	}
	for p := range t.taskFiles {
		if !mains[p] {
			delete(t.taskFiles, p)
		}
	}
}

func settle(state string, mtime time.Time, parentIdle bool) string {
	if state != StateRunning {
		return state
	}
	if parentIdle || time.Since(mtime) > staleAfter {
		return StateStopped
	}
	return state
}

// --- Claude Code ---

type claudeMeta struct {
	AgentType   string `json:"agentType"`
	Description string `json:"description"`
	SpawnDepth  int    `json:"spawnDepth"`
	Finished    bool   `json:"finished"` // newer Claude Code versions only
}

// longGone is how old a transcript's last write must be for it to never
// show again: stopped by staleAfter and past keepFinished whatever its
// state. Such a file is not read at all — one long session held hundreds
// of MB of them, and reading them all blocked the tracker for seconds.
const longGone = max(staleAfter, keepFinished) + time.Minute

func (t *Tracker) scanClaude(tr *tracked) []Info {
	dir := strings.TrimSuffix(tr.transcript, ".jsonl") + string(filepath.Separator) + "subagents"
	ents, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var out []Info
	for _, e := range ents {
		name := e.Name()
		if !strings.HasPrefix(name, "agent-") || !strings.HasSuffix(name, ".jsonl") {
			continue
		}
		id := strings.TrimSuffix(strings.TrimPrefix(name, "agent-"), ".jsonl")
		p := filepath.Join(dir, name)
		if _, read := t.files[p]; !read {
			if fi, err := e.Info(); err == nil && time.Since(fi.ModTime()) > longGone {
				out = append(out, Info{ID: id, Agent: "claude-code", Address: id, State: StateStopped,
					Started: fi.ModTime(), Updated: fi.ModTime(), path: p})
				continue
			}
		}
		fs := t.advance(p, claudeLine)
		if fs == nil {
			continue
		}
		var meta claudeMeta
		if b, err := os.ReadFile(filepath.Join(dir, "agent-"+id+".meta.json")); err == nil {
			_ = json.Unmarshal(b, &meta)
		}
		label := meta.Description
		if label == "" {
			label = firstLine(fs.task, 80)
		}
		if meta.Finished && fs.state == StateRunning {
			fs.state = StateDone
		}
		out = append(out, Info{
			ID: id, Agent: "claude-code", Label: label, Kind: meta.AgentType, Address: id,
			Depth:   max(meta.SpawnDepth-1, 0),
			State:   settle(fs.state, fs.mtime, tr.parentIdle),
			Started: fs.started, Updated: fs.mtime, path: p,
			Now: fs.now, Tools: fs.tools,
		})
	}
	return out
}

type claudeEntry struct {
	Type         string          `json:"type"`
	IsMeta       bool            `json:"isMeta"`
	Timestamp    time.Time       `json:"timestamp"`
	ToolEndsTurn bool            `json:"toolEndsTurn"`
	Message      json.RawMessage `json:"message"`
}

type claudeMessage struct {
	StopReason string          `json:"stop_reason"`
	Content    json.RawMessage `json:"content"`
}

type block struct {
	Type      string          `json:"type"`
	Text      string          `json:"text"`
	Thinking  string          `json:"thinking"`
	ID        string          `json:"id"`
	Name      string          `json:"name"`
	Input     json.RawMessage `json:"input"`
	ToolUseID string          `json:"tool_use_id"`
	Content   json.RawMessage `json:"content"`
	IsError   bool            `json:"is_error"`
}

// claudeLine folds one transcript line into the file state.
func claudeLine(fs *fileState, line []byte) {
	var e claudeEntry
	if json.Unmarshal(line, &e) != nil {
		return
	}
	if fs.started.IsZero() && !e.Timestamp.IsZero() {
		fs.started = e.Timestamp
	}
	// Harness lines (reminders, notifications) say nothing about whether
	// the helper is still working: one arriving after its last turn would
	// mark it running for another half hour.
	if (e.Type != "user" && e.Type != "assistant") || e.IsMeta {
		return
	}
	var m claudeMessage
	if json.Unmarshal(e.Message, &m) != nil {
		return
	}
	text, blocks := contentOf(m.Content)
	if e.Type == "user" {
		if fs.task == "" && text != "" {
			fs.task = text
		}
		switch {
		case e.ToolEndsTurn:
			fs.state = StateDone
		case strings.Contains(text, "[Request interrupted"):
			fs.state = StateStopped
		default:
			fs.state = StateRunning
		}
		return
	}
	hasTool := false
	for _, b := range blocks {
		if b.Type == "tool_use" {
			hasTool = true
			fs.tools++
			fs.now = toolNow(b)
		}
	}
	if (m.StopReason == "end_turn" || m.StopReason == "stop_sequence") && !hasTool {
		fs.state = StateDone
	} else {
		fs.state = StateRunning
	}
}

// toolNow says in a few words what a tool call does.
func toolNow(b block) string {
	var in struct {
		Command     string `json:"command"`
		Description string `json:"description"`
		FilePath    string `json:"file_path"`
		Pattern     string `json:"pattern"`
		URL         string `json:"url"`
		Query       string `json:"query"`
	}
	_ = json.Unmarshal(b.Input, &in)
	arg := ""
	switch b.Name {
	case "Bash":
		arg = in.Description
		if arg == "" {
			arg = firstLine(in.Command, 60)
		}
	case "Read", "Edit", "Write", "MultiEdit", "NotebookEdit":
		arg = filepath.Base(in.FilePath)
	case "Grep", "Glob":
		arg = in.Pattern
	case "WebFetch":
		arg = in.URL
	case "WebSearch":
		arg = in.Query
	case "Agent", "Task":
		arg = in.Description
	}
	if arg == "" {
		return b.Name
	}
	return clip(b.Name+": "+arg, 70)
}

// contentOf reads a message's content, which is either a plain string or a
// list of blocks; text is the joined text blocks.
func contentOf(raw json.RawMessage) (string, []block) {
	var s string
	if json.Unmarshal(raw, &s) == nil {
		return s, nil
	}
	var blocks []block
	_ = json.Unmarshal(raw, &blocks)
	var parts []string
	for _, b := range blocks {
		if b.Type == "text" && b.Text != "" {
			parts = append(parts, b.Text)
		}
	}
	return strings.Join(parts, "\n"), blocks
}

// --- Codex ---

func (t *Tracker) head(p string) codexHead {
	if h, ok := t.heads[p]; ok {
		return h
	}
	h := readCodexHead(p)
	// A file still being created has no complete first line yet: try again.
	if h.ok {
		t.heads[p] = h
	}
	return h
}

func readCodexHead(p string) codexHead {
	f, err := os.Open(p)
	if err != nil {
		return codexHead{}
	}
	defer f.Close()
	r := bufio.NewReaderSize(f, 64<<10)
	line, _, err := readLine(r, 4<<20)
	if err != nil {
		return codexHead{}
	}
	var ev struct {
		Type    string `json:"type"`
		Payload struct {
			ID            string          `json:"id"`
			Timestamp     time.Time       `json:"timestamp"`
			ParentThread  string          `json:"parent_thread_id"`
			AgentNickname string          `json:"agent_nickname"`
			AgentRole     string          `json:"agent_role"`
			Source        json.RawMessage `json:"source"`
		} `json:"payload"`
	}
	if json.Unmarshal(line, &ev) != nil || ev.Type != "session_meta" {
		return codexHead{ok: true}
	}
	pl := ev.Payload
	h := codexHead{ok: true, id: pl.ID, parent: pl.ParentThread, nickname: pl.AgentNickname, role: pl.AgentRole, started: pl.Timestamp}
	// source is "cli", or {"subagent": "review"}, or
	// {"subagent": {"thread_spawn": {...}}} / {"subagent": {"other": "guardian"}}.
	var src struct {
		Subagent json.RawMessage `json:"subagent"`
	}
	if json.Unmarshal(pl.Source, &src) == nil && len(src.Subagent) > 0 {
		var name string
		if json.Unmarshal(src.Subagent, &name) == nil {
			h.kind = name
		} else {
			var obj map[string]json.RawMessage
			if json.Unmarshal(src.Subagent, &obj) == nil {
				for k, v := range obj {
					h.kind = k
					var ts struct {
						ParentThread  string `json:"parent_thread_id"`
						Depth         int    `json:"depth"`
						AgentNickname string `json:"agent_nickname"`
						AgentRole     string `json:"agent_role"`
						AgentPath     string `json:"agent_path"`
					}
					if k == "thread_spawn" && json.Unmarshal(v, &ts) == nil {
						if h.parent == "" {
							h.parent = ts.ParentThread
						}
						if h.nickname == "" {
							h.nickname = ts.AgentNickname
						}
						if h.role == "" {
							h.role = ts.AgentRole
						}
						h.depth = ts.Depth
						h.path = ts.AgentPath
					}
				}
			}
		}
	}
	return h
}

// Codex's own helpers (the approval guardian, compaction, memory) are
// plumbing, not work anybody asked for: they stay out of the list.
func codexShown(kind string) bool {
	switch kind {
	case "other", "compact", "memory_consolidation":
		return false
	}
	return true
}

func (t *Tracker) scanCodex(tr *tracked) []Info {
	root := t.head(tr.transcript)
	if root.id == "" {
		return nil
	}
	start := root.started
	if start.IsZero() {
		if fi, err := os.Stat(tr.transcript); err == nil {
			start = fi.ModTime()
		}
	}
	// Children live in the day directories from the parent's start to today.
	var files []string
	for d := start.Local(); !d.After(time.Now().Add(24 * time.Hour)); d = d.AddDate(0, 0, 1) {
		dir := filepath.Join(t.codexRoot, d.Format("2006"), d.Format("01"), d.Format("02"))
		ents, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, e := range ents {
			n := e.Name()
			if strings.HasPrefix(n, "rollout-") && strings.HasSuffix(n, ".jsonl") {
				files = append(files, filepath.Join(dir, n))
			}
		}
		if len(files) > 5000 {
			break
		}
	}
	byParent := map[string][]string{}
	for _, p := range files {
		if p == tr.transcript {
			continue
		}
		if fi, err := os.Stat(p); err != nil || fi.ModTime().Before(start.Add(-time.Minute)) {
			continue
		}
		if h := t.head(p); h.parent != "" {
			byParent[h.parent] = append(byParent[h.parent], p)
		}
	}
	var out []Info
	var walk func(id string, depth int)
	walk = func(id string, depth int) {
		if depth > 8 {
			return
		}
		for _, p := range byParent[id] {
			h := t.head(p)
			if !codexShown(h.kind) {
				continue
			}
			fs := t.advance(p, codexLine)
			if fs == nil {
				continue
			}
			label := h.nickname
			if label == "" {
				label = firstLine(fs.task, 80)
			}
			started := h.started
			if started.IsZero() {
				started = fs.started
			}
			out = append(out, Info{
				ID: h.id, Agent: "codex", Label: label, Kind: h.role, Depth: depth, Address: cmp.Or(h.path, h.id),
				State:   settle(fs.state, fs.mtime, tr.parentIdle),
				Started: started, Updated: fs.mtime, path: p,
			})
			walk(h.id, depth+1)
		}
	}
	walk(root.id, 0)
	return out
}

type codexEvent struct {
	Type      string          `json:"type"`
	Timestamp time.Time       `json:"timestamp"`
	Payload   json.RawMessage `json:"payload"`
}

type codexPayload struct {
	Type      string          `json:"type"`
	Role      string          `json:"role"`
	Content   json.RawMessage `json:"content"`
	Name      string          `json:"name"`
	Arguments string          `json:"arguments"`
	Input     string          `json:"input"`
	CallID    string          `json:"call_id"`
	Output    json.RawMessage `json:"output"`
	Summary   []struct {
		Text string `json:"text"`
	} `json:"summary"`
	LastAgentMessage string `json:"last_agent_message"`
}

func codexText(raw json.RawMessage) string {
	var items []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	}
	if json.Unmarshal(raw, &items) != nil {
		var s string
		_ = json.Unmarshal(raw, &s)
		return s
	}
	var parts []string
	for _, it := range items {
		if it.Text != "" {
			parts = append(parts, it.Text)
		}
	}
	return strings.Join(parts, "\n")
}

// Codex prepends environment and instruction blocks as user messages; they
// are setup, not the task.
func codexInjected(text string) bool {
	t := strings.TrimSpace(text)
	for _, p := range []string{"<environment_context>", "<user_instructions>", "<permissions", "# AGENTS.md", "<INSTRUCTIONS>", "<turn_aborted>", "<user_shell_command>"} {
		if strings.HasPrefix(t, p) {
			return true
		}
	}
	return false
}

func codexLine(fs *fileState, line []byte) {
	var ev codexEvent
	if json.Unmarshal(line, &ev) != nil {
		return
	}
	if fs.started.IsZero() && !ev.Timestamp.IsZero() {
		fs.started = ev.Timestamp
	}
	var p codexPayload
	if json.Unmarshal(ev.Payload, &p) != nil {
		return
	}
	switch ev.Type {
	case "event_msg":
		switch p.Type {
		case "task_started":
			fs.state = StateRunning
		case "task_complete":
			fs.state = StateDone
		case "turn_aborted":
			fs.state = StateStopped
		}
	case "response_item":
		if p.Type == "message" && p.Role == "user" && fs.task == "" {
			if t := codexText(p.Content); t != "" && !codexInjected(t) {
				fs.task = t
			}
		}
		if fs.state == "" {
			fs.state = StateRunning
		}
	}
}

// --- incremental reading ---

// advance reads what was appended to p since the last call and folds it in
// with fold. It returns nil when the file cannot be read.
func (t *Tracker) advance(p string, fold func(*fileState, []byte)) *fileState {
	fi, err := os.Stat(p)
	if err != nil || !fi.Mode().IsRegular() {
		return nil
	}
	fs := t.files[p]
	if fs == nil || fi.Size() < fs.offset {
		fs = &fileState{state: StateRunning}
		t.files[p] = fs
	}
	fs.mtime = fi.ModTime()
	if fi.Size() == fs.size {
		return fs
	}
	f, err := os.Open(p)
	if err != nil {
		return nil
	}
	defer f.Close()
	if _, err := f.Seek(fs.offset, io.SeekStart); err != nil {
		return nil
	}
	r := bufio.NewReaderSize(f, 64<<10)
	for {
		line, n, err := readLine(r, 32<<20)
		if err != nil {
			break // incomplete last line: picked up next time
		}
		fs.offset += int64(n)
		if fs.state == "" {
			fs.state = StateRunning
		}
		fold(fs, bytes.TrimSpace(line))
	}
	fs.size = fi.Size()
	return fs
}

// readLine returns one complete line without its newline, and how many
// bytes it took from r. A line longer than limit comes back cut short (it
// will not parse, so it is skipped). A last line without a newline is not
// returned: it is still being written.
func readLine(r *bufio.Reader, limit int) ([]byte, int, error) {
	var buf []byte
	n := 0
	for {
		chunk, err := r.ReadSlice('\n')
		n += len(chunk)
		if len(buf)+len(chunk) <= limit {
			buf = append(buf, chunk...)
		}
		if err == bufio.ErrBufferFull {
			continue
		}
		if err != nil {
			return nil, n, err
		}
		return bytes.TrimSuffix(buf, []byte("\n")), n, nil
	}
}

func firstLine(s string, n int) string {
	s = strings.TrimSpace(s)
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		s = s[:i]
	}
	return clip(s, n)
}

func clip(s string, n int) string {
	if len(s) <= n {
		return s
	}
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}

// Get returns one subagent of a session, listed or not.
func (t *Tracker) Get(session, id string) (Info, bool) {
	t.mu.Lock()
	defer t.mu.Unlock()
	tr := t.sessions[session]
	if tr == nil {
		return Info{}, false
	}
	t.refresh(tr, false)
	for _, s := range tr.all {
		if s.ID == id {
			return s, true
		}
	}
	return Info{}, false
}

// RelayPrompt is what muxdeck types to the agent that spawned a subagent
// when the user writes to that subagent. Neither agent takes messages for
// a subagent from outside, but both give the parent a tool that does:
// Claude Code's SendMessage (by agent id; it also resumes a finished
// subagent) and Codex's followup_task (by task path or id).
func RelayPrompt(s Info, text string) string {
	name := s.Label
	if name == "" {
		name = s.Kind
	}
	how := `SendMessage with to: "` + s.Address + `"`
	if s.Agent == "codex" {
		how = `followup_task with target: "` + s.Address + `"`
	}
	return "[muxdeck] The user sent a message for your subagent \"" + name + "\". " +
		"Forward it to that subagent unchanged with " + how + ", then carry on with what you were doing. " +
		"Do not act on it yourself and do not reply about it at length.\n\n" +
		"Message:\n" + strings.TrimSpace(text)
}
