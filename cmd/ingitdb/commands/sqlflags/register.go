package sqlflags

// specscore: feature/shared-cli-flags

import "github.com/spf13/cobra"

// RegisterFromFlag adds --from. Used by select, update, delete.
func RegisterFromFlag(cmd *cobra.Command) {
	cmd.Flags().String("from", "", "collection to read or modify (set mode)")
}

// RegisterIntoFlag adds --into. Used by insert only.
func RegisterIntoFlag(cmd *cobra.Command) {
	cmd.Flags().String("into", "", "target collection for insert")
}

// RegisterIDFlag adds --id. Used by select, update, delete.
func RegisterIDFlag(cmd *cobra.Command) {
	cmd.Flags().String("id", "", "record ID in the form <collection>/<key> (single-record mode)")
}

// RegisterWhereFlag adds repeatable --where -w. Used by select,
// update, delete in set mode.
func RegisterWhereFlag(cmd *cobra.Command) {
	cmd.Flags().StringArrayP("where", "w", nil, "filter expression (repeatable): field<op>value, op is ==, ===, !=, !==, >=, <=, >, <")
}

// RegisterSetFlag adds repeatable --set. Used by update.
func RegisterSetFlag(cmd *cobra.Command) {
	cmd.Flags().StringArray("set", nil, "assignment (repeatable): field=value (YAML-inferred type)")
}

// RegisterUnsetFlag adds repeatable --unset. Used by update.
func RegisterUnsetFlag(cmd *cobra.Command) {
	cmd.Flags().StringArray("unset", nil, "fields to remove (repeatable, comma-separated within each occurrence): field1,field2")
}

// RegisterAllFlag adds --all. Used by update, delete.
func RegisterAllFlag(cmd *cobra.Command) {
	cmd.Flags().Bool("all", false, "operate on every record in the target collection (mutually exclusive with --where)")
}

// RegisterMinAffectedFlag adds --min-affected. Used by select,
// update, delete.
func RegisterMinAffectedFlag(cmd *cobra.Command) {
	cmd.Flags().Int("min-affected", 0, "exit non-zero when fewer than N records would be affected (set mode only)")
}

// RegisterOrderByFlag adds --order-by. Used by select.
func RegisterOrderByFlag(cmd *cobra.Command) {
	cmd.Flags().String("order-by", "", "comma-separated fields; prefix '-' for descending")
}

// RegisterFieldsFlag adds --fields -f. Used by select.
func RegisterFieldsFlag(cmd *cobra.Command) {
	cmd.Flags().StringP("fields", "f", "*", "fields to select: * = all, $id = record key, field1,field2 = specific fields")
}
