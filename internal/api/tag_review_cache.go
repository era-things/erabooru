package api

import (
	"container/list"
	"sync"
)

type tagReviewKey struct {
	revision                     int64
	tag, profile, filter, cursor string
}

type tagReviewEntry struct {
	key  tagReviewKey
	body []byte
}

// Each entry is an immutable serialized response. Revisions belong to the
// request's repeatable-read snapshot, so concurrent writes cannot publish a
// response under a revision from a different database state.
type tagReviewCache struct {
	mu       sync.Mutex
	capacity int
	entries  map[tagReviewKey]*list.Element
	order    *list.List
}

func newTagReviewCache(capacity int) *tagReviewCache {
	return &tagReviewCache{capacity: capacity, entries: make(map[tagReviewKey]*list.Element), order: list.New()}
}

func (c *tagReviewCache) get(key tagReviewKey) ([]byte, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	e, ok := c.entries[key]
	if !ok {
		return nil, false
	}
	c.order.MoveToFront(e)
	return e.Value.(tagReviewEntry).body, true
}

func (c *tagReviewCache) put(key tagReviewKey, body []byte) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if e, ok := c.entries[key]; ok {
		e.Value = tagReviewEntry{key, body}
		c.order.MoveToFront(e)
		return
	}
	c.entries[key] = c.order.PushFront(tagReviewEntry{key, body})
	if c.order.Len() > c.capacity {
		e := c.order.Back()
		delete(c.entries, e.Value.(tagReviewEntry).key)
		c.order.Remove(e)
	}
}
