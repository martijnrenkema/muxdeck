package server

import (
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/josecarlosrivas/muxdeck/internal/agent"
	"github.com/josecarlosrivas/muxdeck/internal/subagent"
)

func TestCheckAnswers(t *testing.T) {
	qs := []askQuestion{{Question: "Which?"}, {Question: "And?"}}
	cases := []struct {
		answers map[string]string
		ok      bool
	}{
		{map[string]string{"Which?": "A", "And?": "B, C"}, true},
		{map[string]string{"Which?": "A"}, false},
		{map[string]string{"Which?": "A", "Other?": "B"}, false},
		{map[string]string{"Which?": "A", "And?": "  "}, false},
		{nil, false},
	}
	for i, c := range cases {
		if got := checkAnswers(qs, c.answers); got != c.ok {
			t.Errorf("case %d: checkAnswers = %v, want %v", i, got, c.ok)
		}
	}
	if checkAnswers(nil, map[string]string{}) {
		t.Error("no questions must not count as answered")
	}
}

func TestLastLine(t *testing.T) {
	if got := lastLine("step 3\nbuild finished\nuser@host:~/x$ \n\n\n"); got != "build finished" {
		t.Errorf("lastLine = %q", got)
	}
	if got := lastLine("Waiting for job 12\n"); got != "Waiting for job 12" {
		t.Errorf("lastLine = %q", got)
	}
	if got := lastLine("$ \n"); got != "$" {
		t.Errorf("lastLine with only a prompt = %q", got)
	}
	if got := lastLine("\n\n"); got != "" {
		t.Errorf("lastLine of blank screen = %q", got)
	}
}

func TestTakenBy(t *testing.T) {
	two := restoreState{Server: "1-1", Boot: "b1", Sessions: []savedSession{{Name: "a"}, {Name: "b"}}}
	one := restoreState{Server: "1-1", Boot: "b1", Sessions: []savedSession{{Name: "a"}}}
	none := map[string]bool{}
	names := func(l []savedSession) string {
		var n []string
		for _, x := range l {
			n = append(n, x.Name)
		}
		return strings.Join(n, ",")
	}
	if lost, _, rebooted := takenBy(one, none, "b1"); len(lost) != 0 || rebooted {
		t.Errorf("last session ending (exit-empty) counted as loss: %v", names(lost))
	}
	if lost, _, _ := takenBy(two, none, "b1"); names(lost) != "a,b" {
		t.Errorf("server dying with two sessions: %q", names(lost))
	}
	if lost, _, _ := takenBy(two, map[string]bool{"b": true}, "b1"); len(lost) != 0 {
		t.Errorf("one gone, the other on the new server: %q", names(lost))
	}
	if lost, _, rebooted := takenBy(one, none, "b2"); names(lost) != "a" || !rebooted {
		t.Errorf("reboot with one session: %q rebooted=%v", names(lost), rebooted)
	}
	keeps := one
	keeps.KeepsEmpty = true
	if lost, _, _ := takenBy(keeps, none, "b1"); names(lost) != "a" {
		t.Errorf("exit-empty off, server gone: %q", names(lost))
	}
	if lost, _, _ := takenBy(one, none, ""); len(lost) != 0 {
		t.Errorf("no boot id, one session: %q", names(lost))
	}
	if _, closing, _ := takenBy(one, none, "b1"); names(closing) != "a" {
		t.Errorf("a single session gone is kept as closing: %q", names(closing))
	}
	if _, closing, _ := takenBy(two, none, "b1"); len(closing) != 0 {
		t.Errorf("two sessions gone are lost, not closing: %q", names(closing))
	}
}

func TestClaudeGone(t *testing.T) {
	s := newTestServer(t)
	s.agents.Set("gone", agent.Status{Agent: "claude-code", State: agent.StateWorking, Model: "M"})
	s.agents.Set("hooks", agent.Status{Agent: "claude-code", State: agent.StateWorking})
	s.agents.Set("other", agent.Status{Agent: "codex", State: agent.StateWorking})
	panes := map[string][]int{"gone": {10}, "hooks": {11}, "other": {12}}
	both := []subagent.Live{{Session: "gone"}, {Session: "other"}}
	s.claudeGone(panes, both)
	// tmux not answering judges nothing, however long it lasts.
	for i := 0; i < regGoneScans+1; i++ {
		s.claudeGone(nil, nil)
	}
	for i := 0; i < regGoneScans-1; i++ {
		s.claudeGone(panes, nil)
	}
	if st, _ := s.agents.Get("gone"); st.State != agent.StateWorking {
		t.Fatalf("judged after %d scans: %s", regGoneScans-1, st.State)
	}
	s.claudeGone(panes, nil)
	if st, _ := s.agents.Get("gone"); st.State != agent.StateIdle || st.Model != "M" {
		t.Errorf("Claude gone from the registry: state %q model %q, want idle M", st.State, st.Model)
	}
	if st, _ := s.agents.Get("hooks"); st.State != agent.StateWorking {
		t.Errorf("a Claude the registry never listed was judged: %s", st.State)
	}
	if st, _ := s.agents.Get("other"); st.State != agent.StateWorking {
		t.Errorf("another agent's status was touched: %s", st.State)
	}
	// Seen again in time: the count starts over.
	s.agents.Set("back", agent.Status{Agent: "claude-code", State: agent.StateWaiting})
	panes["back"] = []int{13}
	back := []subagent.Live{{Session: "back"}}
	s.claudeGone(panes, back)
	s.claudeGone(panes, nil)
	s.claudeGone(panes, nil)
	s.claudeGone(panes, back)
	s.claudeGone(panes, nil)
	s.claudeGone(panes, nil)
	if st, _ := s.agents.Get("back"); st.State != agent.StateWaiting {
		t.Errorf("a Claude that came back was judged gone: %s", st.State)
	}
}

func TestLogoutRefusesOtherSites(t *testing.T) {
	s := newTestServer(t)
	for site, want := range map[string]int{"same-origin": http.StatusNoContent, "same-site": http.StatusForbidden, "cross-site": http.StatusForbidden} {
		req := httptest.NewRequest(http.MethodPost, "/api/logout", nil)
		req.Header.Set("Sec-Fetch-Site", site)
		rec := httptest.NewRecorder()
		s.ServeHTTP(rec, req)
		if rec.Code != want {
			t.Errorf("logout from %s: %d, want %d", site, rec.Code, want)
		}
	}
}

func TestSameSessions(t *testing.T) {
	a := []savedSession{{Name: "x", Dir: "/a"}}
	if !sameSessions(a, []savedSession{{Name: "x", Dir: "/a"}}) {
		t.Error("equal snapshots differ")
	}
	if sameSessions(a, []savedSession{{Name: "x", Dir: "/a", Quiet: true}}) {
		t.Error("quiet change not seen")
	}
	if got := dropSaved([]savedSession{{Name: "x"}, {Name: "y"}}, "x"); len(got) != 1 || got[0].Name != "y" {
		t.Errorf("dropSaved = %v", got)
	}
}

func TestReplyPreview(t *testing.T) {
	got := replyPreview("## Done\n\n**Fixed** the `upload` bug.\n- tests pass\n- deployed\n- more")
	if got != "Done Fixed the upload bug. tests pass" {
		t.Errorf("replyPreview = %q", got)
	}
	if replyPreview("  \n ") != "" {
		t.Error("blank reply should give no preview")
	}
}

func TestLaunchPlan(t *testing.T) {
	dir := t.TempDir()
	if _, cmd, err := launchPlan("proj", dir, "claude"); err != nil || cmd != "claude -n proj" {
		t.Errorf("claude: %q %v", cmd, err)
	}
	if _, cmd, err := launchPlan("proj", dir, "gemini-resume"); err != nil || cmd != "gemini --resume latest" {
		t.Errorf("gemini-resume: %q %v", cmd, err)
	}
	if _, _, err := launchPlan("p;rm", dir, "claude"); err == nil {
		t.Error("bad session name accepted")
	}
	if _, _, err := launchPlan("proj", dir+"/nope", "claude"); err == nil {
		t.Error("missing folder accepted")
	}
	if _, _, err := launchPlan("proj", dir, "vim"); err == nil {
		t.Error("unknown agent accepted")
	}
}

func TestDeliverInbox(t *testing.T) {
	sock := t.TempDir() + "/in.sock"
	l, err := net.Listen("unix", sock)
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	got := make(chan string, 1)
	go func() {
		c, err := l.Accept()
		if err != nil {
			return
		}
		b, _ := io.ReadAll(c)
		c.Close()
		got <- string(b)
	}()
	if err := deliverInbox(sock, "hello\nthere"); err != nil {
		t.Fatal(err)
	}
	line := <-got
	var m struct {
		Type    string `json:"type"`
		Message struct{ Role, Content string }
	}
	if err := json.Unmarshal([]byte(strings.TrimSuffix(line, "\n")), &m); err != nil || !strings.HasSuffix(line, "\n") {
		t.Fatalf("line %q: %v", line, err)
	}
	if m.Type != "user" || m.Message.Role != "user" || m.Message.Content != "hello\nthere" {
		t.Errorf("message = %+v", m)
	}
	if err := deliverInbox("relative.sock", "x"); err == nil {
		t.Error("relative socket path accepted")
	}
}

func TestClaudeUsedEncoding(t *testing.T) {
	home := t.TempDir()
	dir := "/home/x/code/shop.example.com"
	if err := os.MkdirAll(home+"/.claude/projects/-home-x-code-shop-example-com", 0o700); err != nil {
		t.Fatal(err)
	}
	if claudeUsed(home, dir).IsZero() {
		t.Error("Claude project folder not found")
	}
}

func TestApprovalText(t *testing.T) {
	got := approvalText("Bash", "[Production Deploy]", json.RawMessage(`{"command":"bash tools/publish.sh --live"}`))
	if !strings.Contains(got, "bash tools/publish.sh --live") || !strings.Contains(got, "[Production Deploy]") || !strings.Contains(got, "once") {
		t.Errorf("bash approval = %q", got)
	}
	got = approvalText("Write", "", json.RawMessage(`{"file_path":"/etc/x.conf","content":"y"}`))
	if got != "Approved from muxdeck, once: you may use Write on /etc/x.conf." {
		t.Errorf("write approval = %q", got)
	}
}

func TestCleanOutput(t *testing.T) {
	in := "\x1b[32mok\x1b[0m line\nprogress 10%\rprogress 100%\r\n\x1b]0;title\x07end"
	if got := cleanOutput(in); got != "ok line\nprogress 100%\nend" {
		t.Errorf("cleanOutput = %q", got)
	}
}

func TestBlockStore(t *testing.T) {
	var bs blockStore
	for i := 0; i < 7; i++ {
		bs.add(&blockedAction{ID: string(rune('a' + i)), session: "s", Created: time.Now().Add(time.Duration(i) * time.Second)})
	}
	if l := bs.forSession("s"); len(l) != maxBlocks || l[0].ID != "c" {
		t.Errorf("kept %d, oldest %s", len(l), l[0].ID)
	}
	if !bs.clear("s") || len(bs.forSession("s")) != 0 || bs.clear("s") {
		t.Error("clear failed")
	}
}
