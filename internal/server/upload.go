package server

import (
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/josecarlosrivas/muxdeck/internal/tmux"
)

// maxUpload caps one dropped or pasted file. Screenshots are a few MB; the
// cap is there so a stray drag of a video doesn't fill the disk.
const maxUpload = 50 << 20

// uploadSweep is how often orphaned upload dirs are reaped. A session that
// ends by its shell exiting never goes through handleKill, so the sweep is
// what cleans up after it.
const uploadSweep = 30 * time.Second

var unsafeFileChars = regexp.MustCompile(`[^A-Za-z0-9._-]+`)

// uploadRoot holds one directory per tmux session, named after it, so a
// session's files can go when the session does.
func uploadRoot() string {
	base, err := os.UserCacheDir()
	if err != nil {
		base = os.TempDir()
	}
	return filepath.Join(base, "muxdeck", "uploads")
}

// cleanFileName keeps a dropped file's name recognizable but safe to paste
// into a shell or a Claude Code prompt unquoted: no spaces, no separators.
func cleanFileName(name string) string {
	name = filepath.Base(strings.ReplaceAll(name, "\\", "/"))
	name = strings.Trim(unsafeFileChars.ReplaceAllString(name, "_"), "._")
	if len(name) > 80 {
		ext := filepath.Ext(name)
		if len(ext) > 10 {
			ext = ""
		}
		name = name[:80-len(ext)] + ext
	}
	if name == "" {
		name = "upload"
	}
	return name
}

// handleUpload stores the request body as a file for the session and returns
// its absolute path. The browser pastes that path into the terminal; Claude
// Code turns a pasted image path into an attachment.
func (s *Server) handleUpload(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if !tmux.ValidName(name) {
		writeErr(w, tmux.ErrBadName)
		return
	}
	// The dir is named by the session's rename-proof key, so paths already
	// pasted keep working when the session is renamed. Only a definite "no"
	// refuses: if tmux did not answer, the browser showed this session a
	// moment ago, so take the file (under its name, swept like a key).
	key, err := tmux.Key(name)
	switch {
	case errors.Is(err, tmux.ErrNoSession):
		http.Error(w, "no such session", http.StatusNotFound)
		return
	case err != nil:
		log.Printf("uploads: %s: %v (accepting anyway)", name, err)
		key = name
	}
	if r.ContentLength > maxUpload {
		writeJSON(w, http.StatusRequestEntityTooLarge, map[string]string{"error": "file larger than 50 MB"})
		return
	}
	orig, _ := url.QueryUnescape(r.Header.Get("X-Filename"))
	dir := filepath.Join(uploadRoot(), key)
	// Received into a dot file, renamed when complete: the path only exists
	// once the whole file does, and the sweeper never reaps a dir mid-write.
	tmp, err := uploadTemp(dir)
	if err != nil {
		writeErr(w, err)
		return
	}
	markUploading(key, true)
	defer markUploading(key, false)
	_, err = tmp.ReadFrom(http.MaxBytesReader(w, r.Body, maxUpload))
	if cerr := tmp.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		os.Remove(tmp.Name())
		var tooBig *http.MaxBytesError
		if errors.As(err, &tooBig) {
			writeJSON(w, http.StatusRequestEntityTooLarge, map[string]string{"error": "file larger than 50 MB"})
			return
		}
		// The body broke off: the browser or the network gave up, not us.
		log.Printf("uploads: %s: receiving %q: %v", name, orig, err)
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "upload interrupted"})
		return
	}
	path, err := claimUpload(dir, tmp.Name(), cleanFileName(orig))
	if err != nil {
		os.Remove(tmp.Name())
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]string{"path": path})
}

// uploadTemp creates the receiving file, recreating the session dir if it
// vanished in between (a sweep, a kill racing a drop).
func uploadTemp(dir string) (*os.File, error) {
	var err error
	for i := 0; i < 3; i++ {
		if err = os.MkdirAll(dir, 0o700); err != nil {
			continue
		}
		var f *os.File
		if f, err = os.CreateTemp(dir, ".incoming-*"); err == nil {
			return f, nil
		}
	}
	return nil, err
}

// claimUpload gives a received file its final name without overwriting an
// earlier upload: a hard link fails on an existing name where a rename would
// silently replace it.
func claimUpload(dir, tmp, file string) (string, error) {
	stamp := time.Now().Format("150405")
	for i := 1; i <= 100; i++ {
		path := filepath.Join(dir, fmt.Sprintf("%s-%s", stamp, file))
		if i > 1 {
			path = filepath.Join(dir, fmt.Sprintf("%s-%d-%s", stamp, i, file))
		}
		err := os.Link(tmp, path)
		if err == nil {
			os.Remove(tmp)
			return path, nil
		}
		if !errors.Is(err, os.ErrExist) {
			return "", err
		}
	}
	return "", errors.New("no free file name")
}

var uploading = struct {
	sync.Mutex
	n map[string]int
}{n: map[string]int{}}

func markUploading(session string, on bool) {
	uploading.Lock()
	defer uploading.Unlock()
	if on {
		uploading.n[session]++
	} else if uploading.n[session]--; uploading.n[session] <= 0 {
		delete(uploading.n, session)
	}
}

func isUploading(session string) bool {
	uploading.Lock()
	defer uploading.Unlock()
	return uploading.n[session] > 0
}

var uploadKeyRe = regexp.MustCompile(`^[0-9]+-[0-9]+$`)

// removeUploads deletes a session's upload dir, by key and by its name
// (dirs written before keys, or while tmux did not answer).
func removeUploads(name, key string) {
	for _, d := range []string{key, name} {
		if !(tmux.ValidName(d) || uploadKeyRe.MatchString(d)) {
			continue
		}
		if err := os.RemoveAll(filepath.Join(uploadRoot(), d)); err != nil {
			log.Printf("uploads: remove %s: %v", d, err)
		}
	}
}

// sweepUploads removes the upload dirs of sessions that no longer exist.
//
// Deleting is the one thing here that cannot be taken back — a path already
// pasted into Claude Code would point at nothing — so it needs proof: one
// session list that tmux actually answered, a dir missing from it on two
// sweeps in a row, no upload in flight, and nothing written in the last
// minutes (a session created and dropped on between two sweeps).
func sweepUploads(missing map[string]bool) {
	if !tmux.Found {
		return
	}
	entries, err := os.ReadDir(uploadRoot())
	if err != nil {
		return
	}
	sessions, err := tmux.List()
	if err != nil {
		log.Printf("uploads: sweep skipped: %v", err)
		return
	}
	live := map[string]bool{}
	for _, sess := range sessions {
		live[sess.Name] = true
		live[sess.Key()] = true
	}
	seen := map[string]bool{}
	for _, e := range entries {
		name := e.Name()
		if !e.IsDir() || live[name] || isUploading(name) {
			continue
		}
		if info, err := e.Info(); err != nil || time.Since(info.ModTime()) < uploadGrace {
			continue
		}
		seen[name] = true
		if missing[name] {
			removeUploads(name, "")
		}
	}
	for name := range missing {
		delete(missing, name)
	}
	for name := range seen {
		missing[name] = true
	}
}

// uploadGrace keeps a freshly written dir from being judged by a session
// list that may predate its session.
const uploadGrace = 2 * time.Minute

func StartUploadSweeper() {
	go func() {
		missing := map[string]bool{}
		for {
			sweepUploads(missing)
			time.Sleep(uploadSweep)
		}
	}()
}
