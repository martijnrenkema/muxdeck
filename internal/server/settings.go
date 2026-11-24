package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/josecarlosrivas/muxdeck/internal/push"
)

// Settings are the daemon-wide preferences the settings panel edits. What is
// per device (font, keybar, sidebar details) stays in the browser; this is
// what has to be the same for every device: when to push, how permission
// requests are handled, how a new session starts.
type Settings struct {
	Notify struct {
		Permission bool `json:"permission"`
		Done       bool `json:"done"`
		// Preview puts the start of the agent's last answer in a "done"
		// notification (visible on a lock screen).
		Preview   bool   `json:"preview"`
		Quiet     bool   `json:"quiet"`
		QuietFrom string `json:"quiet_from"`
		QuietTo   string `json:"quiet_to"`
	} `json:"notify"`
	Perms struct {
		Enabled bool `json:"enabled"`
		Wait    int  `json:"wait"`
	} `json:"perms"`
	NewSession struct {
		Dir     string `json:"dir"`
		Command string `json:"command"`
	} `json:"new_session"`
	// Dashboard is an optional link in the sidebar header to a companion page
	// (a status dashboard, say), so the two feel like one tool.
	Dashboard struct {
		URL   string `json:"url"`
		Label string `json:"label"`
	} `json:"dashboard"`
}

// The PermissionRequest hook is configured with a 150 s timeout; a wait past
// that would be cut off by Claude Code before muxdeck answers.
const (
	permWaitMin = 10
	permWaitCap = 140
)

func defaultSettings() Settings {
	var st Settings
	st.Notify.Permission = true
	st.Notify.Done = true
	st.Notify.Preview = true
	st.Notify.QuietFrom = "23:00"
	st.Notify.QuietTo = "07:00"
	st.Perms.Enabled = true
	st.Perms.Wait = int(permWaitDefault / time.Second)
	return st
}

func parseClock(s string) (int, error) {
	var h, m int
	if _, err := fmt.Sscanf(s, "%d:%d", &h, &m); err != nil || h < 0 || h > 23 || m < 0 || m > 59 || len(s) != 5 {
		return 0, fmt.Errorf("time must be HH:MM, got %q", s)
	}
	return h*60 + m, nil
}

func expandHome(p string) string {
	if p == "~" || strings.HasPrefix(p, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, strings.TrimPrefix(p, "~"))
		}
	}
	return p
}

func (st *Settings) validate() error {
	if _, err := parseClock(st.Notify.QuietFrom); err != nil {
		return err
	}
	if _, err := parseClock(st.Notify.QuietTo); err != nil {
		return err
	}
	if st.Perms.Wait < permWaitMin || st.Perms.Wait > permWaitCap {
		return fmt.Errorf("wait must be %d–%d seconds", permWaitMin, permWaitCap)
	}
	st.NewSession.Dir = strings.TrimSpace(st.NewSession.Dir)
	if d := st.NewSession.Dir; d != "" {
		fi, err := os.Stat(expandHome(d))
		if err != nil || !fi.IsDir() {
			return fmt.Errorf("no such directory: %s", d)
		}
	}
	st.NewSession.Command = strings.TrimSpace(st.NewSession.Command)
	if strings.ContainsAny(st.NewSession.Command, "\r\n") || len(st.NewSession.Command) > 200 {
		return errors.New("command must be one line, at most 200 characters")
	}
	st.Dashboard.URL = strings.TrimSpace(st.Dashboard.URL)
	if u := st.Dashboard.URL; u != "" {
		p, err := url.Parse(u)
		if err != nil || (p.Scheme != "http" && p.Scheme != "https") || p.Host == "" || len(u) > 300 {
			return fmt.Errorf("dashboard link must be an http(s) URL, got %q", u)
		}
	}
	st.Dashboard.Label = strings.TrimSpace(st.Dashboard.Label)
	if len(st.Dashboard.Label) > 40 {
		return errors.New("dashboard name must be at most 40 characters")
	}
	return nil
}

// quietNow reports whether the clock is inside the quiet window; a window
// whose end is before its start runs over midnight.
func (st Settings) quietNow(now time.Time) bool {
	if !st.Notify.Quiet {
		return false
	}
	from, err1 := parseClock(st.Notify.QuietFrom)
	to, err2 := parseClock(st.Notify.QuietTo)
	if err1 != nil || err2 != nil || from == to {
		return false
	}
	m := now.Hour()*60 + now.Minute()
	if from < to {
		return m >= from && m < to
	}
	return m >= from || m < to
}

type settingsStore struct {
	mu   sync.Mutex
	path string // empty: in memory only, as in tests
	cur  *Settings
}

func (ss *settingsStore) get() Settings {
	ss.mu.Lock()
	defer ss.mu.Unlock()
	if ss.cur == nil {
		st := defaultSettings()
		if ss.path != "" {
			if data, err := os.ReadFile(ss.path); err == nil {
				// Unmarshal over the defaults, so a field added later starts
				// at its default instead of its zero value.
				if err := json.Unmarshal(data, &st); err != nil {
					// Keep the broken file for a look; running on defaults
					// silently would turn alerts back on unannounced.
					log.Printf("settings: %s unreadable, using defaults (copy kept as .broken): %v", ss.path, err)
					os.WriteFile(ss.path+".broken", data, 0o600)
					st = defaultSettings()
				}
			}
		}
		ss.cur = &st
	}
	return *ss.cur
}

func (ss *settingsStore) set(st Settings) error {
	ss.mu.Lock()
	defer ss.mu.Unlock()
	if ss.path != "" {
		data, err := json.MarshalIndent(st, "", "  ")
		if err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(ss.path), 0o700); err != nil {
			return err
		}
		if err := push.WriteFileSync(ss.path, data); err != nil {
			return err
		}
	}
	ss.cur = &st
	return nil
}

// SetSettingsPath makes settings persistent; without it they live in memory.
func (s *Server) SetSettingsPath(path string) { s.settings.path = path }

func (s *Server) handleSettingsGet(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, s.settings.get())
}

func (s *Server) handleSettingsPut(w http.ResponseWriter, r *http.Request) {
	st := s.settings.get()
	r.Body = http.MaxBytesReader(w, r.Body, 16<<10)
	if err := json.NewDecoder(r.Body).Decode(&st); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	if err := st.validate(); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	if err := s.settings.set(st); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, st)
}

// handleLogout forgets this device's login.
func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	// Not behind auth (a logged-out device may ask too), so it checks for
	// itself that the request is this site's own.
	if foreignWrite(r) {
		http.Error(w, "cross-site request refused", http.StatusForbidden)
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name: tokenCookie, Value: "", Path: "/", MaxAge: -1, HttpOnly: true,
		Secure:   r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https",
		SameSite: http.SameSiteStrictMode,
	})
	w.WriteHeader(http.StatusNoContent)
}

// --- version and update check ---

const releaseURL = "https://api.github.com/repos/josecarlosrivas/muxdeck/releases/latest"

type releaseCheck struct {
	mu      sync.Mutex
	latest  string
	checked time.Time
}

// SetVersion tells the settings panel what is running.
func (s *Server) SetVersion(v string) { s.version = v }

func (s *Server) latestRelease() string {
	rc := &s.release
	rc.mu.Lock()
	defer rc.mu.Unlock()
	if time.Since(rc.checked) < 6*time.Hour {
		return rc.latest
	}
	rc.checked = time.Now()
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(releaseURL)
	if err != nil {
		return rc.latest
	}
	defer resp.Body.Close()
	var rel struct {
		Tag string `json:"tag_name"`
	}
	if resp.StatusCode == http.StatusOK && json.NewDecoder(resp.Body).Decode(&rel) == nil {
		rc.latest = rel.Tag
	}
	return rc.latest
}

func (s *Server) handleVersion(w http.ResponseWriter, r *http.Request) {
	// Builds from this tree are "<upstream tag>-<suffix>"; the tag is what an
	// update moves.
	base, _, _ := strings.Cut(s.version, "-")
	latest := s.latestRelease()
	writeJSON(w, http.StatusOK, map[string]any{
		"version": s.version,
		"latest":  latest,
		"update":  latest != "" && strings.HasPrefix(base, "v") && latest != base,
	})
}
