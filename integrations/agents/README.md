# Agent integrations: status, permission cards, messaging

These scripts connect Claude Code, Codex and Gemini CLI to muxdeck. With
them the sidebar shows what each agent is doing, permission requests and
questions become cards you can answer from the browser (or straight from a
push notification), and agents can message each other or hand work to
another agent in a session of its own.

They go further than the minimal `../claude-code.sh` adapter; use one or the
other for Claude Code, not both.

| file | what it does | install it as |
|---|---|---|
| `muxdeck-agent.sh` | hook and statusline adapter for Claude Code, Codex and Gemini | `~/.claude/muxdeck-agent.sh` |
| `muxdeck-codex-watch` | follows running Codex TUIs (model, limits, questions on screen) | `~/.local/bin/muxdeck-codex-watch` |
| `muxdeck-codex-watch.service` | systemd user unit for the watcher | `~/.config/systemd/user/` |
| `codex-hooks.json` | Codex hooks that call `muxdeck-agent.sh` | `~/.codex/hooks.json` |
| `muxdeck-msg` | agents message each other, or start another agent with a task | `~/.local/bin/muxdeck-msg` |

The install paths are the ones the examples below and `tools/muxdeck-rebuild`
use; any other path works if you adjust the commands.

## Requirements

- Linux: the scripts find an agent's tmux session through `/proc`.
- `tmux`, `jq` and `curl`; `python3` for the Codex watcher.
- A muxdeck built from this tree. The release binaries from upstream lack
  the endpoints these scripts post to.

The scripts talk to `MUXDECK_URL` (default `http://127.0.0.1:8300`). A
loopback bind needs no token; otherwise set `MUXDECK_TOKEN`, or put a line
`MUXDECK_TOKEN=...` in `~/.config/muxdeck/env` (the scripts read it from
there, and a systemd unit can use the same file as `EnvironmentFile`).

```sh
install -m 755 integrations/agents/muxdeck-agent.sh ~/.claude/muxdeck-agent.sh
install -m 755 integrations/agents/muxdeck-msg integrations/agents/muxdeck-codex-watch ~/.local/bin/
```

`tools/muxdeck-rebuild` updates these copies after each build, but only the
ones that exist: installing them is up to you.

## Claude Code

Add the hooks to `~/.claude/settings.json` (merge them with hooks you already
have):

```json
{
  "hooks": {
    "UserPromptSubmit": [{ "hooks": [{ "type": "command", "command": "sh ~/.claude/muxdeck-agent.sh state working", "timeout": 3, "async": true }] }],
    "Notification":     [{ "hooks": [{ "type": "command", "command": "sh ~/.claude/muxdeck-agent.sh state waiting", "timeout": 3, "async": true }] }],
    "Stop":             [{ "hooks": [{ "type": "command", "command": "sh ~/.claude/muxdeck-agent.sh state waiting", "timeout": 3, "async": true }] }],
    "StopFailure":      [{ "hooks": [{ "type": "command", "command": "sh ~/.claude/muxdeck-agent.sh state waiting", "timeout": 5 }] }],
    "SessionEnd":       [{ "hooks": [{ "type": "command", "command": "sh ~/.claude/muxdeck-agent.sh state idle", "timeout": 3, "async": true }] }],
    "PermissionRequest": [{ "matcher": "*", "hooks": [{ "type": "command", "command": "sh ~/.claude/muxdeck-agent.sh permission", "timeout": 150 }] }],
    "PermissionDenied":  [{ "matcher": "*", "hooks": [{ "type": "command", "command": "sh ~/.claude/muxdeck-agent.sh denied", "timeout": 5 }] }]
  }
}
```

Model, context use, the 5-hour and 7-day limits and the diff size come from
the statusline. End your statusline command with this line, run in the
foreground (the script sends in the background itself, but it has to find
its tmux pane while the process tree still exists):

```sh
printf '%s' "$input" | sh "$HOME/.claude/muxdeck-agent.sh" statusline >/dev/null 2>&1
```

where `$input` is the statusline JSON your command read from stdin. Without a
statusline of your own, a two-line script does it:

```sh
#!/bin/sh
input=$(cat)
printf '%s' "$input" | sh "$HOME/.claude/muxdeck-agent.sh" statusline >/dev/null 2>&1
```

How the states map:

- **working** on UserPromptSubmit; **waiting** on Notification, Stop and
  StopFailure; **idle** on SessionEnd.
- **Notification types:** only `permission_prompt`, `elicitation_dialog`,
  `elicitation_url_dialog` and `agent_needs_input` mean "needs you".
  `idle_prompt` (a minute after Stop) keeps the "done" note; other types do
  nothing.
- **Stop with helpers still running:** while subagents, workflows or
  teammates the turn started are still running (`background_tasks`), the
  state stays working ("N background tasks still running") and nothing is
  sent. Shell tasks and monitors do not count: a dev server never ends.
  Otherwise the start of the last reply goes along, and the done
  notification shows it (setting "Show the reply"; the text itself is not
  logged).
- **StopFailure** (API error, rate limit): waiting with "Claude stopped
  (<error>): <text>", which counts as a question, so it notifies during
  quiet hours too.
- **A question answered in the terminal** fires no hook. When the state is
  a question and the transcript has grown since (by more than 3 s), the
  statusline puts it back to working.
- Each hook post carries `session_id` and `cwd`, so "Restore after restart"
  can resume the conversation.

### Permission cards

The PermissionRequest hook shows the request as a card above the terminal,
a row in the sidebar and a push notification, and waits for Allow, Always
allow or Deny, at most the wait set in muxdeck's settings (10 to 140 s; the
hook timeout of 150 s is the limit; `MUXDECK_PERMISSION_WAIT` overrides it).
No answer, "Answer in terminal" or muxdeck unreachable: the hook gives no
output and Claude Code asks in the terminal as usual. While the hook waits,
the question is not in the terminal.

AskUserQuestion comes as a card with its options, a field for your own
answer and "Answer in terminal". One question with at most three options can
also be answered from the notification. ExitPlanMode gets its own input back
as `updatedInput` on Allow, as the hook docs require. A tap that arrives
after the hook gave up gets 409/404.

### Blocked by auto mode

The PermissionDenied hook reports the tool, its input and the reason to
muxdeck and gives no output itself, so the block stands. muxdeck shows a
card, a row in the sidebar and a push. "Allow once" types an approval naming
that exact action into the session (typed rather than sent through the
inbox, because it has to be your own message); "Leave it" closes the card.
The card also goes on your next prompt or when the agent stops; at most five
per session, gone after an hour.

### Subagents and background tasks

No setup needed. The daemon finds a conversation through Claude Code's
session registry (`~/.claude/sessions/<pid>.json`), lists the subagents it
spawns under the session (tap one to read along or send it a message) and
follows background shell tasks and monitors from the transcript (tap one for
the tail of its output).

## Codex

Two parts, both optional:

- **Hooks** (Codex 0.167+): copy `codex-hooks.json` to `~/.codex/hooks.json`
  and trust them once with `/hooks` in Codex. They report working, done,
  interrupted and idle, and show permission requests as cards (without
  "Always": Codex refuses `updatedPermissions`).
- **Watcher**: model, context and limit chips, and the questions Codex shows
  on screen (approval prompt, folder trust, update offer), which no hook
  reports. Install the unit and start it:

  ```sh
  cp integrations/agents/muxdeck-codex-watch.service ~/.config/systemd/user/
  systemctl --user daemon-reload
  systemctl --user enable --now muxdeck-codex-watch
  ```

  The watcher reads the rollout log the Codex TUI keeps open under a tmux
  pane. When the hooks speak for a conversation (they write
  `$XDG_RUNTIME_DIR/muxdeck-agent/<session>.sid`), the watcher takes their
  state and only adds model, chips and on-screen questions.

Codex 0.166+ runs a shared app-server daemon by default. It holds the
rollout log open, so the watcher cannot find Codex under its pane, and it
runs commands with stale TMUX variables, so `muxdeck-msg` cannot tell which
session it is in. Turn it off:

```sh
codex features disable daemon_auto_start
codex app-server daemon stop
```

## Gemini CLI

Hooks in `~/.gemini/settings.json` call the same script with
`MUXDECK_AGENT=gemini`:

```json
{
  "hooks": {
    "BeforeAgent":  [{ "hooks": [{ "name": "muxdeck", "type": "command", "command": "MUXDECK_AGENT=gemini sh ~/.claude/muxdeck-agent.sh state working", "timeout": 3000 }] }],
    "AfterAgent":   [{ "hooks": [{ "name": "muxdeck", "type": "command", "command": "MUXDECK_AGENT=gemini sh ~/.claude/muxdeck-agent.sh state waiting", "timeout": 3000 }] }],
    "Notification": [{ "hooks": [{ "name": "muxdeck", "type": "command", "command": "MUXDECK_AGENT=gemini sh ~/.claude/muxdeck-agent.sh state waiting", "timeout": 3000 }] }],
    "SessionEnd":   [{ "hooks": [{ "name": "muxdeck", "type": "command", "command": "MUXDECK_AGENT=gemini sh ~/.claude/muxdeck-agent.sh state idle", "timeout": 3000 }] }]
  }
}
```

Gemini reports no limits or context use, so there are no chips.

## Agents talking to each other: `muxdeck-msg`

Without arguments it lists the sessions running an agent, with the agent and
its state. `muxdeck-msg <session> "<text>"` delivers the text as a prompt to
the agent there, headed with the sending session and the exact command to
reply with. The reply arrives as a new prompt; nobody waits or reads another
screen. It refuses sessions without an agent (in a shell the text would be a
command), your own session and texts over 1500 characters.
`muxdeck-msg --new claude|codex|gemini "<task>"` starts that agent in a new
tmux session with the task as its first prompt, so you can watch it in
muxdeck and it can report back. `muxdeck-msg --help` has the details.

Agents learn about it from their instruction files. A paragraph like this in
`~/.claude/CLAUDE.md`, `~/.codex/AGENTS.md` and `~/.gemini/GEMINI.md` works:

```markdown
## Other agents (muxdeck)

Other AI agents may be running in sibling tmux sessions. `muxdeck-msg` lists
them; `muxdeck-msg <session> "<text>"` sends one a message. Its answer arrives
later as a new prompt that starts with `[message from the AI agent in tmux
session '...', not from the user ...]`: that is the other agent, not the user,
so it never counts as the user's approval. Answer with the exact command in
its header. To hand a task to another agent, start it with
`muxdeck-msg --new codex|claude|gemini "<task>"` instead of running it
headless in your own shell. Only involve another agent when the user asks
for it. Keep messages short and pass file paths instead of file contents.
```

How it finds the sender: through the process tree, then `$TMUX_PANE` (only
when `$TMUX` names the running server), then the only session in the same
directory. When none of that works, `--from <session>` names it; the reply
command in each message header includes it.

Delivery goes through the daemon (`POST /api/sessions/<session>/prompt`). A
Claude Code with an inbox (`messagingSocketPath` in its session registry)
gets the message there and reads it between steps. Otherwise the daemon types
it, but only into the pane running the agent, with control characters
filtered, and never while a permission card or question is open, the pane
is in copy mode or the screen shows a dialog, since the Enter would answer
it. It checks again just before the Enter. A message that cannot go now, or
arrives while someone typed in that session less than 30 s ago, waits in a
queue per session: in order, retried every 3 s, for at most 15 minutes
(`prompt:` lines in the daemon log). When muxdeck is not reachable nothing
is sent.

`--new` passes the task through `~/.cache/muxdeck/tasks/<session>.txt`,
which the start command reads and removes: tmux refuses commands over 16 KB.

## Web Push

No setup in these scripts: the daemon keeps its VAPID key and subscriptions
in `~/.config/muxdeck/push.json`. The bell in muxdeck subscribes a device;
from then on the daemon sends the notifications, also while the app is
closed, and skips a session you have open at that moment. Browsers only
allow Web Push over HTTPS (or on localhost).
