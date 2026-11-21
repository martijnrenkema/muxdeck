package server

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Two uploads of the same name in the same second must both survive, and
// the temporary file must not be left behind.
func TestClaimUploadKeepsEarlierFile(t *testing.T) {
	dir := t.TempDir()
	var paths []string
	for _, body := range []string{"first", "second"} {
		tmp, err := uploadTemp(dir)
		if err != nil {
			t.Fatal(err)
		}
		tmp.WriteString(body)
		tmp.Close()
		path, err := claimUpload(dir, tmp.Name(), "paste.png")
		if err != nil {
			t.Fatal(err)
		}
		paths = append(paths, path)
	}
	for i, want := range []string{"first", "second"} {
		got, err := os.ReadFile(paths[i])
		if err != nil || string(got) != want {
			t.Errorf("%s = %q, %v; want %q", paths[i], got, err, want)
		}
	}
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".incoming") {
			t.Errorf("temporary file left: %s", e.Name())
		}
	}
}

// A session dir removed between drops is recreated rather than failing.
func TestUploadTempRecreatesDir(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "sess")
	f, err := uploadTemp(dir)
	if err != nil {
		t.Fatal(err)
	}
	f.Close()
}
