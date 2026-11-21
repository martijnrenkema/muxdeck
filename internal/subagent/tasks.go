package subagent

import (
	"bufio"
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Background work a Claude Code session started itself: Bash commands run
// with run_in_background (a CI watch, a test suite, a build) and Monitor
// watches. They live in the main transcript: the tool call says what it is,
// its result carries the task id and output file, and a <task-notification>
// line says how it ended. Nothing of that is a documented format, so
// anything that does not parse is skipped.

type Task struct {
	ID      string    `json:"id"`
	Kind    string    `json:"kind"` // shell or monitor
	Label   string    `json:"label"`
	State   string    `json:"state"` // running, done, failed, stopped
	Summary string    `json:"summary,omitempty"`
	Started time.Time `json:"started"`
	Ended   time.Time `json:"ended,omitzero"`
	output  string
}

const (
	TaskFailed = "failed"
	// taskTail is how much of a transcript a first read looks at: tasks
	// further back than that are long over.
	taskTail   = 8 << 20
	keepTasks  = 30 * time.Minute // finished tasks stay listed this long
	maxTaskAge = 24 * time.Hour
	maxTasks   = 6
)

type pendingTask struct {
	kind, label string
	at          time.Time
}

type taskFile struct {
	offset, size int64
	partial      bool // offset is mid-line: skip to the next newline
	pending      map[string]pendingTask
	tasks        map[string]*Task
}

var (
	outputRe = regexp.MustCompile(`Output is being written to: (\S+?\.output)`)
	notifRe  = regexp.MustCompile(`(?s)<task-notification>(.*?)</task-notification>`)
	taskIDRe = regexp.MustCompile(`^[A-Za-z0-9_-]{4,40}$`)
)

func tag(s, name string) string {
	i := strings.Index(s, "<"+name+">")
	if i < 0 {
		return ""
	}
	s = s[i+len(name)+2:]
	if j := strings.Index(s, "</"+name+">"); j >= 0 {
		return strings.TrimSpace(s[:j])
	}
	return ""
}

func (tf *taskFile) fold(line []byte) {
	switch {
	case bytes.Contains(line, []byte(`"run_in_background":true`)) || bytes.Contains(line, []byte(`"name":"Monitor"`)):
		tf.foldCall(line)
	case bytes.Contains(line, []byte(`"backgroundTaskId"`)) || bytes.Contains(line, []byte(`"taskId"`)):
		tf.foldStart(line)
	}
	if bytes.Contains(line, []byte("<task-notification>")) {
		tf.foldEnd(line)
	}
}

// foldCall notes a tool call that starts background work.
func (tf *taskFile) foldCall(line []byte) {
	var e claudeEntry
	if json.Unmarshal(line, &e) != nil || e.Type != "assistant" {
		return
	}
	var m claudeMessage
	if json.Unmarshal(e.Message, &m) != nil {
		return
	}
	_, blocks := contentOf(m.Content)
	for _, b := range blocks {
		if b.Type != "tool_use" {
			continue
		}
		var in struct {
			Command     string `json:"command"`
			Description string `json:"description"`
			Background  bool   `json:"run_in_background"`
		}
		_ = json.Unmarshal(b.Input, &in)
		label := in.Description
		if label == "" {
			label = firstLine(in.Command, 80)
		}
		switch {
		case b.Name == "Bash" && in.Background:
			tf.pending[b.ID] = pendingTask{"shell", clip(label, 100), e.Timestamp}
		case b.Name == "Monitor":
			tf.pending[b.ID] = pendingTask{"monitor", clip(label, 100), e.Timestamp}
		}
	}
}

// foldStart turns a noted call into a task once its result names the task.
func (tf *taskFile) foldStart(line []byte) {
	var e struct {
		Type          string          `json:"type"`
		Timestamp     time.Time       `json:"timestamp"`
		Message       json.RawMessage `json:"message"`
		ToolUseResult json.RawMessage `json:"toolUseResult"`
	}
	if json.Unmarshal(line, &e) != nil || e.Type != "user" {
		return
	}
	var r struct {
		BackgroundTaskID string `json:"backgroundTaskId"`
		TaskID           string `json:"taskId"`
	}
	_ = json.Unmarshal(e.ToolUseResult, &r)
	id := r.BackgroundTaskID
	if id == "" {
		id = r.TaskID
	}
	if !taskIDRe.MatchString(id) {
		return
	}
	var m claudeMessage
	if json.Unmarshal(e.Message, &m) != nil {
		return
	}
	_, blocks := contentOf(m.Content)
	for _, b := range blocks {
		p, ok := tf.pending[b.ToolUseID]
		if b.Type != "tool_result" || !ok {
			continue
		}
		delete(tf.pending, b.ToolUseID)
		text, _ := contentOf(b.Content)
		started := p.at
		if started.IsZero() {
			started = e.Timestamp
		}
		t := &Task{ID: id, Kind: p.kind, Label: p.label, State: StateRunning, Started: started}
		if mm := outputRe.FindStringSubmatch(text); mm != nil {
			t.output = mm[1]
		}
		tf.tasks[id] = t
	}
}

// foldEnd settles tasks a notification reports on.
func (tf *taskFile) foldEnd(line []byte) {
	var e struct {
		Type      string          `json:"type"`
		Operation string          `json:"operation"`
		Timestamp time.Time       `json:"timestamp"`
		Content   json.RawMessage `json:"content"`
		Message   json.RawMessage `json:"message"`
	}
	if json.Unmarshal(line, &e) != nil {
		return
	}
	var text string
	switch {
	case e.Type == "queue-operation" && e.Operation == "enqueue":
		_ = json.Unmarshal(e.Content, &text)
	case e.Type == "user":
		var m claudeMessage
		if json.Unmarshal(e.Message, &m) == nil {
			text, _ = contentOf(m.Content)
		}
	}
	for _, mm := range notifRe.FindAllStringSubmatch(text, -1) {
		t := tf.tasks[tag(mm[1], "task-id")]
		if t == nil || t.State != StateRunning {
			continue
		}
		switch tag(mm[1], "status") {
		case "completed":
			t.State = StateDone
		case "failed":
			t.State = TaskFailed
		case "killed", "stopped":
			t.State = StateStopped
		default:
			continue
		}
		t.Ended = e.Timestamp
		if t.Ended.IsZero() {
			t.Ended = time.Now()
		}
		t.Summary = clip(tag(mm[1], "summary"), 160)
		if out := tag(mm[1], "output-file"); out != "" && t.output == "" {
			t.output = out
		}
	}
}

// advanceTasks reads what a transcript gained since the last read; a
// first read starts near the end.
func (t *Tracker) advanceTasks(p string) *taskFile {
	fi, err := os.Stat(p)
	if err != nil || !fi.Mode().IsRegular() {
		return nil
	}
	tf := t.taskFiles[p]
	if tf == nil || fi.Size() < tf.offset {
		tf = &taskFile{pending: map[string]pendingTask{}, tasks: map[string]*Task{}}
		if fi.Size() > taskTail {
			tf.offset, tf.partial = fi.Size()-taskTail, true
		}
		t.taskFiles[p] = tf
	}
	if fi.Size() == tf.size {
		return tf
	}
	f, err := os.Open(p)
	if err != nil {
		return nil
	}
	defer f.Close()
	if _, err := f.Seek(tf.offset, io.SeekStart); err != nil {
		return nil
	}
	r := bufio.NewReaderSize(f, 64<<10)
	if tf.partial {
		skip, err := r.ReadSlice('\n')
		for err == bufio.ErrBufferFull {
			tf.offset += int64(len(skip))
			skip, err = r.ReadSlice('\n')
		}
		if err != nil {
			return tf
		}
		tf.offset += int64(len(skip))
		tf.partial = false
	}
	for {
		line, n, err := readLine(r, 32<<20)
		if err != nil {
			break
		}
		tf.offset += int64(n)
		tf.fold(bytes.TrimSpace(line))
	}
	tf.size = fi.Size()
	return tf
}

// settleTasks drops what is too old to show, and stops running tasks of
// an agent that is gone (they die with it).
func (tf *taskFile) settle(now time.Time, parentIdle bool) []Task {
	for id, p := range tf.pending {
		if now.Sub(p.at) > 10*time.Minute {
			delete(tf.pending, id)
		}
	}
	var out []Task
	for id, t := range tf.tasks {
		if t.State == StateRunning && parentIdle {
			t.State, t.Ended = StateStopped, now
		}
		if now.Sub(t.Started) > maxTaskAge || (t.State != StateRunning && now.Sub(t.Ended) > keepTasks) {
			delete(tf.tasks, id)
			continue
		}
		out = append(out, *t)
	}
	sort.Slice(out, func(i, j int) bool {
		ri, rj := out[i].State == StateRunning, out[j].State == StateRunning
		if ri != rj {
			return ri
		}
		return out[i].Started.After(out[j].Started)
	})
	if len(out) > maxTasks {
		out = out[:maxTasks]
	}
	return out
}

// Tasks lists the background work of a session's Claude: running first.
func (t *Tracker) Tasks(session string) []Task {
	t.mu.Lock()
	defer t.mu.Unlock()
	if tr := t.sessions[session]; tr != nil {
		return tr.tasks
	}
	return nil
}

// TaskOutput returns a listed task and the file its output goes to.
func (t *Tracker) TaskOutput(session, id string) (Task, string, bool) {
	t.mu.Lock()
	defer t.mu.Unlock()
	tr := t.sessions[session]
	if tr == nil {
		return Task{}, "", false
	}
	tf := t.taskFiles[tr.transcript]
	if tf == nil || tf.tasks[id] == nil {
		return Task{}, "", false
	}
	tk := tf.tasks[id]
	return *tk, tk.output, true
}

func (t *Tracker) scanTasks(tr *tracked) {
	if !under(t.claudeRoot, tr.transcript) {
		tr.tasks = nil
		return
	}
	tf := t.advanceTasks(tr.transcript)
	if tf == nil {
		tr.tasks = nil
		return
	}
	tr.tasks = tf.settle(time.Now(), tr.parentIdle)
}

// TaskOutputDir is where Claude Code keeps task output for this user.
func TaskOutputDir() string {
	return filepath.Join("/tmp", "claude-"+strconv.Itoa(os.Getuid()))
}
