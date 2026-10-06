package commands

// specscore: feature/cli/select

import (
	"encoding/json"
	"fmt"
	"math/big"
	"reflect"
	"strconv"

	"github.com/ingitdb/ingitdb-cli/cmd/ingitdb/commands/sqlflags"
)

// evalAllWhere returns true when every condition matches the record
// (logical AND). The record's key is used when a condition's field is
// the "$id" pseudo-field.
func evalAllWhere(record map[string]any, key string, conds []sqlflags.Condition) (bool, error) {
	for _, c := range conds {
		ok, err := evalWhere(record, key, c)
		if err != nil {
			return false, err
		}
		if !ok {
			return false, nil
		}
	}
	return true, nil
}

// evalWhere returns true when the record matches a single condition.
func evalWhere(record map[string]any, key string, c sqlflags.Condition) (bool, error) {
	lhs, present := resolveField(record, key, c.Field)
	switch c.Op {
	case sqlflags.OpLooseEq:
		if !present {
			return false, nil
		}
		return looseEqual(lhs, c.Value), nil
	case sqlflags.OpStrictEq:
		if !present {
			return false, nil
		}
		return strictEqual(lhs, c.Value), nil
	case sqlflags.OpLooseNeq:
		if !present {
			return true, nil
		}
		return !looseEqual(lhs, c.Value), nil
	case sqlflags.OpStrictNeq:
		if !present {
			return true, nil
		}
		return !strictEqual(lhs, c.Value), nil
	case sqlflags.OpGt, sqlflags.OpLt, sqlflags.OpGte, sqlflags.OpLte:
		if !present {
			return false, nil
		}
		return compareOrdered(lhs, c.Value, c.Op)
	default:
		return false, fmt.Errorf("unsupported operator: %v", c.Op)
	}
}

// resolveField returns (value, present). The pseudo-field "$id"
// resolves to the record key.
func resolveField(record map[string]any, key, field string) (any, bool) {
	if field == "$id" {
		return key, true
	}
	v, ok := record[field]
	return v, ok
}

// strictEqual returns true only when the two operands have identical
// Go types AND identical values.
func strictEqual(a, b any) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	if fmt.Sprintf("%T", a) != fmt.Sprintf("%T", b) {
		return false
	}
	return directEqual(a, b)
}

// looseEqual returns true when the operands compare equal under
// type coercion: numeric vs numeric-parsable-string, etc.
func looseEqual(a, b any) bool {
	if directEqual(a, b) {
		return true
	}
	// A binary value is comparable only to another binary value. Formatting it
	// as text would make a string containing its Go slice representation match.
	if _, ok := a.([]byte); ok {
		return false
	}
	if _, ok := b.([]byte); ok {
		return false
	}
	if cmp, ok := compareExactNumeric(a, b); ok {
		return cmp == 0
	}
	af, aok := asFloat(a)
	bf, bok := asFloat(b)
	if aok && bok {
		return af == bf
	}
	return fmt.Sprintf("%v", a) == fmt.Sprintf("%v", b)
}

// directEqual preserves Go's direct equality for comparable values while
// avoiding a panic for imported binary values (which are []byte).
func directEqual(a, b any) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	if reflect.TypeOf(a).Comparable() && reflect.TypeOf(b).Comparable() {
		return a == b
	}
	return reflect.DeepEqual(a, b)
}

// asExactInt64 retains integer identity before any float coercion. Numeric
// strings are included for text-encoded provider values such as DECIMAL.
func asExactInt64(v any) (int64, bool) {
	switch x := v.(type) {
	case int:
		return int64(x), true
	case int64:
		return x, true
	case string:
		n, err := strconv.ParseInt(x, 10, 64)
		return n, err == nil
	default:
		return 0, false
	}
}

// compareExactNumeric avoids float64 rounding when either operand is an
// integer outside its exact range or a high-precision numeric literal.
// Ordinary loose decimal comparisons retain their existing coercion behavior.
func compareExactNumeric(a, b any) (int, bool) {
	ai, aInt := asExactInt64(a)
	bi, bInt := asExactInt64(b)
	const maxSafeInteger = int64(1<<53 - 1)
	largeA := aInt && (ai > maxSafeInteger || ai < -maxSafeInteger)
	largeB := bInt && (bi > maxSafeInteger || bi < -maxSafeInteger)
	_, exactA := a.(json.Number)
	_, exactB := b.(json.Number)
	_, aString := a.(string)
	_, bString := b.(string)
	exact := aInt && bInt || largeA || largeB || exactA || exactB || aString && bString
	if !exact {
		return 0, false
	}
	ar, aOK := asExactNumericRat(a)
	br, bOK := asExactNumericRat(b)
	if !aOK || !bOK {
		return 0, false
	}
	return ar.Cmp(br), true
}

func asExactNumericRat(v any) (*big.Rat, bool) {
	switch x := v.(type) {
	case int:
		return new(big.Rat).SetInt64(int64(x)), true
	case int64:
		return new(big.Rat).SetInt64(x), true
	case float32:
		r := new(big.Rat).SetFloat64(float64(x))
		return r, r != nil
	case float64:
		r := new(big.Rat).SetFloat64(x)
		return r, r != nil
	case string:
		return new(big.Rat).SetString(x)
	case json.Number:
		return new(big.Rat).SetString(string(x))
	default:
		return nil, false
	}
}

// asFloat tries to coerce v to a float64. Returns the value and a
// success flag. Booleans are NOT coerced (true != 1.0 for our spec).
func asFloat(v any) (float64, bool) {
	switch x := v.(type) {
	case float64:
		return x, true
	case float32:
		return float64(x), true
	case int:
		return float64(x), true
	case int64:
		return float64(x), true
	case string:
		f, err := strconv.ParseFloat(x, 64)
		return f, err == nil
	default:
		return 0, false
	}
}

// compareValues returns -1, 0, +1 comparing a and b. Numeric comparison
// is preferred when both can coerce; otherwise lexicographic on the
// fmt-formatted strings.
func compareValues(a, b any) int {
	if cmp, ok := compareExactNumeric(a, b); ok {
		return cmp
	}
	af, aok := asFloat(a)
	bf, bok := asFloat(b)
	if aok && bok {
		switch {
		case af < bf:
			return -1
		case af > bf:
			return 1
		default:
			return 0
		}
	}
	as := fmt.Sprintf("%v", a)
	bs := fmt.Sprintf("%v", b)
	switch {
	case as < bs:
		return -1
	case as > bs:
		return 1
	default:
		return 0
	}
}

// compareOrdered evaluates >, <, >=, <= using numeric coercion when
// possible, falling back to lexicographic string comparison.
func compareOrdered(lhs, rhs any, op sqlflags.Operator) (bool, error) {
	if _, ok := lhs.([]byte); ok {
		return false, fmt.Errorf("ordered comparison of binary values is unsupported")
	}
	if _, ok := rhs.([]byte); ok {
		return false, fmt.Errorf("ordered comparison of binary values is unsupported")
	}
	if cmp, ok := compareExactNumeric(lhs, rhs); ok {
		switch op {
		case sqlflags.OpGt:
			return cmp > 0, nil
		case sqlflags.OpLt:
			return cmp < 0, nil
		case sqlflags.OpGte:
			return cmp >= 0, nil
		case sqlflags.OpLte:
			return cmp <= 0, nil
		}
	}
	lf, lok := asFloat(lhs)
	rf, rok := asFloat(rhs)
	if lok && rok {
		switch op {
		case sqlflags.OpGt:
			return lf > rf, nil
		case sqlflags.OpLt:
			return lf < rf, nil
		case sqlflags.OpGte:
			return lf >= rf, nil
		case sqlflags.OpLte:
			return lf <= rf, nil
		}
	}
	ls := fmt.Sprintf("%v", lhs)
	rs := fmt.Sprintf("%v", rhs)
	switch op {
	case sqlflags.OpGt:
		return ls > rs, nil
	case sqlflags.OpLt:
		return ls < rs, nil
	case sqlflags.OpGte:
		return ls >= rs, nil
	case sqlflags.OpLte:
		return ls <= rs, nil
	}
	return false, fmt.Errorf("compareOrdered: unsupported op %v", op)
}
