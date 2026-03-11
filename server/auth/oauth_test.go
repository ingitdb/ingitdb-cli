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

// mockTransport routes requests through an in-process handler.
type mockTransport struct {
	handler http.HandlerFunc
}

func (m *mockTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	w := httptest.NewRecorder()
	m.handler(w, req)
	return w.Result(), nil
}

// errorTransport always returns the configured error from RoundTrip.
type errorTransport struct{ err error }

func (e *errorTransport) RoundTrip(*http.Request) (*http.Response, error) {
	return nil, e.err
}

// tokenSuccessHandler is a reusable handler that returns a valid access_token
// response for POST requests containing code=valid-code.
func tokenSuccessHandler(w http.ResponseWriter, r *http.Request) {
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
}

func TestExchangeCodeForToken(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		code string
		// buildConfig constructs the Config for this case. serverURL is the URL of
		// the per-subtest httptest.Server that can be used as tokenExchangeURL.
		// If nil, a default Config (no seams) is used.
		buildConfig func(serverURL string) Config
		// buildClient returns the *http.Client argument. Returning nil exercises the
		// nil-client fallback path inside ExchangeCodeForToken.
		// If nil, a mockTransport wrapping tokenSuccessHandler is used.
		buildClient func(serverURL string) *http.Client
		wantToken   string
		wantErr     bool
		errContain  string
	}{
		{
			name:      "success",
			code:      "valid-code",
			wantToken: "token123",
		},
		{
			name:       "empty code",
			code:       "   ",
			wantErr:    true,
			errContain: "code is required",
		},
		{
			name: "http status error",
			code: "valid-code",
			buildClient: func(_ string) *http.Client {
				return &http.Client{Transport: &mockTransport{handler: func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(http.StatusInternalServerError)
					_, _ = fmt.Fprintln(w, `{}`)
				}}}
			},
			wantErr:    true,
			errContain: "token exchange failed with status 500",
		},
		{
			name: "github error response",
			code: "bad-code",
			buildClient: func(_ string) *http.Client {
				return &http.Client{Transport: &mockTransport{handler: func(w http.ResponseWriter, r *http.Request) {
					w.Header().Set("Content-Type", "application/json")
					_, _ = fmt.Fprintln(w, `{"error": "bad_verification_code", "error_description": "The code passed is incorrect or expired."}`)
				}}}
			},
			wantErr:    true,
			errContain: "bad_verification_code",
		},
		{
			name: "invalid json response",
			code: "valid-code",
			buildClient: func(_ string) *http.Client {
				return &http.Client{Transport: &mockTransport{handler: func(w http.ResponseWriter, r *http.Request) {
					w.Header().Set("Content-Type", "application/json")
					_, _ = fmt.Fprintln(w, `not json`)
				}}}
			},
			wantErr:    true,
			errContain: "failed to decode token exchange response",
		},
		{
			name: "missing access token",
			code: "valid-code",
			buildClient: func(_ string) *http.Client {
				return &http.Client{Transport: &mockTransport{handler: func(w http.ResponseWriter, r *http.Request) {
					w.Header().Set("Content-Type", "application/json")
					_, _ = fmt.Fprintln(w, `{"foo": "bar"}`)
				}}}
			},
			wantErr:    true,
			errContain: "did not include access_token",
		},
		{
			// Exercises `client == nil` → `c.defaultHTTPClient != nil` branch:
			// the httpClient arg is nil, but Config.defaultHTTPClient is set.
			name: "nil httpClient uses config defaultHTTPClient",
			code: "valid-code",
			buildConfig: func(_ string) Config {
				return Config{
					GitHubClientID:     "client-id",
					GitHubClientSecret: "client-secret",
					CallbackURL:        "https://example.com/callback",
					defaultHTTPClient: &http.Client{
						Transport: &mockTransport{handler: tokenSuccessHandler},
					},
				}
			},
			buildClient: func(_ string) *http.Client { return nil },
			wantToken:   "token123",
		},
		{
			// Exercises `client == nil` → `else { client = http.DefaultClient }` branch:
			// both httpClient arg and Config.defaultHTTPClient are nil. The local
			// httptest.Server is used via tokenExchangeURL so http.DefaultClient
			// never touches the real network.
			name: "nil httpClient and nil defaultHTTPClient falls back to http.DefaultClient",
			code: "valid-code",
			buildConfig: func(serverURL string) Config {
				return Config{
					GitHubClientID:     "client-id",
					GitHubClientSecret: "client-secret",
					CallbackURL:        "https://example.com/callback",
					tokenExchangeURL:   serverURL,
					// defaultHTTPClient intentionally left nil
				}
			},
			buildClient: func(_ string) *http.Client { return nil },
			wantToken:   "token123",
		},
		{
			// Exercises the `client.Do` network-error branch via errorTransport.
			name: "transport network error",
			code: "valid-code",
			buildClient: func(_ string) *http.Client {
				return &http.Client{Transport: &errorTransport{err: fmt.Errorf("simulated network error")}}
			},
			wantErr:    true,
			errContain: "token exchange request failed",
		},
		{
			// Exercises the `http.NewRequestWithContext` error branch via invalid URL seam.
			name: "invalid token exchange URL",
			code: "valid-code",
			buildConfig: func(_ string) Config {
				return Config{
					GitHubClientID:     "client-id",
					GitHubClientSecret: "client-secret",
					CallbackURL:        "https://example.com/callback",
					tokenExchangeURL:   ":not-a-valid-url",
				}
			},
			wantErr:    true,
			errContain: "failed to build token request",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			// Each sub-test gets its own server so parallel tests don't share state.
			srv := httptest.NewServer(http.HandlerFunc(tokenSuccessHandler))
			t.Cleanup(srv.Close)

			// Build Config.
			var c Config
			if tc.buildConfig != nil {
				c = tc.buildConfig(srv.URL)
			} else {
				c = Config{
					GitHubClientID:     "client-id",
					GitHubClientSecret: "client-secret",
					CallbackURL:        "https://example.com/callback",
					tokenExchangeURL:   srv.URL,
				}
			}

			// Build client.
			var client *http.Client
			switch {
			case tc.buildClient != nil:
				client = tc.buildClient(srv.URL)
			default:
				client = &http.Client{Transport: &mockTransport{handler: tokenSuccessHandler}}
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
