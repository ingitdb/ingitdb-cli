package sqlflags

import (
	"testing"

	"github.com/spf13/cobra"
)

func TestRegisterAllFlags_DefinesEveryFlag(t *testing.T) {
	t.Parallel()

	cmd := &cobra.Command{Use: "test"}
	RegisterFromFlag(cmd)
	RegisterIntoFlag(cmd)
	RegisterIDFlag(cmd)
	RegisterWhereFlag(cmd)
	RegisterSetFlag(cmd)
	RegisterUnsetFlag(cmd)
	RegisterAllFlag(cmd)
	RegisterMinAffectedFlag(cmd)
	RegisterOrderByFlag(cmd)
	RegisterFieldsFlag(cmd)

	expected := []string{
		"from", "into", "id", "where", "set", "unset",
		"all", "min-affected", "order-by", "fields",
	}
	for _, name := range expected {
		if cmd.Flags().Lookup(name) == nil {
			t.Errorf("flag --%s not registered", name)
		}
	}
}

func TestRegisterWhereFlag_IsRepeatable(t *testing.T) {
	t.Parallel()
	cmd := &cobra.Command{Use: "test"}
	RegisterWhereFlag(cmd)
	flag := cmd.Flags().Lookup("where")
	if flag == nil {
		t.Fatal("--where not registered")
	}
	if flag.Value.Type() != "stringArray" {
		t.Errorf("--where should be stringArray (repeatable), got %q", flag.Value.Type())
	}
}

func TestRegisterSetUnsetFlags_AreRepeatable(t *testing.T) {
	t.Parallel()
	cmd := &cobra.Command{Use: "test"}
	RegisterSetFlag(cmd)
	RegisterUnsetFlag(cmd)
	for _, name := range []string{"set", "unset"} {
		f := cmd.Flags().Lookup(name)
		if f == nil {
			t.Fatalf("--%s not registered", name)
		}
		if f.Value.Type() != "stringArray" {
			t.Errorf("--%s should be stringArray, got %q", name, f.Value.Type())
		}
	}
}
