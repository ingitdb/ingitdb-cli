package sqlflags

import (
	"testing"

	"github.com/spf13/cobra"
)

// TestExampleVerbPipeline simulates how a verb command would consume
// sqlflags end-to-end: register flags, resolve mode, parse user input,
// enforce applicability. This is the API the verb plans depend on.
func TestExampleVerbPipeline(t *testing.T) {
	t.Parallel()

	// 1. A verb sets up its cobra command.
	cmd := &cobra.Command{Use: "select"}
	RegisterIDFlag(cmd)
	RegisterFromFlag(cmd)
	RegisterWhereFlag(cmd)
	RegisterOrderByFlag(cmd)
	RegisterFieldsFlag(cmd)
	RegisterMinAffectedFlag(cmd)

	// 2. The user invokes: select --from=countries --where='population>1,000,000' --order-by='-population' --fields='$id,name'
	if err := cmd.ParseFlags([]string{
		"--from=countries",
		"--where=population>1,000,000",
		"--order-by=-population",
		"--fields=$id,name",
	}); err != nil {
		t.Fatalf("flag parse: %v", err)
	}

	// 3. Resolve the operating mode.
	id, _ := cmd.Flags().GetString("id")
	from, _ := cmd.Flags().GetString("from")
	mode, err := ResolveMode(id, from)
	if err != nil {
		t.Fatalf("resolve mode: %v", err)
	}
	if mode != ModeFrom {
		t.Fatalf("want ModeFrom, got %v", mode)
	}

	// 4. Parse the predicates.
	whereExprs, _ := cmd.Flags().GetStringArray("where")
	if len(whereExprs) != 1 {
		t.Fatalf("expected 1 --where, got %d", len(whereExprs))
	}
	cond, err := ParseWhere(whereExprs[0])
	if err != nil {
		t.Fatalf("parse where: %v", err)
	}
	if cond.Field != "population" || cond.Op != OpGt || cond.Value != float64(1000000) {
		t.Errorf("unexpected condition: %+v", cond)
	}

	orderRaw, _ := cmd.Flags().GetString("order-by")
	orders, err := ParseOrderBy(orderRaw)
	if err != nil {
		t.Fatalf("parse order-by: %v", err)
	}
	if len(orders) != 1 || orders[0].Field != "population" || !orders[0].Descending {
		t.Errorf("unexpected order: %+v", orders)
	}

	fieldsRaw, _ := cmd.Flags().GetString("fields")
	fields := ParseFields(fieldsRaw)
	if len(fields) != 2 || fields[0] != "$id" || fields[1] != "name" {
		t.Errorf("unexpected fields: %v", fields)
	}

	// 5. Enforce applicability.
	allSupplied, _ := cmd.Flags().GetBool("all")
	whereSupplied := len(whereExprs) > 0
	if err := RejectSetModeFlags(SetModeFlags{
		WhereSupplied: whereSupplied,
		AllSupplied:   allSupplied,
	}, mode); err != nil {
		t.Errorf("applicability check failed: %v", err)
	}
}
