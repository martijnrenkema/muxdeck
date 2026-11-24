// Minimal service worker: makes muxdeck installable as a PWA and keeps the
// shell loadable. Network-first so deploys are never masked by stale caches;
// the app is useless offline anyway (it needs a live WebSocket).
const CACHE = "muxdeck-v7";
const ASSETS = [
  "/",
  "/style.css",
  "/app.js",
  "/manifest.webmanifest",
  "/vendor/xterm.js",
  "/vendor/xterm.css",
  "/vendor/addon-fit.js",
  "/vendor/addon-search.js",
  "/vendor/addon-web-links.js",
];

self.addEventListener("install", (e) => {
  e.waitUntil(caches.open(CACHE).then((c) => c.addAll(ASSETS)));
  self.skipWaiting();
});

self.addEventListener("activate", (e) => {
  e.waitUntil(
    caches.keys().then((keys) =>
      Promise.all(keys.filter((k) => k !== CACHE).map((k) => caches.delete(k)))
    )
  );
  self.clients.claim();
});

self.addEventListener("fetch", (e) => {
  const url = new URL(e.request.url);
  if (e.request.method !== "GET" || url.pathname.startsWith("/api/")) return;
  e.respondWith(
    fetch(e.request)
      .then((res) => {
        const copy = res.clone();
        caches.open(CACHE).then((c) => c.put(e.request, copy));
        return res;
      })
      .catch(() => caches.match(e.request))
  );
});

// Web Push from the daemon: agent alerts and permission requests, delivered
// while muxdeck is closed. Open windows are told to refresh so a permission
// card appears there too.
const PERM_TITLES = { allow: "Allow", deny: "Deny", always: "Always allow" };

self.addEventListener("push", (e) => {
  let m = {};
  try { m = e.data?.json() || {}; } catch {}
  const max = self.Notification?.maxActions ?? 2;
  // A question's options come with their own labels (titles).
  const all = (m.perm || m.block ? m.actions || [] : [])
    .map((a, i) => ({ action: a, title: PERM_TITLES[a] || m.titles?.[i] || a }));
  // The options of a question go on buttons only when all of them fit: two
  // out of three would look like the whole choice. Otherwise a tap opens the
  // card, which has room for every option.
  const partial = all.length > max && all.some((a) => a.action.startsWith("opt:"));
  const actions = partial ? [] : all.slice(0, max);
  if (partial) m.body = `${m.body || ""}\nTap to choose from ${all.length} options`.trim();
  e.waitUntil(Promise.all([
    closeExpired(),
    self.registration.showNotification(m.title || "muxdeck", {
      body: m.body || "",
      tag: m.tag || undefined,
      renotify: !!m.tag,
      requireInteraction: !!(m.perm || m.block),
      icon: "icons/icon-192.png",
      data: { session: m.session, perm: m.perm, block: m.block, expires: m.expires, shown: Date.now() },
      actions,
    }),
    self.clients.matchAll({ type: "window", includeUncontrolled: true })
      .then((list) => list.forEach((c) => c.postMessage({ type: "refresh" }))),
  ]));
});

// A permission request nobody waits on any more leaves its notification
// with buttons that can only fail; any later push clears those.
function closeExpired() {
  return self.registration.getNotifications().then((list) => {
    for (const n of list) if (n.data?.expires && n.data.expires < Date.now()) n.close();
  }).catch(() => {});
}

// Agent alerts are shown from here (app.js notifyAgentWaiting and push). A
// permission button answers the request without opening anything; any other
// click focuses an open muxdeck window and asks it to attach the session, or
// opens one on that session's hash.
self.addEventListener("notificationclick", (e) => {
  e.notification.close();
  const session = e.notification.data?.session;
  const perm = e.notification.data?.perm;
  const block = e.notification.data?.block;
  if (block && /^block-(allow|dismiss)$/.test(e.action || "")) {
    const tell = (body) => self.registration.showNotification("muxdeck", {
      body, tag: e.notification.tag || undefined, icon: "icons/icon-192.png", data: { session },
    });
    e.waitUntil(
      fetch(`/api/agent/blocked/${encodeURIComponent(block)}`, {
        method: "POST",
        credentials: "same-origin",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ decision: e.action === "block-allow" ? "allow" : "dismiss" }),
      }).then(async (res) => {
        if (res.status === 404) return tell("That block was already handled");
        if (res.status === 202) return tell("Approval queued: it goes in as soon as the session can take it");
        if (!res.ok) {
          const msg = await res.json().then((j) => j.error).catch(() => "");
          return tell(`Not sent${msg ? `: ${msg}` : ` (${res.status})`} — open muxdeck`);
        }
      }, () => tell("Could not reach muxdeck — open it to answer"))
    );
    return;
  }
  const pick = /^opt:(\d+)$/.exec(e.action || "");
  if (perm && (PERM_TITLES[e.action] || pick)) {
    // A tap that did not land says so, instead of vanishing with the
    // notification as if it had worked.
    const tell = (body) => self.registration.showNotification("muxdeck", {
      body, tag: e.notification.tag || undefined, icon: "icons/icon-192.png", data: { session },
    });
    e.waitUntil(
      fetch(`/api/agent/permission/${encodeURIComponent(perm)}`, {
        method: "POST",
        credentials: "same-origin",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(pick ? { decision: "answer", pick: +pick[1] } : { decision: e.action }),
      }).then((res) => {
        if (res.status === 404 || res.status === 409) return tell("That request was already answered or has expired");
        if (!res.ok) return tell(`Not sent (${res.status}) — open muxdeck to answer`);
      }, () => tell("Could not reach muxdeck — open it to answer"))
    );
    return;
  }
  e.waitUntil(
    self.clients.matchAll({ type: "window", includeUncontrolled: true }).then((list) => {
      const c = list[0];
      if (c) {
        if (session) c.postMessage({ type: "attach", session });
        return c.focus();
      }
      return self.clients.openWindow(session ? "/#" + encodeURIComponent(session) : "/");
    })
  );
});
