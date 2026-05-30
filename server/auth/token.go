package auth

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/google/go-github/v88/github"
)

// ResolveTokenFromRequest resolves token from Authorization header first, then cookie.
func ResolveTokenFromRequest(r *http.Request, cookieName string) string {
	authHeader := strings.TrimSpace(r.Header.Get("Authorization"))
	if after, ok := strings.CutPrefix(authHeader, "Bearer "); ok {
		token := strings.TrimSpace(after)
		if token != "" {
			return token
		}
	}
	if cookieName == "" {
		return ""
	}
	cookie, err := r.Cookie(cookieName)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(cookie.Value)
}

// ValidateGitHubToken validates a GitHub OAuth token by calling GET /user.
func ValidateGitHubToken(ctx context.Context, token string, httpClient *http.Client) error {
	if strings.TrimSpace(token) == "" {
		return fmt.Errorf("token is required")
	}
	opts := []github.ClientOptionsFunc{github.WithAuthToken(token)}
	if httpClient != nil {
		opts = append(opts, github.WithHTTPClient(httpClient))
	}
	// github.NewClient only returns an error when an option function fails.
	// WithAuthToken fails on empty token (already guarded above) and
	// WithHTTPClient fails on nil (only appended when httpClient != nil), so
	// this call is unconditionally successful and the error is discarded.
	client, _ := github.NewClient(opts...)
	_, _, err := client.Users.Get(ctx, "")
	if err != nil {
		return fmt.Errorf("github token validation failed: %w", err)
	}
	return nil
}
