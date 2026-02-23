package auth

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestAuthorizeURL(t *testing.T) {
	t.Parallel()
	c := Config{
		GitHubClientID: "client-id",
		CallbackURL:    "https://example.com/callback",
		Scopes:         []string{"scope1", "scope2"},
	}
	got := c.AuthorizeURL("state123")
	wantPrefix := "https://github.com/login/oauth/authorize?"
	if !strings.HasPrefix(got, wantPrefix) {
		t.Errorf("got %q, want prefix %q", got, wantPrefix)
	}
	if !strings.Contains(got, "client_id=client-id") {
		t.Error("missing client_id")
	}
	if !strings.Contains(got, "redirect_uri=https%3A%2F%2Fexample.com%2Fcallback") {
		t.Error("missing redirect_uri")
	}
	if !strings.Contains(got, "scope=scope1+scope2") {
		t.Error("missing scope")
	}
	if !strings.Contains(got, "state=state123") {
		t.Error("missing state")
	}
}

type mockTransport struct {
	handler http.HandlerFunc
}

func (m *mockTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	w := httptest.NewRecorder()
	m.handler(w, req)
	return w.Result(), nil
}

func TestExchangeCodeForToken(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		code       string
		handler    http.HandlerFunc
		wantToken  string
		wantErr    bool
		errContain string
	}{
		{
			name: "success",
			code: "valid-code",
			handler: func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodPost {
					w.WriteHeader(http.StatusMethodNotAllowed)
					return
				}
				body, _ := io.ReadAll(r.Body)
				values, _ := url.ParseQuery(string(body))
				if values.Get("code") != "valid-code" {
					w.WriteHeader(http.StatusBadRequest)
					return
				}
				w.Header().Set("Content-Type", "application/json")
				_, _ = fmt.Fprintln(w, `{"access_token": "token123", "token_type": "bearer"}`)
			},
			wantToken: "token123",
		},
		{
			name:       "empty code",
			code:       "   ",
			wantErr:    true,
			errContain: "code is required",
		},
		{
			name: "http error",
			code: "valid-code",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusInternalServerError)
				_, _ = fmt.Fprintln(w, `{}`)
			},
			wantErr:    true,
			errContain: "token exchange failed with status 500",
		},
		{
			name: "github error response",
			code: "bad-code",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				_, _ = fmt.Fprintln(w, `{"error": "bad_verification_code", "error_description": "The code passed is incorrect or expired."}`)
			},
			wantErr:    true,
			errContain: "bad_verification_code",
		},
		{
			name: "invalid json response",
			code: "valid-code",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				_, _ = fmt.Fprintln(w, `not json`)
			},
			wantErr:    true,
			errContain: "failed to decode token exchange response",
		},
		{
			name: "missing access token",
			code: "valid-code",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				_, _ = fmt.Fprintln(w, `{"foo": "bar"}`)
			},
			wantErr:    true,
			errContain: "did not include access_token",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			c := Config{
				GitHubClientID:     "client-id",
				GitHubClientSecret: "client-secret",
				CallbackURL:        "https://example.com/callback",
			}
			var client *http.Client
			if tc.handler != nil {
				client = &http.Client{
					Transport: &mockTransport{handler: tc.handler},
				}
			}

			token, err := c.ExchangeCodeForToken(context.Background(), tc.code, client)
			if (err != nil) != tc.wantErr {
				t.Fatalf("ExchangeCodeForToken() error = %v, wantErr %v", err, tc.wantErr)
			}
			if err != nil && tc.errContain != "" {
				if !strings.Contains(err.Error(), tc.errContain) {
					t.Errorf("error %q does not contain %q", err.Error(), tc.errContain)
				}
			}
			if token != tc.wantToken {
				t.Errorf("ExchangeCodeForToken() = %q, want %q", token, tc.wantToken)
			}
		})
	}
}
