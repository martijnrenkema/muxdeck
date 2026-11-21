package subagent

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

func writeLines(t *testing.T, p string, lines ...any) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	var b strings.Builder
	for _, l := range lines {
		j, err := json.Marshal(l)
		if err != nil {
			t.Fatal(err)
		}
		b.Write(j)
		b.WriteByte('\n')
	}
	f, err := os.OpenFile(p, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	f.WriteString(b.String())
	f.Close()
}

type M = map[string]any

func claudeFixture(t *testing.T, home string) (parent, subdir string) {
	proj := filepath.Join(home, ".claude", "projects", "-home-x")
	parent = filepath.Join(proj, "sess.jsonl")
	writeLines(t, parent, M{"type": "user", "message": M{"content": "hi"}})
	subdir = filepath.Join(proj, "sess", "subagents")
	now := time.Now().UTC().Format(time.RFC3339)
	// a finished one
	writeLines(t, filepath.Join(subdir, "agent-aaa.jsonl"),
		M{"type": "user", "timestamp": now, "message": M{"content": "Find the bug"}},
		M{"type": "assistant", "message": M{"content": []M{{"type": "tool_use", "id": "t1", "name": "Bash", "input": M{"command": "ls"}}}}},
		M{"type": "user", "message": M{"content": []M{{"type": "tool_result", "tool_use_id": "t1", "content": "a.go"}}}},
		M{"type": "assistant", "message": M{"stop_reason": "end_turn", "content": []M{{"type": "text", "text": "Found it."}}}},
	)
	os.WriteFile(filepath.Join(subdir, "agent-aaa.meta.json"), []byte(`{"agentType":"Explore","description":"Bug hunt","spawnDepth":1}`), 0o644)
	// a running one, without meta
	writeLines(t, filepath.Join(subdir, "agent-bbb.jsonl"),
		M{"type": "user", "timestamp": now, "message": M{"content": "Write the docs\nsecond line"}},
		M{"type": "assistant", "message": M{"content": []M{{"type": "tool_use", "id": "t2", "name": "Read", "input": M{"file_path": "/x"}}}}},
	)
	return parent, subdir
}

func TestClaudeListAndEvents(t *testing.T) {
	home := t.TempDir()
	parent, subdir := claudeFixture(t, home)
	tr := New(home)
	tr.Track("s", "claude-code", parent, "working")

	list := tr.List("s")
	if len(list) != 2 {
		t.Fatalf("want 2 subagents, got %+v", list)
	}
	if list[0].ID != "bbb" || list[0].State != StateRunning || list[0].Label != "Write the docs" {
		t.Errorf("running first, label from task: %+v", list[0])
	}
	if list[1].ID != "aaa" || list[1].State != StateDone || list[1].Label != "Bug hunt" || list[1].Kind != "Explore" {
		t.Errorf("finished one: %+v", list[1])
	}

	tx, err := tr.Events("s", "aaa", 0)
	if err != nil {
		t.Fatal(err)
	}
	var kinds []string
	for _, e := range tx.Events {
		kinds = append(kinds, e.K)
	}
	if got := strings.Join(kinds, ","); got != "user,tool,result,text" {
		t.Errorf("events = %s", got)
	}
	if tx.Events[2].ID != "t1" || tx.Events[2].Text != "a.go" {
		t.Errorf("result: %+v", tx.Events[2])
	}

	// Incremental: nothing new after Next, then the appended line only.
	tx2, _ := tr.Events("s", "bbb", 0)
	again, _ := tr.Events("s", "bbb", tx2.Next)
	if len(again.Events) != 0 {
		t.Errorf("no new events expected, got %+v", again.Events)
	}
	writeLines(t, filepath.Join(subdir, "agent-bbb.jsonl"),
		M{"type": "user", "toolEndsTurn": true, "message": M{"content": []M{{"type": "tool_result", "tool_use_id": "t2", "content": "ok"}}}})
	more, _ := tr.Events("s", "bbb", tx2.Next)
	if len(more.Events) != 1 || more.Events[0].K != "result" {
		t.Errorf("appended: %+v", more.Events)
	}
	if !tr.Scan() {
		// first Scan always differs from the empty signature
		t.Log("scan reported no change")
	}
	for _, s := range tr.List("s") {
		if s.ID == "bbb" && s.State != StateDone {
			t.Errorf("toolEndsTurn should finish bbb: %+v", s)
		}
	}
	if tr.Scan() {
		t.Errorf("second scan without changes should report none")
	}
}

func TestParentIdleStopsRunning(t *testing.T) {
	home := t.TempDir()
	parent, _ := claudeFixture(t, home)
	tr := New(home)
	tr.Track("s", "claude-code", parent, "idle")
	for _, s := range tr.List("s") {
		if s.ID == "bbb" && s.State != StateStopped {
			t.Errorf("parent gone: want stopped, got %s", s.State)
		}
	}
}

func TestTrackRejectsForeignPaths(t *testing.T) {
	home := t.TempDir()
	other := filepath.Join(home, "secret.jsonl")
	os.WriteFile(other, []byte("{}\n"), 0o644)
	tr := New(home)
	for _, p := range []string{other, filepath.Join(home, ".claude", "projects", "..", "..", "secret.jsonl"), "/etc/passwd"} {
		tr.Track("s", "claude-code", p, "working")
		if len(tr.sessions) != 0 {
			t.Fatalf("accepted %s", p)
		}
	}
}

func TestCodexChildren(t *testing.T) {
	home := t.TempDir()
	day := time.Now()
	dir := filepath.Join(home, ".codex", "sessions", day.Format("2006"), day.Format("01"), day.Format("02"))
	ts := day.Add(-time.Minute).UTC().Format(time.RFC3339)
	root := filepath.Join(dir, "rollout-a-root.jsonl")
	writeLines(t, root, M{"type": "session_meta", "payload": M{"id": "root", "timestamp": ts, "source": "cli"}})
	child := filepath.Join(dir, "rollout-b-child.jsonl")
	writeLines(t, child,
		M{"type": "session_meta", "payload": M{"id": "kid", "timestamp": ts,
			"source": M{"subagent": M{"thread_spawn": M{"parent_thread_id": "root", "depth": 1, "agent_nickname": "Euclid", "agent_role": "explorer"}}}}},
		M{"type": "response_item", "payload": M{"type": "message", "role": "user", "content": []M{{"type": "input_text", "text": "<environment_context>x</environment_context>"}}}},
		M{"type": "response_item", "payload": M{"type": "message", "role": "user", "content": []M{{"type": "input_text", "text": "Map the repo"}}}},
		M{"type": "event_msg", "payload": M{"type": "task_started"}},
		M{"type": "response_item", "payload": M{"type": "function_call", "name": "exec_command", "arguments": `{"cmd":"ls"}`, "call_id": "c1"}},
		M{"type": "response_item", "payload": M{"type": "function_call_output", "call_id": "c1", "output": "a\nb"}},
		M{"type": "response_item", "payload": M{"type": "message", "role": "assistant", "content": []M{{"type": "output_text", "text": "Two files."}}}},
		M{"type": "event_msg", "payload": M{"type": "task_complete"}},
	)
	guardian := filepath.Join(dir, "rollout-c-guard.jsonl")
	writeLines(t, guardian, M{"type": "session_meta", "payload": M{"id": "g", "parent_thread_id": "root", "timestamp": ts, "source": M{"subagent": M{"other": "guardian"}}}})

	tr := New(home)
	tr.Track("s", "codex", root, "working")
	list := tr.List("s")
	if len(list) != 1 {
		t.Fatalf("want only the spawned thread, got %+v", list)
	}
	s := list[0]
	if s.ID != "kid" || s.Label != "Euclid" || s.Kind != "explorer" || s.State != StateDone || s.Agent != "codex" {
		t.Errorf("child: %+v", s)
	}
	tx, err := tr.Events("s", "kid", 0)
	if err != nil {
		t.Fatal(err)
	}
	var kinds []string
	for _, e := range tx.Events {
		kinds = append(kinds, e.K)
	}
	if got := strings.Join(kinds, ","); got != "user,tool,result,text" {
		t.Errorf("events = %s (%+v)", got, tx.Events)
	}
}

func TestDiscoverClaudeFromRegistry(t *testing.T) {
	home := t.TempDir()
	parent, _ := claudeFixture(t, home)
	reg := filepath.Join(home, ".claude", "sessions")
	os.MkdirAll(reg, 0o755)
	// This test process stands in for the Claude process; its parent plays
	// the pane shell.
	pid, ppid := os.Getpid(), os.Getppid()
	os.WriteFile(filepath.Join(reg, "1.json"), []byte(`{"pid":`+itoa(pid)+`,"sessionId":"sess"}`), 0o644)
	tr := New(home)
	tr.DiscoverClaude(map[string][]int{"work": {ppid}})
	if got := tr.sessions["work"]; got == nil || got.transcript != parent {
		t.Fatalf("not discovered: %+v", got)
	}
	if len(tr.List("work")) != 2 {
		t.Errorf("subagents not listed")
	}
	// A codex reporter on the session keeps its claim.
	tr2 := New(home)
	tr2.sessions["work"] = &tracked{agent: "codex", transcript: "/x.jsonl"}
	tr2.DiscoverClaude(map[string][]int{"work": {ppid}})
	if tr2.sessions["work"].agent != "codex" {
		t.Errorf("discovery overrode another agent")
	}
}

func itoa(n int) string { return strconv.Itoa(n) }

func TestRelayPrompt(t *testing.T) {
	c := RelayPrompt(Info{Agent: "claude-code", Label: "Bug hunt", Address: "aaa"}, "  stop at the parser\nthanks ")
	if !strings.Contains(c, `SendMessage with to: "aaa"`) || !strings.HasSuffix(c, "stop at the parser\nthanks") || !strings.Contains(c, `"Bug hunt"`) {
		t.Errorf("claude prompt: %q", c)
	}
	x := RelayPrompt(Info{Agent: "codex", Label: "Euclid", Address: "/root/euclid"}, "hi")
	if !strings.Contains(x, `followup_task with target: "/root/euclid"`) {
		t.Errorf("codex prompt: %q", x)
	}
}

func TestClaudeLineIgnoresMetaAndStopSequence(t *testing.T) {
	fs := &fileState{state: StateRunning}
	claudeLine(fs, []byte(`{"type":"assistant","message":{"stop_reason":"stop_sequence","content":[{"type":"text","text":"done"}]}}`))
	if fs.state != StateDone {
		t.Fatalf("stop_sequence: state %q, want done", fs.state)
	}
	claudeLine(fs, []byte(`{"type":"user","isMeta":true,"message":{"content":"[SYSTEM NOTIFICATION] something"}}`))
	if fs.state != StateDone {
		t.Fatalf("meta line moved the state to %q", fs.state)
	}
}

func TestRegistryPane(t *testing.T) {
	for in, want := range map[string]string{"mux:@18.%18": "%18", "": "", "x:@1.%": "", "a:@1.%2b": ""} {
		if got := registryPane(in); got != want {
			t.Errorf("registryPane(%q) = %q, want %q", in, got, want)
		}
	}
}
