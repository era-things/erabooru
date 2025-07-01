package search

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"era/booru/ent"

	"github.com/blevesearch/bleve/v2"
	q "github.com/blevesearch/bleve/v2/search/query"
)

// parseQuery turns a string like "width>300 type=image" into a Bleve query.
// Numeric fields support range comparisons (> < >= <= =) while string fields
// only allow equality checks.
func boolPtr(b bool) *bool { return &b }

func parseQuery(expr string) q.Query {
	tokens := strings.Fields(expr)
	parts := make([]q.Query, 0, len(tokens))
	for _, t := range tokens {
		var field, op, val string
		for _, o := range []string{"<=", ">=", "<", ">", "="} {
			if idx := strings.Index(t, o); idx > 0 {
				field = t[:idx]
				op = o
				val = t[idx+len(o):]
				break
			}
		}
		if field == "" {
			// If no field/op, treat as tag search
			tq := bleve.NewTermQuery(t)
			tq.SetField("tags")
			parts = append(parts, tq)
			continue
		}

		if n, err := strconv.ParseFloat(val, 64); err == nil {
			var qr *q.NumericRangeQuery
			switch op {
			case "=":
				qr = bleve.NewNumericRangeInclusiveQuery(&n, &n, boolPtr(true), boolPtr(true))
			case ">":
				qr = bleve.NewNumericRangeInclusiveQuery(&n, nil, boolPtr(false), nil)
			case ">=":
				qr = bleve.NewNumericRangeInclusiveQuery(&n, nil, boolPtr(true), nil)
			case "<":
				qr = bleve.NewNumericRangeInclusiveQuery(nil, &n, nil, boolPtr(false))
			case "<=":
				qr = bleve.NewNumericRangeInclusiveQuery(nil, &n, nil, boolPtr(true))
			}
			qr.SetField(field)
			parts = append(parts, qr)
		} else if op == "=" {
			tq := bleve.NewTermQuery(val)
			tq.SetField(field)
			parts = append(parts, tq)
		}
	}

	switch len(parts) {
	case 0:
		return bleve.NewMatchAllQuery()
	case 1:
		return parts[0]
	default:
		return bleve.NewConjunctionQuery(parts...)
	}
}

// SearchMedia executes a query against the Bleve index and returns the matching
// Media documents. It does not touch the Postgres database.
func SearchMedia(expr string, limit, offset int) ([]*ent.Media, int, error) {
	if IDX == nil {
		return nil, 0, fmt.Errorf("index not open")
	}
	query := parseQuery(expr)
	log.Printf("search query: %s", expr)
	req := bleve.NewSearchRequestOptions(query, limit, offset, false)
	req.Fields = []string{"*"}
	res, err := IDX.Search(req)
	if err != nil {
		return nil, 0, err
	}
	items := make([]*ent.Media, 0, len(res.Hits))

	for _, hit := range res.Hits {
		var out struct {
			ent.Media
			Dates map[string]string `json:"dates"`
		}
		b, err := json.Marshal(hit.Fields)
		//log.Printf("search hit: %s", string(b))
		if err != nil {
			return nil, 0, err
		}
		if err := json.Unmarshal(b, &out); err != nil {
			return nil, 0, err
		}
		m := out.Media
		if len(out.Dates) > 0 {
			m.Edges.Dates = make([]*ent.Date, 0, len(out.Dates))
			for name, val := range out.Dates {
				t, _ := time.Parse("2006-01-02", val)
				m.Edges.Dates = append(m.Edges.Dates, &ent.Date{
					Name:  name,
					Edges: ent.DateEdges{MediaDates: []*ent.MediaDate{{Value: t}}},
				})
			}
		}
		items = append(items, &m)
	}
	return items, int(res.Total), nil
}

var IDX bleve.Index // global handle

// OpenOrCreate initialises the index at start-up.
func OpenOrCreate(path string) error {
	var err error
	// Check if path exists and is a valid Bleve index
	if _, err = os.Stat(path); os.IsNotExist(err) {
		mapping := bleve.NewIndexMapping()
		IDX, err = bleve.New(path, mapping)
		return err
	}
	// Try to open; if fails due to metadata, recreate
	IDX, err = bleve.Open(path)
	if err != nil && strings.Contains(err.Error(), "metadata missing") {
		mapping := bleve.NewIndexMapping()
		IDX, err = bleve.New(path, mapping)
	}
	return err
}

// IndexMedia indexes the media metadata in the Bleve index.
func IndexMedia(m *ent.Media) error {
	if IDX == nil {
		return fmt.Errorf("index not open")
	}
	log.Printf("indexing media %s", m.ID)
	doc := struct {
		ent.Media
		Tags  []string          `json:"tags"`
		Dates map[string]string `json:"dates"`
	}{Media: *m}
	if m.Edges.Tags != nil {
		doc.Tags = make([]string, len(m.Edges.Tags))
		for i, t := range m.Edges.Tags {
			doc.Tags[i] = t.Name
		}
	}
	if m.Edges.Dates != nil {
		doc.Dates = make(map[string]string, len(m.Edges.Dates))
		for _, d := range m.Edges.Dates {
			if len(d.Edges.MediaDates) > 0 {
				v := d.Edges.MediaDates[0].Value.Format("2006-01-02")
				doc.Dates[d.Name] = v
			}
		}
	}
	return IDX.Index(string(m.ID), doc)
}

// DeleteMedia removes the document from the Bleve index.
func DeleteMedia(id string) error {
	if IDX == nil {
		return fmt.Errorf("index not open")
	}
	return IDX.Delete(string(id))
}

// Close closes the Bleve index handle if open.
func Close() error {
	if IDX != nil {
		err := IDX.Close()
		IDX = nil
		return err
	}
	return nil
}

// IndexAllMedia indexes all media records from the database.
func IndexAllMedia(ctx context.Context, db *ent.Client) error {
	items, err := db.Media.Query().
		WithTags().
		WithDates(func(q *ent.DateQuery) { q.WithMediaDates() }).
		All(ctx)
	if err != nil {
		return err
	}
	for _, m := range items {
		if err := IndexMedia(m); err != nil {
			return err
		}
	}
	return nil
}

// Rebuild intelligently rebuilds the index - uses soft rebuild if index exists with content,
// otherwise ensures index exists and reindexes all media
func Rebuild(ctx context.Context, db *ent.Client, path string) error {
	log.Printf("Starting intelligent index rebuild at path: %s", path)

	// If index is not open, try to open/create it
	if IDX == nil {
		log.Printf("Index not open, attempting to open/create...")
		if err := OpenOrCreate(path); err != nil {
			return fmt.Errorf("failed to open/create index: %v", err)
		}
	}

	// Check if index has any content
	query := bleve.NewMatchAllQuery()
	req := bleve.NewSearchRequestOptions(query, 1, 0, false) // Just need to check if anything exists
	result, err := IDX.Search(req)

	if err != nil {
		log.Printf("Index search failed (possibly corrupted), reindexing all: %v", err)
		return IndexAllMedia(ctx, db)
	}

	if result.Total == 0 {
		log.Printf("Index is empty, reindexing all media...")
		return IndexAllMedia(ctx, db)
	}

	// Index has content, use soft rebuild
	log.Printf("Index has %d documents, performing soft rebuild...", result.Total)
	return SoftRebuild(ctx, db)
}

// SoftRebuild clears the existing index and repopulates it without removing files
func SoftRebuild(ctx context.Context, db *ent.Client) error {
	if IDX == nil {
		return fmt.Errorf("index not open")
	}

	log.Printf("Starting soft rebuild (clearing and repopulating existing index)...")

	// Clear all documents in batches to handle arbitrary amounts
	if err := clearAllDocuments(); err != nil {
		return fmt.Errorf("failed to clear existing documents: %v", err)
	}

	// Reindex all media
	log.Printf("Reindexing all media...")
	return IndexAllMedia(ctx, db)
}

// clearAllDocuments removes all documents from the index in batches
func clearAllDocuments() error {
	const batchSize = 1000

	for {
		// Get a batch of document IDs
		query := bleve.NewMatchAllQuery()
		req := bleve.NewSearchRequestOptions(query, batchSize, 0, false)
		req.Fields = []string{} // We only need IDs

		result, err := IDX.Search(req)
		if err != nil {
			return fmt.Errorf("failed to search documents: %v", err)
		}

		// If no more documents, we're done
		if len(result.Hits) == 0 {
			log.Printf("All documents cleared from index")
			break
		}

		// Create batch for deletion
		batch := IDX.NewBatch()
		for _, hit := range result.Hits {
			batch.Delete(hit.ID)
		}

		// Execute the batch deletion
		log.Printf("Deleting batch of %d documents...", len(result.Hits))
		if err := IDX.Batch(batch); err != nil {
			return fmt.Errorf("failed to execute batch deletion: %v", err)
		}

		// If we got fewer results than requested, we're done
		if len(result.Hits) < batchSize {
			log.Printf("All documents cleared from index")
			break
		}
	}

	return nil
}
