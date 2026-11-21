package server

import "testing"

func TestDialogOpen(t *testing.T) {
	prompt := "● Done.\n\n────────\n> \n────────\n  ? for shortcuts\n"
	if dialogOpen(prompt) {
		t.Error("plain prompt seen as a dialog")
	}
	claude := "Bash command\n  rm -rf build\n\n Do you want to proceed?\n ❯ 1. Yes\n   2. No\n"
	codex := "Would you like to run the following command?\n  $ ls\n› 1. Yes, proceed (y)\n"
	for _, s := range []string{claude, codex} {
		if !dialogOpen(s) {
			t.Errorf("dialog not seen: %q", s)
		}
	}
	// Old output scrolled far above the bottom does not count.
	old := "Do you want to proceed?\n"
	for i := 0; i < 40; i++ {
		old += "line\n"
	}
	if dialogOpen(old) {
		t.Error("dialog text far above the bottom counted")
	}
}
