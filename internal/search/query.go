package search

import (
	"strconv"
	"strings"

	"github.com/blevesearch/bleve/v2"
	q "github.com/blevesearch/bleve/v2/search/query"
)

// parseQuery turns a string like "width>300 type=image" into a Bleve query.
// Numeric fields support range comparisons (> < >= <= =) while string fields
// only allow equality checks. Tokens prefixed with a hyphen (e.g. "-cat") are
// treated as exclusions.
func parseQuery(expr string) q.Query {
	tokens := strings.Fields(expr)
	must := make([]q.Query, 0, len(tokens))
	mustNot := make([]q.Query, 0)
	for _, token := range tokens {
		negative := false
		t := token
		if strings.HasPrefix(t, "-") {
			negative = true
			t = strings.TrimPrefix(t, "-")
			if t == "" {
				continue
			}
		}

		field, op, val := splitToken(t)
		var part q.Query
		if field == "" {
			part = newTagQuery(t)
		} else {
			part = buildFieldQuery(field, op, val)
		}
		if part == nil {
			continue
		}
		if negative {
			mustNot = append(mustNot, part)
		} else {
			must = append(must, part)
		}
	}
	return combineClauses(must, mustNot)
}

func newTagQuery(term string) q.Query {
	tq := bleve.NewTermQuery(term)
	tq.SetField("tags")
	return tq
}

func splitToken(token string) (field, op, val string) {
	for _, candidate := range []string{"<=", ">=", "<", ">", "="} {
		if idx := strings.Index(token, candidate); idx > 0 {
			field = token[:idx]
			op = candidate
			val = token[idx+len(candidate):]
			break
		}
	}
	return
}

func buildFieldQuery(field, op, val string) q.Query {
	if val == "" {
		return nil
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
		if qr != nil {
			qr.SetField(field)
			return qr
		}
	}
	if op == "=" {
		tq := bleve.NewTermQuery(val)
		tq.SetField(field)
		return tq
	}
	return nil
}

func combineClauses(must, mustNot []q.Query) q.Query {
	if len(mustNot) == 0 {
		switch len(must) {
		case 0:
			return bleve.NewMatchAllQuery()
		case 1:
			return must[0]
		default:
			return bleve.NewConjunctionQuery(must...)
		}
	}
	bq := bleve.NewBooleanQuery()
	if len(must) == 0 {
		bq.AddMust(bleve.NewMatchAllQuery())
	} else {
		for _, clause := range must {
			bq.AddMust(clause)
		}
	}
	for _, clause := range mustNot {
		bq.AddMustNot(clause)
	}
	return bq
}

func boolPtr(b bool) *bool { return &b }
