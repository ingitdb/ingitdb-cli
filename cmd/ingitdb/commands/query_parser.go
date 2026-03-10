package commands

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/dal-go/dalgo/dal"
)

// parseWhereExpr parses a single WHERE expression like "field>=value" into a dal.Condition.
// Supported operators: >=, <=, >, <, ==, = (= is treated as ==).
// Commas are stripped from numeric literals before parsing (e.g. "1,000,000" → "1000000").
func parseWhereExpr(s string) (dal.Condition, error) {
	operators := []string{">=", "<=", ">", "<", "==", "="}
	for _, op := range operators {
		idx := strings.Index(s, op)
		if idx < 0 {
			continue
		}
		field := strings.TrimSpace(s[:idx])
		rawVal := strings.TrimSpace(s[idx+len(op):])
		if field == "" {
			return nil, fmt.Errorf("missing field name in %q", s)
		}
		if rawVal == "" {
			return nil, fmt.Errorf("missing value in %q", s)
		}

		var dalOp dal.Operator
		switch op {
		case ">=":
			dalOp = dal.GreaterOrEqual
		case "<=":
			dalOp = dal.LessOrEqual
		case ">":
			dalOp = dal.GreaterThen
		case "<":
			dalOp = dal.LessThen
		case "==", "=":
			dalOp = dal.Equal
		}

		val := parseWhereValue(rawVal)
		return dal.WhereField(field, dalOp, val), nil
	}
	return nil, fmt.Errorf("no supported operator found in %q (use >=, <=, >, <, ==, or =)", s)
}

// parseWhereValue attempts to parse the raw string as a number (stripping commas first)
// and falls back to a plain string.
func parseWhereValue(raw string) any {
	stripped := strings.ReplaceAll(raw, ",", "")
	if f, err := strconv.ParseFloat(stripped, 64); err == nil {
		return f
	}
	return raw
}

// parseOrderBy parses a comma-separated list of field names into ORDER BY expressions.
// A leading "-" indicates descending order (e.g. "-population").
// An empty string returns an empty slice without error.
func parseOrderBy(s string) ([]dal.OrderExpression, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, nil
	}
	parts := strings.Split(s, ",")
	exprs := make([]dal.OrderExpression, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		if strings.HasPrefix(part, "-") {
			field := strings.TrimSpace(part[1:])
			if field == "" {
				return nil, fmt.Errorf("empty field name after '-' in order-by %q", s)
			}
			exprs = append(exprs, dal.DescendingField(field))
		} else {
			exprs = append(exprs, dal.AscendingField(part))
		}
	}
	return exprs, nil
}

// parseFields splits the fields string into a slice.
// "*" or empty returns nil (meaning all fields).
func parseFields(s string) []string {
	s = strings.TrimSpace(s)
	if s == "" || s == "*" {
		return nil
	}
	parts := strings.Split(s, ",")
	fields := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			fields = append(fields, p)
		}
	}
	return fields
}

// projectRecord returns a map containing only the requested fields from data.
// If fields is nil or empty, all fields are returned with $id injected.
// The special field "$id" resolves to the record's key string.
func projectRecord(data map[string]any, id string, fields []string) map[string]any {
	if len(fields) == 0 {
		result := make(map[string]any, len(data)+1)
		result["$id"] = id
		for k, v := range data {
			result[k] = v
		}
		return result
	}
	result := make(map[string]any, len(fields))
	for _, f := range fields {
		if f == "$id" {
			result["$id"] = id
		} else if v, ok := data[f]; ok {
			result[f] = v
		}
	}
	return result
}
