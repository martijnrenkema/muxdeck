package server

import (
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/josecarlosrivas/muxdeck/internal/push"
	"github.com/josecarlosrivas/muxdeck/internal/tmux"
)

// The project launcher: a new session started in a project folder with an
// agent already running in it. The projects are the folders under the
// start dir from the settings, the most recently used first: by the last
// launch from here, or the last Claude Code conversation in that folder.

type project struct {
	Name string    `json:"name"`
	Path string    `json:"path"`
	Used time.Time `json:"used,omitzero"`
}

var launches struct {
	mu     sync.Mutex
	loaded bool
	m      map[string]time.Time
}

// stateDir is where the daemon keeps its state files (next to restore.json).
var stateDir string

func launchesPath() string { return filepath.Join(stateDir, "launches.json") }

func loadLaunches() {
	if launches.loaded {
		return
	}
	launches.loaded = true
	launches.m = map[string]time.Time{}
	if stateDir == "" {
		return
	}
	if data, err := os.ReadFile(launchesPath()); err == nil {
		_ = json.Unmarshal(data, &launches.m)
	}
}

func (s *Server) noteLaunch(dir string) {
	launches.mu.Lock()
	defer launches.mu.Unlock()
	loadLaunches()
	launches.m[dir] = time.Now()
	if stateDir == "" {
		return
	}
	if data, err := json.Marshal(launches.m); err == nil {
		_ = os.MkdirAll(filepath.Dir(launchesPath()), 0o700)
		_ = push.WriteFileSync(launchesPath(), data)
	}
}

// claudeUsed is when Claude Code last wrote a conversation for dir: its
// project folder is the path with every non-alphanumeric turned into "-".
func claudeUsed(home, dir string) time.Time {
	enc := strings.Map(func(r rune) rune {
		if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' {
			return r
		}
		return '-'
	}, dir)
	fi, err := os.Stat(filepath.Join(home, ".claude", "projects", enc))
	if err != nil {
		return time.Time{}
	}
	return fi.ModTime()
}

func (s *Server) handleProjects(w http.ResponseWriter, r *http.Request) {
	home, _ := os.UserHomeDir()
	base := expandHome(s.settings.get().NewSession.Dir)
	if base == "" {
		base = home
	} else if !filepath.IsAbs(base) {
		base = filepath.Join(home, base)
	}
	entries, err := os.ReadDir(base)
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]any{"base": base, "projects": []project{}})
		return
	}
	launches.mu.Lock()
	loadLaunches()
	used := make(map[string]time.Time, len(launches.m))
	for k, v := range launches.m {
		used[k] = v
	}
	launches.mu.Unlock()
	out := []project{}
	for _, e := range entries {
		name := e.Name()
		if !e.IsDir() || strings.HasPrefix(name, ".") || strings.HasPrefix(name, "_") {
			continue
		}
		p := project{Name: name, Path: filepath.Join(base, name)}
		p.Used = claudeUsed(home, p.Path)
		if t := used[p.Path]; t.After(p.Used) {
			p.Used = t
		}
		out = append(out, p)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if !out[i].Used.Equal(out[j].Used) {
			return out[i].Used.After(out[j].Used)
		}
		return strings.ToLower(out[i].Name) < strings.ToLower(out[j].Name)
	})
	writeJSON(w, http.StatusOK, map[string]any{"base": base, "projects": out})
}

// launchPlan checks a launcher request and says what to run where.
func launchPlan(name, dir, agent string) (string, string, error) {
	if !tmux.ValidName(name) {
		return "", "", tmux.ErrBadName
	}
	dir = expandHome(dir)
	if fi, err := os.Stat(dir); !filepath.IsAbs(dir) || err != nil || !fi.IsDir() {
		return "", "", errors.New("no such folder: " + dir)
	}
	cmds := map[string]string{
		"shell":         "",
		"claude":        "claude -n " + name, // Claude's own session name matches the tmux one
		"claude-resume": "claude --resume",   // its picker lists this folder's conversations
		"codex":         "codex",
		"codex-resume":  "codex resume",
		"gemini":        "gemini",
		"gemini-resume": "gemini --resume latest",
	}
	cmd, ok := cmds[agent]
	if !ok {
		return "", "", errors.New("unknown agent: " + agent)
	}
	return dir, cmd, nil
}
