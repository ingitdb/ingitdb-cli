package commands

import (
	"fmt"

	"github.com/dal-go/dalgo/dal"
	"github.com/spf13/cobra"

	"github.com/ingitdb/ingitdb-cli/pkg/ingitdb"
)

// Serve returns the serve command.
func Serve(
	homeDir func() (string, error),
	getWd func() (string, error),
	readDefinition func(string, ...ingitdb.ReadOption) (*ingitdb.Definition, error),
	newDB func(string, *ingitdb.Definition) (dal.DB, error),
	logf func(...any),
) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "serve",
		Short: "Start one or more servers (MCP, HTTP API, watcher)",
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := cmd.Context()
			mcpFlag, _ := cmd.Flags().GetBool("mcp")
			if mcpFlag {
				dirPath, err := resolveDBPath(cmd, homeDir, getWd)
				if err != nil {
					return err
				}
				return serveMCP(ctx, dirPath, readDefinition, newDB, logf)
			}
			httpFlag, _ := cmd.Flags().GetBool("http")
			if httpFlag {
				port, _ := cmd.Flags().GetString("http-port")
				apiDomains, _ := cmd.Flags().GetStringArray("api-domains")
				mcpDomains, _ := cmd.Flags().GetStringArray("mcp-domains")
				return serveHTTP(ctx, port, apiDomains, mcpDomains, logf)
			}
			return fmt.Errorf("no server mode specified; use --mcp, --http, or --watcher")
		},
	}
	addPathFlag(cmd)
	cmd.Flags().Bool("mcp", false, "enable MCP server over stdio")
	cmd.Flags().Bool("http", false, "enable HTTP API server")
	cmd.Flags().String("http-port", "8080", "port for HTTP server")
	cmd.Flags().StringArray("api-domains", nil, "domains that route to the API handler")
	cmd.Flags().StringArray("mcp-domains", nil, "domains that route to the MCP handler")
	cmd.Flags().Bool("watcher", false, "enable file watcher")
	return cmd
}
