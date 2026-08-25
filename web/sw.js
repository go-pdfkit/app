// The whole tool is four files. Take them on install and serve them from the
// cache afterwards, so the workbench opens with the network off — which is the
// point of a tool that never uploads anything.
const CACHE = 'pdf-workbench-v1';
const SHELL = ['./', './index.html', './wasm_exec.js', './main.wasm', './manifest.webmanifest'];

self.addEventListener('install', (e) => {
  e.waitUntil(caches.open(CACHE).then((c) => c.addAll(SHELL)).then(() => self.skipWaiting()));
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
    caches.match(e.request).then((hit) => hit || fetch(e.request).then((res) => {
      // Keep what was fetched, so a first visit that missed the install still
      // works offline afterwards.
      const copy = res.clone();
      caches.open(CACHE).then((c) => c.put(e.request, copy)).catch(() => {});
      return res;
    })));
});
