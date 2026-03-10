package commands

import "github.com/spf13/cobra"

// addPathFlag adds --path flag (DB directory). Used by almost every command.
func addPathFlag(cmd *cobra.Command) {
	cmd.Flags().String("path", "", "path to the database directory (default: current directory)")
}

// addGitHubFlags adds --github and --token flags. Used by record CRUD + list collections.
func addGitHubFlags(cmd *cobra.Command) {
	cmd.Flags().String("github", "", "GitHub source as owner/repo[@branch|tag|commit]")
	cmd.Flags().String("token", "", "GitHub personal access token (or set GITHUB_TOKEN env var)")
}

// addFormatFlag adds --format flag with a caller-specified default.
// Used by: query (default "csv"), read record (default "yaml"), watch, migrate.
func addFormatFlag(cmd *cobra.Command, defaultValue string) {
	cmd.Flags().String("format", defaultValue, "output format")
}

// addCollectionFlag adds --collection flag. Pass required=true to mark it required.
// Used by: query, truncate, read collection, delete collection, delete records, docs update.
func addCollectionFlag(cmd *cobra.Command, required bool) {
	cmd.Flags().StringP("collection", "c", "", "collection ID (e.g. todo.tags)")
	if required {
		_ = cmd.MarkFlagRequired("collection")
	}
}

// addMaterializeFlags adds the flags shared by materialize and ci commands.
func addMaterializeFlags(cmd *cobra.Command) {
	addPathFlag(cmd)
	cmd.Flags().String("views", "", "comma-separated list of views to materialize")
	cmd.Flags().Int("records-delimiter", 0,
		"write a '#-' delimiter after each record in INGR output; 0=default (enabled), 1=enabled, -1=disabled")
}
