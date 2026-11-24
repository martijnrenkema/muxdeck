"use strict";

const $ = (sel) => document.querySelector(sel);

const isMac = /Mac|iP(hone|ad|od)/.test(navigator.platform || navigator.userAgent);

// --- icons ---
// One stroke-based set (lucide-style paths) for all UI chrome, so nothing
// falls back to platform emoji rendering. Sized via CSS (.icon).

const ICONS = {
  bell: '<path d="M10.27 21a2 2 0 0 0 3.46 0"/><path d="M6 8a6 6 0 0 1 12 0c0 4.5 1.41 5.96 2.74 7.33A1 1 0 0 1 20 17H4a1 1 0 0 1-.74-1.67C4.59 13.96 6 12.5 6 8"/>',
  columns: '<rect width="18" height="18" x="3" y="3" rx="2"/><path d="M12 3v18"/>',
  refresh: '<path d="M21 12a9 9 0 1 1-2.64-6.36L21 8"/><path d="M21 3v5h-5"/>',
  pencil: '<path d="M17 3a2.828 2.828 0 1 1 4 4L7.5 20.5 2 22l1.5-5.5L17 3z"/>',
  x: '<path d="M18 6 6 18"/><path d="m6 6 12 12"/>',
  plus: '<path d="M5 12h14"/><path d="M12 5v14"/>',
  grip: '<circle cx="9" cy="5" r="1"/><circle cx="9" cy="12" r="1"/><circle cx="9" cy="19" r="1"/><circle cx="15" cy="5" r="1"/><circle cx="15" cy="12" r="1"/><circle cx="15" cy="19" r="1"/>',
  chevronLeft: '<path d="m15 18-6-6 6-6"/>',
  chevronRight: '<path d="m9 18 6-6-6-6"/>',
  chevronDown: '<path d="m6 9 6 6 6-6"/>',
  chevronUp: '<path d="m18 15-6-6-6 6"/>',
  linkOff: '<path d="M9 17H7A5 5 0 0 1 7 7"/><path d="M15 7h2a5 5 0 0 1 4 8"/><path d="M8 12h4"/><path d="m2 2 20 20"/>',
  branch: '<path d="M6 3v12"/><circle cx="18" cy="6" r="3"/><circle cx="6" cy="18" r="3"/><path d="M18 9a9 9 0 0 1-9 9"/>',
  plug: '<path d="M9 2v6"/><path d="M15 2v6"/><path d="M6 8h12v4a6 6 0 0 1-6 6 6 6 0 0 1-6-6Z"/><path d="M12 18v4"/>',
  // the vocabulary an agent may pick from for a status chip; the daemon
  // drops any name that is not one of these (see internal/agent)
  check: '<path d="M20 6 9 17l-5-5"/>',
  alert: '<circle cx="12" cy="12" r="10"/><path d="M12 8v5"/><path d="M12 17h.01"/>',
  clock: '<circle cx="12" cy="12" r="10"/><path d="M12 6v6l4 2"/>',
  flask: '<path d="M9 2h6"/><path d="M10 2v6.6L4.6 18A2 2 0 0 0 6.3 21h11.4a2 2 0 0 0 1.7-3L14 8.6V2"/><path d="M7 15h10"/>',
  coins: '<circle cx="9" cy="9" r="6"/><path d="M18.1 11.4A6 6 0 1 1 11.4 18.1"/>',
  file: '<path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8Z"/><path d="M14 2v6h6"/>',
  gauge: '<path d="m12 14 4-4"/><path d="M3.34 19a10 10 0 1 1 17.32 0"/>',
  gear: '<path d="M12.22 2h-.44a2 2 0 0 0-2 2v.18a2 2 0 0 1-1 1.73l-.43.25a2 2 0 0 1-2 0l-.15-.08a2 2 0 0 0-2.73.73l-.22.38a2 2 0 0 0 .73 2.73l.15.1a2 2 0 0 1 1 1.72v.51a2 2 0 0 1-1 1.74l-.15.09a2 2 0 0 0-.73 2.73l.22.38a2 2 0 0 0 2.73.73l.15-.08a2 2 0 0 1 2 0l.43.25a2 2 0 0 1 1 1.73V20a2 2 0 0 0 2 2h.44a2 2 0 0 0 2-2v-.18a2 2 0 0 1 1-1.73l.43-.25a2 2 0 0 1 2 0l.15.08a2 2 0 0 0 2.73-.73l.22-.39a2 2 0 0 0-.73-2.73l-.15-.08a2 2 0 0 1-1-1.74v-.5a2 2 0 0 1 1-1.74l.15-.09a2 2 0 0 0 .73-2.73l-.22-.38a2 2 0 0 0-2.73-.73l-.15.08a2 2 0 0 1-2 0l-.43-.25a2 2 0 0 1-1-1.73V4a2 2 0 0 0-2-2z"/><circle cx="12" cy="12" r="3"/>',
};

function icon(name) {
  return `<svg class="icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">${ICONS[name]}</svg>`;
}

$("#notify").innerHTML = icon("bell");
$("#split").innerHTML = icon("columns");
$("#settings-btn").innerHTML = icon("gear");
$("#dashboard-link").innerHTML = icon("gauge");
$("#settings .set-close").innerHTML = icon("x");
$("#launcher .set-close").innerHTML = icon("x");
$("#refresh").innerHTML = icon("refresh");
$("#new-session").innerHTML = icon("plus");

// --- persisted prefs ---

let fontSize = Math.max(9, Math.min(24, +localStorage.getItem("muxdeck-font") || 13));
let order = [];
try { order = JSON.parse(localStorage.getItem("muxdeck-order")) || []; } catch {}

// Per-device display preferences; daemon-wide settings live in /api/settings.
const PREF_DEFAULTS = { keybar: true, keys: "", limits: true, chips: true, imefix: true, keysOpen: false };
let prefs = { ...PREF_DEFAULTS };
try { prefs = { ...PREF_DEFAULTS, ...JSON.parse(localStorage.getItem("muxdeck-prefs")) }; } catch {}
function savePrefs() { try { localStorage.setItem("muxdeck-prefs", JSON.stringify(prefs)); } catch {} }

function saveOrder() { localStorage.setItem("muxdeck-order", JSON.stringify(order)); }

// --- API helpers ---

async function api(path, opts = {}) {
  const res = await fetch(path, {
    headers: { "Content-Type": "application/json" },
    ...opts,
  });
  if (res.status === 401) {
    // The relay marks its own refusals: this session or address is done at
    // the edge — the daemon's token prompt would be the wrong ask. A full
    // navigation lets the relay route to its login (or the app's home
    // button leads back to the picker).
    if (res.headers.get("X-Muxdeck-Relay-Auth")) {
      showRelayRefused();
    } else {
      showLogin();
    }
    throw new Error("unauthorized");
  }
  if (!res.ok) {
    let msg = res.statusText;
    try { msg = (await res.json()).error || msg; } catch {}
    const err = new Error(msg);
    err.status = res.status;
    throw err;
  }
  return res.status === 204 ? null : res.json();
}

// apiRetry repeats a request that is safe to repeat when it failed for a
// reason that passes: the network dropped it (fetch throws without a status,
// e.g. the tailnet path re-establishing), the server hiccuped (5xx), or an
// upload broke off mid-body. Answers about the request itself (404, 413,
// 401) come back at once.
async function apiRetry(path, opts = {}, delays = [400, 1200, 3000]) {
  for (let i = 0; ; i++) {
    try {
      return await api(path, opts);
    } catch (err) {
      const passing = err.status === undefined ? err.message !== "unauthorized"
        : err.status >= 500 || err.message === "upload interrupted";
      // A request that never reached the daemon leaves no trace in its log;
      // this one does.
      wsdbg.log("api-failed", { p: path, m: opts.method || "GET", st: err.status || 0, err: String(err.message).slice(0, 120), try: i + 1 });
      if (!passing || i >= delays.length) throw err;
      await new Promise((r) => setTimeout(r, delays[i]));
    }
  }
}

let relayRefusedShown = false;
function showRelayRefused() {
  if (relayRefusedShown) return;
  relayRefusedShown = true;
  const d = document.createElement("div");
  d.id = "relay-refused";
  d.innerHTML = "<p>This address is no longer signed in at the relay.</p><p>Reopen it from the muxdeck app, or sign in again:</p>";
  const a = document.createElement("a");
  a.href = "/relay/login";
  a.textContent = "relay sign-in";
  d.appendChild(a);
  const login = $("#login");
  login.hidden = false;
  login.querySelector("form")?.remove();
  login.prepend(d);
}

// --- login ---

// Every 401 lands here — the 10s session poll and each pane's reconnect
// loop included — so only the first call may move focus; re-focusing on
// every tick fought the phone keyboard while the token was being typed.
function showLogin() {
  if (!loginShown()) {
    $("#login").hidden = false;
    $("#token").focus();
  }
}

function loginShown() {
  return !$("#login").hidden;
}

$("#login-form").addEventListener("submit", async (e) => {
  e.preventDefault();
  const res = await fetch("/api/login", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ token: $("#token").value }),
  });
  if (res.ok) {
    $("#login").hidden = true;
    $("#login-error").hidden = true;
    $("#token").value = "";
    await refreshSessions();
    if (!panes.some((p) => p.session)) attachFromHash();
    for (const p of panes) { p.retries = 0; p.reconnectNow(); }
  } else {
    $("#login-error").hidden = false;
  }
});

// --- clipboard ---
// Copy flows: browser selection is copied at gesture end; tmux copy-mode
// yanks arrive as OSC 52 (set-clipboard, enabled server-side) and are
// forwarded to the browser clipboard. Paste: native Cmd/Ctrl+V on desktop,
// keybar button on touch.

function osc52ToClipboard(data) {
  const i = data.indexOf(";");
  if (i < 0) return;
  const b64 = data.slice(i + 1);
  if (b64 === "?") return; // clipboard read request: unsupported
  let text;
  try { text = new TextDecoder().decode(Uint8Array.from(atob(b64), (c) => c.charCodeAt(0))); }
  catch { return; }
  lastCopy = text;
  writeClipboard(text);
}

// OSC 52 lands outside any user gesture, and Firefox and iOS Safari reject
// clipboard writes without one. Keep the text so Cmd+C can copy it again,
// and offer a tap-to-copy button when the direct write is refused.
let lastCopy = "";

function writeClipboard(text) {
  if (!navigator.clipboard) { offerCopy(text); return; }
  navigator.clipboard.writeText(text).then(hideCopyOffer, () => offerCopy(text));
}

let copyOfferTimer = 0;

function offerCopy(text) {
  const btn = $("#copy-offer");
  btn.textContent = `tap to copy (${text.length} chars)`;
  btn.onclick = () => {
    const done = () => { btn.textContent = "copied"; clearTimeout(copyOfferTimer); copyOfferTimer = setTimeout(hideCopyOffer, 1200); };
    (navigator.clipboard ? navigator.clipboard.writeText(text) : Promise.reject())
      .then(done, () => { if (legacyCopy(text)) done(); });
  };
  btn.hidden = false;
  clearTimeout(copyOfferTimer);
  copyOfferTimer = setTimeout(hideCopyOffer, 10000);
}

function hideCopyOffer() {
  clearTimeout(copyOfferTimer);
  $("#copy-offer").hidden = true;
}

function legacyCopy(text) {
  const ta = document.createElement("textarea");
  ta.value = text;
  ta.style.cssText = "position:fixed;top:0;left:0;opacity:0";
  document.body.appendChild(ta);
  ta.select();
  let ok = false;
  try { ok = document.execCommand("copy"); } catch {}
  ta.remove();
  return ok;
}

function copySelection(term) {
  if (term && term.hasSelection()) {
    navigator.clipboard?.writeText(term.getSelection()).catch(() => {});
  }
}

// --- file upload ---
// A file dropped on a pane, pasted (a screenshot on the clipboard) or picked
// with the keybar's file button is stored next to the session on the server,
// and its path is pasted into the terminal: Claude Code attaches a pasted
// image path. The server drops the files when the session ends.

const MAX_UPLOAD = 50 * 1024 * 1024;
let uploadNoteTimer = 0;

function uploadNote(text, sticky, ms = 4000) {
  const el = $("#upload-note");
  el.textContent = text;
  el.hidden = false;
  clearTimeout(uploadNoteTimer);
  if (!sticky) uploadNoteTimer = setTimeout(() => { el.hidden = true; }, ms);
}

async function uploadFiles(p, files) {
  files = [...files];
  if (!files.length) return;
  if (!p?.session || !p.term) { uploadNote("open a session first"); return; }
  // The session and terminal the files were dropped on: a pane switched
  // to another session mid-batch must not receive these paths.
  const session = p.session;
  const paths = [];
  const failed = [];
  for (const [i, f] of files.entries()) {
    const name = f.name || "paste.png";
    if (f.size > MAX_UPLOAD) { failed.push(`${name}: larger than 50 MB`); continue; }
    uploadNote(files.length > 1 ? `uploading ${i + 1}/${files.length} ${name}…` : `uploading ${name}…`, true);
    try {
      // The bytes are read before the request: a File handle can go stale
      // (the source file replaced, a pasted blob released) and then fails
      // every retry, where a buffer re-sends as often as needed.
      const body = await f.arrayBuffer();
      const { path } = await apiRetry(keyApi(session, "/upload"), {
        method: "POST",
        headers: { "Content-Type": "application/octet-stream", "X-Filename": encodeURIComponent(name) },
        body,
      });
      paths.push(path);
    } catch (err) {
      wsdbg.log("upload-failed", { s: session, f: name.slice(0, 80), size: f.size, err: String(err.message).slice(0, 120) });
      if (err.message === "unauthorized") { $("#upload-note").hidden = true; return; }
      failed.push(`${name}: ${err.message}`);
    }
  }
  if (failed.length) uploadNote(failed.length === 1 ? failed[0] : `${failed.length} files failed — ${failed.join("; ")}`, false, 10000);
  else $("#upload-note").hidden = true;
  if (!paths.length || p.session !== session || !p.term) return;
  p.term.paste(paths.join(" ") + " ");
  p.term.focus();
}

function hasFiles(e) {
  return [...(e.dataTransfer?.types || [])].includes("Files");
}

async function pasteToFocused() {
  const p = paneFor();
  if (!p?.term) return;
  try {
    const text = await navigator.clipboard.readText();
    if (text) p.term.paste(text);
  } catch { alert("clipboard read not permitted"); }
  p.term.focus();
}

// --- keyboard chords (Cmd+K / Cmd+F on mac, Ctrl+Shift+K / +F elsewhere;
// plain Ctrl combos belong to the shell) ---

function chord(e, key) {
  return e.key.toLowerCase() === key && !e.altKey &&
    (isMac ? e.metaKey && !e.ctrlKey : e.ctrlKey && e.shiftKey);
}

document.addEventListener("keydown", (e) => {
  if (chord(e, "k")) {
    e.preventDefault();
    $("#palette").hidden ? openPalette("switch") : closePalette();
  } else if (chord(e, "f") && paneFor()?.term) {
    e.preventDefault();
    paneFor().toggleFind(true);
  } else if (chord(e, "b")) {
    e.preventDefault();
    toggleSidebar();
  }
});

// --- panes ---

let panes = [];

// --- ws freeze hunt: socket forensics, shipped to the daemon log ---
// Frozen panes on iPad survive every readyState check, so the evidence has
// to be collected client-side and read back server-side. Plain fetch, not
// api(): a failed log write must never surface auth UI or break the deck.
// Set once this device holds a Web Push subscription the daemon accepted.
let pushActive = false;

const wsdbg = (() => {
  const buf = [];
  let flushTimer = null;
  function flush() {
    flushTimer = null;
    if (!buf.length) return;
    const entries = buf.splice(0);
    fetch("/api/debug/client-log", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ entries }),
    }).catch(() => buf.unshift(...entries.slice(-100)));
  }
  return {
    log(ev, detail = {}) {
      buf.push({ t: Date.now(), v: document.hidden ? 0 : 1, ev, ...detail });
      if (buf.length > 400) buf.splice(0, buf.length - 400);
      if (!flushTimer) flushTimer = setTimeout(flush, 5000);
    },
  };
})();
let focusedPane = null;

function paneFor() { return focusedPane || panes[0]; }

function setFocus(p) {
  focusedPane = p;
  for (const q of panes) q.el.classList.toggle("focused", q === p);
}

class Pane {
  constructor() {
    this.session = null;
    this.term = null;
    this.fit = null;
    this.search = null;
    this.ws = null;
    this.retries = 0;
    this.retryTimer = null;
    this.deliberate = false; // detach in progress: ignore the ws close

    this.el = document.createElement("section");
    this.el.className = "pane";
    this.el.innerHTML = `
      <div class="pane-head">
        <span class="pane-tile" hidden></span>
        <span class="pane-title">no session</span>
        <span class="pane-tools">
          <button class="p-font" data-d="-1" title="smaller text">A&minus;</button>
          <button class="p-font" data-d="1" title="larger text">A+</button>
          <button class="p-find" title="find in scrollback">find</button>
          <button class="p-quiet" hidden title="notify me when this session stops printing (a build, a deploy, a job over SSH)">${icon("bell")}</button>
          <button class="p-mouse" hidden title="tmux mouse mode: wheel/touch scrolls the session instead of the page">mouse</button>
          <button class="p-keys" hidden title="show or hide the key bar">keys</button>
          <button class="p-refresh" hidden title="refresh view">${icon("refresh")}</button>
          <button class="p-close" title="close pane">${icon("x")}</button>
        </span>
      </div>
      <div class="findbar" hidden>
        <input placeholder="find" spellcheck="false" autocomplete="off">
        <button class="f-prev" title="previous match">${icon("chevronLeft")}</button>
        <button class="f-next" title="next match">${icon("chevronRight")}</button>
        <button class="f-close" title="close">${icon("x")}</button>
      </div>
      <div class="pane-perm" hidden></div>
      <div class="pane-body">
        <div class="pane-term"></div>
        <div class="pane-view" hidden></div>
        <div class="pane-msg">select or create a session</div>
      </div>`;

    this.el.addEventListener("pointerdown", () => setFocus(this));
    this.el.addEventListener("dragover", (e) => {
      if (!hasFiles(e)) return;
      e.preventDefault();
      e.dataTransfer.dropEffect = "copy";
      this.el.classList.add("drop-target");
    });
    this.el.addEventListener("dragleave", (e) => {
      if (!this.el.contains(e.relatedTarget)) this.el.classList.remove("drop-target");
    });
    this.el.addEventListener("drop", (e) => {
      this.el.classList.remove("drop-target");
      if (!hasFiles(e)) return;
      e.preventDefault();
      setFocus(this);
      uploadFiles(this, e.dataTransfer.files);
    });
    // Capture phase, so an image paste is claimed before xterm's textarea
    // pastes the (empty) text part.
    this.el.addEventListener("paste", (e) => {
      const files = e.clipboardData?.files;
      if (!files?.length) return;
      e.preventDefault();
      e.stopPropagation();
      uploadFiles(this, files);
    }, true);
    this.el.querySelector(".p-close").addEventListener("click", () => closePane(this));
    this.el.querySelector(".p-mouse").addEventListener("click", () => this.toggleMouse());
    this.el.querySelector(".p-quiet").addEventListener("click", () => this.toggleQuiet());
    this.el.querySelector(".p-find").addEventListener("click", () => this.toggleFind());
    // pointerdown + preventDefault, like the bar's own keys: the terminal
    // keeps focus, so the software keyboard stays where it is.
    this.el.querySelector(".p-keys").addEventListener("pointerdown", (e) => {
      e.preventDefault();
      prefs.keysOpen = !prefs.keysOpen;
      savePrefs();
      applyKeybarPref();
    });
    syncKeysButtons(this.el);
    this.el.querySelector(".p-refresh").addEventListener("click", () => this.refreshView());
    this.view = null;
    this.viewTimer = null;
    for (const b of this.el.querySelectorAll(".p-font")) {
      b.addEventListener("click", () => setFontSize(fontSize + +b.dataset.d));
    }
    this.el.querySelector(".pane-msg").addEventListener("click", () => this.reconnectNow());

    const fb = this.el.querySelector(".findbar");
    this.findInput = fb.querySelector("input");
    this.findInput.addEventListener("input", () => this.findNext(true));
    this.findInput.addEventListener("keydown", (e) => {
      if (e.key === "Enter") (e.shiftKey ? this.findPrev() : this.findNext(false));
      if (e.key === "Escape") this.toggleFind(false);
    });
    fb.querySelector(".f-next").addEventListener("click", () => this.findNext(false));
    fb.querySelector(".f-prev").addEventListener("click", () => this.findPrev());
    fb.querySelector(".f-close").addEventListener("click", () => this.toggleFind(false));

    new ResizeObserver(() => this.sendResize()).observe(this.el.querySelector(".pane-term"));
  }

  // Open permission requests for the attached session, as cards with the
  // answers the agent's hook accepts. Rebuilt only when the set changes, so
  // a refresh tick doesn't wipe a card mid-tap.
  showPermissions(list) {
    const box = this.el.querySelector(".pane-perm");
    const ids = (list || []).map((p) => p.id).join(",");
    if (box.dataset.ids === ids) return;
    box.dataset.ids = ids;
    box.innerHTML = "";
    for (const p of list || []) box.appendChild(permCard(p));
    box.hidden = !box.childElementCount;
  }

  msg(text) {
    const m = this.el.querySelector(".pane-msg");
    m.textContent = text || "";
    m.hidden = !text;
  }

  attach(name) {
    if (this.view) this.closeView();
    if (this.session) this.detach(true);
    this.session = name;
    this.el.querySelector(".pane-title").textContent = name;
    this.msg("");
    this.makeTerm();
    this.connect();
    this.refreshMouse();
    updateHash();
  }

  // --- viewer mode: the pane shows a rendered document instead of a
  // terminal. view = {type: "diff"} or {type: "md", path}, plus key: the
  // session whose cwd anchors the content. Polls for changes while visible.

  openView(view) {
    if (this.session) this.detach(true);
    this.view = view;
    this.viewStamp = null;
    this.msg("");
    this.el.querySelector(".pane-term").hidden = true;
    this.el.querySelector(".pane-view").innerHTML = "";
    this.el.querySelector(".pane-view").hidden = false;
    this.el.querySelector(".p-refresh").hidden = false;
    this.showPermissions([]);
    this.showTile(null);
    this.el.querySelector(".pane-title").textContent =
      view.type === "diff" ? `diff · ${view.key}` : view.type === "task" ? `${view.label} · ${view.key}` : `${view.path} · ${view.key}`;
    this.refreshView();
    clearInterval(this.viewTimer);
    this.viewTimer = setInterval(() => {
      if (!document.hidden) this.refreshView();
    }, 3000);
    updateHash();
  }

  closeView() {
    clearInterval(this.viewTimer);
    this.viewTimer = null;
    this.stopPresence();
    if (this.mushWS) { this.mushWS.onclose = null; this.mushWS.close(); this.mushWS = null; }
    this.view = null;
    this.viewStamp = null;
    this.subR = null;
    const v = this.el.querySelector(".pane-view");
    v.hidden = true;
    v.innerHTML = "";
    this.el.querySelector(".pane-term").hidden = false;
    this.el.querySelector(".p-refresh").hidden = true;
    this.el.querySelector(".pane-title").textContent = "no session";
    this.msg("select or create a session");
  }

  // --- subagent pane: read-only view of a helper an agent spawned. The
  // daemon turns the helper's transcript into events; this pane polls from
  // the last offset while visible. view = {type: "sub", key, id, back, next}
  // where back is the session this pane showed before (phones open the view
  // in place of the terminal, and offer the way back).

  openSub(key, a, back = null) {
    if (this.session) this.detach(true);
    if (this.view) this.closeView();
    const view = { type: "sub", key, id: a.id, back, next: 0 };
    this.view = view;
    this.el.querySelector(".pane-term").hidden = true;
    const box = this.el.querySelector(".pane-view");
    box.hidden = false;
    box.className = "pane-view mush-view sub-view";
    box.innerHTML = "";
    this.subR = subRenderer(box, key, back ? () => this.attach(back) : null, (text) =>
      api(keyApi(key, `/subagents/${encodeURIComponent(a.id)}/message`), { method: "POST", body: JSON.stringify({ text }) }));
    this.el.querySelector(".p-refresh").hidden = true;
    this.setSubTitle(a);
    this.msg("");
    this.pollSub();
    clearInterval(this.viewTimer);
    this.viewTimer = setInterval(() => {
      if (!document.hidden) this.pollSub();
    }, 2000);
    refreshSessions();
  }

  setSubTitle(a) {
    const who = AGENT_LABELS[a.agent] || a.agent || "agent";
    this.el.querySelector(".pane-title").textContent = `${who} · ${a.label || a.kind || a.id}`;
  }

  async pollSub() {
    const v = this.view;
    if (!v || v.type !== "sub" || this.subBusy) return;
    this.subBusy = true;
    let more = false;
    try {
      const t = await api(keyApi(v.key, `/subagents/${encodeURIComponent(v.id)}?after=${v.next}`));
      if (this.view !== v) return;
      if (t.reset && v.next) this.subR.clear();
      this.subR.add(t.events);
      v.next = t.next;
      this.subR.status(t);
      this.setSubTitle(t);
      more = t.events.length >= 600;
    } catch (err) {
      if (this.view === v) this.subR.status({ state: "error", error: err.status === 404 ? "this subagent is no longer listed" : err.message });
    } finally {
      this.subBusy = false;
    }
    if (more) this.pollSub();
  }

  // --- mush run pane: a viewer over a run's journal. The daemon replays the
  // journal and tails it over a websocket; the ledger row rides along as
  // `_run` / `_state` frames. Approval cards answer over the same socket —
  // the daemon routes the decision to whoever owns the run (its own engine,
  // or the mush serve behind serve.sock). view = {type: "mush", host, runId}.

  openMush(host, run) {
    if (this.session) this.detach(true);
    if (this.view) this.closeView();
    this.view = { type: "mush", host: host || "", runId: run.id, run };
    this.el.querySelector(".pane-term").hidden = true;
    const v = this.el.querySelector(".pane-view");
    v.hidden = false;
    v.className = "pane-view mush-view";
    this.el.querySelector(".p-refresh").hidden = false;
    this.setMushTitle(run);
    this.msg("");
    this.connectMush();
    refreshSessions();
  }

  setMushTitle(run) {
    this.el.querySelector(".pane-title").textContent =
      `mush · ${run.state ? run.state.replace("_", " ") + " · " : ""}${runLabel(run).slice(0, 60)}`;
  }

  connectMush() {
    if (this.mushWS) { this.mushWS.onclose = null; this.mushWS.close(); }
    const { host, runId } = this.view;
    const box = this.el.querySelector(".pane-view");
    box.innerHTML = "";
    const head = mushRunHeader(host, this.view.run, this);
    box.appendChild(head.el);
    const r = mushRenderer(box, (frame) => this.mushWS?.send(JSON.stringify(frame)));
    const proto = location.protocol === "https:" ? "wss:" : "ws:";
    const ws = new WebSocket(`${proto}//${location.host}${mushApi(host, `/${encodeURIComponent(runId)}/stream`)}`);
    this.mushWS = ws;
    ws.onopen = () => this.startPresence();
    ws.onmessage = (e) => {
      let env;
      try { env = JSON.parse(e.data); } catch { return; }
      if (env.type === "_run" || env.type === "_state") {
        this.view.run = env.data;
        head.update(env.data);
        this.setMushTitle(env.data);
        if (env.type === "_state") refreshSessions();
        return;
      }
      r.render(env);
    };
    ws.onclose = () => r.note("stream closed — ↻ to reconnect");
  }

  async refreshView() {
    if (!this.view) return;
    if (this.view.type === "mush") { this.connectMush(); return; }
    if (this.view.type === "sub") { this.pollSub(); return; }
    if (this.view.type === "task") { this.refreshTask(); return; }
    const { type, key, path } = this.view;
    const box = this.el.querySelector(".pane-view");
    try {
      if (type === "diff") {
        const { root, text } = await api(keyApi(key, "/diff"));
        const stamp = `${root}\n${text}`;
        if (stamp === this.viewStamp) return;
        this.viewStamp = stamp;
        box.className = "pane-view diff-view";
        box.innerHTML = "";
        box.appendChild(renderDiff(root, text));
      } else {
        const { text } = await api(keyApi(key, `/file?path=${encodeURIComponent(path)}`));
        if (text === this.viewStamp) return;
        this.viewStamp = text;
        box.innerHTML = DOMPurify.sanitize(marked.parse(text));
        box.className = "pane-view md-view";
      }
    } catch (err) {
      box.className = "pane-view";
      box.textContent = err.message;
    }
  }

  async refreshTask() {
    const v = this.view;
    const box = this.el.querySelector(".pane-view");
    let r;
    try {
      r = await api(keyApi(v.key, `/tasks/${encodeURIComponent(v.id)}`));
    } catch (err) {
      if (this.view !== v) return;
      box.className = "pane-view mush-view task-view";
      box.textContent = err.status === 404 ? "This task is no longer listed (finished more than 30 minutes ago, or the agent restarted)." : err.message;
      this.viewStamp = null;
      return;
    }
    if (this.view !== v) return;
    const t = r.task;
    const started = Date.parse(t.started) || Date.now();
    const status = t.state === "running" ? `running · ${fmtAge(Date.now() - started)}` : (t.summary || t.state);
    const stamp = `${status}\n${r.output ?? ""}\n${r.missing}`;
    if (stamp === this.viewStamp) return;
    this.viewStamp = stamp;
    let pre = box.querySelector("pre.task-out");
    const atEnd = !pre || pre.scrollHeight - pre.scrollTop - pre.clientHeight < 40;
    if (!box.querySelector(".task-status")) {
      box.className = "pane-view mush-view task-view";
      box.innerHTML = "";
      const head = document.createElement("div");
      head.className = "m-head";
      const line = document.createElement("span");
      line.className = "m-head-line task-status";
      head.appendChild(line);
      if (v.back) {
        const actions = document.createElement("span");
        actions.className = "m-head-actions";
        const back = document.createElement("button");
        back.textContent = `back to ${v.back}`;
        back.addEventListener("click", () => this.attach(v.back));
        actions.appendChild(back);
        head.appendChild(actions);
      }
      box.appendChild(head);
      pre = document.createElement("pre");
      pre.className = "task-out";
      box.appendChild(pre);
    }
    this.el.querySelector(".pane-title").textContent = `${t.label || t.id} · ${v.key}`;
    box.querySelector(".task-status").textContent = status;
    pre.textContent = r.missing ? "(no output file: it was cleaned up, or this task writes none)" :
      (r.truncated ? "…\n" : "") + (r.output || "(no output yet)");
    if (atEnd) pre.scrollTop = pre.scrollHeight;
  }

  makeTerm() {
    if (this.term) this.term.dispose();
    this.term = new Terminal({
      fontFamily: "SF Mono, Menlo, Monaco, monospace",
      fontSize,
      cursorBlink: true,
      // 3k rows ≈ a few MB per pane; 10k made attached panes the biggest
      // JS-heap line item once Claude Code TUIs filled them.
      scrollback: 3000,
      macOptionClickForcesSelection: true,
      allowProposedApi: true, // Unicode11Addon's version switch needs it
      theme: { background: "#000000" },
    });
    this.fit = new FitAddon.FitAddon();
    this.search = new SearchAddon.SearchAddon();
    this.term.loadAddon(this.fit);
    this.term.loadAddon(this.search);
    this.term.loadAddon(new WebLinksAddon.WebLinksAddon((e, uri) => window.open(uri, "_blank", "noopener")));
    // Match tmux's modern wcwidth: with the default Unicode 6 tables,
    // glyphs like ⏺ and ⚡ (wide since Unicode 9) disagree with tmux's
    // layout and TUI columns drift — the "garbled box art" bug.
    this.term.loadAddon(new Unicode11Addon.Unicode11Addon());
    this.term.unicode.activeVersion = "11";
    this.term.parser.registerOscHandler(52, (data) => { osc52ToClipboard(data); return true; });
    this.term.onData((d) => this.send(applyCtrl(d), TERM_REPORT.test(d)));
    this.term.attachCustomKeyEventHandler((e) => {
      // Cmd+C with no xterm selection: re-copy what the app last sent via
      // OSC 52 (e.g. a Claude Code selection), inside the key gesture.
      if (e.type === "keydown" && e.metaKey && !e.ctrlKey && e.key === "c" && !this.term.hasSelection() && lastCopy) {
        writeClipboard(lastCopy);
        return false;
      }
      return !(chord(e, "k") || chord(e, "f") || chord(e, "b"));
    });
    const mount = this.el.querySelector(".pane-term");
    mount.innerHTML = "";
    this.term.open(mount);
    // WebGL renderer (must load after open): draws box/block glyphs itself
    // via customGlyphs, avoiding the DOM renderer's font-metric seams.
    // Falls back to the DOM renderer wherever a context isn't available.
    try {
      const gl = new WebglAddon.WebglAddon();
      gl.onContextLoss(() => gl.dispose());
      this.term.loadAddon(gl);
    } catch (e) {
      console.warn("muxdeck: WebGL unavailable, falling back to DOM renderer", e);
    }
    // Clipboard writes need transient activation, so copy at gesture end
    // rather than on every selection change.
    // The mount outlives every terminal made in it (one per attach), so its
    // listeners go on once and read the current terminal when they fire.
    if (!this.mountBound) {
      this.mountBound = true;
      mount.addEventListener("mouseup", () => this.term && copySelection(this.term));
      mount.addEventListener("touchend", () => this.term && copySelection(this.term));
      this.bindTouch(mount);
    }
    this.bindIme(mount);
    this.fit.fit();
    if (!loginShown()) this.term.focus();
  }

  // xterm.js reads a software keyboard by comparing its hidden textarea
  // before and after each key, and only understands text added at the end.
  // When the keyboard rewrites earlier text — a picked suggestion, an
  // autocorrect, a grammar fix — it sends the whole textarea again, which
  // lands in the terminal as repeated words and sentences. On touch devices
  // the textarea is read here instead: what changed since the last look goes
  // out as backspaces plus the new tail.
  //
  // `floor` marks how much of the textarea was typed before something else
  // moved the terminal's cursor (a key bar key, an arrow, a paste). Text
  // before it can no longer be rewritten with backspaces, so a correction
  // reaching back that far is skipped rather than applied in the wrong place.
  bindIme(mount) {
    if (!IME_FIX || this.imeBound) return;
    this.imeBound = true;
    let shadow = "", floor = 0;
    const ta = () => this.term?.textarea;
    const own = (e) => e.target === ta();
    this.imeMark = () => { shadow = ta()?.value || ""; floor = shadow.length; };
    const sync = () => {
      const el = ta();
      if (!el) return;
      const v = el.value;
      let { keep, del, add } = imeDiff(shadow, v);
      if (keep < floor) {
        if (add) {
          // The rewrite cannot be applied, but the space or punctuation
          // typed with it (what set off the autocorrect) still can.
          const last = v.slice(-1);
          const typed = v.length > shadow.length && /[ .,!?;:]/.test(last) && !shadow.endsWith(last);
          this.imeMark();
          if (typed) { this.send(last, true, true); shadow = v; }
          return;
        }
        floor = keep;
      }
      shadow = v;
      if (!del && !add) return;
      // Several lines at once (the keyboard's clipboard chip) is a paste: as
      // keys, every line break would be an Enter that submits.
      if (add.length > 1 && /[\r\n]/.test(add)) {
        if (del) this.send("\x7f".repeat(del), true, true);
        this.term.paste(add);
        el.value = ""; shadow = ""; floor = 0;
        return;
      }
      const hadBreak = /[\r\n]/.test(add);
      if (hadBreak) add = add.replace(/\r?\n/g, "\r");
      const out = !del && add.length === 1 ? applyCtrl(add) : add;
      this.send("\x7f".repeat(del) + out, true, true);
      if (hadBreak || out !== add) { el.value = ""; shadow = ""; floor = 0; }
    };
    const swallow = (e) => { if (own(e)) e.stopPropagation(); };
    mount.addEventListener("compositionstart", swallow, true);
    mount.addEventListener("compositionupdate", swallow, true);
    mount.addEventListener("compositionend", (e) => { if (own(e)) { e.stopPropagation(); sync(); } }, true);
    mount.addEventListener("input", (e) => { if (own(e)) { e.stopPropagation(); sync(); } }, true);
    mount.addEventListener("keydown", (e) => {
      if (!own(e)) return;
      // 229 = "the keyboard is composing"; the input event carries the text.
      // A backspace with text in the textarea is left to the browser, so the
      // textarea and the terminal lose the same character.
      const plainBackspace = e.keyCode === 8 && !e.ctrlKey && !e.altKey && !e.metaKey && ta().value;
      if (e.keyCode === 229 || plainBackspace) e.stopPropagation();
    }, true);
    mount.addEventListener("focusout", (e) => { if (own(e)) { shadow = ""; floor = 0; } }, true);
  }

  // xterm.js drops touch input while the app has mouse tracking on (tmux
  // `mouse on`, Claude Code fullscreen), so swiping scrolled nothing on
  // phones. Translate one-finger vertical drags (plus a short fling) into
  // line-mode wheel events, which xterm forwards as mouse-wheel reports.
  // tmux and Claude Code move several lines per wheel report, so one report
  // per three rows of finger travel keeps the content roughly under it.
  // A long press opens the select view (native text selection) instead.
  bindTouch(mount) {
    let startX = 0, startY = 0, lastY = 0, lastT = 0, acc = 0, vel = 0;
    let x = 0, y = 0, active = false, fling = 0, held = false, holdTimer = 0, trk = false;
    const cancelHold = () => { clearTimeout(holdTimer); holdTimer = 0; };
    const tracking = () => !!this.term && this.term.modes.mouseTrackingMode !== "none";
    const rowPx = () => { const t = this.term; return 3 * ((t?.element?.querySelector(".xterm-screen")?.clientHeight || 0) / (t?.rows || 1) || 16); };
    const wheel = (lines) => {
      const el = this.term?.element;
      if (!el) return;
      const dir = Math.sign(lines);
      for (let i = 0; i < Math.abs(lines); i++) {
        el.dispatchEvent(new WheelEvent("wheel", {
          deltaY: dir, deltaMode: WheelEvent.DOM_DELTA_LINE,
          clientX: x, clientY: y, bubbles: true, cancelable: true,
        }));
      }
    };
    const scrollBy = (px) => {
      acc += px;
      const lines = Math.trunc(acc / rowPx());
      if (lines) { acc -= lines * rowPx(); wheel(lines); }
    };
    const stopFling = () => { cancelAnimationFrame(fling); fling = 0; };
    mount.addEventListener("touchstart", (e) => {
      stopFling();
      cancelHold();
      active = false;
      held = false;
      if (e.touches.length !== 1 || !this.term) return;
      // Without mouse tracking xterm scrolls by itself, but the long press
      // is still the only way to select text on a phone.
      trk = tracking();
      const t = e.touches[0];
      startX = x = t.clientX; startY = lastY = y = t.clientY;
      lastT = e.timeStamp; acc = 0; vel = 0;
      holdTimer = setTimeout(() => {
        holdTimer = 0;
        held = true;
        navigator.vibrate?.(15);
        this.openSelectView(startX, startY);
      }, 380);
    }, { capture: true, passive: true });
    mount.addEventListener("touchmove", (e) => {
      if (e.touches.length !== 1) return;
      const t = e.touches[0];
      if (held) return;
      if (!active) {
        const dx = Math.abs(t.clientX - startX), dy = Math.abs(t.clientY - startY);
        if (dx > 10 || dy > 10) cancelHold();
        if (!trk) return;
        if (dy < 10 || dy < dx) return;
        active = true;
        lastY = t.clientY;
      }
      e.preventDefault();
      e.stopPropagation();
      const d = lastY - t.clientY;
      const dt = Math.max(1, e.timeStamp - lastT);
      vel = 0.8 * (d / dt) + 0.2 * vel;
      lastY = t.clientY; lastT = e.timeStamp;
      scrollBy(d);
    }, { capture: true, passive: false });
    mount.addEventListener("contextmenu", (e) => { if (held || holdTimer) e.preventDefault(); }, { capture: true });
    mount.addEventListener("touchcancel", cancelHold, { capture: true });
    mount.addEventListener("touchend", (e) => {
      cancelHold();
      if (held) { e.preventDefault(); return; }
      if (!active) return;
      active = false;
      if (performance.now() - lastT > 80 || Math.abs(vel) < 0.3) return;
      let v = vel * 16, prev = performance.now();
      const step = (now) => {
        const dt = Math.min(50, now - prev); prev = now;
        scrollBy(v * dt / 16);
        v *= Math.pow(0.95, dt / 16);
        fling = Math.abs(v) > 0.5 ? requestAnimationFrame(step) : 0;
      };
      fling = requestAnimationFrame(step);
    }, { capture: true });
  }

  // Touch select view: the visible screen as plain text over the terminal,
  // so Android/iOS selection handles and the system copy menu work. The word
  // under the long press is preselected.
  openSelectView(cx, cy) {
    const term = this.term;
    if (!term) return;
    this.closeSelectView();
    const host = this.el.querySelector(".pane-term");
    const r = term.element.querySelector(".xterm-screen").getBoundingClientRect();
    const buf = term.buffer.active;
    const el = (tag, cls, text) => {
      const n = document.createElement(tag);
      if (cls) n.className = cls;
      if (text) n.textContent = text;
      return n;
    };
    // Rows that run to the last column continue on the next row (soft wrap,
    // or a TUI hard-wrapping a long URL), so join them without a newline.
    const rows = [];
    for (let i = 0; i < term.rows; i++) {
      const line = buf.getLine(buf.viewportY + i);
      rows.push({ full: line?.translateToString(false) ?? "", wrapped: !!line?.isWrapped });
    }
    let text = "";
    const rowStart = [];
    rows.forEach((row, i) => {
      const next = rows[i + 1];
      const joins = next && (next.wrapped || /\S$/.test(row.full) && row.full.length >= term.cols);
      rowStart.push(text.length);
      text += joins ? row.full : row.full.replace(/\s+$/, "");
      if (next && !joins) text += "\n";
    });
    const view = el("div", "select-view");
    const bar = el("div", "select-bar");
    const copyBtn = el("button", "", "copy");
    const allBtn = el("button", "", "select all");
    const closeBtn = el("button", "", "close");
    bar.append(copyBtn, allBtn, closeBtn);
    const pre = el("pre", "select-text");
    pre.textContent = text;
    pre.style.font = `${fontSize}px SF Mono, Menlo, Monaco, monospace`;
    pre.style.lineHeight = `${r.height / term.rows}px`;
    view.append(bar, pre);
    host.parentElement.appendChild(view);
    const close = () => { view.remove(); this.term?.focus(); };
    const selected = () => {
      const s = document.getSelection();
      return s && pre.contains(s.anchorNode) ? s.toString() : "";
    };
    copyBtn.addEventListener("click", () => {
      const text = selected() || pre.textContent;
      const done = () => { copyBtn.textContent = "copied"; setTimeout(close, 600); };
      (navigator.clipboard ? navigator.clipboard.writeText(text) : Promise.reject())
        .then(done, () => { if (legacyCopy(text)) done(); });
    });
    allBtn.addEventListener("click", () => {
      const range = document.createRange();
      range.selectNodeContents(pre);
      const s = document.getSelection();
      s.removeAllRanges();
      s.addRange(range);
    });
    closeBtn.addEventListener("click", close);
    const row = Math.min(term.rows - 1, Math.max(0, Math.floor((cy - r.top) / (r.height / term.rows))));
    const col = Math.max(0, Math.floor((cx - r.left) / (r.width / term.cols)));
    const at = rowStart[row] + Math.min(col, Math.max(0, (rowStart[row + 1] ?? text.length) - rowStart[row] - 1));
    if (/\S/.test(text[at] || "")) {
      let a = at, b = at;
      while (a > 0 && /\S/.test(text[a - 1])) a--;
      while (b < text.length && /\S/.test(text[b])) b++;
      const range = document.createRange();
      range.setStart(pre.firstChild, a);
      range.setEnd(pre.firstChild, b);
      const sel = document.getSelection();
      sel.removeAllRanges();
      sel.addRange(range);
    }
  }

  closeSelectView() {
    this.el.querySelector(".select-view")?.remove();
  }

  connect() {
    // Captured for the close handler: a pane re-used for another session
    // must not resurrect the old one. (Previously this read the global
    // window.name, so unexpected closes never reconnected at all.)
    const name = this.session;
    const proto = location.protocol === "https:" ? "wss:" : "ws:";
    // Attach at the size this pane will have, so tmux never sees a stand-in
    // size first (each resize makes the app inside redraw).
    // The resize sent on open still goes out: a remote daemon without
    // ?cols&rows support would otherwise stay at its default size, and the
    // kernel signals nothing when the size did not change.
    let size = "";
    this.sentSize = "";
    try {
      this.fit?.fit();
      if (this.term?.cols > 1 && this.term?.rows > 1) {
        size = `?cols=${this.term.cols}&rows=${this.term.rows}`;
      }
    } catch {}
    const ws = new WebSocket(`${proto}//${location.host}${keyApi(this.session, "/attach")}${size}`);
    ws.binaryType = "arraybuffer";
    this.ws = ws;
    this.lastRx = Date.now();
    this.hbSeen = false;
    this.connectStarted = Date.now();
    wsdbg.log("connect", { s: name });
    // Right after a phone resumes, the tailnet path needs a few seconds to
    // come back. A socket started inside that window sits in CONNECTING on
    // TCP retransmit backoff (open only after ~16s), with no error and no
    // close — and alive() calls it healthy, so nothing retried and the pane
    // stayed blank. Say that it is connecting, and replace a socket that
    // does not open in time with a fresh one.
    this.clearConnectTimers();
    this.slowTimer = setTimeout(() => {
      if (this.ws === ws && ws.readyState === WebSocket.CONNECTING) this.msg("connecting… (tap to retry now)");
    }, 1500);
    const limit = Math.min(10000, 4000 + 2000 * (this.connectAttempts || 0));
    this.connectTimer = setTimeout(() => {
      if (this.ws !== ws || ws.readyState !== WebSocket.CONNECTING) return;
      wsdbg.log("connect-timeout", { s: name, ms: limit });
      this.connectAttempts = (this.connectAttempts || 0) + 1;
      ws.onclose = null;
      try { ws.close(); } catch {}
      this.ws = null;
      if (this.session === name && !this.deliberate) this.connect();
    }, limit);
    ws.onopen = () => {
      if (this.ws !== ws) { try { ws.close(); } catch {} return; }
      this.clearConnectTimers();
      this.connectAttempts = 0;
      this.retries = 0;
      this.lastRx = Date.now();
      wsdbg.log("open", { s: name });
      this.msg("");
      // The previous tmux client's stream may have been cut anywhere: inside
      // an escape sequence, with a scroll region or colours still set. The
      // new client repaints everything, but onto that leftover state its
      // paint landed wrong — doubled and misplaced text after switching
      // devices. A full reset first gives it the blank terminal it assumes.
      this.term.write("\x1bc");
      try { this.term.clearTextureAtlas(); } catch {}
      this.sendResize(true);
      this.flushPendingInput();
      this.startPresence();
      refreshSessions();
    };
    ws.onmessage = (e) => {
      if (this.ws !== ws) return;
      this.lastRx = Date.now();
      if (e.data instanceof ArrayBuffer) {
        this.term.write(new Uint8Array(e.data));
        return;
      }
      try {
        const m = JSON.parse(e.data);
        if (m.type === "hb") { this.hbSeen = true; return; }
        if (m.type === "error") this.term.write(`\r\n[muxdeck] ${m.data}\r\n`);
      } catch {}
    };
    ws.onerror = () => wsdbg.log("error", { s: name, rs: ws.readyState });
    ws.onclose = (e) => {
      wsdbg.log("close", { s: name, code: e.code, clean: e.wasClean ? 1 : 0 });
      if (this.ws !== ws) return; // a socket already replaced by a reconnect
      this.clearConnectTimers();
      if (!this.deliberate && this.session === name) this.scheduleReconnect();
    };
  }

  // A suspension kills sockets without delivering close events, and the
  // zombie still reports readyState OPEN — after any real time away,
  // reconnect on evidence, not on the socket's word.
  forceReconnect() {
    if (!this.session) return;
    wsdbg.log("force-reconnect", {
      s: this.session,
      rs: this.ws ? this.ws.readyState : -1,
      gap: this.lastRx ? Date.now() - this.lastRx : -1,
    });
    clearTimeout(this.retryTimer);
    if (this.ws) {
      this.ws.onclose = null;
      try { this.ws.close(); } catch {}
      this.ws = null;
    }
    this.msg("");
    this.connect();
  }

  static resumeAll(awayMs) {
    for (const p of panes) {
      if (!p.session) continue;
      if (!p.alive() || awayMs > 15000) p.forceReconnect();
    }
  }

  alive() {
    return this.ws && (this.ws.readyState === WebSocket.OPEN || this.ws.readyState === WebSocket.CONNECTING);
  }

  async scheduleReconnect() {
    const name = this.session;
    try {
      // List the host the session actually lives on — checking a remote
      // key against the local list would declare it dead on any blip.
      const i = name.indexOf(":");
      const listPath = i < 0 ? "/api/sessions" : `/api/remotes/${encodeURIComponent(name.slice(0, i))}/proxy/sessions`;
      const sessions = await api(listPath);
      if (!sessions.some((s) => s.name === name.slice(i + 1))) {
        this.detach();
        this.msg("session ended");
        refreshSessions();
        return;
      }
    } catch {} // server unreachable: keep retrying
    // Logged out: retrying only piles up 401s; login reconnects every pane.
    if (loginShown()) { this.msg(""); return; }
    // A resume may have reconnected while the session list was in flight;
    // the banner would then sit over a live terminal.
    if (this.session !== name || this.alive()) return;
    const delay = Math.min(15000, 1000 * 2 ** this.retries++);
    this.msg(`disconnected — retrying in ${Math.round(delay / 1000)}s (tap to retry now)`);
    clearTimeout(this.retryTimer);
    this.retryTimer = setTimeout(() => {
      if (this.session === name && !this.alive()) this.connect();
    }, delay);
  }

  clearConnectTimers() {
    clearTimeout(this.slowTimer);
    clearTimeout(this.connectTimer);
  }

  reconnectNow() {
    // A tap on "connecting…" means the user is done waiting for that socket.
    if (this.session && this.ws?.readyState === WebSocket.CONNECTING && Date.now() - this.connectStarted > 1500) {
      this.forceReconnect();
      return;
    }
    if (this.session && !this.alive()) {
      clearTimeout(this.retryTimer);
      this.msg("");
      this.connect();
    }
  }

  // --- presence: a pane that is on screen tells its daemon so every 15s
  // over the socket it already holds (terminal attach or mush stream), and
  // says "inactive" when the page hides. The daemon owns the timing: a
  // lease outlives its last renewal by a minute, so a phone that locks
  // without a close frame still lets the machine sleep. Nothing here keeps
  // the socket alive — that is the transport's job, and it earns no lease.

  presenceSocket() {
    if (this.view?.type === "mush") return this.mushWS;
    return this.session ? this.ws : null;
  }

  sendPresence(active) {
    const ws = this.presenceSocket();
    if (!ws || ws.readyState !== WebSocket.OPEN) return;
    try { ws.send(JSON.stringify({ type: "presence", data: active ? "active" : "inactive" })); } catch {}
  }

  startPresence() {
    this.stopPresence();
    if (!document.hidden) this.sendPresence(true);
    this.presenceTimer = setInterval(() => { if (!document.hidden) this.sendPresence(true); }, 15000);
  }

  stopPresence() {
    clearInterval(this.presenceTimer);
    this.presenceTimer = null;
  }

  detach(reattaching = false) {
    this.closeSelectView();
    this.deliberate = true;
    this.pendingInput = "";
    this.stopPresence();
    clearTimeout(this.retryTimer);
    this.clearConnectTimers();
    this.connectAttempts = 0;
    if (this.ws) {
      this.ws.onclose = null;
      this.ws.close();
      this.ws = null;
    }
    if (this.term) {
      this.term.dispose();
      this.term = null;
      this.fit = null;
      this.search = null;
    }
    this.session = null;
    this.retries = 0;
    this.deliberate = false;
    this.toggleFind(false);
    this.el.querySelector(".p-mouse").hidden = true;
    if (!reattaching) {
      this.el.querySelector(".pane-title").textContent = "no session";
      this.msg("select or create a session");
      updateHash();
    }
  }

  // The same tile the session has in the sidebar, beside the pane's title.
  showTile(s) {
    const slot = this.el.querySelector(".pane-tile");
    if (!s) { slot.hidden = true; return; }
    const agent = s.agent?.agent || "";
    if (slot.hidden || slot.dataset.agent !== agent) {
      slot.replaceChildren(agentTile(s.agent?.agent));
      slot.dataset.agent = agent;
      slot.hidden = false;
    }
  }

  // Input typed while the socket is down — a phone resuming reconnects on
  // every return to the app — is held and sent once the new socket opens,
  // instead of being dropped. Bounded: a dead session must not grow it.
  // passive: not typed by the user here (a terminal report, or keyboard
  // text the IME diff already accounts for), so it does not reset that
  // diff. hold: worth sending after a reconnect; typed text is.
  send(data, passive = false, hold = !passive) {
    if (!passive) this.imeMark?.();
    if (this.ws && this.ws.readyState === WebSocket.OPEN) {
      this.ws.send(JSON.stringify({ type: "input", data }));
      return;
    }
    // Terminal reports (mouse clicks, focus, device replies) describe a
    // screen that is gone by the time a new socket opens: drop them.
    if (!this.session || !hold) return;
    this.pendingInput = (this.pendingInput || "") + data;
    if (this.pendingInput.length > 16384) this.pendingInput = this.pendingInput.slice(-16384);
  }

  flushPendingInput() {
    const data = this.pendingInput;
    if (!data || !this.ws || this.ws.readyState !== WebSocket.OPEN) return;
    this.pendingInput = "";
    wsdbg.log("input-held", { s: this.session, n: data.length });
    this.ws.send(JSON.stringify({ type: "input", data }));
  }

  // The software keyboard's animation fires the viewport, window and
  // ResizeObserver callbacks several times each. Every size that reached the
  // pty was a SIGWINCH and a full TUI repaint, and a repaint drawn for one
  // size landing in a terminal already at the next one left stray, doubled
  // lines. Fit locally at once, tell the server only the settled size.
  sendResize(now = false) {
    if (!this.ws || this.ws.readyState !== WebSocket.OPEN || !this.fit) return;
    this.fit.fit();
    clearTimeout(this.resizeTimer);
    if (now) this.flushResize();
    else this.resizeTimer = setTimeout(() => this.flushResize(), 150);
  }

  // tmux sizes a session to the client that was last active. A device that
  // stayed attached while another one took over keeps showing that device's
  // size until a key is typed here; claiming says "this one is in use now".
  claim() {
    if (!this.ws || this.ws.readyState !== WebSocket.OPEN) return;
    const now = Date.now();
    if (now - (this.claimedAt || 0) < 3000) return;
    this.claimedAt = now;
    this.ws.send(JSON.stringify({ type: "claim" }));
  }

  flushResize() {
    if (!this.ws || this.ws.readyState !== WebSocket.OPEN || !this.term) return;
    const size = `${this.term.cols}x${this.term.rows}`;
    if (size === this.sentSize) return;
    this.sentSize = size;
    this.ws.send(JSON.stringify({ type: "resize", cols: this.term.cols, rows: this.term.rows }));
  }

  async refreshMouse() {
    const btn = this.el.querySelector(".p-mouse");
    try {
      const { enabled } = await api(keyApi(this.session, "/mouse"));
      btn.hidden = false;
      btn.classList.toggle("armed", enabled);
    } catch { btn.hidden = true; }
  }

  // The quiet alert lives on the session (a tmux option), so the button
  // follows the list: showQuiet is called on every refresh.
  showQuiet(s) {
    const btn = this.el.querySelector(".p-quiet");
    btn.hidden = !s || !!s.remote;
    btn.dataset.title ||= btn.title;
    btn.classList.toggle("armed", !!s?.quiet);
    btn.title = s?.quiet ? "quiet alert on: you get a notification once this session stops printing" : btn.dataset.title;
  }

  async toggleQuiet() {
    if (!this.session) return;
    const btn = this.el.querySelector(".p-quiet");
    const want = !btn.classList.contains("armed");
    try {
      await api(keyApi(this.session, "/quiet"), { method: "POST", body: JSON.stringify({ enabled: want }) });
      btn.classList.toggle("armed", want);
      btn.title = want ? "quiet alert on: you get a notification once this session stops printing" : btn.dataset.title;
      refreshSessions();
    } catch (err) { alert(err.message); }
  }

  async toggleMouse() {
    if (!this.session) return;
    const btn = this.el.querySelector(".p-mouse");
    const want = !btn.classList.contains("armed");
    try {
      await api(keyApi(this.session, "/mouse"), {
        method: "POST",
        body: JSON.stringify({ enabled: want }),
      });
      btn.classList.toggle("armed", want);
      this.term?.focus();
    } catch (err) { alert(err.message); }
  }

  toggleFind(show = this.el.querySelector(".findbar").hidden) {
    const fb = this.el.querySelector(".findbar");
    if (fb.hidden === !show) return;
    fb.hidden = !show;
    if (show) {
      this.findInput.select();
      this.findInput.focus();
    } else {
      this.search?.clearDecorations?.();
      this.term?.focus();
    }
  }

  findNext(incremental) {
    if (this.search && this.findInput.value) this.search.findNext(this.findInput.value, { incremental });
  }

  findPrev() {
    if (this.search && this.findInput.value) this.search.findPrevious(this.findInput.value);
  }
}

function addPane() {
  const p = new Pane();
  panes.push(p);
  $("#panes").appendChild(p.el);
  $("#panes").classList.toggle("split", panes.length > 1);
  $("#split").classList.toggle("armed", panes.length > 1);
  updateDivider();
  setFocus(p);
  return p;
}

function closePane(p) {
  if (p.view) p.closeView();
  if (panes.length === 1) {
    p.detach();
    return;
  }
  p.detach();
  p.el.remove();
  panes = panes.filter((q) => q !== p);
  $("#panes").classList.toggle("split", panes.length > 1);
  $("#split").classList.toggle("armed", panes.length > 1);
  updateDivider();
  if (focusedPane === p) setFocus(panes[0]);
  updateHash();
  refreshSessions();
}

$("#split").addEventListener("click", () => {
  if (panes.length === 1) addPane();
  else closePane(panes[panes.length - 1]);
});

// --- layout: sidebar width/collapse + split ratio, persisted ---

function sidebarW() {
  return Math.max(230, Math.min(480, +localStorage.getItem("muxdeck-sidebar-w") || 230));
}

function applySidebar() {
  document.documentElement.style.setProperty("--sidebar-w", `${sidebarW()}px`);
  $("#sidebar").classList.toggle("collapsed", localStorage.getItem("muxdeck-sidebar-collapsed") === "1");
  paintSidebarToggle();
}

function toggleSidebar() {
  const collapsed = $("#sidebar").classList.toggle("collapsed");
  localStorage.setItem("muxdeck-sidebar-collapsed", collapsed ? "1" : "");
  paintSidebarToggle();
}

// On a phone the sidebar sits above the terminal, so it folds up instead of
// sideways.
const stacked = matchMedia("(max-width: 640px)");

function paintSidebarToggle() {
  const collapsed = $("#sidebar").classList.contains("collapsed");
  const [open, shut] = stacked.matches ? ["chevronUp", "chevronDown"] : ["chevronLeft", "chevronRight"];
  $("#sidebar-toggle").innerHTML = icon(collapsed ? shut : open);
  $("#sidebar-toggle").title = collapsed ? "Expand sidebar" : "Collapse sidebar";
}
stacked.addEventListener("change", paintSidebarToggle);
$("#sidebar h1").addEventListener("click", () => {
  if (stacked.matches && $("#sidebar").classList.contains("collapsed")) toggleSidebar();
});

// Horizontal drag: pointer capture keeps moves on the handle even when the
// cursor crosses the terminal canvas.
function hDrag(el, move) {
  el.addEventListener("pointerdown", (e) => {
    e.preventDefault();
    el.setPointerCapture(e.pointerId);
    el.classList.add("dragging");
    const onMove = (ev) => move(ev.clientX);
    const onUp = () => {
      el.classList.remove("dragging");
      el.removeEventListener("pointermove", onMove);
      el.removeEventListener("pointerup", onUp);
    };
    el.addEventListener("pointermove", onMove);
    el.addEventListener("pointerup", onUp);
  });
}

hDrag($("#sidebar-resizer"), (x) => {
  localStorage.setItem("muxdeck-sidebar-w", Math.round(Math.max(230, Math.min(480, x))));
  document.documentElement.style.setProperty("--sidebar-w", `${sidebarW()}px`);
});
$("#sidebar-resizer").addEventListener("dblclick", toggleSidebar);
$("#sidebar-toggle").addEventListener("click", toggleSidebar);

let divider = null;

function applySplit() {
  if (panes.length < 2) return;
  const ratio = +localStorage.getItem("muxdeck-split") || 0;
  panes[0].el.style.flex = ratio ? `0 0 ${(ratio * 100).toFixed(1)}%` : "";
}

function updateDivider() {
  if (panes.length === 2 && !divider) {
    divider = document.createElement("div");
    divider.id = "pane-divider";
    divider.title = "drag to resize · double-click for 50/50";
    $("#panes").insertBefore(divider, panes[1].el);
    hDrag(divider, (x) => {
      const r = $("#panes").getBoundingClientRect();
      const ratio = Math.max(0.15, Math.min(0.85, (x - r.left) / r.width));
      localStorage.setItem("muxdeck-split", ratio.toFixed(3));
      applySplit();
    });
    divider.addEventListener("dblclick", () => {
      localStorage.removeItem("muxdeck-split");
      applySplit();
    });
    applySplit();
  } else if (panes.length < 2 && divider) {
    divider.remove();
    divider = null;
    panes[0]?.el.style.removeProperty("flex");
  }
}

function updateHash() {
  const names = panes.map((p) => p.session).filter(Boolean);
  history.replaceState(null, "", names.length ? `#${names.map(encodeURIComponent).join(",")}` : location.pathname);
}

function attachFromHash() {
  const names = location.hash.slice(1).split(",").filter(Boolean).map(decodeURIComponent);
  if (names[0]) panes[0].attach(names[0]);
  if (names[1] && matchMedia("(min-width: 700px)").matches) addPane().attach(names[1]);
}

// --- session list ---

let lastSessions = [];
let remotes = []; // /api/remotes statuses
let cloudStatus = null; // /api/cloud — the account whose machines fill the cloud group
let mushRuns = new Map(); // host ("" = local) → /api/mush/runs payload
const remoteSessions = new Map(); // remote name -> its session list
const seenActivity = new Map(); // key -> last activity we consider seen
const agentStates = new Map(); // key -> last agent state, for waiting-transition alerts

// Sessions are identified by key: "name" locally, "remote:name" on a remote
// (session names can't contain ":", so the split is unambiguous). Keys flow
// through panes, the palette, the hash, and the activity/agent maps.

function keyApi(key, rest = "") {
  const i = key.indexOf(":");
  if (i < 0) return `/api/sessions/${encodeURIComponent(key)}${rest}`;
  return `/api/remotes/${encodeURIComponent(key.slice(0, i))}/proxy/sessions/${encodeURIComponent(key.slice(i + 1))}${rest}`;
}

function allSessions() {
  const out = orderedSessions().map((s) => ({ ...s, key: s.name }));
  for (const r of remotes) {
    for (const s of remoteSessions.get(r.name) || []) {
      out.push({ ...s, key: `${r.name}:${s.name}`, remote: r.name });
    }
  }
  return out;
}

let collapsedRemotes = {};
try { collapsedRemotes = JSON.parse(localStorage.getItem("muxdeck-remotes-collapsed")) || {}; } catch {}

// Two notification backends: the web Notification API in browsers, and the
// Tauri notification plugin in the desktop app (WKWebView has no web API;
// the plugin posts to macOS Notification Center instead — works even with
// the window closed).
const tauriNotif = window.__TAURI__?.notification;

async function notifGranted() {
  if (tauriNotif) return tauriNotif.isPermissionGranted();
  return typeof Notification !== "undefined" && Notification.permission === "granted";
}

async function notifyAgentWaiting(s) {
  if (!(await notifGranted())) return;
  const title = `${s.key}: agent needs input`;
  const body = s.agent.note || `${s.agent.agent} is waiting for you`;
  if (tauriNotif) {
    // Clicking a Notification Center alert focuses the app (macOS default);
    // per-notification click handlers aren't exposed by the plugin.
    tauriNotif.sendNotification({ title, body });
    return;
  }
  const opts = {
    body,
    tag: `muxdeck-agent-${s.key}`, // collapse repeats for the same session
    icon: "icons/icon-192.png",
    data: { session: s.key },
  };
  // Android Chrome (tab or PWA) has no page Notification constructor — it
  // throws "Illegal constructor"; only the service worker can show one, and
  // its notificationclick posts the session back here to attach.
  const reg = await navigator.serviceWorker?.getRegistration().catch(() => null);
  if (reg) {
    try {
      await reg.showNotification(title, { ...opts, renotify: true });
      return;
    } catch {}
  }
  let n;
  try {
    n = new Notification(title, opts);
  } catch {
    return;
  }
  n.onclick = () => {
    window.focus();
    paneFor().attach(s.key);
    n.close();
  };
}

function orderedSessions() {
  const idx = new Map(order.map((n, i) => [n, i]));
  return [...lastSessions].sort(
    (a, b) => (idx.has(a.name) ? idx.get(a.name) : 1e9) - (idx.has(b.name) ? idx.get(b.name) : 1e9)
  );
}

// --- macOS folder-privacy (TCC) banner ---
// The daemon records protected folders macOS denied it (GET /api/doctor);
// without this a remote user just watches ls hang in the terminal. The poll
// is passive — the daemon never probes on its own, so no consent prompts
// fire on a screen nobody is watching.

let tccSupported = true; // until the daemon says otherwise
let tccDismissed = [];
try { tccDismissed = JSON.parse(localStorage.getItem("muxdeck-tcc-dismissed")) || []; } catch {}

async function refreshTcc() {
  if (!tccSupported) return;
  let rep;
  try { rep = await api("/api/doctor"); } catch { return; }
  tccSupported = rep.supported;
  const banner = $("#tcc-banner");
  const hits = (rep.hits || []).filter((p) => !tccDismissed.includes(p));
  if (!hits.length) { banner.hidden = true; return; }
  banner.innerHTML = icon("alert");
  const span = document.createElement("span");
  span.className = "tcc-text";
  span.append(`macOS is blocking the daemon from ${hits.join(", ")} — grant Full Disk Access to muxdeck and tmux (System Settings › Privacy & Security), or run `);
  const code = document.createElement("code");
  code.textContent = "muxdeck doctor";
  code.title = "click to copy";
  code.addEventListener("click", () => navigator.clipboard?.writeText("muxdeck doctor"));
  span.append(code, " on the Mac.");
  const dismiss = document.createElement("button");
  dismiss.className = "tcc-dismiss";
  dismiss.title = "dismiss for these folders";
  dismiss.innerHTML = icon("x");
  dismiss.addEventListener("click", () => {
    tccDismissed = [...new Set([...tccDismissed, ...hits])];
    localStorage.setItem("muxdeck-tcc-dismissed", JSON.stringify(tccDismissed));
    banner.hidden = true;
  });
  banner.append(span, dismiss);
  banner.hidden = false;
}

function renderTmuxMissing() {
  const ul = $("#sessions");
  ul.innerHTML = "";
  const li = document.createElement("li");
  li.className = "tmux-missing";
  const cmd = /Mac/.test(navigator.platform) ? "brew install tmux" : "sudo apt install tmux";
  const b = document.createElement("b");
  b.textContent = "muxdeck needs tmux";
  const p = document.createElement("p");
  p.textContent = "Install it, then hit refresh up top:";
  const code = document.createElement("code");
  code.textContent = cmd;
  code.title = "click to copy";
  code.addEventListener("click", () => {
    navigator.clipboard?.writeText(cmd);
    p.textContent = "Copied. Install it, then hit refresh up top:";
  });
  li.append(b, p, code);
  ul.appendChild(li);
}

// The machine name behind this daemon (/api/meta). Through the relay every
// daemon looks alike, so the sidebar heads the daemon's own sessions with
// this instead of a generic "Local". Fetched once; a failure (old daemon,
// not yet logged in) retries on the next tick and falls back to "Local".
let machineName = "";
let machineNameFetched = false;
async function fetchMachineName() {
  if (machineNameFetched) return;
  try {
    machineName = (await api("/api/meta")).name || "";
    machineNameFetched = true;
  } catch {}
}

// One refresh at a time: the tick, the long poll, pushes and socket opens
// all ask for one, often together. A request during a refresh queues one
// more run after it instead of a second refresh in parallel.
let refreshing = null, refreshQueued = false, refreshWhenVisible = false;
function refreshSessions() {
  // A hidden page renders nothing; it only refreshes to raise its own
  // alerts, which a device with Web Push leaves to the daemon. Sessions on
  // a remote are not pushed from here, so with remotes it keeps refreshing.
  if (document.hidden && pushActive && !remotes.length) { refreshWhenVisible = true; return Promise.resolve(); }
  if (refreshing) { refreshQueued = true; return refreshing; }
  refreshing = (async () => {
    try {
      do { refreshQueued = false; await refreshSessionsNow(); } while (refreshQueued);
    } finally { refreshing = null; }
  })();
  return refreshing;
}

// What hardly changes (the remote list, cloud sign-in, mush runs when
// there are none) is fetched once a minute, not on every refresh.
let slowFetchedAt = 0;
// Once a minute while remotes, the cloud or mush are in use; otherwise only
// now and then, to notice that one got set up.
const slowEvery = () => (remotes.length || cloudStatus?.signed_in || mushRuns.size ? 60000 : 15 * 60000);

// A tap on a row must not land in the middle of a rebuild of the list.
let sidebarPress = false, sidebarStale = false;

async function refreshSessionsNow() {
  void loadServerSettingsOnce();
  await fetchMachineName();
  let sessions;
  const askedAt = Date.now();
  try {
    sessions = await api("/api/sessions");
  } catch (err) {
    if (err.message === "tmux-missing") renderTmuxMissing();
    return;
  }
  lastSessions = sessions;
  if (!document.hidden) prunePermNotifications(sessions, askedAt);
  try { lostSessions = (await api("/api/restore")).lost || []; } catch { lostSessions = []; }

  // Remote lists ride the same refresh tick; a dead remote must not stall
  // or break the local render, so failures just flip its state to down.
  const slow = Date.now() - slowFetchedAt > slowEvery();
  if (slow) slowFetchedAt = Date.now();
  if (slow || remotes.length) {
    try { remotes = await api("/api/remotes"); } catch { remotes = []; }
  }
  if (slow || cloudStatus?.signed_in) {
    try { cloudStatus = await api("/api/cloud"); } catch { cloudStatus = null; }
  }
  await Promise.all(remotes.map(async (r) => {
    if (r.state !== "ok") { remoteSessions.delete(r.name); return; }
    try {
      remoteSessions.set(r.name, await api(`/api/remotes/${encodeURIComponent(r.name)}/proxy/sessions`));
    } catch (err) {
      remoteSessions.delete(r.name);
      r.state = "down";
      r.error = err.message;
    }
  }));

  const attached = new Set(panes.map((p) => p.session).filter(Boolean));
  for (const s of allSessions()) {
    // Viewing a session counts as seeing its output; new sessions start seen.
    if (attached.has(s.key) || !seenActivity.has(s.key)) seenActivity.set(s.key, s.activity);

    // Alert on the transition into waiting, not while it persists — and not
    // when the session is already on screen in a visible tab.
    const state = s.agent?.state;
    if (
      state === "waiting" &&
      agentStates.get(s.key) !== "waiting" &&
      agentStates.has(s.key) &&
      (document.hidden || !attached.has(s.key)) &&
      !(pushActive && !s.remote)
    ) {
      notifyAgentWaiting(s);
    }
    if (state) agentStates.set(s.key, state);
    else agentStates.delete(s.key);
  }

  for (const p of panes) {
    const s = lastSessions.find((x) => x.name === p.session);
    p.showPermissions(p.view ? [] : [...(s?.permissions || []), ...(s?.blocked || []).map((b) => ({ ...b, blocked: true }))]);
    p.showTile(p.view ? null : allSessions().find((x) => x.key === p.session));
    p.showQuiet(lastSessions.find((x) => x.name === p.session));
  }

  // mush runs ride the same tick: the local ledger plus each live remote's.
  // A daemon without mush answers available:false and an empty list.
  if (slow || mushRuns.size || remotes.length) await refreshRuns();
  refreshTcc(); // fire and forget: the banner must never stall the list

  const needYou = allSessions().filter((s) => agentPhase(s) === "needs").length;
  let alert = $("#sidebar h1 .hdr-alert");
  if (!alert) {
    alert = document.createElement("span");
    alert.className = "hdr-alert";
    $("#sidebar h1").appendChild(alert);
  }
  alert.textContent = needYou ? `${needYou} need${needYou === 1 ? "s" : ""} you` : "";
  alert.title = needYou ? `${needYou} session${needYou === 1 ? "" : "s"} waiting for you` : "";

  if (sidebarPress) { sidebarStale = true; return; }
  const ul = $("#sessions");
  ul.innerHTML = "";
  if (lostSessions.length) restoreSection(ul);
  ul.appendChild(sectionHeader(machineName || "Local", lastSessions.length));
  const limits = prefs.limits ? limitsRow(lastSessions) : null;
  if (limits) ul.appendChild(limits);
  for (const s of orderedSessions()) ul.appendChild(sessionRow({ ...s, key: s.name }, attached));
  const manual = remotes.filter((r) => !r.cloud), cloud = remotes.filter((r) => r.cloud);
  const remoteGroup = (list) => {
    for (const r of list) {
      ul.appendChild(remoteHeader(r));
      if (collapsedRemotes[r.name] || r.state !== "ok") continue;
      const limits = prefs.limits ? limitsRow(remoteSessions.get(r.name) || []) : null;
      if (limits) ul.appendChild(limits);
      for (const s of remoteSessions.get(r.name) || []) {
        ul.appendChild(sessionRow({ ...s, key: `${r.name}:${s.name}`, remote: r.name }, attached));
      }
    }
  };
  if (manual.length) ul.appendChild(sectionHeader("Remotes", manual.length));
  remoteGroup(manual);
  // The account's machines are a group of their own: signed in, it lists
  // every other box on the account; revoked, it stays as the place that
  // says so until the next sign-in.
  if (cloudStatus?.signed_in || cloudStatus?.state === "revoked") {
    const head = sectionHeader("muxdeck cloud", cloudStatus.signed_in ? cloud.length : "!");
    head.classList.add("cloud-head");
    head.title = cloudStatus.signed_in
      ? `${cloudStatus.account || "signed in"} · ${cloudStatus.state}${cloudStatus.error ? ` — ${cloudStatus.error}` : ""} · :cloud to manage`
      : `${cloudStatus.error || "signed out"} · :cloud signin`;
    ul.appendChild(head);
    remoteGroup(cloud);
  }
  const runs = allRuns();
  if (runs.length) {
    const head = sectionHeader("Runs", runs.length);
    head.title = "mush runs · click to collapse";
    head.style.cursor = "pointer";
    head.addEventListener("click", () => {
      runsCollapsed = !runsCollapsed;
      localStorage.setItem("muxdeck-runs-collapsed", runsCollapsed ? "1" : "");
      refreshSessions();
    });
    ul.appendChild(head);
    if (!runsCollapsed) for (const r of runs.slice(0, RUNS_SHOWN)) ul.appendChild(runRow(r));
  }
  if (!$("#palette").hidden) renderPalette();
}

const RUNS_SHOWN = 12;
let runsCollapsed = localStorage.getItem("muxdeck-runs-collapsed") === "1";

async function refreshRuns() {
  const hosts = ["", ...remotes.filter((r) => r.state === "ok").map((r) => r.name)];
  await Promise.all(hosts.map(async (host) => {
    try {
      const res = await api(mushApi(host, "?limit=30"));
      if (res.available) mushRuns.set(host, res); else mushRuns.delete(host);
    } catch { mushRuns.delete(host); }
  }));
  for (const host of [...mushRuns.keys()]) if (!hosts.includes(host)) mushRuns.delete(host);
}

// allRuns flattens every host's ledger rows, newest first (ids sort
// chronologically by construction).
function allRuns() {
  const out = [];
  for (const [host, res] of mushRuns) for (const r of res.runs || []) out.push({ ...r, host });
  return out.sort((a, b) => (a.id < b.id ? 1 : a.id > b.id ? -1 : 0));
}

const RUN_LIVE = new Set(["queued", "planning", "implementing", "awaiting_approval", "verifying"]);
const RUN_OK = new Set(["done", "pr_open", "merged"]);

function runState(r) {
  return r.state === "awaiting_approval" ? "wait" : RUN_LIVE.has(r.state) ? "live" : RUN_OK.has(r.state) ? "ok" : "bad";
}

function runLabel(r) {
  if (r.source === "issue" && r.source_ref) return `#${r.source_ref}`;
  if (r.source === "schedule" && r.source_ref) return `⏰ ${r.source_ref}`;
  return r.source_ref || r.id;
}

function runDuration(r) {
  const start = Date.parse(r.started_at || r.created_at);
  if (!start || start < 0) return "";
  const end = Date.parse(r.finished_at);
  const secs = Math.max(0, Math.round(((end > 0 ? end : Date.now()) - start) / 1000));
  if (secs < 60) return `${secs}s`;
  if (secs < 3600) return `${Math.floor(secs / 60)}m`;
  return `${Math.floor(secs / 3600)}h${Math.floor((secs % 3600) / 60)}m`;
}

function runRow(r) {
  const li = document.createElement("li");
  li.className = "run-row";
  li.dataset.run = `${r.host}:${r.id}`;
  li.classList.toggle("active", panes.some((p) => p.view?.type === "mush" && p.view.runId === r.id && (p.view.host || "") === r.host));
  const dot = document.createElement("span");
  dot.className = `run-dot ${runState(r)}`;
  dot.title = r.state;
  const copy = document.createElement("span");
  copy.className = "session-copy";
  const name = document.createElement("span");
  name.className = "name";
  name.textContent = runLabel(r);
  const meta = document.createElement("span");
  meta.className = "meta";
  const proj = (r.project || "").split("/").filter(Boolean).pop() || "";
  meta.textContent = [r.host, proj, r.state.replace("_", " "), r.model, runDuration(r), r.verdict && r.verdict !== "-" ? r.verdict : ""]
    .filter(Boolean).join(" · ");
  copy.append(name, meta);
  li.append(dot, copy);
  if (!RUN_LIVE.has(r.state)) {
    const rm = document.createElement("button");
    rm.className = "kill";
    rm.innerHTML = icon("x");
    rm.title = `remove ${r.id} from the ledger`;
    rm.addEventListener("click", async (e) => {
      e.stopPropagation();
      await removeRun(r);
    });
    const actions = document.createElement("span");
    actions.className = "session-actions";
    actions.append(rm);
    li.append(actions);
  }
  li.title = `${r.id}\n${r.project}`;
  li.addEventListener("click", () => viewerPane().openMush(r.host, r));
  return li;
}

function sectionHeader(label, count) {
  const li = document.createElement("li");
  li.className = "section-head";
  const name = document.createElement("span");
  name.textContent = label;
  const total = document.createElement("span");
  total.className = "section-count";
  total.textContent = count;
  li.append(name, total);
  return li;
}

// The leaf and its parent are what identify a working directory at sidebar
// width; the full path stays in the row tooltip.
function shortPath(p) {
  const parts = p.split("/").filter(Boolean);
  return parts.slice(-2).join("/") || p;
}

// agentExtra renders the optional half of a pushed agent status — how far
// along the work is, and whatever facts the agent chose to publish alongside
// it. Returns null when there is nothing to show, so the row stays one line.
// Chips keyed "… limit" (the 5-hour and 7-day windows) belong to the agent's
// login, not to a session: every session of that agent reports the same
// numbers. They show once per agent under the machine's header instead.
const isAccountChip = (c) => / limit$/.test(c.key || "");
const AGENT_LABELS = { "claude-code": "Claude", codex: "Codex", gemini: "Gemini" };

// Brand marks from the Simple Icons set, drawn in the tile's own colour.
const AGENT_LOGOS = {
  "claude-code": "m4.7144 15.9555 4.7174-2.6471.079-.2307-.079-.1275h-.2307l-.7893-.0486-2.6956-.0729-2.3375-.0971-2.2646-.1214-.5707-.1215-.5343-.7042.0546-.3522.4797-.3218.686.0608 1.5179.1032 2.2767.1578 1.6514.0972 2.4468.255h.3886l.0546-.1579-.1336-.0971-.1032-.0972L6.973 9.8356l-2.55-1.6879-1.3356-.9714-.7225-.4918-.3643-.4614-.1578-1.0078.6557-.7225.8803.0607.2246.0607.8925.686 1.9064 1.4754 2.4893 1.8336.3643.3035.1457-.1032.0182-.0728-.164-.2733-1.3539-2.4467-1.445-2.4893-.6435-1.032-.17-.6194c-.0607-.255-.1032-.4674-.1032-.7285L6.287.1335 6.6997 0l.9957.1336.419.3642.6192 1.4147 1.0018 2.2282 1.5543 3.0296.4553.8985.2429.8318.091.255h.1579v-.1457l.1275-1.706.2368-2.0947.2307-2.6957.0789-.7589.3764-.9107.7468-.4918.5828.2793.4797.686-.0668.4433-.2853 1.8517-.5586 2.9021-.3643 1.9429h.2125l.2429-.2429.9835-1.3053 1.6514-2.0643.7286-.8196.85-.9046.5464-.4311h1.0321l.759 1.1293-.34 1.1657-1.0625 1.3478-.8804 1.1414-1.2628 1.7-.7893 1.36.0729.1093.1882-.0183 2.8535-.607 1.5421-.2794 1.8396-.3157.8318.3886.091.3946-.3278.8075-1.967.4857-2.3072.4614-3.4364.8136-.0425.0304.0486.0607 1.5482.1457.6618.0364h1.621l3.0175.2247.7892.522.4736.6376-.079.4857-1.2142.6193-1.6393-.3886-3.825-.9107-1.3113-.3279h-.1822v.1093l1.0929 1.0686 2.0035 1.8092 2.5075 2.3314.1275.5768-.3218.4554-.34-.0486-2.2039-1.6575-.85-.7468-1.9246-1.621h-.1275v.17l.4432.6496 2.3436 3.5214.1214 1.0807-.17.3521-.6071.2125-.6679-.1214-1.3721-1.9246L14.38 17.959l-1.1414-1.9428-.1397.079-.674 7.2552-.3156.3703-.7286.2793-.6071-.4614-.3218-.7468.3218-1.4753.3886-1.9246.3157-1.53.2853-1.9004.17-.6314-.0121-.0425-.1397.0182-1.4328 1.9672-2.1796 2.9446-1.7243 1.8456-.4128.164-.7164-.3704.0667-.6618.4008-.5889 2.386-3.0357 1.4389-1.882.929-1.0868-.0062-.1579h-.0546l-6.3385 4.1164-1.1293.1457-.4857-.4554.0608-.7467.2307-.2429 1.9064-1.3114Z",
  codex: "M22.2819 9.8211a5.9847 5.9847 0 0 0-.5157-4.9108 6.0462 6.0462 0 0 0-6.5098-2.9A6.0651 6.0651 0 0 0 4.9807 4.1818a5.9847 5.9847 0 0 0-3.9977 2.9 6.0462 6.0462 0 0 0 .7427 7.0966 5.98 5.98 0 0 0 .511 4.9107 6.051 6.051 0 0 0 6.5146 2.9001A5.9847 5.9847 0 0 0 13.2599 24a6.0557 6.0557 0 0 0 5.7718-4.2058 5.9894 5.9894 0 0 0 3.9977-2.9001 6.0557 6.0557 0 0 0-.7475-7.0729zm-9.022 12.6081a4.4755 4.4755 0 0 1-2.8764-1.0408l.1419-.0804 4.7783-2.7582a.7948.7948 0 0 0 .3927-.6813v-6.7369l2.02 1.1686a.071.071 0 0 1 .038.052v5.5826a4.504 4.504 0 0 1-4.4945 4.4944zm-9.6607-4.1254a4.4708 4.4708 0 0 1-.5346-3.0137l.142.0852 4.783 2.7582a.7712.7712 0 0 0 .7806 0l5.8428-3.3685v2.3324a.0804.0804 0 0 1-.0332.0615L9.74 19.9502a4.4992 4.4992 0 0 1-6.1408-1.6464zM2.3408 7.8956a4.485 4.485 0 0 1 2.3655-1.9728V11.6a.7664.7664 0 0 0 .3879.6765l5.8144 3.3543-2.0201 1.1685a.0757.0757 0 0 1-.071 0l-4.8303-2.7865A4.504 4.504 0 0 1 2.3408 7.872zm16.5963 3.8558L13.1038 8.364 15.1192 7.2a.0757.0757 0 0 1 .071 0l4.8303 2.7913a4.4944 4.4944 0 0 1-.6765 8.1042v-5.6772a.79.79 0 0 0-.407-.667zm2.0107-3.0231l-.142-.0852-4.7735-2.7818a.7759.7759 0 0 0-.7854 0L9.409 9.2297V6.8974a.0662.0662 0 0 1 .0284-.0615l4.8303-2.7866a4.4992 4.4992 0 0 1 6.6802 4.66zM8.3065 12.863l-2.02-1.1638a.0804.0804 0 0 1-.038-.0567V6.0742a4.4992 4.4992 0 0 1 7.3757-3.4537l-.142.0805L8.704 5.459a.7948.7948 0 0 0-.3927.6813zm1.0976-2.3654l2.602-1.4998 2.6069 1.4998v2.9994l-2.5974 1.4997-2.6067-1.4997Z",
  gemini: "M11.04 19.32Q12 21.51 12 24q0-2.49.93-4.68.96-2.19 2.58-3.81t3.81-2.55Q21.51 12 24 12q-2.49 0-4.68-.93a12.3 12.3 0 0 1-3.81-2.58 12.3 12.3 0 0 1-2.58-3.81Q12 2.49 12 0q0 2.49-.96 4.68-.93 2.19-2.55 3.81a12.3 12.3 0 0 1-3.81 2.58Q2.49 12 0 12q2.49 0 4.68.96 2.19.93 3.81 2.55t2.55 3.81",
};
const SHELL_MARK = '<svg viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.6" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="M3 4.5 6.5 8 3 11.5M8.5 12H13"/></svg>';

// The tile that says at a glance what runs in a session: the agent's mark,
// or a prompt for a plain shell.
function agentTile(agent) {
  const tile = document.createElement("span");
  const logo = AGENT_LOGOS[agent];
  tile.className = `tile tile-${logo ? agent : agent ? "other" : "shell"}`;
  if (logo) tile.innerHTML = `<svg viewBox="0 0 24 24" fill="currentColor" aria-hidden="true"><path d="${logo}"/></svg>`;
  else if (agent) tile.textContent = agent.slice(0, 2);
  else tile.innerHTML = SHELL_MARK;
  tile.title = agent ? AGENT_LABELS[agent] || agent : "shell";
  return tile;
}

// Agents report three states, but "waiting" covers both a finished turn and
// a question that blocks the work. The hooks say which in the note, and an
// open permission request is always the blocking kind.
function agentPhase(s) {
  if (s.permissions?.length || s.blocked?.length) return "needs";
  const a = s.agent;
  if (!a) return "";
  if (a.state === "working") return "working";
  if (a.state === "waiting") return / is done /.test(a.note || "") ? "done" : "needs";
  return "idle";
}

const PHASES = {
  working: ["working", '<path d="M6 1.2a4.8 4.8 0 1 1-4.8 4.8" fill="none" stroke="currentColor" stroke-width="1.6" stroke-linecap="round"/>'],
  needs: ["needs you", '<circle cx="6" cy="6" r="4" fill="currentColor"/>'],
  done: ["done", '<path d="M2.5 6.5 5 9l4.5-6" fill="none" stroke="currentColor" stroke-width="1.6" stroke-linecap="round" stroke-linejoin="round"/>'],
  idle: ["idle", '<circle cx="6" cy="6" r="3.6" fill="none" stroke="currentColor" stroke-width="1.4"/>'],
};

function phaseEl(phase, a) {
  const el = document.createElement("span");
  el.className = `phase phase-${phase}`;
  el.innerHTML = `<svg viewBox="0 0 12 12" aria-hidden="true">${PHASES[phase][1]}</svg>`;
  el.append(PHASES[phase][0]);
  if (a) {
    el.title = `${a.agent} is ${a.state}` + (a.note ? ` — ${a.note}` : "") +
      ` · updated ${new Date(a.updated_at).toLocaleTimeString()}`;
  }
  return el;
}

// One window of an account limit as a bar: "5h 60% · 13:00" becomes the
// window, how full it is, and when it resets.
function limitCells(c, expired) {
  const m = /^(\S+)\s+(\d+)%(?:\s*·\s*(.*))?$/.exec(c.value || "");
  if (!m) {
    const chip = chipEl(c);
    chip.classList.add("limit-raw");
    return [chip];
  }
  const pct = expired ? 0 : Math.min(100, +m[2]);
  const tone = !expired && c.color ? ` limit-${c.color}` : "";
  const cell = (cls, text) => {
    const el = document.createElement("span");
    el.className = cls;
    el.textContent = text;
    el.title = c.key ? `${c.key}: ${c.value}` : c.value;
    return el;
  };
  const bar = cell(`limit-bar${tone}`, "");
  const fill = document.createElement("span");
  fill.style.width = `${pct}%`;
  bar.appendChild(fill);
  return [cell("limit-win", m[1]), bar, cell(`limit-pct${tone}`, `${pct}%`), cell("limit-reset", expired ? "" : m[3] || "")];
}

function chipEl(c) {
  const chip = document.createElement("span");
  chip.className = c.color ? `chip chip-${c.color}` : "chip";
  // The daemon validates the icon name, but a row can also come from a
  // remote running a build that knows names this one does not.
  if (c.icon && ICONS[c.icon]) chip.innerHTML = icon(c.icon);
  chip.append(c.value);
  chip.title = c.key ? `${c.key}: ${c.value}` : c.value;
  return chip;
}

// Per agent and per window, the report on the newest window wins (latest
// resets_at), then the freshest report: an idle session keeps re-posting the
// numbers it last saw, so recency alone can resurrect an old window. A window
// whose reset has passed shows as unused until a report on the next one comes.
function limitsRow(sessions) {
  const latest = new Map();
  for (const s of sessions) {
    const a = s.agent;
    if (!a?.chips) continue;
    const t = Date.parse(a.updated_at) || 0;
    for (const c of a.chips) {
      if (!isAccountChip(c)) continue;
      if (!latest.has(a.agent)) latest.set(a.agent, new Map());
      const byKey = latest.get(a.agent);
      const prev = byKey.get(c.key);
      const r = c.resets_at || 0, pr = prev?.chip.resets_at || 0;
      if (!prev || r > pr || (r === pr && t > prev.t)) byKey.set(c.key, { chip: c, t });
    }
  }
  if (!latest.size) return null;
  const now = Date.now() / 1000;
  const li = document.createElement("li");
  li.className = "limits";
  for (const name of [...latest.keys()].sort()) {
    const byKey = latest.get(name);
    const t = Math.max(...[...byKey.values()].map((e) => e.t));
    let first = true;
    for (const { chip: c } of byKey.values()) {
      const label = document.createElement("span");
      label.className = `limit-agent tile-${AGENT_LOGOS[name] ? name : "other"}`;
      if (first) {
        label.textContent = AGENT_LABELS[name] || name;
        label.title = `${name} · updated ${new Date(t).toLocaleTimeString()}`;
      }
      first = false;
      li.append(label, ...limitCells(c, !!c.resets_at && c.resets_at <= now));
    }
  }
  return li;
}

function agentExtra(a) {
  const p = a?.progress;
  const bar = p && (p.label || p.value > 0);
  const chips = prefs.chips ? (a?.chips || []).filter((c) => !isAccountChip(c)) : [];
  const model = a ? (a.model || AGENT_LABELS[a.agent] || a.agent) + (a.cost_usd ? ` · $${a.cost_usd.toFixed(2)}` : "") : "";
  if (!bar && !chips.length && !model) return null;

  const extra = document.createElement("span");
  extra.className = "detail agent-extra";
  if (model) {
    const m = document.createElement("span");
    m.className = "model";
    m.textContent = model;
    extra.appendChild(m);
  }

  if (bar) {
    const wrap = document.createElement("span");
    wrap.className = "progress";
    const pct = Math.round(Math.min(Math.max(p.value || 0, 0), 1) * 100);
    const track = document.createElement("span");
    track.className = "progress-track";
    const fill = document.createElement("span");
    fill.className = "progress-fill";
    fill.style.width = `${pct}%`;
    track.appendChild(fill);
    wrap.appendChild(track);
    if (p.label) {
      const label = document.createElement("span");
      label.className = "progress-label";
      label.textContent = p.label;
      wrap.appendChild(label);
    }
    wrap.title = p.label ? `${p.label} — ${pct}%` : `${pct}%`;
    extra.appendChild(wrap);
  }

  for (const c of chips) extra.appendChild(chipEl(c));
  return extra;
}

function sessionRow(s, attached) {
  const li = document.createElement("li");
  li.dataset.name = s.key;
  li.classList.toggle("active", attached.has(s.key));
  li.classList.toggle("remote-session", !!s.remote);

  // Drag-reorder is a local-list affordance; remote rows keep group order.
  const phase = agentPhase(s);
  li.classList.toggle("needs", phase === "needs");
  const grip = agentTile(s.agent?.agent);
  if (!s.remote) {
    grip.classList.add("grip");
    grip.title += " · drag to reorder";
    initDrag(li, grip);
  }

  const copy = document.createElement("span");
  copy.className = "session-copy";
  const name = document.createElement("span");
  name.className = "name";
  name.textContent = s.name;
  if (s.activity > seenActivity.get(s.key)) {
    const dot = document.createElement("span");
    dot.className = "dot";
    dot.title = "new output";
    name.append(" ", dot);
  }

  const detail = document.createElement("span");
  detail.className = "detail";
  const meta = document.createElement("span");
  meta.className = "meta";
  // Where a session is working says more at a glance than how its windows are
  // arranged, so the layout counts fall back to the row tooltip, which
  // already carries them.
  meta.textContent = s.path
    ? `${shortPath(s.path)}${s.command ? ` · ${s.command}` : ""}`
    : `${s.windows} window${s.windows === 1 ? "" : "s"}${s.attached > 0 ? " · attached" : ""}`;
  detail.appendChild(meta);

  if (s.quiet) {
    const q = document.createElement("span");
    q.className = "meta quiet-on";
    q.innerHTML = icon("bell");
    q.title = "notifies when it goes quiet";
    detail.appendChild(q);
  }

  if (s.from) {
    const from = document.createElement("span");
    from.className = "meta from";
    from.textContent = `from ${s.from}`;
    from.title = `started by the agent in ${s.from}`;
    detail.appendChild(from);
  }

  if (s.branch) {
    const branch = document.createElement("span");
    branch.className = s.dirty ? "branch dirty" : "branch";
    branch.innerHTML = icon("branch");
    branch.append(s.dirty ? `${s.branch}*` : s.branch);
    branch.title = s.dirty ? `${s.branch} · uncommitted changes` : s.branch;
    detail.appendChild(branch);
  }

  // What a session is serving is the one thing about it you cannot read off
  // the terminal without going looking, so it earns a chip of its own.
  if (s.ports?.length) {
    const ports = document.createElement("span");
    ports.className = "ports";
    ports.innerHTML = icon("plug");
    const shown = s.ports.slice(0, 2).map((p) => `:${p}`).join(" ");
    ports.append(s.ports.length > 2 ? `${shown} +${s.ports.length - 2}` : shown);
    ports.title = `listening on ${s.ports.map((p) => `:${p}`).join(", ")}`;
    detail.appendChild(ports);
  }

  const head = document.createElement("span");
  head.className = "row-head";
  head.appendChild(name);
  if (phase) head.appendChild(phaseEl(phase, s.agent));
  copy.append(head, detail);

  // Progress and chips get a line of their own. The line above is already a
  // competition between path, branch, ports and badge, and a progress bar
  // squeezed in beside them is not one anybody can read.
  const extra = agentExtra(s.agent);
  if (extra) copy.appendChild(extra);
  if (s.permissions?.length) {
    const ask = document.createElement("span");
    ask.className = "detail perm-ask";
    const p = s.permissions[0];
    ask.textContent = `${p.tool}${p.summary ? `: ${p.summary}` : ""}` +
      (s.permissions.length > 1 ? ` (+${s.permissions.length - 1})` : "");
    ask.title = p.questions ? "Claude asks you something" : "permission needed";
    const quick = document.createElement("span");
    quick.className = "perm-quick";
    if (p.questions) {
      ask.textContent = p.summary + (s.permissions.length > 1 ? ` (+${s.permissions.length - 1})` : "");
      // A single plain question is answered right here; anything more
      // opens the session, where the card has room for it.
      const q = p.questions[0];
      if (p.questions.length === 1 && !q.multiSelect && q.options.length <= 4) {
        for (const o of q.options) {
          const b = document.createElement("button");
          b.textContent = o.label;
          b.title = o.description || o.label;
          b.addEventListener("click", (e) => {
            e.stopPropagation();
            answerPermission(s.remote, p.id, "answer", quick, { answers: { [q.question]: o.label } });
          });
          quick.appendChild(b);
        }
      }
    } else {
      for (const d of ["allow", "deny"]) {
        const b = document.createElement("button");
        b.className = `perm-${d}`;
        b.textContent = PERM_LABELS[d];
        b.addEventListener("click", (e) => {
          e.stopPropagation();
          answerPermission(s.remote, p.id, d, quick);
        });
        quick.appendChild(b);
      }
    }
    copy.append(ask);
    if (quick.childElementCount) copy.append(quick);
  }

  if (s.blocked?.length && !s.permissions?.length) {
    const b = s.blocked[s.blocked.length - 1];
    const ask = document.createElement("span");
    ask.className = "detail perm-ask block-ask";
    ask.textContent = `Blocked${b.reason ? ` ${b.reason}` : ""}: ${b.tool}${b.summary ? ` ${b.summary}` : ""}` +
      (s.blocked.length > 1 ? ` (+${s.blocked.length - 1})` : "");
    ask.title = "auto mode blocked this action";
    const quick = document.createElement("span");
    quick.className = "perm-quick";
    for (const [d, label] of [["allow", "Allow once"], ["dismiss", "Leave it"]]) {
      const btn = document.createElement("button");
      btn.className = d === "allow" ? "perm-allow" : "";
      btn.textContent = label;
      btn.addEventListener("click", (e) => {
        e.stopPropagation();
        answerBlocked(s.remote, b.id, d, quick);
      });
      quick.appendChild(btn);
    }
    copy.append(ask, quick);
  }

  if (s.subagents?.length) copy.appendChild(subagentList(s));
  if (s.tasks?.length) copy.appendChild(taskList(s));
  if (!s.remote) {
    const started = delegatedList(s);
    if (started) copy.appendChild(started);
  }

  li.title = `${s.key} · ${s.windows} window${s.windows === 1 ? "" : "s"} · created ${new Date(s.created).toLocaleString()}${s.attached > 0 ? " · attached" : ""}` +
    (s.path ? `\n${s.path}` : "");

  const rename = document.createElement("button");
  rename.className = "rename";
  rename.innerHTML = icon("pencil");
  rename.title = `rename ${s.key}`;
  rename.addEventListener("click", (e) => {
    e.stopPropagation();
    renameSession(s.key);
  });

  const kill = document.createElement("button");
  kill.className = "kill";
  kill.innerHTML = icon("x");
  kill.title = `kill ${s.key}`;
  kill.addEventListener("click", async (e) => {
    e.stopPropagation();
    if (!confirm(`Kill session "${s.key}"?`)) return;
    await killSession(s.key);
  });

  const actions = document.createElement("span");
  actions.className = "session-actions";
  actions.append(rename, kill);
  li.append(grip, copy, actions);
  li.addEventListener("click", () => paneFor().attach(s.key));
  return li;
}

function remoteHeader(r) {
  const li = document.createElement("li");
  li.className = "remote-head";
  const chev = document.createElement("span");
  chev.className = "chev";
  chev.innerHTML = icon(collapsedRemotes[r.name] ? "chevronRight" : "chevronDown");
  const name = document.createElement("span");
  name.className = "name";
  name.textContent = r.name;
  const state = document.createElement("span");
  state.className = `rstate rstate-${r.state}`;
  if (r.state === "ok") {
    state.textContent = `${(remoteSessions.get(r.name) || []).length}`;
    li.title = `${r.name} · ${r.mode === "ssh" ? `ssh ${r.host}` : r.url}`;
  } else if (r.state === "off") {
    li.classList.add("off");
    state.textContent = "off";
    li.title = `${r.name} · off — :remote on ${r.name} to reconnect`;
  } else {
    state.innerHTML = icon("linkOff");
    li.title = `${r.name} · ${r.error || "unreachable"}`;
  }
  li.append(chev, name, state);
  li.addEventListener("click", () => {
    collapsedRemotes[r.name] = !collapsedRemotes[r.name];
    localStorage.setItem("muxdeck-remotes-collapsed", JSON.stringify(collapsedRemotes));
    refreshSessions();
  });
  return li;
}

function initDrag(li, grip) {
  grip.addEventListener("pointerdown", (e) => {
    e.preventDefault();
    e.stopPropagation();
    grip.setPointerCapture(e.pointerId);
    li.classList.add("dragging");
    const ul = $("#sessions");
    const move = (ev) => {
      const over = document.elementFromPoint(ev.clientX, ev.clientY)
        ?.closest("#sessions li:not(.section-head):not(.remote-head):not(.remote-session)");
      if (over && over !== li) {
        const r = over.getBoundingClientRect();
        ul.insertBefore(li, ev.clientY < r.top + r.height / 2 ? over : over.nextSibling);
      }
    };
    grip.addEventListener("pointermove", move);
    grip.addEventListener("pointerup", () => {
      grip.removeEventListener("pointermove", move);
      li.classList.remove("dragging");
      order = [...ul.querySelectorAll("li:not(.section-head):not(.remote-head):not(.remote-session)")].map((n) => n.dataset.name);
      saveOrder();
    }, { once: true });
  });
}

async function renameSession(oldKey, name = null) {
  const prefix = oldKey.includes(":") ? oldKey.slice(0, oldKey.indexOf(":") + 1) : "";
  name = name ?? prompt("rename session", oldKey.slice(prefix.length));
  if (!name || prefix + name === oldKey) return;
  try {
    await api(keyApi(oldKey, "/rename"), {
      method: "POST",
      body: JSON.stringify({ name }),
    });
  } catch (err) {
    alert(err.message);
    return;
  }
  const newKey = prefix + name;
  for (const p of panes) {
    if (p.session === oldKey) {
      p.session = newKey;
      p.el.querySelector(".pane-title").textContent = newKey;
    }
  }
  const i = order.indexOf(oldKey);
  if (i >= 0) { order[i] = newKey; saveOrder(); }
  if (seenActivity.has(oldKey)) {
    seenActivity.set(newKey, seenActivity.get(oldKey));
    seenActivity.delete(oldKey);
  }
  updateHash();
  refreshSessions();
}

$("#refresh").addEventListener("click", refreshSessions);

// --- permission cards ---

const PERM_LABELS = { allow: "Allow", always: "Always allow", deny: "Deny", terminal: "Answer in terminal" };

function permCard(p) {
  if (p.questions) return questionCard(p);
  if (p.blocked) return blockCard(p, null);
  const card = document.createElement("div");
  card.className = "perm-card";
  const head = document.createElement("div");
  head.className = "perm-head";
  head.textContent = `${p.tool}${p.summary ? ` · ${p.summary}` : ""}`;
  card.appendChild(head);
  if (p.detail && p.detail !== p.summary) {
    const pre = document.createElement("pre");
    pre.className = "perm-detail";
    pre.textContent = p.detail;
    card.appendChild(pre);
  }
  const row = document.createElement("div");
  row.className = "perm-row";
  const choices = p.can_always ? ["allow", "always", "deny", "terminal"] : ["allow", "deny", "terminal"];
  for (const d of choices) {
    const b = document.createElement("button");
    b.className = `perm-${d}`;
    b.textContent = PERM_LABELS[d];
    b.addEventListener("click", (e) => {
      e.stopPropagation();
      answerPermission(null, p.id, d, row);
    });
    row.appendChild(b);
  }
  card.appendChild(row);
  return card;
}

// Claude's AskUserQuestion: up to four questions, each with its options,
// one or several to pick, or an answer of your own. Sent together once every
// question has one; "Answer in terminal" hands it back to the terminal UI.
function questionCard(p) {
  const card = document.createElement("div");
  card.className = "perm-card ask-card";
  const picks = p.questions.map(() => new Set());
  const own = p.questions.map(() => "");
  const row = document.createElement("div");
  row.className = "perm-row";
  const send = document.createElement("button");
  send.className = "perm-allow";
  send.textContent = "Send answer";
  const answerOf = (i) => {
    const q = p.questions[i];
    const chosen = q.options.filter((o) => picks[i].has(o.label)).map((o) => o.label);
    if (own[i].trim()) chosen.push(own[i].trim());
    return chosen.join(", ");
  };
  const ready = () => p.questions.every((_, i) => answerOf(i) !== "");
  const update = () => {
    send.disabled = !ready();
    for (const b of card.querySelectorAll(".ask-opt")) b.classList.toggle("picked", picks[+b.dataset.q].has(b.dataset.label));
  };
  p.questions.forEach((q, i) => {
    const sec = document.createElement("div");
    sec.className = "ask-q";
    const head = document.createElement("div");
    head.className = "perm-head";
    head.textContent = q.question;
    if (q.header) head.title = q.header;
    sec.appendChild(head);
    if (q.multiSelect) {
      const hint = document.createElement("div");
      hint.className = "ask-hint";
      hint.textContent = "pick one or more";
      sec.appendChild(hint);
    }
    const opts = document.createElement("div");
    opts.className = "ask-opts";
    for (const o of q.options) {
      const b = document.createElement("button");
      b.className = "ask-opt";
      b.dataset.q = i;
      b.dataset.label = o.label;
      const l = document.createElement("span");
      l.className = "ask-label";
      l.textContent = o.label;
      b.appendChild(l);
      if (o.description) {
        const d = document.createElement("span");
        d.className = "ask-desc";
        d.textContent = o.description;
        b.appendChild(d);
      }
      b.addEventListener("click", (e) => {
        e.stopPropagation();
        if (q.multiSelect) {
          picks[i].has(o.label) ? picks[i].delete(o.label) : picks[i].add(o.label);
        } else {
          const was = picks[i].has(o.label);
          picks[i].clear();
          if (!was) picks[i].add(o.label);
          own[i] = "";
          input.value = "";
          // One question, one pick: nothing left to decide.
          if (!was && p.questions.length === 1) { submit(); return; }
        }
        update();
      });
      opts.appendChild(b);
    }
    sec.appendChild(opts);
    const input = document.createElement("input");
    input.className = "ask-own";
    input.placeholder = "or type your own answer";
    input.spellcheck = true;
    input.addEventListener("input", () => {
      own[i] = input.value;
      if (!q.multiSelect && input.value.trim()) picks[i].clear();
      update();
    });
    input.addEventListener("keydown", (e) => {
      e.stopPropagation();
      if (e.key === "Enter" && ready()) submit();
    });
    sec.appendChild(input);
    card.appendChild(sec);
  });
  const submit = () => {
    const answers = {};
    p.questions.forEach((q, i) => (answers[q.question] = answerOf(i)));
    answerPermission(null, p.id, "answer", row, { answers });
  };
  send.addEventListener("click", (e) => { e.stopPropagation(); if (ready()) submit(); });
  const term = document.createElement("button");
  term.className = "perm-terminal";
  term.textContent = PERM_LABELS.terminal;
  term.addEventListener("click", (e) => { e.stopPropagation(); answerPermission(null, p.id, "terminal", row); });
  row.append(send, term);
  card.appendChild(row);
  update();
  return card;
}

// Sessions the last tmux server had when it went away (a reboot): offered
// back with their directory and, where an agent ran, its conversation.
let lostSessions = [];

function restoreSection(ul) {
  const head = sectionHeader("Restore after restart", lostSessions.length);
  head.classList.add("restore-head");
  head.title = "these sessions were open when tmux stopped (a reboot?)";
  const all = document.createElement("span");
  all.className = "restore-all";
  const yes = document.createElement("button");
  yes.textContent = "restore all";
  yes.addEventListener("click", (e) => { e.stopPropagation(); restoreSessions([], all); });
  const no = document.createElement("button");
  no.textContent = "dismiss";
  no.title = "forget these sessions";
  no.addEventListener("click", (e) => { e.stopPropagation(); dismissLost([]); });
  all.append(yes, no);
  head.appendChild(all);
  ul.appendChild(head);
  for (const x of lostSessions) {
    const li = document.createElement("li");
    li.className = "restore-row";
    const copy = document.createElement("span");
    copy.className = "session-copy";
    const name = document.createElement("span");
    name.className = "name";
    name.textContent = x.name;
    const meta = document.createElement("span");
    meta.className = "detail";
    const where = document.createElement("span");
    where.className = "meta";
    const what = x.resume ? ({ codex: "codex resume", gemini: "gemini --resume" }[x.agent] || "claude --resume") : "shell";
    where.textContent = `${x.dir ? shortPath(x.dir) : "~"} · ${what}`;
    meta.appendChild(where);
    copy.append(name, meta);
    li.title = `${x.dir || "home"}${x.resume ? `
${x.agent} ${x.resume}` : ""}
last seen ${new Date(x.seen).toLocaleString()}`;
    const btns = document.createElement("span");
    btns.className = "restore-btns";
    const go = document.createElement("button");
    go.textContent = "restore";
    go.addEventListener("click", (e) => { e.stopPropagation(); restoreSessions([x.name], btns); });
    const drop = document.createElement("button");
    drop.innerHTML = icon("x");
    drop.title = "forget this one";
    drop.addEventListener("click", (e) => { e.stopPropagation(); dismissLost([x.name]); });
    btns.append(go, drop);
    li.append(copy, btns);
    ul.appendChild(li);
  }
}

async function restoreSessions(names, box) {
  box.querySelectorAll("button").forEach((b) => (b.disabled = true));
  try {
    const r = await api("/api/restore", { method: "POST", body: JSON.stringify({ names }) });
    const failed = (r.results || []).filter((x) => x.error);
    if (failed.length) alert(failed.map((x) => `${x.name}: ${x.error}`).join("\n"));
    const first = (r.results || []).find((x) => !x.error);
    await refreshSessions();
    if (first && names.length === 1) paneFor().attach(first.name);
  } catch (err) {
    alert(err.message);
    box.querySelectorAll("button").forEach((b) => (b.disabled = false));
  }
}

async function dismissLost(names) {
  try {
    await api("/api/restore", { method: "DELETE", body: JSON.stringify({ names }) });
  } catch (err) { alert(err.message); }
  refreshSessions();
}

// Permission notifications on this device whose request is no longer open
// (answered elsewhere, or the hook gave up) are closed once the app sees it.
// Only notifications shown before the list was asked for: a newer one may
// be for a request the list does not have yet.
function prunePermNotifications(sessions, askedAt) {
  if (!("serviceWorker" in navigator)) return;
  const open = new Set(sessions.flatMap((s) => (s.permissions || []).map((p) => p.id)));
  navigator.serviceWorker.getRegistration()
    .then((reg) => reg?.getNotifications())
    .then((list) => { for (const n of list || []) if (n.data?.perm && n.data.shown < askedAt && !open.has(n.data.perm)) n.close(); })
    .catch(() => {});
}

// answerPermission sends a decision for a permission request; remote is
// the remote's name for a session on another daemon. The buttons in box
// stay disabled once it went through (or was answered elsewhere, or
// expired), and come back when it could not be sent, so a failed tap can
// be repeated instead of leaving a dead card until the hook gives up.
async function answerPermission(remote, id, decision, box, extra = {}) {
  const buttons = [...box.querySelectorAll("button")];
  buttons.forEach((x) => (x.disabled = true));
  box.querySelector(".perm-err")?.remove();
  const path = remote
    ? `/api/remotes/${encodeURIComponent(remote)}/proxy/agent/permission/${encodeURIComponent(id)}`
    : `/api/agent/permission/${encodeURIComponent(id)}`;
  try {
    await apiRetry(path, { method: "POST", body: JSON.stringify({ decision, ...extra }) });
  } catch (err) {
    if (err.status !== 404 && err.status !== 409) {
      buttons.forEach((x) => (x.disabled = false));
      if (err.message !== "unauthorized") {
        const note = document.createElement("span");
        note.className = "perm-err";
        note.textContent = "not sent, try again";
        box.appendChild(note);
      }
      return;
    }
  }
  refreshSessions();
}

// Long poll on the daemon's change counter: a permission request or an agent
// state change refreshes the list at once instead of on the next 10 s tick.
// A long poll started before the phone slept can hang on a dead tailnet
// connection, so it has a deadline and a resume cuts it short.
let changesAbort = null;
async function watchChanges() {
  let v = null;
  for (;;) {
    changesAbort = new AbortController();
    try {
      const signal = AbortSignal.any
        ? AbortSignal.any([changesAbort.signal, AbortSignal.timeout(40000)])
        : changesAbort.signal;
      const r = await api(`/api/changes?since=${v ?? 0}`, { signal });
      if (v !== null && r.v !== v) refreshSessions();
      v = r.v;
    } catch (err) {
      if (err.name !== "AbortError" && err.name !== "TimeoutError") {
        await new Promise((res) => setTimeout(res, 5000));
      }
    }
  }
}

// --- Web Push: alerts that reach a device whose muxdeck is closed. Once a
// device has a subscription the daemon pushes the alerts and this page stops
// raising its own, so a notification never arrives twice.

function b64urlBytes(s) {
  const bin = atob(s.replace(/-/g, "+").replace(/_/g, "/"));
  return Uint8Array.from(bin, (c) => c.charCodeAt(0));
}

function pushOff() { try { return localStorage.getItem("muxdeck-push-off") === "1"; } catch { return false; } }
function setPushOff(off) { try { off ? localStorage.setItem("muxdeck-push-off", "1") : localStorage.removeItem("muxdeck-push-off"); } catch {} }

async function pushSubscription() {
  const reg = await navigator.serviceWorker?.ready.catch(() => null);
  return reg?.pushManager ? reg.pushManager.getSubscription().catch(() => null) : null;
}

async function disablePush() {
  const sub = await pushSubscription();
  if (sub) {
    await api("/api/push", { method: "DELETE", body: JSON.stringify({ endpoint: sub.endpoint }) }).catch(() => {});
    await sub.unsubscribe().catch(() => {});
  }
  setPushOff(true);
  pushActive = false;
}

async function enablePush() {
  try {
    const reg = await navigator.serviceWorker?.ready;
    if (!reg?.pushManager) return false;
    const { key } = await api("/api/push");
    const appKey = b64urlBytes(key);
    let sub = await reg.pushManager.getSubscription();
    const same = sub && sub.options?.applicationServerKey &&
      new Uint8Array(sub.options.applicationServerKey).every((b, i) => b === appKey[i]);
    if (sub && !same) {
      await sub.unsubscribe();
      sub = null;
    }
    if (!sub) sub = await reg.pushManager.subscribe({ userVisibleOnly: true, applicationServerKey: appKey });
    await api("/api/push", { method: "POST", body: JSON.stringify(sub.toJSON()) });
    pushActive = true;
    return true;
  } catch (err) {
    wsdbg?.log?.(`push: ${err.message}`);
    return false;
  }
}

// --- settings panel ---

function el2(tag, cls, text) {
  const e = document.createElement(tag);
  if (cls) e.className = cls;
  if (text != null) e.textContent = text;
  return e;
}

function setSection(title, ...rows) {
  const sec = el2("section", "set-sec");
  sec.appendChild(el2("h3", null, title));
  for (const r of rows) if (r) sec.appendChild(r);
  return sec;
}

function setRow(label, control, hint) {
  const row = el2("label", "set-row");
  const text = el2("span", "set-label", label);
  if (hint) text.appendChild(el2("small", null, hint));
  row.append(text, control);
  return row;
}

function setToggle(checked, onChange) {
  const c = document.createElement("input");
  c.type = "checkbox";
  c.checked = !!checked;
  c.addEventListener("change", () => onChange(c.checked));
  return c;
}

function setInput(type, value, onChange, attrs = {}) {
  const i = document.createElement("input");
  i.type = type;
  i.value = value ?? "";
  Object.assign(i, attrs);
  i.spellcheck = false;
  i.autocapitalize = "off";
  i.addEventListener("change", () => onChange(i.value));
  return i;
}

function setButton(text, onClick, cls = "") {
  const b = el2("button", `set-btn ${cls}`, text);
  b.type = "button";
  b.addEventListener("click", onClick);
  return b;
}

let serverSettings = null;

// Optional link to a companion dashboard, from the daemon settings.
function applyDashboard() {
  const a = $("#dashboard-link");
  const d = serverSettings?.dashboard;
  a.hidden = !d?.url;
  if (!d?.url) return;
  a.href = d.url;
  a.title = d.label || "Dashboard";
  a.setAttribute("aria-label", a.title);
}

let settingsLoading = false;
async function loadServerSettingsOnce() {
  if (serverSettings || settingsLoading) return;
  settingsLoading = true;
  try { serverSettings = await api("/api/settings"); applyDashboard(); } catch {}
  settingsLoading = false;
}

async function saveServerSettings(mutate, status) {
  const next = structuredClone(serverSettings);
  mutate(next);
  try {
    serverSettings = await api("/api/settings", { method: "PUT", body: JSON.stringify(next) });
    applyDashboard();
    status.textContent = "Saved";
    status.className = "set-status ok";
  } catch (err) {
    status.textContent = err.message;
    status.className = "set-status bad";
  }
  setTimeout(() => { if (status.textContent === "Saved") status.textContent = ""; }, 1500);
}

async function openSettings() {
  const box = $("#settings");
  const body = box.querySelector(".set-body");
  body.innerHTML = "";
  body.appendChild(el2("p", "set-status", "Loading…"));
  box.hidden = false;
  let pushInfo = null, ver = null;
  try { serverSettings = await api("/api/settings"); } catch (err) { body.firstChild.textContent = err.message; return; }
  try { pushInfo = await api("/api/push"); } catch {}
  try { ver = await api("/api/version"); } catch {}
  renderSettings(body, pushInfo, ver);
}

function closeSettings() { $("#settings").hidden = true; }

async function renderSettings(body, pushInfo, ver) {
  body.innerHTML = "";
  const status = el2("p", "set-status");
  const st = serverSettings;
  const save = (fn) => saveServerSettings(fn, status);

  // Notifications
  const sub = await pushSubscription();
  const canPush = !tauriNotif && "PushManager" in window && pushInfo;
  const here = el2("span", "set-value", !canPush ? "Not supported here" : sub && pushActive ? "On" : "Off");
  const hereRow = el2("div", "set-row");
  hereRow.append(el2("span", "set-label", "Push on this device"), here);
  if (canPush) {
    const btns = el2("span", "set-btns");
    if (sub && pushActive) {
      btns.append(
        setButton("Test", () => api("/api/push/test", { method: "POST" }).catch(() => {})),
        setButton("Turn off", async () => { await disablePush(); openSettings(); }),
      );
    } else {
      btns.append(setButton("Turn on", async () => {
        if (Notification.permission === "default") await Notification.requestPermission();
        if (Notification.permission !== "granted") { alert("Notifications are blocked for this site — enable them in browser settings."); return; }
        setPushOff(false);
        if (await enablePush()) api("/api/push/test", { method: "POST" }).catch(() => {});
        $("#notify").classList.toggle("armed", true);
        openSettings();
      }, "primary"));
    }
    hereRow.appendChild(btns);
  }
  const quietFrom = setInput("time", st.notify.quiet_from, (v) => save((s) => { s.notify.quiet_from = v; }));
  const quietTo = setInput("time", st.notify.quiet_to, (v) => save((s) => { s.notify.quiet_to = v; }));
  const quietTimes = el2("span", "set-inline");
  quietTimes.append(quietFrom, el2("span", null, "–"), quietTo);
  const devices = el2("ul", "set-devices");
  for (const d of pushInfo?.devices || []) {
    const li = el2("li");
    const mine = sub && sub.endpoint === d.endpoint;
    li.append(el2("span", null, `${d.label || "Unknown device"}${mine ? " (this device)" : ""}`),
      el2("small", null, d.added ? new Date(d.added).toLocaleDateString() : ""));
    li.appendChild(setButton("Remove", async () => {
      if (mine) await disablePush();
      else await api("/api/push", { method: "DELETE", body: JSON.stringify({ endpoint: d.endpoint }) }).catch(() => {});
      openSettings();
    }));
    devices.appendChild(li);
  }
  if (!devices.childElementCount) devices.appendChild(el2("li", "set-empty", "No devices receive push yet"));

  body.appendChild(setSection("Notifications",
    hereRow,
    setRow("Permission requests", setToggle(st.notify.permission, (v) => save((s) => { s.notify.permission = v; }))),
    setRow("Agent done or waiting", setToggle(st.notify.done, (v) => save((s) => { s.notify.done = v; }))),
    setRow("Show the reply", setToggle(st.notify.preview, (v) => save((s) => { s.notify.preview = v; })), "The start of the agent's last answer in a done notification, also on the lock screen"),
    setRow("Quiet hours", setToggle(st.notify.quiet, (v) => save((s) => { s.notify.quiet = v; })), "Only permission requests come through"),
    setRow("Quiet from – to", quietTimes),
    el2("h4", null, "Devices with push"),
    devices,
  ));

  // Permission requests
  body.appendChild(setSection("Permission requests",
    setRow("Answer in muxdeck", setToggle(st.perms.enabled, (v) => save((s) => { s.perms.enabled = v; })), "Off: Claude asks in the terminal right away"),
    setRow("Wait before terminal", setInput("number", st.perms.wait, (v) => save((s) => { s.perms.wait = +v; }), { min: 10, max: 140, step: 10, className: "set-num" }), "Seconds, 10–140"),
  ));

  // Display (this device)
  const fontVal = el2("span", "set-value", `${fontSize}px`);
  const fontCtl = el2("span", "set-btns");
  fontCtl.append(
    setButton("A−", () => { setFontSize(fontSize - 1); fontVal.textContent = `${fontSize}px`; }),
    fontVal,
    setButton("A+", () => { setFontSize(fontSize + 1); fontVal.textContent = `${fontSize}px`; }),
  );
  const touch = matchMedia("(pointer: coarse)").matches;
  const keys = document.createElement("textarea");
  keys.className = "set-keys";
  keys.rows = 3;
  keys.spellcheck = false;
  keys.placeholder = "^C\nyes=y\\n\nclear=clear\\n";
  keys.value = prefs.keys;
  keys.addEventListener("change", () => { prefs.keys = keys.value; savePrefs(); applyKeybarPref(); });
  body.appendChild(setSection("Display · this device",
    setRow("Text size", fontCtl),
    touch && setRow("Key bar", setToggle(prefs.keybar, (v) => { prefs.keybar = v; savePrefs(); applyKeybarPref(); })),
    touch && setRow("Keyboard corrections", setToggle(prefs.imefix, (v) => { prefs.imefix = v; savePrefs(); }), "Suggestions and autocorrect replace text instead of repeating it. Reload to apply"),
    touch && setRow("Extra keys", keys, "One per line: label=keys. ^C is ctrl-C, \\n is enter, \\e is escape"),
    setRow("Limits row in sidebar", setToggle(prefs.limits, (v) => { prefs.limits = v; savePrefs(); refreshSessions(); })),
    setRow("Agent chips in sidebar", setToggle(prefs.chips, (v) => { prefs.chips = v; savePrefs(); refreshSessions(); })),
  ));

  // New sessions
  body.appendChild(setSection("New sessions",
    setRow("Start in", setInput("text", st.new_session.dir, (v) => save((s) => { s.new_session.dir = v; }), { placeholder: "~" })),
    setRow("Run on start", setInput("text", st.new_session.command, (v) => save((s) => { s.new_session.command = v; }), { placeholder: "e.g. claude" })),
  ));
  body.appendChild(setSection("Dashboard link",
    setRow("URL", setInput("url", st.dashboard?.url, (v) => save((s) => { s.dashboard = { ...s.dashboard, url: v }; }), { placeholder: "https://…" }), "Optional: an icon in the sidebar header opens this page. Leave empty to hide the icon"),
    setRow("Name", setInput("text", st.dashboard?.label, (v) => save((s) => { s.dashboard = { ...s.dashboard, label: v }; }), { placeholder: "Dashboard" })),
  ));

  // System
  const verRow = el2("div", "set-row");
  verRow.append(el2("span", "set-label", "Version"), el2("span", "set-value", ver?.version || "?"));
  const upd = el2("div", "set-row");
  upd.append(el2("span", "set-label", "Latest release"),
    el2("span", `set-value${ver?.update ? " warn" : ""}`, ver?.latest ? (ver.update ? `${ver.latest} — run muxdeck-rebuild --update` : `${ver.latest} (up to date)`) : "unknown"));
  body.appendChild(setSection("System",
    verRow, upd,
    el2("div", "set-row end", null),
  ));
  body.lastChild.lastChild.appendChild(setButton("Log out on this device", async () => {
    if (!confirm("Log out on this device?")) return;
    await disablePush();
    await fetch("/api/logout", { method: "POST" }).catch(() => {});
    location.reload();
  }, "danger"));

  body.appendChild(status);
}

$("#settings-btn").addEventListener("click", openSettings);
$("#settings .set-close").addEventListener("click", closeSettings);
$("#settings").addEventListener("click", (e) => { if (e.target.id === "settings") closeSettings(); });
document.addEventListener("keydown", (e) => { if (e.key === "Escape" && !$("#settings").hidden) closeSettings(); });

// Notification opt-in. Shown where a backend exists: the page Notification
// API in browsers, the Tauri plugin in the desktop app. Neither exists on
// iOS Safari — that needs Web Push, which muxdeck doesn't do yet.
if (tauriNotif || typeof Notification !== "undefined") {
  const btn = $("#notify");
  btn.hidden = false;
  const paint = async () => btn.classList.toggle("armed", await notifGranted());
  paint();
  btn.addEventListener("click", async () => {
    if (tauriNotif) {
      if (!(await tauriNotif.isPermissionGranted())) await tauriNotif.requestPermission();
    } else if (Notification.permission === "default") {
      await Notification.requestPermission();
    } else if (Notification.permission === "denied") {
      alert("Notifications are blocked for this site — enable them in browser settings.");
    }
    if (!tauriNotif && Notification.permission === "granted") {
      setPushOff(false);
      if (await enablePush()) api("/api/push/test", { method: "POST" }).catch(() => {});
    }
    paint();
  });
  if (!tauriNotif && Notification.permission === "granted" && !pushOff()) enablePush();
}

$("#new-session").addEventListener("click", () => openLauncher());

// --- command palette ---
// One overlay, several modes. "switch" is the spotlight default: fuzzy
// session matching, ":" for commands, and a create fallback when nothing
// matches. Sub-modes chain like shell prompts: :rename → pick → type new
// name, :kill → pick → y/N.

let pal = { mode: "switch", verb: null, arg: null, index: 0 };

const VALID_NAME = /^[A-Za-z0-9_-]+$/;

const PAL_COMMANDS = [
  { name: ":new",     hint: "create session",         run: () => setPalMode("new") },
  { name: ":rename",  hint: "rename session",         run: () => setPalMode("pick", "rename") },
  { name: ":kill",    hint: "kill session",           run: () => setPalMode("pick", "kill") },
  { name: ":split",   hint: "toggle split view",      run: () => { closePalette(); $("#split").click(); } },
  { name: ":sidebar", hint: "collapse/expand sidebar", run: () => { closePalette(); toggleSidebar(); } },
  { name: ":remote",  hint: "add or remove remotes",   run: () => setPalMode("remote") },
  { name: ":cloud",   hint: "muxdeck cloud account",    run: () => setPalMode("cloud") },
  { name: ":awake",   hint: "keep a plugged-in Mac awake while viewing", run: () => setPalMode("awake") },
  { name: ":diff",    hint: "git diff of session cwd", run: () => { closePalette(); openDiffView(); } },
  { name: ":mush",    hint: "run an agent task here",  run: () => setPalMode("mush") },
  { name: ":runs",    hint: "open a mush run",         run: () => setPalMode("runs") },
  { name: ":md",      hint: "preview a markdown file", run: () => openMdPicker() },
  { name: ":mouse",   hint: "toggle tmux mouse mode", run: () => { closePalette(); paneFor()?.toggleMouse(); } },
  { name: ":font +",  hint: "bigger text",  keep: true, run: () => setFontSize(fontSize + 1) },
  { name: ":font -",  hint: "smaller text", keep: true, run: () => setFontSize(fontSize - 1) },
  { name: ":refresh", hint: "refresh sessions",       run: () => { closePalette(); refreshSessions(); } },
];

function fuzzy(q) {
  q = q.toLowerCase();
  return (name) => {
    let i = 0;
    for (const c of name.toLowerCase()) if (c === q[i]) i++;
    return i >= q.length;
  };
}

function creatableKey(q) {
  const i = q.indexOf(":");
  if (i < 0) return VALID_NAME.test(q) && !lastSessions.some((s) => s.name === q);
  const host = q.slice(0, i), name = q.slice(i + 1);
  return VALID_NAME.test(name) &&
    remotes.some((r) => r.name === host && r.state === "ok") &&
    !(remoteSessions.get(host) || []).some((s) => s.name === name);
}

function palItems() {
  const q = $("#pal-input").value.trim();
  if (pal.mode === "remote") return remoteSuggest().items.map((it) => ({ kind: "suggest", ...it }));
  if (pal.mode === "cloud") return cloudSuggest().items.map((it) => ({ kind: "suggest", ...it }));
  if (pal.mode === "awake") return awakeSuggest().items.map((it) => ({ kind: "suggest", ...it }));
  if (pal.mode === "mdpick")
    return (pal.files || []).filter(fuzzy(q)).map((f) => ({ kind: "file", name: f }));
  if (pal.mode === "pick")
    return allSessions().filter((s) => fuzzy(q)(s.key))
      .map((s) => ({ kind: "session", name: s.key, s }));
  if (pal.mode === "runs")
    return allRuns().filter((r) => fuzzy(q)(`${r.host} ${runLabel(r)} ${r.state} ${r.project}`))
      .map((r) => ({ kind: "run", name: runLabel(r), r }));
  if (pal.mode !== "switch") return [];
  if (q.startsWith(":"))
    return PAL_COMMANDS.filter((c) => c.name.startsWith(q)).map((c) => ({ kind: "command", ...c }));
  const items = allSessions().filter((s) => fuzzy(q)(s.key))
    .map((s) => ({ kind: "session", name: s.key, s }));
  if (q && creatableKey(q))
    items.push({ kind: "create", name: q });
  return items;
}

function renderPalette() {
  const ul = $(".pal-list");
  const items = palItems();
  pal.index = Math.max(0, Math.min(pal.index, items.length - 1));
  ul.innerHTML = "";
  items.forEach((it, i) => {
    const li = document.createElement("li");
    li.classList.toggle("selected", i === pal.index);
    const label = document.createElement("span");
    const right = document.createElement("span");
    right.className = "dim";
    if (it.kind === "session") {
      label.textContent = it.name;
      right.textContent = `${it.s.windows}w${it.s.attached > 0 ? " ●" : ""}`;
      if (it.s.agent) right.textContent += ` · ${{ working: "◐", waiting: "✋", idle: "○" }[it.s.agent.state] || ""}`;
    } else if (it.kind === "command" || it.kind === "suggest") {
      label.textContent = it.name;
      label.className = "cmd";
      right.textContent = it.hint;
    } else if (it.kind === "file") {
      label.textContent = it.name;
      right.textContent = "markdown";
    } else if (it.kind === "run") {
      const r = it.r;
      label.textContent = `${{ live: "◐", wait: "✋", ok: "✓", bad: "✕" }[runState(r)]} ${it.name}`;
      right.textContent = [r.host, (r.project || "").split("/").filter(Boolean).pop(), r.state.replace("_", " ")].filter(Boolean).join(" · ");
    } else {
      label.textContent = `+ create "${it.name}"`;
      label.className = "cmd";
      right.textContent = "new session";
    }
    li.append(label, right);
    li.addEventListener("click", () => palActivate(it));
    ul.appendChild(li);
  });
  updateCaret();
  updateGhost();
}

async function palActivate(it) {
  if (it.kind === "session") {
    if (pal.mode === "pick" && pal.verb === "rename") return setPalMode("input", "rename", it.name);
    if (pal.mode === "pick" && pal.verb === "kill") return setPalMode("confirm", "kill", it.name);
    closePalette();
    paneFor().attach(it.name);
  } else if (it.kind === "command") {
    it.run();
    if (it.keep) renderPalette();
  } else if (it.kind === "suggest") {
    palComplete(it);
  } else if (it.kind === "file") {
    const key = pal.arg;
    closePalette();
    viewerPane().openView({ type: "md", key, path: it.name });
  } else if (it.kind === "run") {
    closePalette();
    viewerPane().openMush(it.r.host, it.r);
  } else {
    await createSession(it.name);
  }
}

// The project launcher: the folders under the start dir, most recently
// used first; a tap starts a session named after the folder, in it, with
// the chosen agent running. Typed text filters the list, or names an empty
// session started the old way (settings' start dir and command).
const LAUNCH_AGENTS = [
  ["claude", "Claude"], ["claude-resume", "Claude resume"], ["codex", "Codex"],
  ["codex-resume", "Codex resume"], ["gemini", "Gemini"], ["shell", "Shell"],
];
let launchProjects = [];
let launchAgent = (() => { try { return localStorage.getItem("muxdeck-launch-agent") || "claude"; } catch { return "claude"; } })();

function sinceText(t) {
  const d = (Date.now() - new Date(t)) / 1000;
  if (!t || t.startsWith?.("0001") || d < 0) return "";
  if (d < 3600) return `${Math.max(1, Math.round(d / 60))} min ago`;
  if (d < 86400) return `${Math.round(d / 3600)} h ago`;
  if (d < 86400 * 60) return `${Math.round(d / 86400)} d ago`;
  return "";
}

function sessionNameFor(base) {
  const stem = base.replace(/[^A-Za-z0-9_-]+/g, "-").replace(/^-+|-+$/g, "").slice(0, 56) || "session";
  const taken = new Set(lastSessions.map((s) => s.name));
  if (!taken.has(stem)) return stem;
  for (let n = 2; ; n++) if (!taken.has(`${stem}-${n}`)) return `${stem}-${n}`;
}

async function openLauncher() {
  const box = $("#launcher");
  box.hidden = false;
  const input = box.querySelector(".launch-filter");
  input.value = "";
  renderLauncher();
  if (!matchMedia("(pointer: coarse)").matches) input.focus();
  try {
    launchProjects = (await api("/api/projects")).projects || [];
  } catch (err) { launchProjects = []; }
  renderLauncher();
}

function closeLauncher() {
  $("#launcher").hidden = true;
  paneFor()?.term?.focus();
}

function renderLauncher() {
  const box = $("#launcher");
  const agents = box.querySelector(".launch-agents");
  agents.innerHTML = "";
  for (const [id, label] of LAUNCH_AGENTS) {
    const b = el2("button", id === launchAgent ? "picked" : "", label);
    b.addEventListener("click", () => {
      launchAgent = id;
      try { localStorage.setItem("muxdeck-launch-agent", id); } catch {}
      renderLauncher();
    });
    agents.appendChild(b);
  }
  const q = box.querySelector(".launch-filter").value.trim();
  const ql = q.toLowerCase();
  const list = box.querySelector(".launch-list");
  list.innerHTML = "";
  const shown = launchProjects.filter((p) => !ql || p.name.toLowerCase().includes(ql));
  for (const p of shown) {
    const li = el2("li", "launch-row");
    const copy = el2("span", "launch-copy");
    copy.append(el2("span", "launch-name", p.name));
    const open = lastSessions.filter((s) => s.path === p.path);
    const meta = [sinceText(p.used), open.length ? `${open.length} open` : ""].filter(Boolean).join(" · ");
    if (meta) copy.append(el2("span", "launch-meta", meta));
    li.appendChild(copy);
    for (const s of open.slice(0, 2)) {
      const go = el2("button", "launch-open", s.name);
      go.title = `go to ${s.name}`;
      go.addEventListener("click", (e) => { e.stopPropagation(); closeLauncher(); paneFor().attach(s.name); });
      li.appendChild(go);
    }
    li.addEventListener("click", () => launchProject(p));
    list.appendChild(li);
  }
  if (!shown.length) list.appendChild(el2("li", "launch-empty", launchProjects.length ? "no project matches" : "no projects found in the start folder"));
  const plain = box.querySelector(".launch-plain");
  const name = q && creatableKey(q) ? q : "";
  plain.disabled = !name;
  plain.textContent = name ? `Empty session "${name}"` : "Type a name for an empty session";
}

async function launchProject(p) {
  const name = sessionNameFor(p.name);
  try {
    await api("/api/sessions", { method: "POST", body: JSON.stringify({ name, dir: p.path, agent: launchAgent }) });
  } catch (err) { alert(err.message); return; }
  closeLauncher();
  await refreshSessions();
  paneFor().attach(name);
}

$("#launcher .set-close").addEventListener("click", closeLauncher);
$("#launcher").addEventListener("click", (e) => { if (e.target.id === "launcher") closeLauncher(); });
$("#launcher .launch-filter").addEventListener("input", renderLauncher);
$("#launcher .launch-filter").addEventListener("keydown", (e) => {
  e.stopPropagation();
  if (e.key === "Escape") closeLauncher();
  if (e.key === "Enter") $("#launcher .launch-row")?.click();
});
$("#launcher .launch-plain").addEventListener("click", async () => {
  const q = $("#launcher .launch-filter").value.trim();
  if (!creatableKey(q)) return;
  closeLauncher();
  await createSession(q);
});

async function createSession(key) {
  const i = key.indexOf(":");
  const path = i < 0 ? "/api/sessions" : `/api/remotes/${encodeURIComponent(key.slice(0, i))}/proxy/sessions`;
  try {
    await api(path, { method: "POST", body: JSON.stringify({ name: key.slice(i + 1) }) });
  } catch (err) { alert(err.message); return; }
  closePalette();
  await refreshSessions();
  paneFor().attach(key);
}

async function killSession(key) {
  // The kill closes these panes' sockets; without this they would read the
  // close as a dropped connection and race the kill with reconnects.
  const attached = panes.filter((p) => p.session === key);
  for (const p of attached) p.deliberate = true;
  try {
    await apiRetry(keyApi(key), { method: "DELETE" });
  } catch (err) {
    for (const p of attached) {
      p.deliberate = false;
      if (p.session === key) p.reconnectNow();
    }
    if (err.message !== "unauthorized") alert(`kill ${key}: ${err.message}`);
    refreshSessions();
    return;
  }
  for (const p of attached) if (p.session === key) p.detach();
  refreshSessions();
}

// --- viewers ---

// Unified-diff text → DOM: file headers, hunk markers, +/- line coloring.
// Text nodes only, so diff content can't inject markup.
function renderDiff(root, text) {
  const frag = document.createDocumentFragment();
  const head = document.createElement("div");
  head.className = "d-root";
  head.textContent = text.trim() ? root : `${root} — working tree clean`;
  frag.appendChild(head);
  if (!text.trim()) return frag;
  for (const line of text.split("\n")) {
    if (line.startsWith("index ") || line.startsWith("+++") || line.startsWith("---")) continue;
    const div = document.createElement("div");
    if (line.startsWith("diff --git")) {
      div.className = "d-file";
      div.textContent = line.replace(/^diff --git a\/(.*) b\/.*$/, "$1");
    } else if (line.startsWith("@@")) {
      div.className = "d-hunk";
      div.textContent = line;
    } else {
      div.className = line.startsWith("+") ? "d-add" : line.startsWith("-") ? "d-del" : "d-ctx";
      div.textContent = line || " ";
    }
    frag.appendChild(div);
  }
  return frag;
}

// The viewer opens opposite the focused terminal: split first if needed.
function viewerPane() {
  const src = paneFor();
  if (panes.length === 1) addPane();
  const target = panes.find((p) => p !== src) || panes[panes.length - 1];
  setFocus(src);
  return target;
}

function openDiffView() {
  const key = paneFor()?.session || allSessions()[0]?.key;
  if (!key) { alert("attach a session first"); return; }
  viewerPane().openView({ type: "diff", key });
}

// --- subagents: helpers an agent spawned, under its session row ---

const SUB_WORDS = { running: "working", done: "done", stopped: "stopped" };

function fmtAge(ms) {
  const m = Math.floor(ms / 60000);
  if (m < 1) return `${Math.max(0, Math.floor(ms / 1000))}s`;
  if (m < 60) return `${m}m`;
  return `${Math.floor(m / 60)}h${String(m % 60).padStart(2, "0")}`;
}

function subagentList(s) {
  const box = document.createElement("span");
  box.className = "subagents";
  for (const a of s.subagents) {
    const row = document.createElement("button");
    row.type = "button";
    row.className = `sub sub-${a.state}`;
    if (panes.some((p) => p.view?.type === "sub" && p.view.key === s.key && p.view.id === a.id)) row.classList.add("open");
    if (a.depth) row.style.marginLeft = `${a.depth * 12}px`;
    const phase = a.state === "running" ? "working" : a.state === "done" ? "done" : "idle";
    row.innerHTML = `<svg viewBox="0 0 12 12" aria-hidden="true">${PHASES[phase][1]}</svg>`;
    const label = document.createElement("span");
    label.className = "sub-label";
    label.textContent = a.label || a.kind || a.id;
    if (a.state === "running" && a.now) {
      const now = document.createElement("span");
      now.className = "sub-now";
      now.textContent = `now: ${a.now}${a.tools ? ` · ${a.tools} tool${a.tools === 1 ? "" : "s"}` : ""}`;
      label.appendChild(now);
    }
    const meta = document.createElement("span");
    meta.className = "sub-meta";
    const started = Date.parse(a.started) || Date.now();
    meta.textContent = a.state === "running" ? fmtAge(Date.now() - started) : SUB_WORDS[a.state] || a.state;
    if (a.depth) {
      const nest = document.createElement("span");
      nest.className = "sub-nest";
      nest.textContent = "↳";
      row.prepend(nest);
    }
    row.append(label, meta);
    row.title = `${a.kind ? `${a.kind} · ` : ""}${a.label || a.id}\n${SUB_WORDS[a.state] || a.state}` +
      ` · started ${new Date(started).toLocaleTimeString()}` +
      (a.depth ? " by another subagent" : " by this session's agent") + " · tap to read along";
    row.addEventListener("click", (e) => {
      e.stopPropagation();
      openSubagent(s.key, a);
    });
    box.appendChild(row);
  }
  return box;
}

// Background commands and watches the session's agent started (a CI
// watch, a test run): running first, finished ones for half an hour. A tap
// shows the tail of the output.
const TASK_ICONS = {
  failed: '<path d="M3 3l6 6M9 3 3 9" fill="none" stroke="currentColor" stroke-width="1.6" stroke-linecap="round"/>',
};
function taskList(s) {
  const box = document.createElement("span");
  box.className = "subagents tasks";
  for (const t of s.tasks) {
    const row = document.createElement("button");
    row.type = "button";
    row.className = `sub task task-${t.state}${t.state === "running" ? " sub-running" : ""}${t.state === "stopped" ? " sub-stopped" : ""}`;
    if (panes.some((p) => p.view?.type === "task" && p.view.key === s.key && p.view.id === t.id)) row.classList.add("open");
    const phase = t.state === "running" ? "working" : t.state === "done" ? "done" : "idle";
    row.innerHTML = `<svg viewBox="0 0 12 12" aria-hidden="true">${TASK_ICONS[t.state] || PHASES[phase][1]}</svg>`;
    const label = document.createElement("span");
    label.className = "sub-label";
    label.textContent = t.label || t.id;
    const meta = document.createElement("span");
    meta.className = "sub-meta";
    const started = Date.parse(t.started) || Date.now();
    meta.textContent = t.state === "running" ? fmtAge(Date.now() - started) : t.state;
    row.append(label, meta);
    row.title = `${t.kind === "monitor" ? "watch" : "background command"} · ${t.label}\n` +
      (t.summary || t.state) + ` · started ${new Date(started).toLocaleTimeString()} · tap for its output`;
    row.addEventListener("click", (e) => {
      e.stopPropagation();
      openTask(s.key, t);
    });
    box.appendChild(row);
  }
  return box;
}

function openTask(key, t) {
  const open = panes.find((p) => p.view?.type === "task" && p.view.key === key && p.view.id === t.id);
  if (open) { setFocus(open); return; }
  const view = { type: "task", key, id: t.id, label: t.label };
  if (matchMedia("(min-width: 700px)").matches) {
    viewerPane().openView(view);
  } else {
    const p = paneFor();
    view.back = p.session || p.view?.back || null;
    p.openView(view);
  }
}

// A card for an action auto mode blocked: what it was and why, "Allow
// once" (types an approval naming exactly this action) or "Leave it".
function blockCard(b, remote) {
  const card = document.createElement("div");
  card.className = "perm-card block-card";
  const head = document.createElement("div");
  head.className = "perm-head";
  head.textContent = `Blocked by auto mode${b.reason ? ` ${b.reason}` : ""}`;
  card.appendChild(head);
  const what = document.createElement("div");
  what.className = "block-what";
  what.textContent = `${b.tool}${b.summary ? ` · ${b.summary}` : ""}`;
  card.appendChild(what);
  if (b.detail && b.detail !== b.summary) {
    const pre = document.createElement("pre");
    pre.className = "perm-detail";
    pre.textContent = b.detail;
    card.appendChild(pre);
  }
  const row = document.createElement("div");
  row.className = "perm-row";
  for (const [d, label, cls] of [["allow", "Allow once", "perm-allow"], ["dismiss", "Leave it", "perm-terminal"]]) {
    const btn = document.createElement("button");
    btn.className = cls;
    btn.textContent = label;
    btn.title = d === "allow" ? "types an approval for exactly this action into the session" : "close this card; Claude goes on without it";
    btn.addEventListener("click", (e) => {
      e.stopPropagation();
      answerBlocked(remote, b.id, d, row);
    });
    row.appendChild(btn);
  }
  card.appendChild(row);
  return card;
}

async function answerBlocked(remote, id, decision, box) {
  const buttons = [...box.querySelectorAll("button")];
  buttons.forEach((x) => (x.disabled = true));
  box.querySelector(".perm-err")?.remove();
  const path = remote
    ? `/api/remotes/${encodeURIComponent(remote)}/proxy/agent/blocked/${encodeURIComponent(id)}`
    : `/api/agent/blocked/${encodeURIComponent(id)}`;
  const note = (text) => {
    const n = document.createElement("span");
    n.className = "perm-err";
    n.textContent = text;
    box.appendChild(n);
  };
  try {
    const r = await apiRetry(path, { method: "POST", body: JSON.stringify({ decision }) });
    if (r?.queued) note(`approval queued: ${r.reason}`);
  } catch (err) {
    if (err.status !== 404) {
      buttons.forEach((x) => (x.disabled = false));
      if (err.message !== "unauthorized") note(err.status === 409 ? err.message : "not sent, try again");
      return;
    }
  }
  refreshSessions();
}

// Sessions this session's agent started for another agent (muxdeck-msg
// --new): listed under it like its subagents; a click attaches them.
function delegatedList(s) {
  const kids = allSessions().filter((x) => !x.remote && x.from === s.name);
  if (!kids.length) return null;
  const box = document.createElement("span");
  box.className = "subagents delegated";
  for (const k of kids) {
    const row = document.createElement("button");
    row.type = "button";
    const phase = agentPhase(k) || "idle";
    row.className = `sub sub-session phase-${phase}`;
    if (panes.some((p) => p.session === k.key)) row.classList.add("open");
    row.innerHTML = `<svg viewBox="0 0 12 12" aria-hidden="true">${PHASES[phase][1]}</svg>`;
    const label = document.createElement("span");
    label.className = "sub-label";
    label.textContent = `${AGENT_LABELS[k.agent?.agent] || k.agent?.agent || "session"} · ${k.name}`;
    const meta = document.createElement("span");
    meta.className = "sub-meta";
    meta.textContent = PHASES[phase][0];
    row.append(label, meta);
    row.title = `session ${k.name}, started by the agent in ${s.name} · tap to open`;
    row.addEventListener("click", (e) => {
      e.stopPropagation();
      paneFor().attach(k.key);
    });
    box.appendChild(row);
  }
  return box;
}

// Wide screens read along next to the terminal; on a phone the pane is too
// small to split, so the view takes the pane and offers the way back.
function openSubagent(key, a) {
  const open = panes.find((p) => p.view?.type === "sub" && p.view.key === key && p.view.id === a.id);
  if (open) { setFocus(open); return; }
  if (matchMedia("(min-width: 700px)").matches) {
    viewerPane().openSub(key, a);
  } else {
    const p = paneFor();
    p.openSub(key, a, p.session || p.view?.back || null);
  }
}

function subRenderer(box, key, back, send) {
  const el = (tag, cls, text) => {
    const n = document.createElement(tag);
    if (cls) n.className = cls;
    if (text !== undefined) n.textContent = text;
    return n;
  };
  const head = el("div", "m-head");
  const line = el("span", "m-head-line", "loading…");
  head.appendChild(line);
  if (back) {
    const actions = el("span", "m-head-actions");
    const b = el("button", null, `back to ${key}`);
    b.addEventListener("click", back);
    actions.appendChild(b);
    head.appendChild(actions);
  }
  const stream = el("div", "m-stream");
  const foot = el("div", "m-note");
  box.append(head, stream);
  stream.appendChild(foot);

  // Neither agent takes messages for a subagent from outside: the text goes
  // to the agent that spawned it, which forwards it with its own tool.
  const form = el("form", "sub-compose");
  const input = el("textarea");
  input.rows = 1;
  input.placeholder = "Message this subagent";
  input.title = "Goes to the agent that started it, which passes it on";
  const sendBtn = el("button", null, "send");
  sendBtn.type = "submit";
  const note = el("div", "sub-compose-note");
  note.hidden = true;
  form.append(input, sendBtn, note);
  box.appendChild(form);
  const grow = () => {
    input.style.height = "auto";
    input.style.height = `${Math.min(input.scrollHeight, 140)}px`;
    input.style.overflowY = input.scrollHeight > 140 ? "auto" : "hidden";
  };
  input.addEventListener("input", grow);
  const coarse = matchMedia("(pointer: coarse)").matches;
  input.addEventListener("keydown", (e) => {
    if (e.key === "Enter" && !e.shiftKey && !coarse && !e.isComposing) {
      e.preventDefault();
      form.requestSubmit();
    }
  });
  form.addEventListener("submit", async (e) => {
    e.preventDefault();
    const text = input.value.trim();
    if (!text || sendBtn.disabled) return;
    sendBtn.disabled = true;
    note.hidden = true;
    try {
      await send(text);
      input.value = "";
      grow();
      push(el("div", "m-sent", text));
      box.scrollTop = box.scrollHeight;
      note.textContent = "sent to the agent that started it — it passes the message on";
      note.className = "sub-compose-note";
      note.hidden = false;
      setTimeout(() => { note.hidden = true; }, 6000);
    } catch (err) {
      note.textContent = err.message;
      note.className = "sub-compose-note err";
      note.hidden = false;
    } finally {
      sendBtn.disabled = false;
    }
  });

  let tools = new Map();
  let first = true;
  let pruned = null;
  const MAX_NODES = 400;
  // A call's description goes in its heading; a call that is only a
  // command or a path shows just that.
  const fmtArgs = (args) => {
    if (args == null) return "";
    if (typeof args === "string") return args;
    const rest = { ...args };
    delete rest.description;
    const keys = Object.keys(rest);
    const lead = rest.command ?? rest.cmd ?? rest.file_path ?? rest.path ?? rest.pattern ?? rest.url ?? rest.query;
    let s;
    if (keys.length === 1 && typeof lead === "string") s = lead;
    else if (keys.length === 1 && Array.isArray(lead)) s = lead.join(" ");
    else s = keys.map((k) => {
      const v = rest[k];
      if (typeof v !== "string") return `${k}: ${JSON.stringify(v)}`;
      return v.includes("\n") ? `${k}:\n${v}` : `${k}: ${v}`;
    }).join("\n");
    return s.length > 2048 ? s.slice(0, 2048) + " …" : s;
  };
  const md = (text) => {
    const n = el("div", "m-text m-md");
    n.innerHTML = DOMPurify.sanitize(marked.parse(text));
    return n;
  };
  const push = (n) => {
    stream.insertBefore(n, foot);
    if (stream.childElementCount > MAX_NODES) {
      if (!pruned) {
        pruned = el("div", "m-audit", "(older steps hidden)");
        stream.prepend(pruned);
      }
      while (stream.childElementCount > MAX_NODES) stream.children[1].remove();
    }
  };
  function render(e) {
    switch (e.k) {
      case "user": {
        if (first) {
          const det = el("details", "m-taskbox");
          const sum = el("summary", null, (e.text || "").trim().split("\n")[0].slice(0, 160));
          det.append(sum, el("div", "m-taskbody", e.text));
          push(det);
        } else {
          push(el("div", "m-user", e.text));
        }
        break;
      }
      case "text":
        push(md(e.text || ""));
        break;
      case "think": {
        const det = el("details", "m-think");
        det.append(el("summary", null, "thinking"), el("div", null, e.text));
        push(det);
        break;
      }
      case "tool": {
        const card = el("div", "m-tool");
        const name = el("div", "m-tool-name", e.name || "tool");
        if (typeof e.args?.description === "string") name.append(el("span", "m-tool-desc", ` · ${e.args.description}`));
        card.append(name, el("pre", "m-tool-args", fmtArgs(e.args)));
        if (e.id) tools.set(e.id, card);
        push(card);
        break;
      }
      case "result": {
        const det = el("details", "m-result" + (e.err ? " err" : ""));
        det.append(el("summary", null, e.err ? "result · error" : "result"), el("pre", null, e.text || "(empty)"));
        const card = e.id && tools.get(e.id);
        if (card) { card.appendChild(det); tools.delete(e.id); } else push(det);
        break;
      }
    }
    first = false;
  }
  return {
    add(events) {
      if (!events.length) return;
      const follow = box.scrollTop + box.clientHeight >= box.scrollHeight - 60;
      for (const e of events) render(e);
      if (follow) box.scrollTop = box.scrollHeight;
    },
    clear() {
      stream.innerHTML = "";
      stream.appendChild(foot);
      tools = new Map();
      first = true;
      pruned = null;
    },
    status(t) {
      if (t.state === "error") {
        foot.className = "m-exit";
        foot.textContent = t.error;
        return;
      }
      const who = AGENT_LABELS[t.agent] || t.agent || "agent";
      const started = Date.parse(t.started);
      line.textContent = [who, t.kind, started ? `started ${new Date(started).toLocaleTimeString()}` : ""].filter(Boolean).join(" · ");
      if (t.state === "running") {
        foot.className = "m-note m-live";
        foot.textContent = `working · ${fmtAge(Date.now() - (started || Date.now()))}`;
      } else if (t.state === "done") {
        foot.className = "m-done";
        foot.textContent = "done";
      } else {
        foot.className = "m-exit";
        foot.textContent = "stopped";
      }
    },
  };
}

async function openMdPicker() {
  const key = paneFor()?.session || allSessions()[0]?.key;
  if (!key) { alert("attach a session first"); return; }
  let files;
  try { ({ files } = await api(keyApi(key, "/files"))); }
  catch (err) { alert(err.message); return; }
  if (!files.length) { alert("no markdown files in the session's directory"); return; }
  setPalMode("mdpick", null, key);
  pal.files = files;
  renderPalette();
}

// Stage-aware completion for ":remote": menu items for the token being
// typed, plus a ghost hint of the syntax still ahead. `words` holds only
// completed tokens — a trailing partial is popped off and used as the
// filter prefix.
function remoteSuggest() {
  const q = $("#pal-input").value;
  const words = q.split(/\s+/).filter(Boolean);
  const part = q.endsWith(" ") || !words.length ? "" : words.pop();
  const sugg = (list) => list.filter((it) => it.name.startsWith(part));
  const ghost = (t) => (part ? "" : t);
  const [verb] = words;
  if (!words.length)
    return {
      items: sugg([
        { name: "add", hint: "connect a new remote" },
        { name: "rm",  hint: "delete a remote" },
        { name: "off", hint: "disconnect, keep config" },
        { name: "on",  hint: "reconnect" },
      ]),
      ghost: ghost("add · rm · off · on"),
    };
  if (verb === "rm" || verb === "off" || verb === "on") {
    const pool = verb === "rm" ? remotes.filter((r) => !r.cloud) : remotes.filter((r) => (r.state === "off") === (verb === "on"));
    return {
      items: words.length === 1 ? sugg(pool.map((r) => ({ name: r.name, hint: r.state }))) : [],
      ghost: words.length === 1 ? ghost("<name>") : "",
    };
  }
  if (verb !== "add") return { items: [], ghost: ghost("add · rm · off · on") };
  if (words.length === 1) return { items: [], ghost: ghost("<name>") };
  if (words.length === 2)
    return {
      items: sugg([
        { name: "ssh", hint: "<host[:port]> [token]" },
        { name: "url", hint: "<url> [token]" },
      ]),
      ghost: ghost("ssh · url"),
    };
  if (words.length === 3)
    return { items: [], ghost: ghost(words[2] === "url" ? "<url> [token]" : "<host[:port]> [token]") };
  return { items: [], ghost: words.length === 4 ? ghost("[token]") : "" };
}

function remoteLineValid(line) {
  const w = line.split(/\s+/).filter(Boolean);
  return ["rm", "off", "on"].includes(w[0]) ? w.length === 2 :
    w[0] === "add" && (w[2] === "ssh" || w[2] === "url") && w.length >= 4 && w.length <= 5;
}

// ":remote ❯" one-liners: "add jack ssh jack", "add lab ssh lab.example:9000 TOKEN",
// "add juneau url http://juneau:8300 TOKEN", "rm jack", "off jack", "on jack".
async function runRemoteCommand(line) {
  const words = line.split(/\s+/).filter(Boolean);
  const [verb, name] = words;
  try {
    if (verb === "rm" && name && words.length === 2) {
      await api(`/api/remotes/${encodeURIComponent(name)}`, { method: "DELETE" });
    } else if ((verb === "off" || verb === "on") && name && words.length === 2) {
      await api(`/api/remotes/${encodeURIComponent(name)}`, {
        method: "PATCH",
        body: JSON.stringify({ off: verb === "off" }),
      });
    } else if (verb === "add" && words.length >= 4 && words.length <= 5) {
      const [, , mode, target, token] = words;
      const body = { name, mode, token: token || "" };
      if (mode === "ssh") {
        const m = target.match(/^(.+?)(?::(\d+))?$/);
        body.host = m[1];
        if (m[2]) body.remote_port = +m[2];
      } else if (mode === "url") {
        body.url = target;
      } else {
        throw new Error("mode must be ssh or url");
      }
      await api("/api/remotes", { method: "POST", body: JSON.stringify(body) });
    } else {
      throw new Error("usage: add <name> ssh <host[:port]> [token] · add <name> url <url> [token] · rm <name> · off <name> · on <name>");
    }
  } catch (err) { alert(err.message); return; }
  closePalette();
  refreshSessions();
}

// ":cloud ❯": the account whose machines fill the cloud group. "signin"
// opens the account page — in the desktop app the sign-in returns through
// the muxdeck:// link on its own; in a browser deck the account page mints
// an app token to paste as "signin mdd_…".
function cloudSuggest() {
  const q = $("#pal-input").value;
  const words = q.split(/\s+/).filter(Boolean);
  const part = q.endsWith(" ") || !words.length ? "" : words.pop();
  const sugg = (list) => list.filter((it) => it.name.startsWith(part));
  const ghost = (t) => (part ? "" : t);
  const signedIn = !!cloudStatus?.signed_in;
  if (!words.length)
    return {
      items: sugg(signedIn
        ? [
          { name: "sync",    hint: "refresh the account's machines" },
          { name: "signout", hint: `forget this device (${cloudStatus.account || "signed in"})` },
          { name: "signin",  hint: "switch account" },
        ]
        : [{ name: "signin", hint: "open the account page · or paste an app token" }]),
      ghost: ghost(signedIn ? "sync · signout · signin" : "signin [token]"),
    };
  if (words[0] === "signin") return { items: [], ghost: words.length === 1 ? ghost("[token]") : "" };
  return { items: [], ghost: "" };
}

function cloudLineValid(line) {
  const w = line.split(/\s+/).filter(Boolean);
  return (w[0] === "signin" && w.length <= 2) || ((w[0] === "signout" || w[0] === "sync") && w.length === 1);
}

const CLOUD_URL = "https://cloud.muxdeck.app";

// The desktop app opens the account page in the system browser and gets
// the token back through its URL scheme (?app=1 asks the page for that
// handoff); a plain browser deck gets the page's app-token form instead.
async function openCloudSignin() {
  const base = cloudStatus?.url || CLOUD_URL;
  const T = window.__TAURI__;
  if (T) {
    try { await T.core.invoke("plugin:opener|open_url", { url: base + "/?app=1" }); return; } catch {}
  }
  window.open(base + "/", "_blank", "noopener");
}

async function runCloudCommand(line) {
  const [verb, token] = line.split(/\s+/).filter(Boolean);
  try {
    if (verb === "signin" && !token) {
      await openCloudSignin();
      $(".pal-hint").textContent = "sign in on the account page, then paste an app token here: signin mdd_…";
      $("#pal-input").value = "signin ";
      renderPalette();
      return;
    } else if (verb === "signin") {
      await api("/api/cloud", { method: "POST", body: JSON.stringify({ token }) });
    } else if (verb === "signout") {
      await api("/api/cloud", { method: "DELETE" });
    } else if (verb === "sync") {
      await api("/api/cloud/sync", { method: "POST" });
    } else {
      throw new Error("usage: signin [token] · signout · sync");
    }
  } catch (err) { alert(err.message); return; }
  closePalette();
  refreshSessions();
}

// --- keep awake: the preference belongs to the machine being viewed — the
// focused pane's host, else this daemon — and the palette names it before
// changing anything, so a deck with several machines never toggles the
// wrong one.

function awakeTarget() {
  const p = paneFor();
  const key = p?.session || "";
  const i = key.indexOf(":");
  const host = p?.view?.type === "mush" ? (p.view.host || "") : i < 0 ? "" : key.slice(0, i);
  return { host, label: host || machineName || "this machine" };
}

function powerApi(host) {
  return host ? `/api/remotes/${encodeURIComponent(host)}/proxy/power` : "/api/power";
}

function awakeLabel(st) {
  const viewers = `${st.viewers} viewer${st.viewers === 1 ? "" : "s"}`;
  switch (st.state) {
    case "off": return "off";
    case "waiting": return "on — waiting for a viewer";
    case "keeping_awake": return `keeping awake (${viewers})`;
    case "on_battery": return `on battery — not preventing sleep (${viewers})`;
    case "unsupported": return "unsupported on this machine (macOS only)";
    case "unavailable": return `unavailable — ${st.error || "backend failed"}`;
    default: return st.state || "unknown";
  }
}

function awakeSuggest() {
  const q = $("#pal-input").value;
  const part = q.trim().split(/\s+/).filter(Boolean).pop() || "";
  const { label } = awakeTarget();
  const items = [
    { name: "status", hint: `keep awake on ${label}` },
    { name: "on",     hint: `${label}: stay awake while plugged in and viewed` },
    { name: "off",    hint: `${label}: back to normal sleep` },
  ].filter((it) => it.name.startsWith(part));
  return { items, ghost: part ? "" : `on · off · status  (${label})` };
}

async function runAwakeCommand(line) {
  const verb = line.trim().split(/\s+/)[0] || "status";
  const { host, label } = awakeTarget();
  try {
    let st;
    if (verb === "on" || verb === "off") {
      st = await api(powerApi(host), { method: "POST", body: JSON.stringify({ keep_awake_while_viewing: verb === "on" }) });
    } else if (verb === "status") {
      st = await api(powerApi(host));
    } else {
      throw new Error("usage: on · off · status");
    }
    $(".pal-hint").textContent = `${label}: keep awake while viewing ${awakeLabel(st)}`;
  } catch (err) {
    const old = /not found/i.test(err.message);
    $(".pal-hint").textContent = old ? `${label}: its daemon predates keep awake — update muxdeck there` : `${label}: ${err.message}`;
  }
  $("#pal-input").value = "";
  renderPalette();
}

// The desktop shell hands the sign-in deep link here (muxdeck://signin#token=…).
window.muxdeckDeepLink = async (u) => {
  const m = /^muxdeck:\/\/signin#token=([A-Za-z0-9_]+)$/.exec(String(u));
  if (!m) return;
  try { await api("/api/cloud", { method: "POST", body: JSON.stringify({ token: m[1] }) }); }
  catch (err) { alert(`muxdeck cloud sign-in failed: ${err.message}`); return; }
  if (!$("#palette").hidden && pal.mode === "cloud") closePalette();
  refreshSessions();
};

const PAL_MODES = {
  switch:  { prompt: "❯", ph: "session or :command", hint: "↑↓ move · tab complete · ⏎ open · : commands · esc close" },
  new:     { prompt: "new ❯", ph: "session name (host:name for a remote)", hint: "⏎ create · esc back" },
  pick:    { prompt: null, ph: "which session?", hint: "↑↓ move · ⏎ select · esc back" },
  input:   { prompt: null, ph: "new name", hint: "⏎ rename · esc back" },
  confirm: { prompt: null, ph: "", hint: "y kill · n / esc back" },
  remote:  { prompt: ":remote ❯", ph: "add · rm · off · on", hint: "tab complete · ⏎ run · esc back" },
  cloud:   { prompt: ":cloud ❯", ph: "signin [token] · signout · sync", hint: "tab complete · ⏎ run · esc back" },
  awake:   { prompt: ":awake ❯", ph: "on · off · status", hint: "prevents automatic sleep while this Mac is plugged in and you're viewing a terminal or live run · the display can still sleep" },
  mdpick:  { prompt: ":md ❯", ph: "which file?", hint: "↑↓ move · ⏎ preview · esc back" },
  mush:    { prompt: ":mush ❯", ph: "[-m model] task (runs in the focused session's cwd)", hint: "⏎ run · esc back" },
  runs:    { prompt: ":runs ❯", ph: "which run?", hint: "↑↓ move · ⏎ open · esc back" },
};

function setPalMode(mode, verb = null, arg = null) {
  pal = { mode, verb, arg, index: 0 };
  const m = PAL_MODES[mode];
  const inp = $("#pal-input");
  inp.value = mode === "input" ? arg : "";
  $(".pal-mode").textContent =
    mode === "pick" ? `:${verb} ❯` :
    mode === "input" ? `:${verb} ${arg} ❯` :
    mode === "confirm" ? `:${verb} ${arg}? [y/N]` : m.prompt;
  inp.readOnly = mode === "confirm";
  $(".pal-hint").textContent = m.hint;
  renderPalette();
  inp.focus();
  if (mode === "input") inp.setSelectionRange(0, inp.value.length);
}

function openPalette(mode) {
  $("#palette").hidden = false;
  setPalMode(mode);
}

function closePalette() {
  $("#palette").hidden = true;
  paneFor()?.term?.focus();
}

function palBack() {
  if (pal.mode === "switch") closePalette();
  else setPalMode("switch");
}

// The ghost sits after the typed text (offset one cell for the block
// caret) and plays placeholder when the input is empty: the mode's ph in
// most modes, a stage-aware syntax hint in remote mode.
function updateGhost() {
  const inp = $("#pal-input");
  const g = $(".pal-ghost");
  g.textContent = pal.mode === "remote" ? remoteSuggest().ghost
    : pal.mode === "cloud" ? cloudSuggest().ghost
    : pal.mode === "awake" ? awakeSuggest().ghost
    : inp.value ? "" : PAL_MODES[pal.mode]?.ph || "";
  g.style.left = `${inp.value.length + 1}ch`;
}

// Tab (or tap/⏎ on a suggestion): splice the selected name into the input
// in place of the partial token, without running anything.
function palComplete(it) {
  const inp = $("#pal-input");
  if (it.kind === "suggest") {
    const q = inp.value;
    const head = q.endsWith(" ") ? q : q.slice(0, q.lastIndexOf(" ") + 1);
    inp.value = head + it.name + " ";
  } else if (it.kind === "create") {
    return;
  } else {
    inp.value = it.name;
  }
  pal.index = 0;
  inp.focus();
  inp.setSelectionRange(inp.value.length, inp.value.length);
  renderPalette();
}

function updateCaret() {
  const inp = $("#pal-input");
  const caret = $(".pal-caret");
  caret.hidden = pal.mode === "confirm" || document.activeElement !== inp;
  const pos = inp.selectionStart ?? inp.value.length;
  caret.style.left = `${pos}ch`;
  caret.textContent = inp.value[pos] || " ";
}

$("#palette").addEventListener("click", (e) => {
  if (e.target === $("#palette")) closePalette();
});

$("#pal-input").addEventListener("input", () => {
  pal.index = 0;
  renderPalette();
});
$("#pal-input").addEventListener("focus", updateCaret);
$("#pal-input").addEventListener("blur", updateCaret);
document.addEventListener("selectionchange", () => {
  if (!$("#palette").hidden) updateCaret();
});

$("#pal-input").addEventListener("keydown", async (e) => {
  if (e.key === "Escape") { e.preventDefault(); palBack(); return; }
  if (pal.mode === "confirm") {
    e.preventDefault();
    if (e.key.toLowerCase() === "y") { const name = pal.arg; closePalette(); await killSession(name); }
    else if (e.key === "Enter" || e.key.toLowerCase() === "n") palBack();
    return;
  }
  const items = palItems();
  if (e.key === "Tab") {
    e.preventDefault();
    if (items[pal.index]) palComplete(items[pal.index]);
  } else if (e.key === "ArrowDown" || e.ctrlKey && e.key === "n") {
    e.preventDefault(); pal.index++; renderPalette();
  } else if (e.key === "ArrowUp" || e.ctrlKey && e.key === "p") {
    e.preventDefault(); pal.index--; renderPalette();
  } else if (e.key === "Enter") {
    e.preventDefault();
    const name = $("#pal-input").value.trim();
    if (pal.mode === "new") { if (creatableKey(name)) await createSession(name); }
    else if (pal.mode === "input") {
      if (VALID_NAME.test(name)) { closePalette(); await renameSession(pal.arg, name); }
    } else if (pal.mode === "remote") {
      if (!remoteLineValid(name) && items[pal.index]) palComplete(items[pal.index]);
      else await runRemoteCommand(name);
    } else if (pal.mode === "cloud") {
      if (!cloudLineValid(name) && items[pal.index]) palComplete(items[pal.index]);
      else await runCloudCommand(name);
    } else if (pal.mode === "awake") {
      if (!/^(on|off|status)$/.test(name) && items[pal.index]) palComplete(items[pal.index]);
      else await runAwakeCommand(name);
    }
    else if (pal.mode === "mush") { await runMushCommand(name); }
    else if (items[pal.index]) await palActivate(items[pal.index]);
  } else if (e.key === "Backspace" && !$("#pal-input").value && pal.mode !== "switch") {
    e.preventDefault(); palBack();
  }
});

// --- font size ---

function setFontSize(v) {
  fontSize = Math.max(9, Math.min(24, v));
  localStorage.setItem("muxdeck-font", fontSize);
  for (const p of panes) {
    if (p.term) {
      p.term.options.fontSize = fontSize;
      p.sendResize();
    }
  }
}

// --- touch keyboard bar ---
// The iOS/iPadOS software keyboard has no Esc/Tab/Ctrl/arrows; the bar fills
// the gap. Ctrl is a sticky modifier applied to the next typed character.

let ctrlArmed = false;

// What the terminal itself answers or reports (mouse, focus, status
// replies) rather than something the person typed.
const TERM_REPORT = /^\x1b(\[[<M]|\[[IO]$|\[[?>]?[\d;]*[cRnty]$|\]|P)/;
const IME_FIX = matchMedia("(pointer: coarse)").matches && prefs.imefix !== false;

// What turns the text `prev` into `next` when all a terminal offers is
// backspace and typing at the end: how many leading UTF-16 units stay, how
// many characters to delete after them, and what to type.
function imeDiff(prev, next) {
  let keep = 0;
  const n = Math.min(prev.length, next.length);
  while (keep < n && prev.charCodeAt(keep) === next.charCodeAt(keep)) keep++;
  if (keep > 0 && keep < Math.max(prev.length, next.length)) {
    const c = prev.charCodeAt(keep - 1);
    if (c >= 0xd800 && c <= 0xdbff) keep--;
  }
  return { keep, del: [...prev.slice(keep)].length, add: next.slice(keep) };
}

function applyCtrl(data) {
  if (!ctrlArmed || data.length !== 1) return data;
  const code = data.toUpperCase().charCodeAt(0);
  if (code >= 64 && code < 96) {
    setCtrl(false);
    return String.fromCharCode(code & 0x1f);
  }
  if (data === " ") {
    setCtrl(false);
    return "\x00";
  }
  return data;
}

function setCtrl(on) {
  ctrlArmed = on;
  $("#key-ctrl").classList.toggle("armed", on);
}

// Extra keybar keys, one per line: "label=keys" or just "keys". In keys, ^X
// is ctrl-X, \e escape, \t tab, \n enter; anything else is typed as is.
function parseKeyText(text) {
  return text.replace(/\^([@A-Za-z[\\\]^_?])/g, (_, c) => c === "?" ? "\x7f" : String.fromCharCode(c.toUpperCase().charCodeAt(0) & 31))
    .replace(/\\e/g, "\x1b").replace(/\\t/g, "\t").replace(/\\n/g, "\r");
}

function renderExtraKeys() {
  const bar = $("#keybar");
  bar.querySelectorAll(".key-extra").forEach((b) => b.remove());
  for (const line of prefs.keys.split("\n")) {
    const t = line.trim();
    if (!t) continue;
    const i = t.indexOf("=");
    const label = i > 0 ? t.slice(0, i).trim() : t;
    const keys = i > 0 ? t.slice(i + 1) : t;
    const b = document.createElement("button");
    b.className = "key-extra";
    b.textContent = label;
    b.dataset.key = parseKeyText(keys);
    bar.appendChild(b);
  }
}

// The key bar takes two or three rows of a phone screen and is wanted now
// and then, so it stays folded away behind the pane's "keys" button.
function syncKeysButtons(root = document) {
  if (!matchMedia("(pointer: coarse)").matches) return;
  for (const b of root.querySelectorAll(".p-keys")) {
    b.hidden = !prefs.keybar;
    b.classList.toggle("armed", !!prefs.keysOpen);
  }
}

function applyKeybarPref() {
  if (!matchMedia("(pointer: coarse)").matches) return;
  $("#keybar").hidden = !(prefs.keybar && prefs.keysOpen);
  syncKeysButtons();
  renderExtraKeys();
  panes.forEach((p) => p.sendResize());
}

function initKeybar() {
  if (!matchMedia("(pointer: coarse)").matches) return;
  const bar = $("#keybar");
  bar.hidden = !(prefs.keybar && prefs.keysOpen);
  renderExtraKeys();
  // pointerdown + preventDefault keeps focus (and the software keyboard) on
  // the terminal while tapping bar keys. The paste button is the exception:
  // clipboard reads need a full click activation (and iOS paste UI).
  bar.addEventListener("pointerdown", (e) => {
    const btn = e.target.closest("button");
    if (!btn || btn.id === "key-paste" || btn.id === "key-file") return;
    e.preventDefault();
    if (btn.id === "key-ctrl") {
      setCtrl(!ctrlArmed);
    } else {
      paneFor()?.send(applyCtrl(btn.dataset.key));
    }
  });
  $("#key-paste").addEventListener("click", pasteToFocused);
  const picker = $("#file-pick");
  $("#key-file").addEventListener("click", () => picker.click());
  picker.addEventListener("change", () => {
    uploadFiles(paneFor(), picker.files);
    picker.value = "";
  });

  // Shrink the layout to the visual viewport so the terminal is not hidden
  // behind the software keyboard.
  if (window.visualViewport) {
    const vv = window.visualViewport;
    const adjust = () => {
      $("#app").style.height = `${vv.height}px`;
      panes.forEach((p) => p.sendResize());
    };
    vv.addEventListener("resize", adjust);
  }
}

// --- boot ---

if ("serviceWorker" in navigator) {
  navigator.serviceWorker.register("sw.js").catch(() => {});
  navigator.serviceWorker.addEventListener("message", (e) => {
    if (e.data?.type === "attach" && e.data.session) paneFor().attach(e.data.session);
    if (e.data?.type === "refresh") refreshSessions();
  });
}
applySidebar();
addPane();
initKeybar();
window.addEventListener("resize", () => panes.forEach((p) => p.sendResize()));
// Suspension recovery: reconnectNow() trusts readyState, which a zombie
// socket still reports as OPEN — so resume must go through resumeAll with
// the measured time away, and a bfcache restore counts as a full absence.
let hiddenAt = 0;
document.addEventListener("visibilitychange", () => {
  if (document.hidden) {
    hiddenAt = Date.now();
    wsdbg.log("hidden");
    for (const p of panes) p.sendPresence(false);
    return;
  }
  const away = hiddenAt ? Date.now() - hiddenAt : 0;
  wsdbg.log("visible", { away });
  Pane.resumeAll(away);
  if (away > 15000) changesAbort?.abort();
  if (refreshWhenVisible) { refreshWhenVisible = false; refreshSessions(); }
  for (const p of panes) { p.sendPresence(true); p.claim(); }
});
window.addEventListener("focus", () => panes.forEach((p) => p.claim()));
$("#sessions").addEventListener("pointerdown", () => { sidebarPress = true; }, true);
const sidebarRelease = () => {
  if (!sidebarPress) return;
  sidebarPress = false;
  // After the click that follows, which still needs its row.
  if (sidebarStale) { sidebarStale = false; setTimeout(refreshSessions, 400); }
};
document.addEventListener("pointerup", sidebarRelease, true);
document.addEventListener("pointercancel", sidebarRelease, true);
document.addEventListener("touchstart", () => panes.forEach((p) => p.claim()), { passive: true, capture: true });
window.addEventListener("pageshow", (e) => {
  if (!e.persisted) return;
  wsdbg.log("bfcache-restore");
  Pane.resumeAll(Infinity);
});
window.addEventListener("online", () => wsdbg.log("online"));
window.addEventListener("offline", () => wsdbg.log("offline"));

// Awake-idle watchdog: the daemon heartbeats every 25s, so a visible pane
// more than two beats quiet holds a suspect socket. An api() probe first
// separates a dead network (leave the retry loop alone) from a zombie
// socket (reconnect, and log that it happened while awake).
setInterval(() => {
  if (document.hidden) return;
  for (const p of panes) {
    if (!p.session || !p.ws || !p.hbSeen) continue;
    const gap = Date.now() - p.lastRx;
    if (gap < 65000) continue;
    wsdbg.log("stale", { s: p.session, gap, rs: p.ws.readyState });
    api("/api/sessions").then(
      () => {
        if (!p.session || !p.ws || Date.now() - p.lastRx < 65000) return;
        wsdbg.log("zombie-confirmed", { s: p.session, gap: Date.now() - p.lastRx, rs: p.ws.readyState });
        p.forceReconnect();
      },
      (e) => wsdbg.log("probe-failed", { s: p.session, err: String(e && e.message || e) })
    );
    break; // one suspect per tick keeps the probe singular
  }
}, 15000);
refreshSessions().then(() => { if (!loginShown()) attachFromHash(); });
setInterval(() => { if (!document.hidden) refreshSessions(); }, 10000);
watchChanges();

// --- mush runs: muxdeck as a front-end of the mush agent protocol ---

function mushApi(host, rest = "") {
  if (!host) return `/api/mush/runs${rest}`;
  return `/api/remotes/${encodeURIComponent(host)}/proxy/mush/runs${rest}`;
}

// removeRun drops a finished run from its daemon's ledger (mush runs
// remove): the row and its journal are gone for good, so it asks first, then
// closes any pane still viewing the run.
async function removeRun(r) {
  if (!confirm(`Remove run "${runLabel(r)}" from the ledger? Its journal goes with it.`)) return;
  try {
    await api(mushApi(r.host, `/${encodeURIComponent(r.id)}/remove`), { method: "POST" });
  } catch (err) {
    alert(err.message);
    return;
  }
  for (const p of panes.filter((p) => p.view?.type === "mush" && p.view.runId === r.id && (p.view.host || "") === (r.host || ""))) closePane(p);
  refreshSessions();
}

async function runMushCommand(input) {
  if (!input) return;
  // "-m <model>" prefix overrides the engine's default model for this run.
  let task = input, model = "";
  const m = input.match(/^-m(?:odel)?\s+(\S+)\s+(.+)$/);
  if (m) { model = m[1]; task = m[2]; }
  const key = paneFor()?.session;
  if (!key) { alert("focus a session first — the run uses its working directory"); return; }
  const i = key.indexOf(":");
  const host = i < 0 ? "" : key.slice(0, i);
  const session = i < 0 ? key : key.slice(i + 1);
  try {
    const run = await api(mushApi(host), { method: "POST", body: JSON.stringify({ task, session, model }) });
    closePalette();
    viewerPane().openMush(host, run);
    refreshSessions();
  } catch (err) { alert(err.message); }
}

// mushRenderer builds the event-stream DOM incrementally. All event payloads
// land as text nodes — engine output is untrusted. send() carries protocol
// commands (approval answers) back over the run's websocket.
function mushRenderer(box, send) {
  const stream = document.createElement("div");
  stream.className = "m-stream";
  const approvals = document.createElement("div");
  approvals.className = "m-approvals";
  box.append(stream, approvals);

  let textBlock = null, thinkBlock = null, lastTool = null, prunedNote = null;
  const el = (tag, cls, text) => {
    const n = document.createElement(tag);
    if (cls) n.className = cls;
    if (text !== undefined) n.textContent = text;
    return n;
  };
  // Long runs grow the stream without bound (per-payload caps bound each
  // card, not the card count). Keep the DOM to the recent tail; the full
  // history stays in the daemon's replay buffer.
  const MAX_STREAM_NODES = 400;
  const push = (n) => {
    const follow = box.scrollTop + box.clientHeight >= box.scrollHeight - 40;
    stream.appendChild(n);
    if (stream.childElementCount > MAX_STREAM_NODES) {
      if (!prunedNote) {
        prunedNote = el("div", "m-audit", "(older output pruned)");
        stream.prepend(prunedNote);
      }
      while (stream.childElementCount > MAX_STREAM_NODES) stream.children[1].remove();
    }
    if (follow) box.scrollTop = box.scrollHeight;
  };
  const breakBlocks = () => { textBlock = null; thinkBlock = null; };
  const fmtArgs = (args) => {
    let s = JSON.stringify(args ?? {}, null, 1);
    if (s.length > 2048) s = s.slice(0, 2048) + " …";
    return s;
  };

  function render(env) {
    const d = env.data || {};
    switch (env.type) {
      case "run_queued":
        break;
      case "run_started":
        push(el("div", "m-task", d.task || ""));
        break;
      case "checkpoint":
      case "run_artifact":
        break;
      case "run_state_changed":
        push(el("div", "m-audit", `state · ${(d.state || "").replace("_", " ")}`));
        break;
      case "approval_resolved": {
        // The decision landed (ours, or someone answering elsewhere — the
        // CLI, GitHub, another viewer): retire any card still up.
        if (approvals.childElementCount) {
          approvals.innerHTML = "";
          push(el("div", "m-audit", `${d.approved ? "approved" : "denied"}${d.reason ? " · " + d.reason : ""}`));
        }
        break;
      }
      case "step_started":
        breakBlocks(); lastTool = null;
        push(el("div", "m-step", `step ${d.step} · ${d.model || ""}`));
        break;
      case "thinking": {
        if (!thinkBlock) {
          const det = el("details", "m-think");
          det.appendChild(el("summary", null, "thinking"));
          thinkBlock = el("div");
          det.appendChild(thinkBlock);
          push(det);
        }
        thinkBlock.textContent += d.delta || "";
        break;
      }
      case "assistant_text":
        if (!textBlock) { textBlock = el("div", "m-text"); push(textBlock); }
        textBlock.textContent += d.delta || "";
        break;
      case "tool_called": {
        breakBlocks();
        const card = el("div", "m-tool");
        card.appendChild(el("div", "m-tool-name", d.name || "tool"));
        card.appendChild(el("pre", "m-tool-args", fmtArgs(d.args)));
        lastTool = card;
        push(card);
        break;
      }
      case "tool_result": {
        const home = lastTool || stream;
        const det = el("details", "m-result" + (d.is_error ? " err" : ""));
        det.appendChild(el("summary", null, d.is_error ? "result · error" : "result"));
        let content = d.content || "";
        if (content.length > 8192) content = content.slice(0, 8192) + "\n…";
        det.appendChild(el("pre", null, content));
        home.appendChild(det);
        break;
      }
      case "escalated":
        push(el("div", "m-escalated", `↑ escalated to ${d.to || "?"}`));
        break;
      case "approval_requested": {
        breakBlocks();
        const card = el("div", "m-approve" + (d.risk === "destructive" ? " danger" : ""));
        card.appendChild(el("div", "m-approve-head", `${d.risk || "write"} · ${d.name || "tool"}`));
        card.appendChild(el("pre", "m-tool-args", fmtArgs(d.args)));
        const row = el("div", "m-approve-row");
        const yes = el("button", "m-yes", "approve");
        const no = el("button", "m-no", "deny");
        const answer = (approved) => {
          send({ type: "approval_response", data: { approved } });
          card.remove();
          push(el("div", "m-audit", `${approved ? "approved" : "denied"} · ${d.name}`));
        };
        // A replayed journal may already carry the answer; the card only
        // stays up if no approval_resolved follows (the daemon replays in
        // order, so a stale card is retired within the same burst).
        yes.addEventListener("click", () => answer(true));
        no.addEventListener("click", () => answer(false));
        row.append(yes, no);
        card.appendChild(row);
        approvals.appendChild(card);
        break;
      }
      case "verdict_reached":
        breakBlocks();
        push(el("div", `m-verdict ${d.verdict}`, `verdict: ${d.verdict}${d.feedback ? " — " + d.feedback : ""}`));
        break;
      case "done":
        breakBlocks();
        approvals.innerHTML = "";
        push(el("div", "m-done", `done · ${d.steps ?? "?"} steps${d.text ? " — " + d.text : ""}`));
        break;
      case "_exit":
        approvals.innerHTML = "";
        if (!RUN_OK.has(d.state)) {
          push(el("div", "m-exit", `run ${(d.state || "").replace("_", " ")}`));
          if (d.error) push(el("pre", "m-exit-err", d.error.trim()));
        }
        break;
      case "_error":
        note(env.error || "command error");
        break;
      case "_trimmed":
        push(el("div", "m-audit", "(history trimmed)"));
        break;
      // unknown tags are ignored: forward-compatible with newer engines
    }
  }

  function note(text) { push(el("div", "m-note", text)); }
  return { render, note };
}

// mushRunHeader is the ledger row above the stream: source, model, cost,
// branch/PR, and the actions mush offers a finished or stuck run.
function mushRunHeader(host, run, pane) {
  const el = document.createElement("div");
  el.className = "m-head";
  const line = document.createElement("div");
  line.className = "m-head-line";
  const actions = document.createElement("div");
  actions.className = "m-head-actions";
  const err = document.createElement("div");
  err.className = "m-head-err";
  err.hidden = true;
  el.append(line, actions, err);

  const btn = (label, title, fn) => {
    const b = document.createElement("button");
    b.textContent = label;
    b.title = title;
    b.addEventListener("click", async () => {
      b.disabled = true;
      try { await fn(); } catch (err) { alert(err.message); }
      b.disabled = false;
    });
    return b;
  };
  const post = (rest) => api(mushApi(host, `/${encodeURIComponent(run.id)}${rest}`), { method: "POST" });

  function update(r) {
    run = r;
    const cost = r.cost_usd ? `$${r.cost_usd.toFixed(3)}` : "";
    const tokens = r.tokens_in || r.tokens_out ? `${fmtTokens(r.tokens_in)}/${fmtTokens(r.tokens_out)}` : "";
    line.textContent = [host, (r.project || "").split("/").filter(Boolean).pop(), r.source, r.model, r.provider,
      `${r.steps || 0} steps`, tokens, cost, runDuration(r), r.verdict && r.verdict !== "-" ? `verdict ${r.verdict}` : "",
      r.branch, r.local ? "engine: this daemon" : ""].filter(Boolean).join(" · ");
    actions.innerHTML = "";
    if (r.pr_url) {
      const a = document.createElement("a");
      a.href = r.pr_url; a.target = "_blank"; a.rel = "noopener"; a.textContent = "open PR";
      actions.appendChild(a);
    }
    const live = RUN_LIVE.has(r.state);
    if (live && r.local) actions.appendChild(btn("stop", "interrupt this run", () => api(mushApi(host, `/${encodeURIComponent(r.id)}`), { method: "DELETE" })));
    if (!live) {
      actions.appendChild(btn("retry", "start a fresh run with the same task (mush run --parent)", async () => {
        const child = await post("/retry");
        pane.openMush(host, child);
      }));
      if (r.state === "failed" || r.state === "interrupted" || r.state === "blocked") {
        actions.appendChild(btn("resume", "continue from the last checkpoint (mush resume)", async () => {
          await post("/resume");
          pane.msg("resumed — the new run appears in the list shortly");
          setTimeout(() => { pane.msg(""); refreshSessions(); }, 2500);
        }));
      }
      actions.appendChild(btn("remove", "drop this run and its journal from the ledger (mush runs remove)", () => removeRun({ ...r, host })));
    }
    err.textContent = r.error || "";
    err.hidden = !r.error;
  }
  update(run);
  return { el, update };
}

function fmtTokens(n) {
  n = n || 0;
  return n >= 1000 ? `${(n / 1000).toFixed(n >= 10000 ? 0 : 1)}k` : `${n}`;
}
