package commands

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

// addMaterializeFlags adds the flags shared by materialize and ci commands.
func addMaterializeFlags(cmd *cobra.Command) {
	addPathFlag(cmd)
	cmd.Flags().String("views", "", "comma-separated list of views to materialize")
	cmd.Flags().Int("records-delimiter", 0,
		"write a '#-' delimiter after each record in INGR output; 0=default (enabled), 1=enabled, -1=disabled")
}
