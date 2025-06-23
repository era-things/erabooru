package search

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"

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
func SearchMedia(expr string, limit int) ([]*ent.Media, error) {
	if IDX == nil {
		return nil, fmt.Errorf("index not open")
	}
	query := parseQuery(expr)
	log.Printf("search query: %s", expr)
	req := bleve.NewSearchRequestOptions(query, limit, 0, false)
	req.Fields = []string{"*"}
	res, err := IDX.Search(req)
	if err != nil {
		return nil, err
	}
	items := make([]*ent.Media, 0, len(res.Hits))

	for _, hit := range res.Hits {
		var m ent.Media
		b, err := json.Marshal(hit.Fields)
		//log.Printf("search hit: %s", string(b))
		if err != nil {
			return nil, err
		}
		if err := json.Unmarshal(b, &m); err != nil {
			return nil, err
		}
		items = append(items, &m)
	}
	return items, nil
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
	doc := struct {
		ent.Media
		Tags []string `json:"tags"`
	}{Media: *m}
	if m.Edges.Tags != nil {
		doc.Tags = make([]string, len(m.Edges.Tags))
		for i, t := range m.Edges.Tags {
			doc.Tags[i] = t.Name
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
