package subagent

import (
	"bufio"
	"encoding/json"
	"io"
	"os"
	"strings"
)

// Event is one thing to draw in the transcript view.
//
//	task    the instruction the subagent was started with
//	user    a later message to it (from its parent, or a follow-up)
//	text    something it said
//	think   a visible reasoning summary
//	tool    a tool call: name, args
//	result  a tool's output, matched to its call by id
type Event struct {
	K    string          `json:"k"`
	Text string          `json:"text,omitempty"`
	ID   string          `json:"id,omitempty"`
	Name string          `json:"name,omitempty"`
	Args json.RawMessage `json:"args,omitempty"`
	Err  bool            `json:"err,omitempty"`
}

type Transcript struct {
	Info
	Events []Event `json:"events"`
	// Next is the offset to ask for next time; Reset says the file shrank
	// (rewritten) and the view should start over.
	Next  int64 `json:"next"`
	Reset bool  `json:"reset,omitempty"`
}

// What one event may carry: a pane is not the place for a 2 MB tool output.
const (
	maxText   = 12000
	maxResult = 6000
	maxArg    = 1500
	maxEvents = 600 // per response; the rest follows on the next poll
)

// Events reads a subagent's transcript from offset after on.
func (t *Tracker) Events(session, id string, after int64) (Transcript, error) {
	t.mu.Lock()
	tr := t.sessions[session]
	var info *Info
	if tr != nil {
		t.refresh(tr, false)
		for i := range tr.all {
			if tr.all[i].ID == id {
				c := tr.all[i]
				info = &c
			}
		}
	}
	t.mu.Unlock()
	if info == nil {
		return Transcript{}, ErrNotFound
	}
	out := Transcript{Info: *info, Events: []Event{}}
	f, err := os.Open(info.path)
	if err != nil {
		return Transcript{}, ErrNotFound
	}
	defer f.Close()
	fi, err := f.Stat()
	if err != nil {
		return Transcript{}, ErrNotFound
	}
	if after < 0 || after > fi.Size() {
		after = 0
		out.Reset = true
	}
	if _, err := f.Seek(after, io.SeekStart); err != nil {
		return Transcript{}, err
	}
	conv := claudeEvents
	if info.Agent == "codex" {
		conv = codexEvents()
	}
	r := bufio.NewReaderSize(f, 64<<10)
	out.Next = after
	for len(out.Events) < maxEvents {
		line, n, err := readLine(r, 32<<20)
		if err != nil {
			break
		}
		out.Next += int64(n)
		out.Events = append(out.Events, conv(line)...)
	}
	return out, nil
}

func claudeEvents(line []byte) []Event {
	var e claudeEntry
	if json.Unmarshal(line, &e) != nil || (e.Type != "user" && e.Type != "assistant") || e.IsMeta {
		return nil
	}
	var m claudeMessage
	if json.Unmarshal(e.Message, &m) != nil {
		return nil
	}
	var s string
	if json.Unmarshal(m.Content, &s) == nil {
		if strings.TrimSpace(s) == "" || harnessNote(s) {
			return nil
		}
		return []Event{{K: "user", Text: clip(s, maxText)}}
	}
	var blocks []block
	if json.Unmarshal(m.Content, &blocks) != nil {
		return nil
	}
	var out []Event
	for _, b := range blocks {
		switch b.Type {
		case "text":
			if strings.TrimSpace(b.Text) == "" || (e.Type == "user" && harnessNote(b.Text)) {
				continue
			}
			k := "text"
			if e.Type == "user" {
				k = "user"
			}
			out = append(out, Event{K: k, Text: clip(b.Text, maxText)})
		case "thinking":
			if strings.TrimSpace(b.Thinking) != "" {
				out = append(out, Event{K: "think", Text: clip(b.Thinking, maxText)})
			}
		case "tool_use":
			out = append(out, Event{K: "tool", ID: b.ID, Name: b.Name, Args: clipArgs(b.Input)})
		case "tool_result":
			text, _ := contentOf(b.Content)
			out = append(out, Event{K: "result", ID: b.ToolUseID, Text: clip(text, maxResult), Err: b.IsError})
		}
	}
	return out
}

// harnessNote is text Claude Code adds to a conversation for the model's
// benefit (reminders, hook output), not something anybody said.
func harnessNote(s string) bool {
	return strings.HasPrefix(strings.TrimSpace(s), "<system-reminder>")
}

// codexEvents converts rollout lines. The first real user message is the
// task; Codex's injected setup messages are skipped.
func codexEvents() func([]byte) []Event {
	return func(line []byte) []Event {
		var ev codexEvent
		if json.Unmarshal(line, &ev) != nil || ev.Type != "response_item" {
			return nil
		}
		var p codexPayload
		if json.Unmarshal(ev.Payload, &p) != nil {
			return nil
		}
		switch p.Type {
		case "message":
			text := codexText(p.Content)
			if strings.TrimSpace(text) == "" {
				return nil
			}
			switch p.Role {
			case "user":
				if codexInjected(text) {
					return nil
				}
				return []Event{{K: "user", Text: clip(text, maxText)}}
			case "assistant":
				return []Event{{K: "text", Text: clip(text, maxText)}}
			}
		case "reasoning":
			var parts []string
			for _, s := range p.Summary {
				if s.Text != "" {
					parts = append(parts, s.Text)
				}
			}
			if len(parts) > 0 {
				return []Event{{K: "think", Text: clip(strings.Join(parts, "\n"), maxText)}}
			}
		case "function_call":
			return []Event{{K: "tool", ID: p.CallID, Name: p.Name, Args: clipArgs(json.RawMessage(p.Arguments))}}
		case "custom_tool_call":
			args, _ := json.Marshal(map[string]string{"input": clip(p.Input, maxArg)})
			return []Event{{K: "tool", ID: p.CallID, Name: p.Name, Args: args}}
		case "function_call_output", "custom_tool_call_output":
			var s string
			if json.Unmarshal(p.Output, &s) != nil {
				var o struct {
					Output string `json:"output"`
				}
				if json.Unmarshal(p.Output, &o) == nil {
					s = o.Output
				} else {
					s = codexText(p.Output)
				}
			}
			return []Event{{K: "result", ID: p.CallID, Text: clip(s, maxResult)}}
		}
		return nil
	}
}

// clipArgs keeps tool input readable: long string values (a whole file
// being written, a long prompt) are cut, the structure stays.
func clipArgs(raw json.RawMessage) json.RawMessage {
	var v any
	if len(raw) == 0 || json.Unmarshal(raw, &v) != nil {
		if len(raw) == 0 {
			return nil
		}
		b, _ := json.Marshal(clip(string(raw), maxArg))
		return b
	}
	b, err := json.Marshal(clipValue(v, 0))
	if err != nil {
		return nil
	}
	return b
}

func clipValue(v any, depth int) any {
	switch x := v.(type) {
	case string:
		return clip(x, maxArg)
	case map[string]any:
		if depth > 4 {
			return "…"
		}
		for k, val := range x {
			x[k] = clipValue(val, depth+1)
		}
		return x
	case []any:
		if depth > 4 {
			return "…"
		}
		if len(x) > 50 {
			x = append(x[:50], "…")
		}
		for i, val := range x {
			x[i] = clipValue(val, depth+1)
		}
		return x
	}
	return v
}
