package main

import (
	"container/list"
	"net/http"
	"sync"
	"time"
)

// lruCache: cache LRU con TTL de respuestas JSON ya serializadas (bytes). Se usa
// para /api/search y /api/suggest → repetir una búsqueda es instantáneo y no
// golpea el motor ni consume slot del gate (mata el "frío" de Xapian, DESIGN §1.4).
// TTL corto por si la biblioteca cambia en caliente (--monitorLibrary).
type lruCache struct {
	mu  sync.Mutex
	max int
	ttl time.Duration
	ll  *list.List               // orden de uso (front = más reciente)
	m   map[string]*list.Element // clave → elemento
}

type cacheEntry struct {
	key     string
	data    []byte
	expires time.Time
}

func newLRUCache(max int, ttl time.Duration) *lruCache {
	return &lruCache{max: max, ttl: ttl, ll: list.New(), m: make(map[string]*list.Element)}
}

func (c *lruCache) get(key string) ([]byte, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	el, ok := c.m[key]
	if !ok {
		return nil, false
	}
	ent := el.Value.(*cacheEntry)
	if time.Now().After(ent.expires) {
		c.ll.Remove(el)
		delete(c.m, key)
		return nil, false
	}
	c.ll.MoveToFront(el)
	return ent.data, true
}

func (c *lruCache) set(key string, data []byte) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if el, ok := c.m[key]; ok {
		ent := el.Value.(*cacheEntry)
		ent.data = data
		ent.expires = time.Now().Add(c.ttl)
		c.ll.MoveToFront(el)
		return
	}
	el := c.ll.PushFront(&cacheEntry{key: key, data: data, expires: time.Now().Add(c.ttl)})
	c.m[key] = el
	for c.ll.Len() > c.max {
		back := c.ll.Back()
		if back == nil {
			break
		}
		c.ll.Remove(back)
		delete(c.m, back.Value.(*cacheEntry).key)
	}
}

// writeCachedJSON escribe bytes JSON ya serializados (respuesta de cache o recién
// generada) con la misma cabecera que writeJSON pero sin re-marshalar.
func writeCachedJSON(w http.ResponseWriter, data []byte) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Write(data)
}
