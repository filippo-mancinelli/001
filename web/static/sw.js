// pensieri — service worker
// Strategia volutamente conservativa: l'app è server-rendered con sessioni via
// cookie, quindi NON mettiamo in cache le pagine HTML autenticate (evita di
// servire contenuti stantii o di un altro utente). Mettiamo in cache solo gli
// asset statici e mostriamo una pagina di fallback quando si è offline.

const CACHE = 'pensieri-v2';

// asset "shell" precaricati all'installazione
const PRECACHE = [
  '/static/css/retro.css',
  '/static/js/htmx.min.js',
  '/static/icons/icon-192.png',
  '/static/icons/icon-512.png',
  '/static/offline.html',
  '/manifest.webmanifest',
];

self.addEventListener('install', (event) => {
  event.waitUntil(
    caches.open(CACHE).then((cache) => cache.addAll(PRECACHE)).then(() => self.skipWaiting())
  );
});

self.addEventListener('activate', (event) => {
  event.waitUntil(
    caches.keys().then((keys) =>
      Promise.all(keys.filter((k) => k !== CACHE).map((k) => caches.delete(k)))
    ).then(() => self.clients.claim())
  );
});

self.addEventListener('fetch', (event) => {
  const req = event.request;

  // gestiamo solo GET same-origin
  if (req.method !== 'GET') return;
  const url = new URL(req.url);
  if (url.origin !== self.location.origin) return;

  // asset statici: cache-first (sono versionati/immutabili a livello di app)
  if (url.pathname.startsWith('/static/') || url.pathname === '/manifest.webmanifest') {
    event.respondWith(
      caches.match(req).then((cached) =>
        cached ||
        fetch(req).then((res) => {
          const copy = res.clone();
          caches.open(CACHE).then((c) => c.put(req, copy));
          return res;
        })
      )
    );
    return;
  }

  // navigazioni (pagine HTML): network-first, fallback offline.
  // Non mettiamo in cache la risposta per non conservare HTML autenticato.
  if (req.mode === 'navigate') {
    event.respondWith(
      fetch(req).catch(() => caches.match('/static/offline.html'))
    );
    return;
  }
});
