package api

import "testing"

func TestTagReviewCacheKeysAndEviction(t *testing.T) {
	c := newTagReviewCache(2)
	key := tagReviewKey{revision: 1, tag: "cat", profile: "vision-a", filter: "-hidden"}
	c.put(key, []byte("first"))
	other := key
	other.tag = "dog"
	c.put(other, []byte("second"))
	if body, ok := c.get(key); !ok || string(body) != "first" {
		t.Fatal("missing cached response")
	}
	for _, mutate := range []func(*tagReviewKey){
		func(k *tagReviewKey) { k.revision++ },
		func(k *tagReviewKey) { k.profile = "vision-b" },
		func(k *tagReviewKey) { k.filter = "-other" },
		func(k *tagReviewKey) { k.cursor = "next" },
	} {
		changed := key
		mutate(&changed)
		if _, ok := c.get(changed); ok {
			t.Fatal("response reused for a different query or revision")
		}
	}
	third := key
	third.tag = "bird"
	c.put(third, []byte("third"))
	if _, ok := c.get(other); ok {
		t.Fatal("least recently used response was not evicted")
	}
	if _, ok := c.get(key); !ok {
		t.Fatal("recently used response was evicted")
	}
}
