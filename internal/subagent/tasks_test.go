package subagent

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const taskLines = `{"type":"assistant","timestamp":"2026-11-19T10:00:00Z","message":{"content":[{"type":"tool_use","id":"tu1","name":"Bash","input":{"command":"gh run watch 1","description":"Watch the CI run","run_in_background":true}},{"type":"tool_use","id":"tu2","name":"Monitor","input":{"description":"tail deploy log","command":"tail -f x"}},{"type":"tool_use","id":"tu3","name":"Bash","input":{"command":"ls"}}]}}
{"type":"user","timestamp":"2026-11-19T10:00:01Z","message":{"content":[{"type":"tool_result","tool_use_id":"tu1","content":"Command running in background with ID: bci111. Output is being written to: /tmp/claude-1000/p/s/tasks/bci111.output. You will be notified"}]},"toolUseResult":{"stdout":"","backgroundTaskId":"bci111"}}
{"type":"user","timestamp":"2026-11-19T10:00:02Z","message":{"content":[{"type":"tool_result","tool_use_id":"tu2","content":"Monitor started (task bmon22, expires in 30m)"}]},"toolUseResult":{"taskId":"bmon22","timeoutMs":1800000}}
{"type":"queue-operation","operation":"enqueue","timestamp":"2026-11-19T10:05:00Z","content":"<task-notification>\n<task-id>bci111</task-id>\n<tool-use-id>tu1</tool-use-id>\n<output-file>/tmp/claude-1000/p/s/tasks/bci111.output</output-file>\n<status>failed</status>\n<summary>Background command \"Watch the CI run\" failed (exit code 1)</summary>\n</task-notification>"}
{"type":"queue-operation","operation":"remove","timestamp":"2026-11-19T10:05:01Z","content":"<task-notification>\n<task-id>bci111</task-id>\n<status>completed</status>\n</task-notification>"}
{"type":"queue-operation","operation":"enqueue","timestamp":"2026-11-19T10:05:02Z","content":"<task-notification>\n<task-id>unknown9</task-id>\n<status>completed</status>\n</task-notification>"}
`

func TestTaskFold(t *testing.T) {
	tf := &taskFile{pending: map[string]pendingTask{}, tasks: map[string]*Task{}}
	for _, l := range strings.Split(strings.TrimSpace(taskLines), "\n") {
		tf.fold([]byte(l))
	}
	if len(tf.tasks) != 2 {
		t.Fatalf("tasks = %d, want 2 (%v)", len(tf.tasks), tf.tasks)
	}
	ci := tf.tasks["bci111"]
	if ci.Kind != "shell" || ci.Label != "Watch the CI run" || ci.State != TaskFailed || ci.output != "/tmp/claude-1000/p/s/tasks/bci111.output" {
		t.Errorf("ci task = %+v", ci)
	}
	if !strings.Contains(ci.Summary, "exit code 1") || ci.Ended.IsZero() {
		t.Errorf("ci summary/ended = %q %v", ci.Summary, ci.Ended)
	}
	mon := tf.tasks["bmon22"]
	if mon.Kind != "monitor" || mon.State != StateRunning || mon.Label != "tail deploy log" {
		t.Errorf("monitor = %+v", mon)
	}
	if len(tf.pending) != 0 {
		t.Errorf("pending left: %v", tf.pending)
	}
	// Settling: running stays, a parent that went idle stops it.
	now := time.Date(2026, 11, 19, 10, 10, 0, 0, time.UTC)
	list := tf.settle(now, false)
	if len(list) != 2 || list[0].ID != "bmon22" {
		t.Errorf("settled order = %+v", list)
	}
	list = tf.settle(now.Add(40*time.Minute), true)
	if len(list) != 1 || list[0].State != StateStopped {
		t.Errorf("after idle parent and 40 min = %+v", list)
	}
}

func TestAdvanceTasksStartsNearEnd(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "x.jsonl")
	filler := strings.Repeat(`{"type":"user","message":{"content":"x"}}`+"\n", taskTail/40+10)
	if err := os.WriteFile(p, []byte(filler+taskLines), 0o600); err != nil {
		t.Fatal(err)
	}
	tr := New(dir)
	tf := tr.advanceTasks(p)
	if tf == nil || len(tf.tasks) != 2 {
		t.Fatalf("tasks after tail read = %v", tf)
	}
	f, _ := os.OpenFile(p, os.O_APPEND|os.O_WRONLY, 0)
	f.WriteString(`{"type":"queue-operation","operation":"enqueue","timestamp":"2026-11-19T10:06:00Z","content":"<task-notification><task-id>bmon22</task-id><status>completed</status><summary>done</summary></task-notification>"}` + "\n")
	f.Close()
	tf = tr.advanceTasks(p)
	if tf.tasks["bmon22"].State != StateDone {
		t.Errorf("monitor after append = %+v", tf.tasks["bmon22"])
	}
}

func TestToolNow(t *testing.T) {
	cases := map[string]string{
		`{"type":"tool_use","name":"Bash","input":{"command":"go test ./...","description":"Run the tests"}}`: "Bash: Run the tests",
		`{"type":"tool_use","name":"Read","input":{"file_path":"/a/b/server.go"}}`:                            "Read: server.go",
		`{"type":"tool_use","name":"TodoWrite","input":{}}`:                                                   "TodoWrite",
	}
	for in, want := range cases {
		var b block
		if err := jsonUnmarshal(in, &b); err != nil {
			t.Fatal(err)
		}
		if got := toolNow(b); got != want {
			t.Errorf("toolNow(%s) = %q, want %q", in, got, want)
		}
	}
}

func jsonUnmarshal(s string, v any) error { return json.Unmarshal([]byte(s), v) }
