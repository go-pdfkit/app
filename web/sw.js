// The whole tool is four files. Keep a copy of each so the workbench opens
// with the network off — which is the point of a tool that never uploads
// anything — but ASK THE NETWORK FIRST whenever there is one.
//
// ⛔ This used to be cache-first: `caches.match(req).then(hit => hit || fetch(req))`.
// A hit therefore won forever. Three things had to be true at once for that to
// be survivable, and none of them was:
//
//   - a service worker only reinstalls when its own script changes BYTE FOR
//     BYTE, and this file had not changed since the workbench was published;
//   - the cache was named `pdf-workbench-v1`, a constant, so the `activate`
//     sweep that deletes "every cache that is not the current one" never
//     deleted anything;
//   - nothing revalidated.
//
// So every visitor who had ever opened the workbench kept the build they first
// saw, for as long as their browser kept the cache. Five releases went out —
// a page that fills the window, text drawn at the screen's own pixels, a real
// typeface, a rail of minipages — and none of them reached anybody who had
// been here before. The bug did not look like a distribution failure; it
// looked like the layout regressing, which is what it was reported as.
//
// Network-first fixes it at the root rather than at the symptom. Bumping the
// cache name alone would have shipped THIS build and left the trap armed for
// the next one.
const CACHE = 'pdf-workbench-v2';
const SHELL = ['./', './index.html', './wasm_exec.js', './main.wasm', './manifest.webmanifest'];

self.addEventListener('install', (e) => {
  e.waitUntil(
    caches.open(CACHE).then((c) => Promise.all(SHELL.map((u) =>
      // `cache: 'reload'` because an install that reads the browser's own HTTP
      // cache can lay down a copy of exactly the build we are trying to
      // replace. addAll() does not take the option, so each one is fetched and
      // put by hand.
      fetch(new Request(u, { cache: 'reload' })).then((res) => res.ok && c.put(u, res))
    ))).then(() => self.skipWaiting()));
});

self.addEventListener('activate', (e) => {
  e.waitUntil(
    caches.keys()
      .then((keys) => Promise.all(keys.filter((k) => k !== CACHE).map((k) => caches.delete(k))))
      .then(() => self.clients.claim()));
});

self.addEventListener('fetch', (e) => {
  if (e.request.method !== 'GET') return;
  e.respondWith(
    fetch(e.request)
      .then((res) => {
        // Only a real answer is worth keeping. A 404 cached as the shell is an
        // offline workbench that opens on an error page.
        if (res && res.ok) {
          const copy = res.clone();
          caches.open(CACHE).then((c) => c.put(e.request, copy)).catch(() => {});
        }
        return res;
      })
      // No network, or a network that will not answer: the copy is what it is
      // for. A request we have never seen and cannot fetch fails as it would
      // have without a worker at all.
      .catch(() => caches.match(e.request).then((hit) => hit || Promise.reject(new Error('offline and not cached')))));
});
