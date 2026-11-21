// Package tmux wraps the tmux CLI for session management.
package tmux

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// Bin is the resolved tmux binary. GUI-launched processes (the desktop
// app's sidecar, launchd without a PATH override) inherit a PATH without
// Homebrew or MacPorts, so a bare "tmux" fails on most Macs — probe the
// standard install locations before giving up. Found=false lets the API
// report "tmux missing" as a state instead of a cryptic exec error.
// MUXDECK_TMUX overrides the search entirely.
var Bin, Found = findTmux()

func findTmux() (string, bool) {
	if p := os.Getenv("MUXDECK_TMUX"); p != "" {
		_, err := os.Stat(p)
		return p, err == nil
	}
	if p, err := exec.LookPath("tmux"); err == nil {
		return p, true
	}
	for _, p := range []string{
		"/opt/homebrew/bin/tmux",
		"/usr/local/bin/tmux",
		"/opt/local/bin/tmux",
	} {
		if _, err := os.Stat(p); err == nil {
			return p, true
		}
	}
	return "tmux", false
}

var nameRe = regexp.MustCompile(`^[A-Za-z0-9_-]{1,64}$`)

// ErrBadName is returned when a session name fails validation.
var ErrBadName = errors.New("session name must match [A-Za-z0-9_-]{1,64}")

type Session struct {
	Name     string    `json:"name"`
	Windows  int       `json:"windows"`
	Created  time.Time `json:"created"`
	Attached int       `json:"attached"`
	Activity int64     `json:"activity"`
	Command  string    `json:"command,omitempty"`
	Path     string    `json:"path,omitempty"`
	// From is the session whose agent started this one (muxdeck-msg --new
	// sets the @muxdeck-from option); empty for sessions a person made.
	From string `json:"from,omitempty"`
	// Quiet: alert when the session's output goes quiet (@muxdeck-quiet).
	Quiet bool `json:"quiet,omitempty"`
	// Output is when the current window last printed something (Unix s);
	// Activity is the last input.
	Output int64 `json:"-"`

	id string // tmux's own session id ("$3"): survives a rename
}

// ServerID names the running tmux server (pid and start time), so a
// restart can be told from the same server carrying on. Empty when no
// server runs.
func ServerID() (string, error) {
	out, err := outputRetry("display-message", "-p", "#{pid}-#{start_time}")
	if err != nil {
		if gone(out) {
			return "", nil
		}
		return "", cmdErr("display-message", out, err)
	}
	return strings.TrimSpace(string(out)), nil
}

// ExitEmpty reports whether the server stops by itself once its last
// session ends (tmux's exit-empty, on by default and when it cannot tell).
func ExitEmpty() bool {
	out, err := outputRetry("display-message", "-p", "#{exit-empty}")
	return err != nil || strings.TrimSpace(string(out)) != "0"
}

// SetQuiet turns the quiet alert of a session on or off.
func SetQuiet(name string, on bool) error {
	if !ValidName(name) {
		return ErrBadName
	}
	target := "=" + name + ":"
	if on {
		return runRetry("set-option", "-t", target, "@muxdeck-quiet", "1")
	}
	out, err := outputRetry("set-option", "-u", "-t", target, "@muxdeck-quiet")
	if err != nil && !gone(out) {
		return cmdErr("set-option", out, err)
	}
	return nil
}

// Key names a session for what has to outlive a rename (its upload dir):
// creation time plus tmux's session id, unique even across tmux restarts.
func (s Session) Key() string {
	if s.id == "" {
		return s.Name
	}
	return fmt.Sprintf("%d-%s", s.Created.Unix(), strings.TrimPrefix(s.id, "$"))
}

// ErrNoSession is returned when tmux answered and the session is not there.
var ErrNoSession = errors.New("no such session")

// Key returns the rename-proof key of the named session.
func Key(name string) (string, error) {
	if !ValidName(name) {
		return "", ErrBadName
	}
	out, err := outputRetry("list-sessions", "-F", "#{session_created}|#{session_id}|#{session_name}")
	if err != nil {
		if gone(out) {
			return "", ErrNoSession
		}
		return "", cmdErr("list-sessions", out, err)
	}
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		f := strings.SplitN(line, "|", 3)
		if len(f) == 3 && f[2] == name {
			created, _ := strconv.ParseInt(f[0], 10, 64)
			return Session{Name: name, Created: time.Unix(created, 0), id: f[1]}.Key(), nil
		}
	}
	return "", ErrNoSession
}

func ValidName(name string) bool { return nameRe.MatchString(name) }

func run(args ...string) error {
	out, err := exec.Command(Bin, args...).CombinedOutput()
	if err != nil {
		return cmdErr(args[0], out, err)
	}
	return nil
}

// cmdErr names the tmux subcommand and what tmux said, falling back to the
// exec error itself when tmux printed nothing (killed, fork failed) — an
// empty "tmux list-sessions: " told nobody anything.
func cmdErr(sub string, out []byte, err error) error {
	msg := strings.TrimSpace(string(out))
	if msg == "" {
		msg = err.Error()
	}
	return fmt.Errorf("tmux %s: %s", sub, msg)
}

// retryDelays spaces the extra attempts for commands that are safe to repeat.
// A busy or briefly unreachable server (a pane flooding output, a client
// mid-handshake) fails a command now and then; the same command a moment
// later succeeds. Commands that type or create are never retried.
var retryDelays = []time.Duration{60 * time.Millisecond, 200 * time.Millisecond, 500 * time.Millisecond}

// definite reports failures that are tmux's real answer rather than a blip:
// the target does not exist, or there is no server at all.
func definite(out []byte) bool {
	msg := string(out)
	for _, s := range []string{"can't find session", "session not found", "no server running", "error connecting to", "duplicate session", "bad session name"} {
		if strings.Contains(msg, s) {
			return true
		}
	}
	return false
}

// outputRetry runs an idempotent tmux command, retrying failures that are
// not definite. It returns the last attempt's combined output.
func outputRetry(args ...string) ([]byte, error) {
	var out []byte
	var err error
	for i := 0; ; i++ {
		out, err = exec.Command(Bin, args...).CombinedOutput()
		if err == nil || definite(out) || i == len(retryDelays) {
			return out, err
		}
		time.Sleep(retryDelays[i])
	}
}

func runRetry(args ...string) error {
	out, err := outputRetry(args...)
	if err != nil {
		return cmdErr(args[0], out, err)
	}
	return nil
}

func gone(out []byte) bool {
	msg := string(out)
	return strings.Contains(msg, "can't find session") || strings.Contains(msg, "session not found") ||
		strings.Contains(msg, "no server running") || strings.Contains(msg, "error connecting to")
}

// List returns all sessions on the default tmux server. A missing server
// (tmux not started yet) is reported as an empty list, not an error.
//
// The format uses "|" separators, numeric fields first and the free-text name
// last (split with a count, so a "|" inside a session name stays in the name).
// TAB is not safe here: tmux ≥3.6 sanitizes control characters to "_" when the
// client runs outside tmux — exactly the daemon case.
func List() ([]Session, error) {
	out, err := outputRetry("list-sessions", "-F",
		"#{session_windows}|#{session_created}|#{session_attached}|#{session_activity}|#{s/[^A-Za-z0-9_-]//:@muxdeck-from}|#{window_activity}|#{s/[^1]//:@muxdeck-quiet}|#{session_id}|#{session_name}")
	if err != nil {
		msg := string(out)
		if strings.Contains(msg, "no server running") || strings.Contains(msg, "error connecting to") {
			return []Session{}, nil
		}
		return nil, cmdErr("list-sessions", out, err)
	}
	sessions := []Session{}
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		f := strings.SplitN(line, "|", 9)
		if len(f) != 9 {
			continue
		}
		windows, _ := strconv.Atoi(f[0])
		created, _ := strconv.ParseInt(f[1], 10, 64)
		attached, _ := strconv.Atoi(f[2])
		activity, _ := strconv.ParseInt(f[3], 10, 64)
		output, _ := strconv.ParseInt(f[5], 10, 64)
		sessions = append(sessions, Session{Name: f[8], From: f[4], Windows: windows, Created: time.Unix(created, 0),
			Attached: attached, Activity: activity, Output: output, Quiet: f[6] == "1", id: f[7]})
	}
	if ctx := contexts(); ctx != nil {
		for i, sess := range sessions {
			if c, ok := ctx[sess.Name]; ok {
				sessions[i].Command, sessions[i].Path = c.Command, c.Path
			}
		}
	}
	return sessions, nil
}

// paneContext is the active pane's command and working directory, the two facts
// that say what a session is actually doing.
type paneContext struct{ Command, Path string }

// contexts reads that pair for every session, keyed by session name.
//
// Pane formats resolve against the session's own active pane here, so unlike
// display-message this works from the daemon with no attached client.
//
// Three free-text fields cannot share a "|"-separated line — a path may
// legitimately contain one — so the two leading fields are byte lengths and
// the payload is sliced by count instead of split. #{n:...} needs a tmux new
// enough to have it, which is why this is a second call rather than extra
// fields on the List format: the session list is load-bearing and must not
// break to decorate a sidebar. A nil map simply means no decoration.
func contexts() map[string]paneContext {
	out, err := exec.Command(Bin, "list-sessions", "-F",
		"#{n:pane_current_command}|#{n:pane_current_path}|#{pane_current_command}#{pane_current_path}#{session_name}").Output()
	if err != nil {
		return nil
	}
	return parseContexts(string(out))
}

func parseContexts(out string) map[string]paneContext {
	ctx := map[string]paneContext{}
	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
		f := strings.SplitN(line, "|", 3)
		if len(f) != 3 {
			continue
		}
		nCmd, errCmd := strconv.Atoi(f[0])
		nPath, errPath := strconv.Atoi(f[1])
		if errCmd != nil || errPath != nil || nCmd < 0 || nPath < 0 || nCmd+nPath > len(f[2]) {
			continue
		}
		if name := f[2][nCmd+nPath:]; name != "" {
			ctx[name] = paneContext{Command: f[2][:nCmd], Path: f[2][nCmd : nCmd+nPath]}
		}
	}
	return ctx
}

// PanePIDs lists the process id of every pane, grouped by session — the roots
// of the process trees each session owns.
//
// Numeric field first and the free-text session name last, split once, for the
// same reason as the session list: a name may legitimately contain the
// separator, and a name that shifts fields would attribute one session's
// processes to another.
func PanePIDs() map[string][]int {
	out, err := exec.Command(Bin, "list-panes", "-a", "-F", "#{pane_pid}|#{session_name}").Output()
	if err != nil {
		return nil
	}
	return parsePanePIDs(string(out))
}

func parsePanePIDs(out string) map[string][]int {
	panes := map[string][]int{}
	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
		pidStr, name, ok := strings.Cut(line, "|")
		if !ok || name == "" {
			continue
		}
		pid, err := strconv.Atoi(pidStr)
		if err != nil || pid <= 0 {
			continue
		}
		panes[name] = append(panes[name], pid)
	}
	if len(panes) == 0 {
		return nil
	}
	return panes
}

// EnsureClipboard makes tmux forward copy-mode yanks to the attached client
// as OSC 52, which the web frontend turns into a browser clipboard write.
// set-clipboard only takes effect when the outer terminal advertises the Ms
// capability, which xterm-256color terminfo does not — hence the override.
func EnsureClipboard() error {
	if err := run("set-option", "-s", "set-clipboard", "on"); err != nil {
		return err
	}
	out, err := exec.Command(Bin, "show-options", "-s", "-v", "terminal-overrides").CombinedOutput()
	if err == nil && strings.Contains(string(out), "Ms=") {
		return nil
	}
	return run("set-option", "-s", "-a", "terminal-overrides", `,xterm*:Ms=\E]52;%p1%s;%p2%s\007`)
}

func New(name string) error { return NewIn(name, "") }

// NewIn creates a session starting in dir, or in the home directory when dir
// is empty.
func NewIn(name, dir string) error {
	if !ValidName(name) {
		return ErrBadName
	}
	args := []string{"new-session", "-d", "-s", name}
	// Without -c the session inherits the tmux server's cwd, which is "/"
	// when the daemon was started by the desktop app or launchd.
	if dir == "" {
		dir, _ = os.UserHomeDir()
	}
	if dir != "" {
		args = append(args, "-c", dir)
	}
	if err := run(args...); err != nil {
		return err
	}
	// Web-first default: wheel/touch scrolling only reaches tmux (instead of
	// the browser page) when the application requests mouse tracking.
	// Option commands reject the "=" exact-match prefix; the session was just
	// created under this validated name, so a plain target is safe here.
	return run("set-option", "-t", name, "mouse", "on")
}

// Mouse reports the effective mouse option for a session (session value,
// falling back to the global value when unset).
func Mouse(name string) (bool, error) {
	if !Has(name) {
		return false, fmt.Errorf("no such session: %s", name)
	}
	out, err := exec.Command(Bin, "show-options", "-t", name, "-v", "mouse").CombinedOutput()
	if err != nil {
		return false, fmt.Errorf("tmux show-options: %s", strings.TrimSpace(string(out)))
	}
	v := strings.TrimSpace(string(out))
	if v == "" {
		out, err = exec.Command(Bin, "show-options", "-g", "-v", "mouse").CombinedOutput()
		if err != nil {
			return false, fmt.Errorf("tmux show-options -g: %s", strings.TrimSpace(string(out)))
		}
		v = strings.TrimSpace(string(out))
	}
	return v == "on", nil
}

func SetMouse(name string, on bool) error {
	if !Has(name) {
		return fmt.Errorf("no such session: %s", name)
	}
	v := "off"
	if on {
		v = "on"
	}
	return runRetry("set-option", "-t", name, "mouse", v)
}

// SendKeys types text into a session's active pane, then optionally Enter.
//
// -l sends the text literally, so a payload that happens to name a tmux key
// ("Enter", "C-c") arrives as those characters instead of as that key —
// which is also why submitting is a separate send and a parameter, rather
// than something a caller can smuggle into the text. "--" is required, not
// decorative: without it a text starting with "-" is parsed as a flag and
// send-keys fails.
//
// The pane target is "=name:" rather than "=name": the "=" keeps tmux from
// prefix-matching a different session, and the trailing ":" is what makes it
// resolve to that session's current pane at all.
func SendKeys(name, text string, enter bool) error {
	if !ValidName(name) {
		return ErrBadName
	}
	target := "=" + name + ":"
	if text != "" {
		if err := run("send-keys", "-t", target, "-l", "--", text); err != nil {
			return err
		}
	}
	if enter {
		return run("send-keys", "-t", target, "Enter")
	}
	return nil
}

// Kill ends a session. A session that is already gone counts as killed: the
// shell may have exited a moment earlier, or a first attempt whose answer
// was lost already did the job — either way the caller's wish holds.
func Kill(name string) error {
	if !ValidName(name) {
		return ErrBadName
	}
	out, err := outputRetry("kill-session", "-t", "="+name)
	if err != nil && !gone(out) {
		return cmdErr("kill-session", out, err)
	}
	return nil
}

func Rename(oldName, newName string) error {
	if !ValidName(oldName) || !ValidName(newName) {
		return ErrBadName
	}
	return run("rename-session", "-t", "="+oldName, newName)
}

func Has(name string) bool {
	ok, err := Exists(name)
	return ok && err == nil
}

// Exists tells "no such session" apart from "tmux did not answer". Anything
// that deletes on a negative answer must use this, not Has: a transient
// failure is not proof that the session ended.
func Exists(name string) (bool, error) {
	if !ValidName(name) {
		return false, ErrBadName
	}
	out, err := outputRetry("has-session", "-t", "="+name)
	if err == nil {
		return true, nil
	}
	if gone(out) {
		return false, nil
	}
	return false, cmdErr("has-session", out, err)
}

// Cwd reports the current working directory of the session's active pane.
// list-panes rather than display-message: the latter expands pane formats
// against the attached client's context and comes back empty when the
// caller isn't a tmux client — exactly the daemon case (seen on tmux 3.6a).
func Cwd(name string) (string, error) {
	if !ValidName(name) {
		return "", ErrBadName
	}
	out, err := outputRetry("list-panes", "-t", "="+name,
		"-F", "#{pane_active}|#{pane_current_path}")
	if err != nil {
		return "", cmdErr("list-panes", out, err)
	}
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if path, ok := strings.CutPrefix(line, "1|"); ok && path != "" {
			return path, nil
		}
	}
	return "", errors.New("could not resolve session cwd")
}

// Fits reports whether the session's current window already has the size a
// client with a cols x rows terminal would give it. tmux sizes a window to
// its most recently active client, so false means another client's size is
// in force. Any doubt counts as fitting: the caller acts only on a mismatch.
func Fits(name string, cols, rows int) bool {
	if !ValidName(name) {
		return true
	}
	out, err := exec.Command(Bin, "display-message", "-p", "-t", "="+name+":",
		"#{window_width} #{window_height} #{status}").Output()
	if err != nil {
		return true
	}
	f := strings.Fields(string(out))
	if len(f) != 3 {
		return true
	}
	w, err1 := strconv.Atoi(f[0])
	h, err2 := strconv.Atoi(f[1])
	if err1 != nil || err2 != nil {
		return true
	}
	status := 0
	switch f[2] {
	case "off":
	case "on":
		status = 1
	default:
		status, _ = strconv.Atoi(f[2])
	}
	return w == cols && h == rows-status
}

// Pane is one pane of a session, with what decides whether text may be
// typed into it.
type Pane struct {
	ID      string // "%18"
	Active  bool   // the active pane of the session's active window
	InMode  bool   // copy mode or similar: keys go to tmux, not the app
	Command string
}

// Panes lists every pane of a session.
func Panes(name string) ([]Pane, error) {
	if !ValidName(name) {
		return nil, ErrBadName
	}
	out, err := outputRetry("list-panes", "-s", "-t", "="+name,
		"-F", "#{pane_id}|#{window_active}#{pane_active}|#{pane_in_mode}|#{pane_current_command}")
	if err != nil {
		if gone(out) {
			return nil, ErrNoSession
		}
		return nil, cmdErr("list-panes", out, err)
	}
	var panes []Pane
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		f := strings.SplitN(line, "|", 4)
		if len(f) != 4 || !strings.HasPrefix(f[0], "%") {
			continue
		}
		panes = append(panes, Pane{ID: f[0], Active: f[1] == "11", InMode: f[2] == "1", Command: f[3]})
	}
	return panes, nil
}

var paneRe = regexp.MustCompile(`^%[0-9]{1,6}$`)

// PasteTo puts text into one pane as a single bracketed paste. Unlike
// SendKeys, a newline inside the text stays a newline: an app that
// understands bracketed paste (Claude Code, Codex) takes the whole text as
// one input instead of submitting at the first line break.
func PasteTo(pane, text string) error {
	if !paneRe.MatchString(pane) {
		return errors.New("bad pane id")
	}
	buf := fmt.Sprintf("muxdeck-paste-%d", time.Now().UnixNano())
	load := exec.Command(Bin, "load-buffer", "-b", buf, "-")
	load.Stdin = strings.NewReader(text)
	if out, err := load.CombinedOutput(); err != nil {
		return cmdErr("load-buffer", out, err)
	}
	if err := run("paste-buffer", "-p", "-d", "-b", buf, "-t", pane); err != nil {
		_ = run("delete-buffer", "-b", buf)
		return err
	}
	return nil
}

// Enter presses Enter in one pane.
func Enter(pane string) error {
	if !paneRe.MatchString(pane) {
		return errors.New("bad pane id")
	}
	return run("send-keys", "-t", pane, "Enter")
}

// ScreenOf returns the visible text of one pane.
func ScreenOf(pane string) (string, error) {
	if !paneRe.MatchString(pane) {
		return "", errors.New("bad pane id")
	}
	out, err := exec.Command(Bin, "capture-pane", "-p", "-t", pane).CombinedOutput()
	if err != nil {
		return "", cmdErr("capture-pane", out, err)
	}
	return string(out), nil
}

// LastInput is when someone last typed into the session from any attached
// client (zero when none is attached).
func LastInput(name string) time.Time {
	if !ValidName(name) {
		return time.Time{}
	}
	out, err := exec.Command(Bin, "list-clients", "-t", "="+name, "-F", "#{client_activity}").Output()
	if err != nil {
		return time.Time{}
	}
	var last int64
	for _, f := range strings.Fields(string(out)) {
		if v, err := strconv.ParseInt(f, 10, 64); err == nil && v > last {
			last = v
		}
	}
	if last == 0 {
		return time.Time{}
	}
	return time.Unix(last, 0)
}

// Screen returns the visible text of a session's current pane.
func Screen(name string) (string, error) {
	if !ValidName(name) {
		return "", ErrBadName
	}
	out, err := exec.Command(Bin, "capture-pane", "-p", "-t", "="+name+":").CombinedOutput()
	if err != nil {
		return "", cmdErr("capture-pane", out, err)
	}
	return string(out), nil
}
