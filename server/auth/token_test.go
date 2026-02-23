package auth

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestResolveTokenFromRequest_PrefersBearerHeader(t *testing.T) {
	t.Parallel()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer header-token")
	req.AddCookie(&http.Cookie{Name: "ingitdb_github_token", Value: "cookie-token"})

	got := ResolveTokenFromRequest(req, "ingitdb_github_token")
	if got != "header-token" {
		t.Fatalf("expected header token, got %q", got)
	}
}

func TestResolveTokenFromRequest_FallsBackToCookie(t *testing.T) {
	t.Parallel()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(&http.Cookie{Name: "ingitdb_github_token", Value: "cookie-token"})

	got := ResolveTokenFromRequest(req, "ingitdb_github_token")
	if got != "cookie-token" {
		t.Fatalf("expected cookie token, got %q", got)
	}
}

func TestValidateGitHubToken(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		token      string
		handler    http.HandlerFunc
		wantErr    bool
		errContain string
	}{
		{
			name:  "valid token",
			token: "valid-token",
			handler: func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/user" {
					w.WriteHeader(http.StatusNotFound)
					return
				}
				if r.Header.Get("Authorization") != "Bearer valid-token" {
					w.WriteHeader(http.StatusUnauthorized)
					return
				}
				w.Header().Set("Content-Type", "application/json")
				_, _ = fmt.Fprintln(w, `{"login": "octocat"}`)
			},
			wantErr: false,
		},
		{
			name:       "empty token",
			token:      "",
			wantErr:    true,
			errContain: "token is required",
		},
		{
			name:  "invalid token",
			token: "bad-token",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusUnauthorized)
				_, _ = fmt.Fprintln(w, `{"message": "Bad credentials"}`)
			},
			wantErr:    true,
			errContain: "github token validation failed",
		},
		{
			name:  "http error",
			token: "valid-token",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusInternalServerError)
			},
			wantErr:    true,
			errContain: "github token validation failed",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var client *http.Client
			if tc.handler != nil {
				client = &http.Client{
					Transport: &mockTransport{handler: tc.handler},
				}
			}
			err := ValidateGitHubToken(context.Background(), tc.token, client)
			if (err != nil) != tc.wantErr {
				t.Fatalf("ValidateGitHubToken() error = %v, wantErr %v", err, tc.wantErr)
			}
			if err != nil && tc.errContain != "" {
				if !strings.Contains(err.Error(), tc.errContain) {
					t.Errorf("error %q does not contain %q", err.Error(), tc.errContain)
				}
			}
		})
	}
}
