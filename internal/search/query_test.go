package search

import (
	"sort"
	"testing"

	"github.com/blevesearch/bleve/v2"
)

type tagDoc struct {
	Tags []string `json:"tags"`
}

func newTagIndex(t *testing.T) bleve.Index {
	t.Helper()
	mapping := bleve.NewIndexMapping()
	mapping.DefaultAnalyzer = "keyword"
	idx, err := bleve.NewMemOnly(mapping)
	if err != nil {
		t.Fatalf("failed to create index: %v", err)
	}
	t.Cleanup(func() { _ = idx.Close() })
	docs := map[string][]string{
		"cat":   {"animal", "cat"},
		"dog":   {"animal", "dog"},
		"horse": {"animal", "horse"},
	}
	for id, tags := range docs {
		if err := idx.Index(id, tagDoc{Tags: tags}); err != nil {
			t.Fatalf("failed to index %s: %v", id, err)
		}
	}
	return idx
}

func searchIDs(t *testing.T, idx bleve.Index, expr string) []string {
	t.Helper()
	query := parseQuery(expr)
	req := bleve.NewSearchRequest(query)
	res, err := idx.Search(req)
	if err != nil {
		t.Fatalf("search failed: %v", err)
	}
	ids := make([]string, 0, len(res.Hits))
	for _, hit := range res.Hits {
		ids = append(ids, hit.ID)
	}
	sort.Strings(ids)
	return ids
}

func TestParseQueryNegativeTags(t *testing.T) {
	idx := newTagIndex(t)
	ids := searchIDs(t, idx, "animal -cat -dog")
	expected := []string{"horse"}
	if len(ids) != len(expected) {
		t.Fatalf("expected %d ids, got %d (%v)", len(expected), len(ids), ids)
	}
	for i, id := range ids {
		if id != expected[i] {
			t.Fatalf("unexpected id at %d: %s", i, id)
		}
	}
}

func TestParseQueryOnlyNegative(t *testing.T) {
	idx := newTagIndex(t)
	ids := searchIDs(t, idx, "-cat")
	expected := []string{"dog", "horse"}
	if len(ids) != len(expected) {
		t.Fatalf("expected %d ids, got %d (%v)", len(expected), len(ids), ids)
	}
	for i, id := range ids {
		if id != expected[i] {
			t.Fatalf("unexpected id at %d: %s", i, id)
		}
	}
}
