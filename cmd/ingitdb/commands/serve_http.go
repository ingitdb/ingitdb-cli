package commands

// specscore: feature/cli/serve

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"strings"

	"github.com/ingitdb/ingitdb-cli/server/api"
	"github.com/ingitdb/ingitdb-cli/server/auth"
	"github.com/ingitdb/ingitdb-cli/server/mcp"
)

// newHTTPHandler returns an http.Handler that dispatches requests based on the Host header.
// apiDomains specifies which hosts route to the API handler.
// mcpDomains specifies which hosts route to the MCP handler.
func newHTTPHandler(apiDomains, mcpDomains []string) http.Handler {
	authConfig := auth.LoadConfigFromEnv()
	requireAuth := requiresAuth(apiDomains)
	var apiHandler http.Handler
	var mcpHandler http.Handler
	if requireAuth {
		apiHandler = api.NewHandler()
		mcpHandler = mcp.NewHandler()
	} else {
		apiHandler = api.NewHandlerWithAuth(authConfig, false)
		mcpHandler = mcp.NewHandlerWithAuth(authConfig, false)
	}

	apiDomainMap := make(map[string]bool)
	for _, d := range apiDomains {
		apiDomainMap[d] = true
	}

	mcpDomainMap := make(map[string]bool)
	for _, d := range mcpDomains {
		mcpDomainMap[d] = true
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Try to get the original host from X-Forwarded-Host header (set by Firebase Hosting)
		// Fall back to Host header if X-Forwarded-Host is not present
		host := r.Header.Get("X-Forwarded-Host")
		if host == "" {
			host = r.Host
		}
		if h, _, err := net.SplitHostPort(host); err == nil {
			host = h
		}

		if mcpDomainMap[host] {
			mcpHandler.ServeHTTP(w, r)
		} else {
			// Route to API handler for all other requests (API domains and Firebase rewrites)
			apiHandler.ServeHTTP(w, r)
		}
	})
}

func requiresAuth(apiDomains []string) bool {
	if len(apiDomains) == 0 {
		return false
	}
	if len(apiDomains) == 1 && strings.EqualFold(strings.TrimSpace(apiDomains[0]), "localhost") {
		return false
	}
	return true
}

// serveHTTP starts the HTTP API server on port and blocks until ctx is done.
// apiDomains specifies which hosts route to the API handler.
// mcpDomains specifies which hosts route to the MCP handler.
func serveHTTP(ctx context.Context, port string, apiDomains, mcpDomains []string, logf func(...any)) error {
	_ = logf
	if requiresAuth(apiDomains) {
		authConfig := auth.LoadConfigFromEnv()
		if err := authConfig.ValidateForHTTPMode(); err != nil {
			return fmt.Errorf("invalid auth config: %w", err)
		}
	}
	addr := ":" + port
	srv := &http.Server{Addr: addr, Handler: newHTTPHandler(apiDomains, mcpDomains)}
	errCh := make(chan error, 1)
	go func() {
		errCh <- srv.ListenAndServe()
	}()
	select {
	case err := <-errCh:
		return fmt.Errorf("HTTP server error: %w", err)
	case <-ctx.Done():
		return srv.Shutdown(context.Background())
	}
}
