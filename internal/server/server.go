// Package server exposes the muxdeck HTTP API and the PTY<->WebSocket bridge.
package server

import (
	"bufio"
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log"
	"mime"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/creack/pty"
	"github.com/gorilla/websocket"

	"github.com/josecarlosrivas/muxdeck/internal/agent"
	"github.com/josecarlosrivas/muxdeck/internal/cloud"
	"github.com/josecarlosrivas/muxdeck/internal/mushrun"
	"github.com/josecarlosrivas/muxdeck/internal/power"
	"github.com/josecarlosrivas/muxdeck/internal/push"
	"github.com/josecarlosrivas/muxdeck/internal/remote"
	"github.com/josecarlosrivas/muxdeck/internal/subagent"
	"github.com/josecarlosrivas/muxdeck/internal/tcc"
	"github.com/josecarlosrivas/muxdeck/internal/tmux"
	"github.com/josecarlosrivas/muxdeck/relay"
)

const tokenCookie = "muxdeck_token"

func init() {
	// Not in Go's built-in MIME table; browsers want the real type for PWA installability.
	mime.AddExtensionType(".webmanifest", "application/manifest+json")
}

type Server struct {
	mux      *http.ServeMux
	token    string
	foldCase bool // generated codes are matched case-insensitively, GitHub-device-auth style
	agents   *agent.Store
	subs     *subagent.Tracker
	regMu    sync.Mutex
	regOwned map[string]bool   // sessions whose status came from Claude's registry
	regPanes map[string]string // session → pane its Claude runs in, per the registry
	regLive  map[string]subagent.Live
	regSeen  map[string]int // sessions the registry had a Claude for → scans it has been missing
	restore  restoreStore
	blocks   blockStore
	convMu   sync.Mutex
	convs    map[string]convInfo // session → conversation its agent hooks reported
	quiet    quietWatch
	prompts  promptQueue
	repos    *repoCache
	ports    *portCache
	remotes  *remote.Manager
	mushruns *mushrun.Manager
	powerm   *power.Manager
	relaym   *relay.Manager
	cloudm   *cloud.Manager
	push     *push.Manager
	perms    permStore
	changes  changes
	watch    watchers
	settings settingsStore
	version  string
	release  releaseCheck
}

// SetPush enables Web Push; without it alerts only reach open clients.
func (s *Server) SetPush(m *push.Manager) { s.push = m }

func New(static fs.FS, token string, foldCase bool, remotes *remote.Manager, mushruns *mushrun.Manager, relaym *relay.Manager, cloudm *cloud.Manager, powerm *power.Manager) *Server {
	s := &Server{mux: http.NewServeMux(), token: token, foldCase: foldCase, agents: agent.NewStore(), subs: newSubagentTracker(), repos: newRepoCache(), ports: newPortCache(), remotes: remotes, mushruns: mushruns, relaym: relaym, cloudm: cloudm, powerm: powerm}
	s.mux.Handle("/", http.FileServerFS(static))
	s.mux.HandleFunc("POST /api/login", s.handleLogin)
	s.mux.HandleFunc("GET /api/sessions", s.auth(s.handleList))
	s.mux.HandleFunc("POST /api/sessions", s.auth(s.handleCreate))
	s.mux.HandleFunc("DELETE /api/sessions/{name}", s.auth(s.handleKill))
	s.mux.HandleFunc("POST /api/sessions/{name}/rename", s.auth(s.handleRename))
	s.mux.HandleFunc("GET /api/sessions/{name}/mouse", s.auth(s.handleMouseGet))
	s.mux.HandleFunc("POST /api/sessions/{name}/mouse", s.auth(s.handleMouseSet))
	s.mux.HandleFunc("GET /api/sessions/{name}/attach", s.auth(s.handleAttach))
	s.mux.HandleFunc("POST /api/sessions/{name}/send", s.auth(s.handleSend))
	s.mux.HandleFunc("POST /api/sessions/{name}/upload", s.auth(s.handleUpload))
	s.mux.HandleFunc("GET /api/sessions/{name}/diff", s.auth(s.handleDiff))
	s.mux.HandleFunc("GET /api/sessions/{name}/files", s.auth(s.handleFiles))
	s.mux.HandleFunc("GET /api/sessions/{name}/file", s.auth(s.handleFile))
	s.mux.HandleFunc("GET /api/sessions/{name}/subagents/{id}", s.auth(s.handleSubagent))
	s.mux.HandleFunc("POST /api/sessions/{name}/subagents/{id}/message", s.auth(s.handleSubagentMessage))
	s.mux.HandleFunc("POST /api/sessions/{name}/prompt", s.auth(s.handlePrompt))
	s.mux.HandleFunc("POST /api/sessions/{name}/quiet", s.auth(s.handleQuiet))
	s.mux.HandleFunc("GET /api/projects", s.auth(s.handleProjects))
	s.mux.HandleFunc("POST /api/agent/blocked", s.auth(s.handleBlockedReport))
	s.mux.HandleFunc("POST /api/agent/blocked/{id}", s.auth(s.handleBlockedAnswer))
	s.mux.HandleFunc("GET /api/sessions/{name}/tasks/{id}", s.auth(s.handleTaskOutput))
	s.mux.HandleFunc("GET /api/restore", s.auth(s.handleRestoreList))
	s.mux.HandleFunc("POST /api/restore", s.auth(s.handleRestore))
	s.mux.HandleFunc("DELETE /api/restore", s.auth(s.handleRestoreDismiss))
	s.mux.HandleFunc("POST /api/agent/status", s.auth(s.handleAgentStatus))
	s.mux.HandleFunc("POST /api/agent/permission", s.auth(s.handlePermissionAsk))
	s.mux.HandleFunc("POST /api/agent/permission/{id}", s.auth(s.handlePermissionAnswer))
	s.mux.HandleFunc("GET /api/changes", s.auth(s.handleChanges))
	s.mux.HandleFunc("GET /api/push", s.auth(s.handlePushKey))
	s.mux.HandleFunc("POST /api/push", s.auth(s.handlePushSubscribe))
	s.mux.HandleFunc("DELETE /api/push", s.auth(s.handlePushSubscribe))
	s.mux.HandleFunc("POST /api/push/test", s.auth(s.handlePushTest))
	s.mux.HandleFunc("GET /api/settings", s.auth(s.handleSettingsGet))
	s.mux.HandleFunc("PUT /api/settings", s.auth(s.handleSettingsPut))
	s.mux.HandleFunc("GET /api/version", s.auth(s.handleVersion))
	s.mux.HandleFunc("POST /api/logout", s.handleLogout)
	s.mux.HandleFunc("GET /api/doctor", s.auth(s.handleDoctor))
	s.mux.HandleFunc("GET /api/meta", s.auth(s.handleMeta))
	s.mux.HandleFunc("POST /api/debug/client-log", s.auth(s.handleClientLog))
	s.mux.HandleFunc("GET /api/mush/runs", s.auth(s.handleMushList))
	s.mux.HandleFunc("POST /api/mush/runs", s.auth(s.handleMushStart))
	s.mux.HandleFunc("GET /api/mush/runs/{id}/stream", s.auth(s.handleMushStream))
	s.mux.HandleFunc("DELETE /api/mush/runs/{id}", s.auth(s.handleMushStop))
	s.mux.HandleFunc("POST /api/mush/runs/{id}/retry", s.auth(s.handleMushRetry))
	s.mux.HandleFunc("POST /api/mush/runs/{id}/resume", s.auth(s.handleMushResume))
	s.mux.HandleFunc("POST /api/mush/runs/{id}/remove", s.auth(s.handleMushRemove))
	s.mux.HandleFunc("GET /api/relay", s.auth(s.handleRelayStatus))
	s.mux.HandleFunc("POST /api/relay", s.auth(s.handleRelaySet))
	s.mux.HandleFunc("GET /api/remotes", s.auth(s.handleRemoteList))
	s.mux.HandleFunc("POST /api/remotes", s.auth(s.handleRemoteAdd))
	s.mux.HandleFunc("DELETE /api/remotes/{name}", s.auth(s.handleRemoteDelete))
	s.mux.HandleFunc("PATCH /api/remotes/{name}", s.auth(s.handleRemotePatch))
	s.mux.HandleFunc("/api/remotes/{name}/proxy/{rest...}", s.auth(s.handleRemoteProxy))
	s.mux.HandleFunc("GET /api/cloud", s.auth(s.handleCloudStatus))
	s.mux.HandleFunc("POST /api/cloud", s.auth(s.handleCloudSignIn))
	s.mux.HandleFunc("DELETE /api/cloud", s.auth(s.handleCloudSignOut))
	s.mux.HandleFunc("POST /api/cloud/sync", s.auth(s.handleCloudSync))
	s.mux.HandleFunc("GET /api/power", s.auth(s.handlePowerStatus))
	s.mux.HandleFunc("POST /api/power", s.auth(s.handlePowerSet))
	go s.watchSubagents()
	return s
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
	s.mux.ServeHTTP(rec, r)
	remoteAddr := r.Header.Get("X-Forwarded-For")
	if remoteAddr == "" {
		remoteAddr = r.RemoteAddr
	}
	log.Printf("%s %s %d %s %s", r.Method, r.URL.Path, rec.status,
		time.Since(start).Round(time.Millisecond), remoteAddr)
}

// statusRecorder captures the response status for the access log while
// passing Hijacker/Flusher through — the WebSocket attach path needs both.
type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(code int) {
	r.status = code
	r.ResponseWriter.WriteHeader(code)
}

func (r *statusRecorder) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	h, ok := r.ResponseWriter.(http.Hijacker)
	if !ok {
		return nil, nil, errors.New("underlying ResponseWriter is not a Hijacker")
	}
	return h.Hijack()
}

func (r *statusRecorder) Flush() {
	if f, ok := r.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

// --- doctor: macOS folder-privacy (TCC) diagnostics ---

// handleDoctor reports TCC state. Passive by default — the UI polls it for
// its banner — because an unsolicited probe can raise consent prompts on a
// screen nobody is watching (see internal/tcc). ?probe is the deliberate
// form behind `muxdeck doctor`.
func (s *Server) handleDoctor(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, tcc.Status(r.URL.Query().Has("probe")))
}

// handleMeta names the machine behind this daemon. Through the relay every
// daemon looks alike, so the UI labels the daemon's own sessions with this
// instead of a generic "Local". MUXDECK_NAME overrides the hostname for
// boxes whose hostname isn't the name the human uses.
func (s *Server) handleMeta(w http.ResponseWriter, r *http.Request) {
	name := os.Getenv("MUXDECK_NAME")
	if name == "" {
		if h, err := os.Hostname(); err == nil {
			name, _, _ = strings.Cut(h, ".")
		}
	}
	writeJSON(w, http.StatusOK, map[string]string{"name": name})
}

// --- remotes ---

func (s *Server) handleRemoteList(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, s.remotes.List())
}

func (s *Server) handleRemoteAdd(w http.ResponseWriter, r *http.Request) {
	var body remote.Remote
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	if err := s.remotes.Add(body); err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, remote.ErrBadRemote) || errors.Is(err, remote.ErrCloudManaged) {
			status = http.StatusBadRequest
		}
		writeJSON(w, status, map[string]string{"error": err.Error()})
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleRemoteDelete(w http.ResponseWriter, r *http.Request) {
	if err := s.remotes.Delete(r.PathValue("name")); err != nil {
		status := http.StatusNotFound
		if errors.Is(err, remote.ErrCloudManaged) {
			status = http.StatusBadRequest
		}
		writeJSON(w, status, map[string]string{"error": err.Error()})
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// --- cloud account ---
// The account is a daemon-side concern: the device token is stored here,
// the proxy presents it, and the browser only ever talks to this daemon.

func (s *Server) handleCloudStatus(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, s.cloudm.Status())
}

func (s *Server) handleCloudSignIn(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Token       string `json:"token"`
		URL         string `json:"url"`
		RelayDomain string `json:"relay_domain"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	st, err := s.cloudm.SignIn(body.URL, body.Token, body.RelayDomain)
	switch {
	case errors.Is(err, cloud.ErrBadToken), errors.Is(err, cloud.ErrBadURL):
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
	case err != nil:
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
	default:
		writeJSON(w, http.StatusOK, st)
	}
}

func (s *Server) handleCloudSignOut(w http.ResponseWriter, r *http.Request) {
	if err := s.cloudm.SignOut(); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleCloudSync(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, s.cloudm.Sync())
}

func (s *Server) handleRemotePatch(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Off *bool `json:"off"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Off == nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	if err := s.remotes.SetOff(r.PathValue("name"), *body.Off); err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleRemoteProxy(w http.ResponseWriter, r *http.Request) {
	s.remotes.Proxy(r.PathValue("name"), r.PathValue("rest"), w, r)
}

// --- auth ---

func (s *Server) authorized(r *http.Request) bool {
	if s.token == "" {
		return true
	}
	candidate := ""
	if c, err := r.Cookie(tokenCookie); err == nil {
		candidate = c.Value
	} else if h := r.Header.Get("Authorization"); len(h) > 7 && h[:7] == "Bearer " {
		candidate = h[7:]
	}
	return s.tokenMatch(candidate)
}

func (s *Server) tokenMatch(candidate string) bool {
	if s.foldCase {
		candidate = strings.ToUpper(candidate)
	}
	return subtle.ConstantTimeCompare([]byte(candidate), []byte(s.token)) == 1
}

func (s *Server) auth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !s.authorized(r) {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		if foreignWrite(r) {
			http.Error(w, "cross-site request refused", http.StatusForbidden)
			return
		}
		next(w, r)
	}
}

// foreignWrite is a state-changing request a page on another origin made
// with this browser's cookie. Every node of a tailnet is the same site
// (<tailnet>.ts.net), so SameSite=Strict alone lets any page served from
// one of them answer a permission request or add a push subscription here.
// Scripts using the bearer token send neither header.
func foreignWrite(r *http.Request) bool {
	switch r.Method {
	case http.MethodGet, http.MethodHead, http.MethodOptions:
		return false
	}
	switch r.Header.Get("Sec-Fetch-Site") {
	case "same-origin", "none":
		return false
	case "same-site", "cross-site":
		return true
	}
	origin := r.Header.Get("Origin")
	if origin == "" {
		return false
	}
	u, err := url.Parse(origin)
	return err != nil || u.Host != r.Host
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	if s.token == "" {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	var body struct {
		Token string `json:"token"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	if !s.tokenMatch(body.Token) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	// Store the canonical token so cookie auth is unaffected by typed case.
	// Persistent: a session cookie dies whenever a phone kills the PWA, which
	// asked for the token on nearly every open.
	http.SetCookie(w, &http.Cookie{
		Name:     tokenCookie,
		Value:    s.token,
		Path:     "/",
		MaxAge:   400 * 24 * 60 * 60,
		HttpOnly: true,
		Secure:   r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https",
		SameSite: http.SameSiteStrictMode,
	})
	w.WriteHeader(http.StatusNoContent)
}

// --- session CRUD ---

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, err error) {
	status := http.StatusInternalServerError
	if errors.Is(err, tmux.ErrBadName) {
		status = http.StatusBadRequest
	} else {
		// The request log shows only "500"; this is the why.
		log.Printf("error: %v", err)
	}
	writeJSON(w, status, map[string]string{"error": err.Error()})
}

func (s *Server) handleList(w http.ResponseWriter, r *http.Request) {
	if !tmux.Found {
		// A machine state, not a request error — the UI turns this into an
		// install hint instead of failing silently.
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "tmux-missing"})
		return
	}
	sessions, err := tmux.List()
	if err != nil {
		writeErr(w, err)
		return
	}
	names := make([]string, len(sessions))
	cwds := make([]string, 0, len(sessions))
	for i, sess := range sessions {
		names[i] = sess.Name
		if sess.Path != "" {
			cwds = append(cwds, sess.Path)
		}
	}
	s.agents.Prune(names)
	s.subs.Prune(names)
	s.repos.prune(cwds)
	listening := s.ports.state()

	type listEntry struct {
		tmux.Session
		repoState
		// Ports the session's process tree is listening on, resolved out of
		// band; absent until the first refresh lands.
		Ports []int         `json:"ports,omitempty"`
		Agent *agent.Status `json:"agent,omitempty"`
		// Open permission requests from the session's agent, oldest first.
		Permissions []*permRequest `json:"permissions,omitempty"`
		// Helpers the session's agent spawned: running, or recently done.
		Subagents []subagent.Info `json:"subagents,omitempty"`
		// Background commands and watches the agent started.
		Tasks []subagent.Task `json:"tasks,omitempty"`
		// Actions auto mode blocked, not yet answered.
		Blocked []*blockedAction `json:"blocked,omitempty"`
	}
	out := make([]listEntry, len(sessions))
	for i, sess := range sessions {
		out[i] = listEntry{Session: sess, repoState: s.repos.state(sess.Path), Ports: listening[sess.Name]}
		if st, ok := s.agents.Get(sess.Name); ok {
			out[i].Agent = &st
		}
		out[i].Permissions = s.perms.forSession(sess.Name)
		out[i].Subagents = s.subs.List(sess.Name)
		out[i].Tasks = s.subs.Tasks(sess.Name)
		out[i].Blocked = s.blocks.forSession(sess.Name)
	}
	writeJSON(w, http.StatusOK, out)
}

// handleAgentStatus ingests self-reported agent state. The payload is
// display data, not commands — unknown sessions are accepted so a status
// posted during session startup isn't lost, and Prune reaps them later.
//
// The status is decoded by embedding rather than copied field by field, so a
// field added to agent.Status reaches the API by existing.
func (s *Server) handleAgentStatus(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Session string `json:"session"`
		// The agent's own transcript, where its subagents are found. Kept
		// out of agent.Status: clients get the subagents, not the path.
		Transcript string `json:"transcript"`
		// The agent's conversation id and directory (from its hooks), so a
		// session lost to a reboot can resume it.
		SessionID string `json:"session_id"`
		Cwd       string `json:"cwd"`
		// Reply is the start of the agent's last answer, for the "done"
		// notification. Not kept.
		Reply string `json:"reply"`
		// Event is the hook event behind the report; a prompt the user
		// sends settles the session's blocked actions.
		Event string `json:"event"`
		agent.Status
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Session == "" || body.Agent == "" {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	if !tmux.ValidName(body.Session) {
		writeErr(w, tmux.ErrBadName)
		return
	}
	if !agent.ValidState(body.State) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": agent.ErrBadState.Error()})
		return
	}
	body.Status.Normalize()
	// One session is reported by more than one caller — a statusline that
	// knows the model and the spend, hooks that know only the transition —
	// so a field the caller left out keeps what the last one said rather than
	// blanking it. Saying so explicitly is what an empty value is for: a zero
	// progress object, or an empty chip list.
	prev, had := s.agents.Get(body.Session)
	if had && prev.Agent == body.Agent {
		if body.Model == "" {
			body.Model = prev.Model
		}
		if body.CostUSD == 0 {
			body.CostUSD = prev.CostUSD
		}
		if body.Progress == nil {
			body.Progress = prev.Progress
		}
		if body.Chips == nil {
			body.Chips = prev.Chips
		}
	}
	s.reportedByAgent(body.Session)
	s.agents.Set(body.Session, body.Status)
	s.subs.Track(body.Session, body.Agent, body.Transcript, body.State)
	s.noteConversation(body.Session, body.Agent, body.SessionID, body.Cwd, body.State)
	if (body.Event == "UserPromptSubmit" || body.State == agent.StateIdle) && s.blocks.clear(body.Session) {
		s.changes.bump()
	}
	s.notifyAgentWaiting(body.Session, prev, had, body.Status, body.Reply)
	if !had || prev.State != body.State {
		s.changes.bump()
	}
	w.WriteHeader(http.StatusNoContent)
}

// handleSend types text into a session — what the browser does over the
// attach socket, without attaching. A one-shot client would resize the
// session to its own dimensions for as long as it stayed, which is not
// something a scripted send should do to a layout somebody is looking at.
//
// This is remote code execution into a shell, deliberately: it is the power
// the attach WebSocket has had since the first commit, behind the same auth.
// enter defaults to true — a send that has to be submitted separately is a
// send that half the callers will forget to submit.
func (s *Server) handleSend(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Text  string `json:"text"`
		Enter *bool  `json:"enter"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	name := r.PathValue("name")
	if !tmux.Has(name) {
		http.Error(w, "no such session", http.StatusNotFound)
		return
	}
	if err := tmux.SendKeys(name, body.Text, body.Enter == nil || *body.Enter); err != nil {
		writeErr(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleCreate(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name string `json:"name"`
		// From the project launcher: a project directory and what to run
		// in it. Without them the settings' start dir and command apply.
		Dir   string `json:"dir"`
		Agent string `json:"agent"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	ns := s.settings.get().NewSession
	dir, command := expandHome(ns.Dir), ns.Command
	if body.Dir != "" || body.Agent != "" {
		var err error
		if dir, command, err = launchPlan(body.Name, body.Dir, body.Agent); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
	}
	if err := tmux.NewIn(body.Name, dir); err != nil {
		writeErr(w, err)
		return
	}
	if body.Dir != "" {
		s.noteLaunch(dir)
	}
	if command != "" {
		if err := tmux.SendKeys(body.Name, command, true); err != nil {
			log.Printf("new session %s: start command: %v", body.Name, err)
		}
	}
	writeJSON(w, http.StatusCreated, map[string]string{"name": body.Name})
}

func (s *Server) handleKill(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	key, _ := tmux.Key(name)
	if err := tmux.Kill(name); err != nil {
		writeErr(w, err)
		return
	}
	removeUploads(name, key)
	s.forgetSession(name)
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleRename(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	if err := tmux.Rename(r.PathValue("name"), body.Name); err != nil {
		writeErr(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleMouseGet(w http.ResponseWriter, r *http.Request) {
	on, err := tmux.Mouse(r.PathValue("name"))
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"enabled": on})
}

func (s *Server) handleMouseSet(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Enabled bool `json:"enabled"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	if err := tmux.SetMouse(r.PathValue("name"), body.Enabled); err != nil {
		writeErr(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// handleClientLog lands the deck's client-side socket forensics (the ws
// freeze hunt) in the daemon log — the only surface readable after the fact
// when the client is an iPad in the field. Best-effort and capped; the
// client never acts on the response.
func (s *Server) handleClientLog(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Entries []map[string]any `json:"entries"`
	}
	if err := json.NewDecoder(io.LimitReader(r.Body, 256<<10)).Decode(&body); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	if len(body.Entries) > 200 {
		body.Entries = body.Entries[:200]
	}
	for _, e := range body.Entries {
		line, _ := json.Marshal(e)
		if len(line) > 512 {
			line = line[:512]
		}
		log.Printf("wsdebug %s: %s", r.RemoteAddr, line)
	}
	w.WriteHeader(http.StatusNoContent)
}

// --- attach: PTY <-> WebSocket bridge ---

// keepAlive pings the socket every 25s so idle connections survive proxies
// that kill quiet WebSockets (the hosted relay path's router times out near
// 55s of silence). WriteControl is safe alongside the data writer. It also
// sends an application-level heartbeat frame: protocol pings are invisible
// to browser JS, so the heartbeat is the client's only proof of a live
// socket on an idle terminal. A nil send skips the heartbeat (streams whose
// data writers aren't serialized with ours).
func keepAlive(conn *websocket.Conn, send func(int, []byte) error, stop <-chan struct{}) {
	t := time.NewTicker(25 * time.Second)
	defer t.Stop()
	for {
		select {
		case <-t.C:
			if conn.WriteControl(websocket.PingMessage, nil, time.Now().Add(10*time.Second)) != nil {
				return
			}
			if send != nil {
				hb, _ := json.Marshal(map[string]any{"type": "hb", "t": time.Now().UnixMilli()})
				if send(websocket.TextMessage, hb) != nil {
					return
				}
			}
		case <-stop:
			return
		}
	}
}

var upgrader = websocket.Upgrader{
	ReadBufferSize:  4096,
	WriteBufferSize: 4096,
	// Same-host origin check: a browser terminal is remote code execution
	// by design, so cross-site WebSocket hijacking must be rejected.
	CheckOrigin: func(r *http.Request) bool {
		origin := r.Header.Get("Origin")
		if origin == "" {
			return true // non-browser client
		}
		u, err := url.Parse(origin)
		return err == nil && u.Host == r.Host
	},
}

type controlMsg struct {
	Type string `json:"type"`
	Data string `json:"data"`
	Cols uint16 `json:"cols"`
	Rows uint16 `json:"rows"`
}

// --- viewers: git diff + file reads, anchored to a session's cwd ---

// sessionCwd resolves the session's active-pane directory or writes the error.
func sessionCwd(w http.ResponseWriter, r *http.Request) (string, bool) {
	name := r.PathValue("name")
	if !tmux.Has(name) {
		http.Error(w, "no such session", http.StatusNotFound)
		return "", false
	}
	cwd, err := tmux.Cwd(name)
	if err != nil {
		writeErr(w, err)
		return "", false
	}
	return cwd, true
}

func git(cwd string, args ...string) (string, error) {
	return gitContext(context.Background(), cwd, args...)
}

func gitContext(ctx context.Context, cwd string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", append([]string{"-C", cwd}, args...)...)
	// Killing git on a cancelled context does not by itself unblock the read:
	// anything git spawned that outlives it — a credential helper, a hook —
	// inherits the output pipe and holds it open. WaitDelay forces the pipe
	// shut shortly after, so a deadline is one the caller can rely on.
	cmd.WaitDelay = time.Second
	out, err := cmd.CombinedOutput()
	if err != nil {
		msg := strings.TrimSpace(string(out))
		if msg == "" {
			msg = err.Error()
		}
		return "", errors.New(msg)
	}
	return string(out), nil
}

const maxViewBytes = 2 << 20

func (s *Server) handleDiff(w http.ResponseWriter, r *http.Request) {
	cwd, ok := sessionCwd(w, r)
	if !ok {
		return
	}
	root, err := git(cwd, "rev-parse", "--show-toplevel")
	if err != nil {
		tcc.Note(cwd, err)
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "not a git repository: " + cwd})
		return
	}
	// Working tree vs HEAD, the whole repo. --no-ext-diff keeps any
	// configured external diff tool from hijacking the output.
	text, err := git(cwd, "diff", "HEAD", "--no-color", "--no-ext-diff")
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	if len(text) > maxViewBytes {
		text = text[:maxViewBytes] + "\n… (diff truncated)\n"
	}
	writeJSON(w, http.StatusOK, map[string]string{"root": strings.TrimSpace(root), "text": text})
}

func (s *Server) handleFiles(w http.ResponseWriter, r *http.Request) {
	cwd, ok := sessionCwd(w, r)
	if !ok {
		return
	}
	// Tracked + untracked-but-not-ignored markdown, relative to the session
	// cwd (ls-files scopes to the subtree it runs in). Outside a repo, fall
	// back to the directory's own *.md entries.
	var files []string
	if out, err := git(cwd, "ls-files", "-co", "--exclude-standard", "--", "*.md", "*.markdown"); err == nil {
		files = strings.Fields(strings.TrimSpace(out))
	} else {
		tcc.Note(cwd, err)
		matches, _ := filepath.Glob(filepath.Join(cwd, "*.md"))
		for _, m := range matches {
			files = append(files, filepath.Base(m))
		}
	}
	if files == nil {
		files = []string{}
	}
	if len(files) > 500 {
		files = files[:500]
	}
	writeJSON(w, http.StatusOK, map[string]any{"cwd": cwd, "files": files})
}

func (s *Server) handleFile(w http.ResponseWriter, r *http.Request) {
	cwd, ok := sessionCwd(w, r)
	if !ok {
		return
	}
	rel := r.URL.Query().Get("path")
	clean := filepath.Clean(rel)
	if rel == "" || filepath.IsAbs(clean) || clean == ".." || strings.HasPrefix(clean, "../") {
		http.Error(w, "bad path", http.StatusBadRequest)
		return
	}
	full := filepath.Join(cwd, clean)
	info, err := os.Stat(full)
	if err != nil || info.IsDir() {
		tcc.Note(cwd, err)
		http.Error(w, "no such file", http.StatusNotFound)
		return
	}
	if info.Size() > maxViewBytes {
		http.Error(w, "file too large", http.StatusRequestEntityTooLarge)
		return
	}
	data, err := os.ReadFile(full)
	if err != nil {
		tcc.Note(cwd, err)
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"path": clean, "mtime": info.ModTime().UnixMilli(), "text": string(data)})
}

func (s *Server) handleAttach(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if !tmux.Has(name) {
		http.Error(w, "no such session", http.StatusNotFound)
		return
	}
	if err := tmux.EnsureClipboard(); err != nil {
		log.Printf("attach %s: clipboard setup: %v", name, err)
	}
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	defer conn.Close()
	// Data frames come from the pty reader and the heartbeat ticker; gorilla
	// allows one concurrent writer, so both go through this.
	var wmu sync.Mutex
	send := func(mt int, data []byte) error {
		wmu.Lock()
		defer wmu.Unlock()
		return conn.WriteMessage(mt, data)
	}
	stopPing := make(chan struct{})
	defer close(stopPing)
	go keepAlive(conn, send, stopPing)

	// Each websocket gets its own tmux client attached to the session, so
	// multiple browsers can view the same session just like multiple terminals.
	// -u forces UTF-8: daemons launched by launchd/systemd/GUI apps carry no
	// LANG/LC_*, and tmux replaces every non-ASCII glyph with "_" for a
	// locale-less client.
	cmd := exec.Command(tmux.Bin, "-u", "attach-session", "-t", "="+name)
	cmd.Env = append(os.Environ(), "TERM=xterm-256color")
	// Start the tmux client at the browser's size when it says so: without
	// it tmux sees the pty's default and resizes the window to 80x24 first,
	// then to the real size a moment later, and the app inside draws twice.
	ptmx, err := pty.StartWithSize(cmd, initialSize(r))
	if err != nil {
		log.Printf("attach %s: pty: %v", name, err)
		send(websocket.TextMessage, []byte(`{"type":"error","data":"failed to start tmux client"}`))
		return
	}
	defer func() {
		ptmx.Close()
		cmd.Process.Kill()
		cmd.Wait()
	}()
	// A viewer that is really looking says so over this socket; its
	// keep-awake claim ends with the socket, whatever else is still up.
	viewer := fmt.Sprintf("attach:%s:%p", name, conn)
	defer s.powerm.Release(viewer)
	defer s.watch.set(name, viewer, false)

	// pty -> ws. Closing conn on exit unblocks the read loop below when the
	// tmux client ends (session killed, server exit).
	go func() {
		defer conn.Close()
		buf := make([]byte, 32*1024)
		for {
			n, err := ptmx.Read(buf)
			if n > 0 {
				if werr := send(websocket.BinaryMessage, buf[:n]); werr != nil {
					return
				}
			}
			if err != nil {
				return
			}
		}
	}()

	// ws -> pty
	for {
		mt, data, err := conn.ReadMessage()
		if err != nil {
			return
		}
		if mt != websocket.TextMessage {
			continue
		}
		var msg controlMsg
		if json.Unmarshal(data, &msg) != nil {
			continue
		}
		switch msg.Type {
		case "input":
			io.WriteString(ptmx, msg.Data)
		case "resize":
			if msg.Cols > 0 && msg.Rows > 0 {
				pty.Setsize(ptmx, &pty.Winsize{Cols: msg.Cols, Rows: msg.Rows})
			}
		case "claim":
			// The tmux client re-reports its size on SIGWINCH even when it
			// has not changed, which makes it the session's latest client:
			// the window takes this viewer's size again after another
			// device had it.
			if ws, err := pty.GetsizeFull(ptmx); err == nil && !tmux.Fits(name, int(ws.Cols), int(ws.Rows)) {
				cmd.Process.Signal(syscall.SIGWINCH)
			}
		case "presence":
			if msg.Data == "active" {
				s.powerm.Acquire(viewer)
			} else {
				s.powerm.Release(viewer)
			}
			s.watch.set(name, viewer, msg.Data == "active")
		}
	}
}

// --- relay tunnel ---

func (s *Server) handleRelayStatus(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, s.relaym.Status())
}

// handleRelaySet applies tunnel configuration. A body with a url (re)configures
// and redials; {"off":true|false} alone toggles the existing config — off keeps
// the credential, and on is also the re-arm after a rejection.
func (s *Server) handleRelaySet(w http.ResponseWriter, r *http.Request) {
	var body struct {
		URL   *string `json:"url"`
		Key   *string `json:"key"`
		Off   *bool   `json:"off"`
		Gated *bool   `json:"gated"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	// Enabling the tunnel on an authless daemon would publish an
	// unauthenticated terminal; only off:true is allowed then. A gated
	// relay authenticates clients itself, so it may front a tokenless
	// daemon — gated comes from the claim response, never assumed.
	enabling := body.URL != nil || (body.Off != nil && !*body.Off)
	gated := body.Gated != nil && *body.Gated || body.Gated == nil && s.relaym.Status().Gated
	if enabling && s.token == "" && !gated {
		http.Error(w, "the daemon has no access token, and the relay would expose it publicly — restart with -token auto (or MUXDECK_TOKEN=auto), then retry", http.StatusBadRequest)
		return
	}
	var err error
	switch {
	case body.URL != nil:
		cfg := relay.Config{URL: *body.URL}
		if body.Key != nil {
			cfg.Key = *body.Key
		}
		if body.Off != nil {
			cfg.Off = *body.Off
		}
		if body.Gated != nil {
			cfg.Gated = *body.Gated
		}
		err = s.relaym.Set(cfg)
	case body.Off != nil:
		err = s.relaym.SetOff(*body.Off)
	default:
		http.Error(w, "nothing to set", http.StatusBadRequest)
		return
	}
	if errors.Is(err, relay.ErrBadConfig) {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, s.relaym.Status())
}

// initialSize reads ?cols=&rows= from an attach request; nil leaves the pty
// at its default.
func initialSize(r *http.Request) *pty.Winsize {
	cols, err1 := strconv.Atoi(r.URL.Query().Get("cols"))
	rows, err2 := strconv.Atoi(r.URL.Query().Get("rows"))
	if err1 != nil || err2 != nil || cols < 2 || rows < 2 || cols > 1000 || rows > 1000 {
		return nil
	}
	return &pty.Winsize{Cols: uint16(cols), Rows: uint16(rows)}
}
