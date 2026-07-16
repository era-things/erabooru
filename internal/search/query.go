package search

import (
	"fmt"
	"strconv"
	"strings"
)

type clause struct {
	field    string
	op       string
	value    string
	negative bool
}

// WhereSQL compiles the small search language to a parameterized SQL expression.
// The media table must be aliased as m.
func WhereSQL(expr string, firstArg int) (string, []any) {
	parts := []string{}
	args := []any{}
	for _, clause := range parseClauses(expr) {
		part, value, ok := sqlForClause(clause, firstArg+len(args))
		if !ok {
			continue
		}
		if clause.negative {
			part = "NOT (" + part + ")"
		}
		parts = append(parts, part)
		args = append(args, value)
	}
	return strings.Join(parts, " AND "), args
}

func parseClauses(expr string) []clause {
	tokens := strings.Fields(expr)
	clauses := make([]clause, 0, len(tokens))
	for _, token := range tokens {
		negative := strings.HasPrefix(token, "-")
		token = strings.TrimPrefix(token, "-")
		if token == "" {
			continue
		}
		field, op, value := splitToken(token)
		if field == "" {
			value = token
		}
		clauses = append(clauses, clause{field: strings.ToLower(field), op: op, value: value, negative: negative})
	}
	return clauses
}

func splitToken(token string) (field, op, value string) {
	for _, candidate := range []string{"<=", ">=", "<", ">", "="} {
		if idx := strings.Index(token, candidate); idx > 0 {
			return token[:idx], candidate, token[idx+len(candidate):]
		}
	}
	return "", "", ""
}

func sqlForClause(c clause, arg int) (string, any, bool) {
	if c.value == "" {
		return "", nil, false
	}
	placeholder := fmt.Sprintf("$%d", arg)
	if c.field == "" {
		return `EXISTS (SELECT 1 FROM media_tag mt JOIN tag t ON t.id = mt.tag_id WHERE mt.media_id = m.id AND t.name = ` + placeholder + `)`, c.value, true
	}
	switch c.field {
	case "format":
		return "m.format = " + placeholder, strings.ToLower(c.value), c.op == "="
	case "type":
		if c.op != "=" {
			return "", nil, false
		}
		switch strings.ToLower(c.value) {
		case "image":
			return "m.format = ANY(" + placeholder + ")", []string{"jpg", "jpeg", "png", "gif", "webp"}, true
		case "video":
			return "m.format = ANY(" + placeholder + ")", []string{"mp4", "webm", "avi", "mkv"}, true
		}
	case "width", "height", "duration":
		value, err := strconv.Atoi(c.value)
		if err != nil || !validOperator(c.op) {
			return "", nil, false
		}
		return "m." + c.field + " " + c.op + " " + placeholder, value, true
	}
	return "", nil, false
}

func validOperator(op string) bool {
	switch op {
	case "=", ">", ">=", "<", "<=":
		return true
	default:
		return false
	}
}
