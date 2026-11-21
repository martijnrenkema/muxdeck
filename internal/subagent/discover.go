package subagent

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// claudeLive is one entry of Claude Code's registry of running sessions,
// ~/.claude/sessions/<pid>.json.
type claudeLive struct {
	PID       int    `json:"pid"`
	SessionID string `json:"sessionId"`
	Status    string `json:"status"`    // "busy" or "idle" (waiting for input)
	ProcStart string `json:"procStart"` // field 22 of /proc/<pid>/stat
	Cwd       string `json:"cwd"`
	Socket    string `json:"messagingSocketPath"`
	Kind      string `json:"kind"` // "interactive" for a TUI session
	Tmux      string `json:"tmux"` // "session:@window.%pane"
	UpdatedAt int64  `json:"updatedAt"`
}

// Live is a running Claude Code session found in the registry.
type Live struct {
	Session   string
	Pane      string // "%18", from the registry
	Busy      bool
	SessionID string // Claude's conversation id, what --resume takes
	Cwd       string // where Claude runs; --resume finds the conversation from there
	Socket    string // Claude's inbox for messages from other sessions
}

// procStart is field 22 (start time in clock ticks) of a process, which
// tells a live Claude from a stale registry file whose pid was reused.
func procStart(pid int) string {
	b, err := os.ReadFile("/proc/" + strconv.Itoa(pid) + "/stat")
	if err != nil {
		return ""
	}
	s := string(b)
	j := strings.LastIndexByte(s, ')')
	if j < 0 {
		return ""
	}
	f := strings.Fields(s[j+1:])
	if len(f) < 20 {
		return ""
	}
	return f[19]
}

func registryPane(tmux string) string {
	i := strings.LastIndexByte(tmux, '%')
	if i < 0 {
		return ""
	}
	p := tmux[i:]
	for _, c := range p[1:] {
		if c < '0' || c > '9' {
			return ""
		}
	}
	if len(p) < 2 {
		return ""
	}
	return p
}

// DiscoverClaude finds running Claude Code sessions through Claude's own
// registry and tracks their transcripts, so subagents show up without
// waiting for the agent's next hook or statusline — after a daemon restart,
// or while the main agent sits waiting on background helpers. panes maps a
// tmux session to the pids of its panes; a Claude process belongs to the
// session whose pane it descends from.
func (t *Tracker) DiscoverClaude(panes map[string][]int) []Live {
	if len(panes) == 0 {
		return nil
	}
	var found []Live
	paneOf := map[int]string{}
	for name, pids := range panes {
		for _, p := range pids {
			paneOf[p] = name
		}
	}
	regDir := filepath.Join(filepath.Dir(t.claudeRoot), "sessions")
	files, _ := filepath.Glob(filepath.Join(regDir, "*.json"))
	type entry struct {
		live Live
		path string
		at   int64
	}
	bySession := map[string][]entry{}
	for _, f := range files {
		b, err := os.ReadFile(f)
		if err != nil {
			continue
		}
		var live claudeLive
		if json.Unmarshal(b, &live) != nil || live.PID <= 1 || live.SessionID == "" ||
			strings.ContainsAny(live.SessionID, `/\`) {
			continue
		}
		// claude -p started from a pane is no session of that pane's own.
		if live.Kind != "" && live.Kind != "interactive" {
			continue
		}
		// A registry file a crashed Claude left behind, its pid now reused.
		if live.ProcStart != "" && procStart(live.PID) != live.ProcStart {
			continue
		}
		session := ancestorPane(live.PID, paneOf)
		if session == "" {
			continue
		}
		l := Live{Session: session, Pane: registryPane(live.Tmux), Busy: live.Status == "busy",
			SessionID: live.SessionID, Cwd: live.Cwd, Socket: live.Socket}
		bySession[session] = append(bySession[session], entry{l, t.claudeTranscript(live.SessionID), live.UpdatedAt})
	}
	for session, es := range bySession {
		// Two Claudes in one tmux session: keep the transcript already
		// tracked while its Claude lives, so the list does not flip between
		// them on every scan; otherwise the most recently active one.
		t.mu.Lock()
		cur := t.sessions[session]
		t.mu.Unlock()
		best := -1
		for i, e := range es {
			if cur != nil && e.path != "" && e.path == cur.transcript {
				best = i
				break
			}
			if best < 0 || e.at > es[best].at {
				best = i
			}
		}
		e := es[best]
		found = append(found, e.live)
		if e.path == "" {
			continue
		}
		// A reporter for another agent in the same session keeps its claim.
		if cur != nil && cur.agent != "claude-code" {
			continue
		}
		t.Track(session, "claude-code", e.path, "")
	}
	return found
}

func (t *Tracker) claudeTranscript(id string) string {
	t.mu.Lock()
	p, ok := t.idPaths[id]
	t.mu.Unlock()
	if ok {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	m, _ := filepath.Glob(filepath.Join(t.claudeRoot, "*", id+".jsonl"))
	if len(m) == 0 {
		return ""
	}
	t.mu.Lock()
	t.idPaths[id] = m[0]
	t.mu.Unlock()
	return m[0]
}

// ancestorPane walks up from pid until it meets a pane's shell.
func ancestorPane(pid int, paneOf map[int]string) string {
	for i := 0; i < 32 && pid > 1; i++ {
		if name, ok := paneOf[pid]; ok {
			return name
		}
		b, err := os.ReadFile("/proc/" + strconv.Itoa(pid) + "/stat")
		if err != nil {
			return ""
		}
		s := string(b)
		j := strings.LastIndexByte(s, ')')
		if j < 0 {
			return ""
		}
		f := strings.Fields(s[j+1:])
		if len(f) < 2 {
			return ""
		}
		pid, _ = strconv.Atoi(f[1])
	}
	return ""
}
