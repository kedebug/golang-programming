package cache

import (
	"container/list"
	"github.com/kedebug/golang-programming/15-440/P2-F11/lsplog"
	sp "github.com/kedebug/golang-programming/15-440/P2-F11/storageproto"
	"sync"
	"time"
)

type Cache struct {
	mu    sync.RWMutex
	cache map[string]*entry
}

type entry struct {
	mu      sync.Mutex
	granted bool
	ll      *list.List
	val     interface{}

	leaseTime time.Time
	leaseDur  time.Duration
}

func (e *entry) clean() {
	for {
		l := e.ll.Front()
		if l == nil {
			return
		}
		v := l.Value
		dur := time.Since(v.(time.Time))
		if dur > time.Duration(sp.QUERY_CACHE_SECONDS)*time.Second {
			e.ll.Remove(l)
		} else {
			return
		}
	}
}

func NewCache() *Cache {
	return &Cache{
		cache: make(map[string]*entry),
	}
}

func (c *Cache) Get(key string, args *sp.GetArgs) (interface{}, error) {
	c.mu.Lock()
	c.evict()

	e, ok := c.cache[key]
	if !ok {
		e = &entry{ll: list.New()}
		c.cache[key] = e
	}
	c.mu.Unlock()

	e.mu.Lock()
	defer e.mu.Unlock()

	if e.granted {
		return e.val, nil
	}

	e.ll.PushBack(time.Now())
	if e.ll.Len() > sp.QUERY_CACHE_THRESH {
		args.WantLease = true
	}

	return nil, lsplog.MakeErr("key nonexist in cache")
}

func (c *Cache) evict() {
	for k, e := range c.cache {
		e.mu.Lock()
		if e.granted {
			dur := time.Since(e.leaseTime)
			if dur > e.leaseDur {
				e.granted = false
			}
		}
		e.clean()
		if e.ll.Len() == 0 {
			delete(c.cache, k)
		}
		e.mu.Unlock()
	}
}

func (c *Cache) Revoke(key string) bool {
	c.mu.RLock()
	e, ok := c.cache[key]
	c.mu.RUnlock()

	if ok {
		e.mu.Lock()
		e.granted = false
		e.mu.Unlock()
	}

	return ok
}

func (c *Cache) Grant(key string, val interface{}, lease sp.LeaseStruct) {
	c.mu.RLock()
	e, ok := c.cache[key]
	c.mu.RUnlock()

	if !ok {
		panic("[Cache] key nonexist")
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	e.granted = true
	e.val = val
	e.leaseTime = time.Now()
	e.leaseDur = time.Duration(lease.ValidSeconds) * time.Second
}
