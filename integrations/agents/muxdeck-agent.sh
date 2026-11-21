#!/bin/sh
# Reports Claude Code's state to muxdeck (sidebar badge, notification on "waiting").
#   muxdeck-agent.sh state working|waiting|idle   (hooks, hook JSON on stdin)
#   muxdeck-agent.sh statusline                   (statusline JSON on stdin)
#   muxdeck-agent.sh permission                   (PermissionRequest hook)
#   muxdeck-agent.sh denied                       (PermissionDenied hook, auto mode)
# MUXDECK_AGENT=codex|gemini for the hooks of Codex and Gemini (same JSON).
#
# permission: shows the request as a card in muxdeck (plus a push) and waits
# for an answer there. The wait is set in the muxdeck settings (max 140 s, the
# hook timeout is 150); MUXDECK_PERMISSION_WAIT overrides it. No answer, cards
# off, muxdeck unreachable or "Answer in terminal": no output, and Claude Code
# simply asks in the terminal.
# The last hook state is kept in a file per tmux session, so the statusline
# can pass on model and cost without overwriting the state.
# Env: MUXDECK_URL (default http://127.0.0.1:8300), MUXDECK_TOKEN (falls back
# to MUXDECK_TOKEN= in ~/.config/muxdeck/env). Requires tmux, jq and curl;
# Linux only (finds the session through /proc).
# Always fails quietly: it must never get in Claude Code's way.

command -v jq >/dev/null 2>&1 || exit 0

# The tmux session through the process tree, not $TMUX: with `claude --resume`
# the conversation runs under a Claude daemon without TMUX in its environment,
# and without TMUX_PANE display-message points at the wrong session.
panes=$(tmux list-panes -a -F '#{pane_pid} #{session_name}' 2>/dev/null) || exit 0
session=""
pid=$$
while [ -n "$pid" ] && [ "$pid" -gt 1 ]; do
  session=$(printf '%s\n' "$panes" | awk -v p="$pid" '$1 == p { print $2; exit }')
  [ -n "$session" ] && break
  pid=$(sed 's/.*) //' "/proc/$pid/stat" 2>/dev/null | cut -d' ' -f2)
done
[ -n "$session" ] || exit 0

url="${MUXDECK_URL:-http://127.0.0.1:8300}"
token="${MUXDECK_TOKEN:-}"
[ -n "$token" ] || token=$(sed -n 's/^MUXDECK_TOKEN=//p' "$HOME/.config/muxdeck/env" 2>/dev/null)

dir="${XDG_RUNTIME_DIR:-/tmp}/muxdeck-agent"
mkdir -p "$dir" 2>/dev/null
statefile="$dir/$session"

post() {
  curl -sf --max-time 1 -X POST "$url/api/agent/status" \
    ${token:+-H "Authorization: Bearer $token"} \
    -H 'Content-Type: application/json' -d "$1" >/dev/null 2>&1 || true
}

case "${1:-}" in
state)
  st="$2"
  input=""
  [ -t 0 ] || input=$(cat)
  note=$(printf '%s' "$input" | jq -r '.message // empty' 2>/dev/null | head -c 200)
  # Notification fires for more than questions. Only these wait on the
  # user; idle_prompt ("waiting for your input", a minute after Stop) keeps
  # the "done" note, or the sidebar would call a finished turn a question.
  case "$(printf '%s' "$input" | jq -r '.notification_type // empty' 2>/dev/null)" in
    ''|permission_prompt|elicitation_dialog|elicitation_url_dialog|agent_needs_input) ;;
    idle_prompt)
      note="Claude is done and waiting for you"
      [ "$(cat "$statefile" 2>/dev/null)" = waiting ] && note=$(cat "$statefile.note" 2>/dev/null)
      ;;
    *) exit 0 ;;
  esac
  agent="${MUXDECK_AGENT:-claude-code}"
  case "$agent" in codex) who=Codex ;; gemini) who=Gemini ;; *) who=Claude ;; esac
  event=$(printf '%s' "$input" | jq -r '.hook_event_name // empty' 2>/dev/null)
  reply=""
  case "$event" in
    Stop|AfterAgent)
      # A turn that ends while helpers it started still run is not done:
      # their result wakes it up again. Shell tasks and monitors don't
      # count, a dev server in the background never ends.
      bg=$(printf '%s' "$input" | jq -r '[.background_tasks[]? | select((.type // "") | test("^(subagent|workflow|teammate|cloud session)$"))
        | select((.status // "running") | test("^(running|pending)$"))] | length' 2>/dev/null)
      if [ "${bg:-0}" -gt 0 ] && [ "$st" = waiting ]; then
        st=working
        note="$bg background task$([ "$bg" = 1 ] || echo s) still running"
      elif [ -z "$note" ] && [ "$st" = waiting ]; then
        note="$who is done and waiting for you"
        reply=$(printf '%s' "$input" | jq -r '.last_assistant_message // .prompt_response // empty' 2>/dev/null | head -c 600)
      fi
      ;;
    Interrupt)
      # Codex: the user stopped the turn (Esc). They are looking at it.
      st=waiting
      note="$who is done and waiting for you (interrupted)"
      ;;
    StopFailure)
      # An API error ends the turn without Stop: say why it stopped.
      err=$(printf '%s' "$input" | jq -r '.error // "unknown"' 2>/dev/null)
      msg=$(printf '%s' "$input" | jq -r '.last_assistant_message // .error_details // empty' 2>/dev/null | head -c 140)
      st=waiting
      note="$who stopped ($err)${msg:+: $msg}"
      ;;
  esac
  transcript=$(printf '%s' "$input" | jq -r '.transcript_path // empty' 2>/dev/null)
  sid=$(printf '%s' "$input" | jq -r '.session_id // empty' 2>/dev/null)
  cwd=$(printf '%s' "$input" | jq -r '.cwd // empty' 2>/dev/null)
  printf '%s' "$st" >"$statefile" 2>/dev/null
  printf '%s' "$note" >"$statefile.note" 2>/dev/null
  # Which conversation these hooks speak for: the Codex watcher defers to
  # them for that conversation.
  printf '%s' "$agent $sid" >"$statefile.sid" 2>/dev/null
  post "$(jq -nc --arg s "$session" --arg st "$st" --arg n "$note" --arg a "$agent" --arg t "$transcript" \
    --arg sid "$sid" --arg cwd "$cwd" --arg r "$reply" --arg ev "$event" \
    '{session: $s, agent: $a, state: $st, note: $n, transcript: $t, session_id: $sid, cwd: $cwd, reply: $r, event: $ev}')"
  ;;
denied)
  # PermissionDenied: auto mode blocked a tool call. Report it and return at
  # once without output, so the block stands (no retry) until the user
  # answers the card in muxdeck or in the terminal.
  input=$(cat)
  printf '%s' "$input" | jq -c --arg s "$session" --arg a "${MUXDECK_AGENT:-claude-code}" \
    '{session: $s, agent: $a, tool_name, tool_input: (.tool_input // {}), reason: (.reason // "")}' 2>/dev/null |
    curl -sf --max-time 2 -X POST "$url/api/agent/blocked" \
      ${token:+-H "Authorization: Bearer $token"} \
      -H 'Content-Type: application/json' --data-binary @- >/dev/null 2>&1 || true
  ;;
permission)
  input=$(cat)
  tool=$(printf '%s' "$input" | jq -r '.tool_name // empty' 2>/dev/null)
  [ -n "$tool" ] || exit 0
  wait="${MUXDECK_PERMISSION_WAIT:-0}"
  tmp=$(mktemp "${TMPDIR:-/tmp}/muxdeck-perm.XXXXXX") || exit 0
  # The request goes from a file: a big Write as an argument hits the
  # kernel's 128 KiB limit and curl never starts.
  printf '%s' "$input" | jq -c --arg s "$session" --argjson w "$wait" --arg a "${MUXDECK_AGENT:-claude-code}" \
    '{session: $s, agent: $a, tool_name, tool_input: (.tool_input // {}),
      can_always: ((.permission_suggestions // []) | length > 0), wait: $w}' >"$tmp" 2>/dev/null || { rm -f "$tmp"; exit 0; }
  # When Claude Code gives up on the hook (Esc, timeout) and kills only this
  # shell, take curl along, or the card and notification stay up for nothing.
  curl -sf --max-time 145 -X POST "$url/api/agent/permission" \
    ${token:+-H "Authorization: Bearer $token"} \
    -H 'Content-Type: application/json' --data-binary @"$tmp" >"$tmp.out" 2>/dev/null &
  cpid=$!
  trap 'kill $cpid 2>/dev/null; rm -f "$tmp" "$tmp.out"; exit 0' TERM INT HUP
  wait $cpid
  trap - TERM INT HUP
  answer=$(cat "$tmp.out" 2>/dev/null)
  rm -f "$tmp" "$tmp.out"
  decision=$(printf '%s' "$answer" | jq -r '.decision // empty' 2>/dev/null)
  out() { printf '%s' "$input" | jq -c --argjson a "${2:-null}" "{hookSpecificOutput: {hookEventName: \"PermissionRequest\", decision: $1}}"; }
  case "$decision" in
    # ExitPlanMode only goes ahead with updatedInput (its own input back).
    allow) if [ "$tool" = ExitPlanMode ]; then out '{behavior: "allow", updatedInput: (.tool_input // {})}'
           else out '{behavior: "allow"}'; fi ;;
    always) out '{behavior: "allow", updatedPermissions: (.permission_suggestions // [])}' ;;
    deny) out '{behavior: "deny", message: "Denied from muxdeck"}' ;;
    answer) out '{behavior: "allow", updatedInput: ((.tool_input // {}) + {answers: $a})}' \
      "$(printf '%s' "$answer" | jq -c '.answers // {}' 2>/dev/null)" ;;
  esac
  ;;
statusline)
  input=$(cat)
  st=$(cat "$statefile" 2>/dev/null)
  [ -n "$st" ] || st=working
  note=$(cat "$statefile.note" 2>/dev/null)
  # A question answered in the terminal reports nothing, so "needs you"
  # would stay up until the turn ends. The conversation moving on after the
  # question was reported means it was answered.
  if [ "$st" = waiting ] && [ -n "$note" ] && [ "${note#* is done }" = "$note" ]; then
    tp=$(printf '%s' "$input" | jq -r '.transcript_path // empty' 2>/dev/null)
    if [ -n "$tp" ] && [ -f "$tp" ]; then
      t1=$(stat -c %Y "$tp" 2>/dev/null || echo 0)
      t0=$(stat -c %Y "$statefile" 2>/dev/null || echo 0)
      if [ "$t1" -gt $((t0 + 3)) ]; then
        st=working
        note=""
        printf '%s' "$st" >"$statefile" 2>/dev/null
        : >"$statefile.note" 2>/dev/null
      fi
    fi
  fi
  payload=$(printf '%s' "$input" | jq -c --arg s "$session" --arg st "$st" --arg n "$note" '
    def wd: ["Sun","Mon","Tue","Wed","Thu","Fri","Sat"][strflocaltime("%w") | tonumber];
    def reset(fmt; t): if t == null then "" else " · " + (t | fmt) end;
    def pct(k; lbl; icon; v; r; at):
      if v == null then empty else
        {key: k, value: "\(lbl) \(v | round)%\(r)"}
        + (if at then {resets_at: (at | floor)} else {} end)
        + (if icon then {icon: icon} else {} end)
        + (if v >= 90 then {color: "danger"} elif v >= 70 then {color: "warn"} else {} end)
      end;
    def pct(k; lbl; icon; v): pct(k; lbl; icon; v; ""; null);
    {session: $s, agent: "claude-code", state: $st, note: $n,
     transcript: (.transcript_path // ""),
     model: (.model.display_name // ""), cost_usd: (.cost.total_cost_usd // 0),
     chips: [
       pct("context window"; "ctx"; null; .context_window.used_percentage),
       pct("5-hour limit"; "5h"; "clock"; .rate_limits.five_hour.used_percentage;
         reset(strflocaltime("%H:%M"); .rate_limits.five_hour.resets_at);
         .rate_limits.five_hour.resets_at),
       pct("7-day limit"; "7d"; "clock"; .rate_limits.seven_day.used_percentage;
         reset(wd + strflocaltime(" %H:%M"); .rate_limits.seven_day.resets_at);
         .rate_limits.seven_day.resets_at),
       (((.cost.total_lines_added // 0) + (.cost.total_lines_removed // 0)) as $t
        | if $t > 0 then {key: "diff", icon: "file",
            value: "+\(.cost.total_lines_added // 0)/-\(.cost.total_lines_removed // 0)"}
          else empty end)
     ]}' 2>/dev/null)
  [ -n "$payload" ] && post "$payload" &
  ;;
esac
exit 0
