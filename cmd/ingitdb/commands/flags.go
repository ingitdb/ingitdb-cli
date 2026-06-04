package commands

// specscore: feature/shared-cli-flags

import "github.com/spf13/cobra"

// addPathFlag adds --path flag (DB directory). Used by almost every command.
func addPathFlag(cmd *cobra.Command) {
	cmd.Flags().String("path", "", "path to the database directory (default: current directory)")
}

// addRemoteFlags adds --remote, --token, and --provider flags. Used by record
// CRUD + list collections and the SQL verbs (select, insert, update, delete).
// See spec/features/remote-repo-access for the flag grammar and the provider
// dispatch / token resolution rules.
func addRemoteFlags(cmd *cobra.Command) {
	cmd.Flags().String("remote", "",
		"remote repository, e.g. github.com/owner/repo[@branch|tag|commit] "+
			"(also accepts HTTPS URLs, git@host: SSH form, and bare aliases like 'github/owner/repo')")
	cmd.Flags().String("token", "",
		"personal access token; falls back to host-derived env vars "+
			"(e.g. GITHUB_TOKEN for github.com)")
	cmd.Flags().String("provider", "",
		"explicit provider id (github, gitlab, bitbucket) — required for unknown hosts")
}

// addFormatFlag adds --format flag with a caller-specified default.
// Used by: query (default "csv"), read record (default "yaml"), watch, migrate.
func addFormatFlag(cmd *cobra.Command, defaultValue string) {
	cmd.Flags().String("format", defaultValue, "output format")
}

// addMaterializeFlags adds the flags shared by the ci command.
func addMaterializeFlags(cmd *cobra.Command) {
	addPathFlag(cmd)
	cmd.Flags().String("views", "", "comma-separated list of views to materialize")
	cmd.Flags().Int("records-delimiter", 0,
		"write a '#-' delimiter after each record in INGR output; 0=default (enabled), 1=enabled, -1=disabled")
}

// materializeAllSentinel is the value bound to --collections / --views when the
// flag is supplied without a value (NoOptDefVal). It means "all artifacts of
// that type". It is an unlikely literal so it never collides with a real glob.
const materializeAllSentinel = "\x00all"

// addMaterializeCommandFlags registers the flags for the `materialize` command.
// Both selector flags are tri-state via NoOptDefVal: absent (no value), bare
// (NoOptDefVal sentinel = "all"), or `=<glob-list>`. Because NoOptDefVal is set,
// a list value MUST be attached with `=` (the space-separated form is treated as
// a positional argument).
func addMaterializeCommandFlags(cmd *cobra.Command) {
	addPathFlag(cmd)
	cmd.Flags().String("collections", "",
		"regenerate collection READMEs; bare flag = all, or =GLOB[,GLOB] (use '=', not a space)")
	cmd.Flags().String("views", "",
		"regenerate materialized views; bare flag = all, or =GLOB[,GLOB] (use '=', not a space)")
	cmd.Flags().Int("records-delimiter", 0,
		"write a '#-' delimiter after each record in INGR view output; 0=default (enabled), 1=enabled, -1=disabled")
	cmd.Flags().Lookup("collections").NoOptDefVal = materializeAllSentinel
	cmd.Flags().Lookup("views").NoOptDefVal = materializeAllSentinel
}
