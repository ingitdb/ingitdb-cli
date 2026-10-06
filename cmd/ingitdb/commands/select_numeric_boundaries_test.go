package commands

import (
	"encoding/json"
	"testing"

	"github.com/ingitdb/ingitdb-cli/cmd/ingitdb/commands/sqlflags"
)

func TestNumericComparisonMixedTypesAndBinary(t *testing.T) {
	t.Parallel()
	const wide = int64(9007199254740993)
	if looseEqual("[65 65 69 67]", []byte("AAEC")) {
		t.Fatal("binary value matched its printed representation")
	}
	if !looseEqual(nil, nil) {
		t.Fatal("two null values should compare equal")
	}
	if looseEqual(wide, float32(1)) || looseEqual(wide, float64(9007199254740992)) {
		t.Fatal("mixed float rounded an unsafe integer")
	}
	if looseEqual(json.Number("invalid"), true) {
		t.Fatal("invalid numeric text matched boolean")
	}
	if compareValues(float64(1.5), float64(2.5)) != -1 || compareValues(float64(2.5), float64(1.5)) != 1 {
		t.Fatal("fractional float ordering changed")
	}
	if _, err := compareOrdered("AAEC", []byte("AAEC"), sqlflags.OpGt); err == nil {
		t.Fatal("ordered text/binary comparison should fail")
	}
	if ok, err := compareOrdered(int64(2), int64(3), sqlflags.OpLte); err != nil || !ok {
		t.Fatalf("integer <= comparison = %v, %v", ok, err)
	}
}
