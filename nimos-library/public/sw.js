const CACHE = 'nimos-library-shell-v1';
const SHELL = ['/', '/manifest.webmanifest', '/logo.svg', '/logo-maskable.svg', '/favicon.svg'];
const NEVER_CACHE = ['/api/', '/content/', '/media/', '/maps/', '/mapdata/', '/catalog/'];

self.addEventListener('install', (event) => {
  event.waitUntil(caches.open(CACHE).then((cache) => cache.addAll(SHELL)));
  self.skipWaiting();
});

self.addEventListener('activate', (event) => {
  event.waitUntil(
    caches.keys()
      .then((keys) => Promise.all(keys.filter((key) => key !== CACHE).map((key) => caches.delete(key))))
      .then(() => self.clients.claim()),
  );
});

function isPrivateOrContent(pathname) {
  return NEVER_CACHE.some((prefix) => pathname === prefix.slice(0, -1) || pathname.startsWith(prefix));
}

self.addEventListener('fetch', (event) => {
  const request = event.request;
  if (request.method !== 'GET') return;
  const url = new URL(request.url);
  if (url.origin !== self.location.origin || isPrivateOrContent(url.pathname)) return;

  if (request.mode === 'navigate') {
    event.respondWith(
      fetch(request)
        .then((response) => {
          if (response.ok) caches.open(CACHE).then((cache) => cache.put('/', response.clone()));
          return response;
        })
        .catch(() => caches.match('/')),
    );
    return;
  }

  const appAsset = url.pathname.startsWith('/assets/') ||
    url.pathname.startsWith('/pdfjs/') || SHELL.includes(url.pathname);
  if (!appAsset) return;
  event.respondWith(
    caches.match(request).then((cached) => cached || fetch(request).then((response) => {
      if (response.ok && response.type === 'basic') {
        caches.open(CACHE).then((cache) => cache.put(request, response.clone()));
      }
      return response;
    })),
  );
});
