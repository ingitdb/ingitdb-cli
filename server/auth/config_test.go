package auth

import "testing"

func TestLoadConfigFromEnv_Defaults(t *testing.T) {
	t.Setenv("INGITDB_GITHUB_OAUTH_CLIENT_ID", "")
	t.Setenv("INGITDB_GITHUB_OAUTH_CLIENT_SECRET", "")
	t.Setenv("INGITDB_GITHUB_OAUTH_CALLBACK_URL", "")
	t.Setenv("INGITDB_AUTH_COOKIE_DOMAIN", "")
	t.Setenv("INGITDB_AUTH_COOKIE_NAME", "")
	t.Setenv("INGITDB_AUTH_COOKIE_SECURE", "")
	t.Setenv("INGITDB_AUTH_API_BASE_URL", "")
	t.Setenv("INGITDB_GITHUB_OAUTH_SCOPES", "")

	cfg := LoadConfigFromEnv()
	if cfg.CookieName != defaultCookieName {
		t.Fatalf("expected default cookie name %q, got %q", defaultCookieName, cfg.CookieName)
	}
	if !cfg.CookieSecure {
		t.Fatal("expected cookie secure default true")
	}
	if len(cfg.Scopes) != 3 || cfg.Scopes[0] != "repo" || cfg.Scopes[1] != "read:org" || cfg.Scopes[2] != "read:user" {
		t.Fatalf("unexpected scopes: %#v", cfg.Scopes)
	}
}

func TestLoadConfigFromEnv_ParsesCookieSecure(t *testing.T) {
	t.Setenv("INGITDB_AUTH_COOKIE_SECURE", "false")

	cfg := LoadConfigFromEnv()
	if cfg.CookieSecure {
		t.Fatal("expected cookie secure false")
	}
}

func TestLoadConfigFromEnv_ParsesCustomScopes(t *testing.T) {
	t.Setenv("INGITDB_GITHUB_OAUTH_SCOPES", "read:user,repo read:org repo")
	cfg := LoadConfigFromEnv()
	if len(cfg.Scopes) != 3 {
		t.Fatalf("unexpected scopes length: %#v", cfg.Scopes)
	}
	if cfg.Scopes[0] != "read:user" || cfg.Scopes[1] != "repo" || cfg.Scopes[2] != "read:org" {
		t.Fatalf("unexpected scopes: %#v", cfg.Scopes)
	}
}

func TestValidateForHTTPMode(t *testing.T) {
	t.Parallel()
	cfg := Config{}
	if err := cfg.ValidateForHTTPMode(); err == nil {
		t.Fatal("expected validation error")
	}
	cfg.GitHubClientID = "id"
	cfg.GitHubClientSecret = "secret"
	cfg.CallbackURL = "https://api.ingitdb.com/auth/github/callback"
	cfg.CookieDomain = ".ingitdb.com"
	cfg.AuthAPIBaseURL = "https://api.ingitdb.com"
	if err := cfg.ValidateForHTTPMode(); err != nil {
		t.Fatalf("unexpected validation error: %v", err)
	}
}

func TestValidateForHTTPMode_Errors(t *testing.T) {
	t.Parallel()
	validCfg := Config{
		GitHubClientID:     "id",
		GitHubClientSecret: "secret",
		CallbackURL:        "https://api.ingitdb.com/auth/github/callback",
		CookieDomain:       ".ingitdb.com",
		AuthAPIBaseURL:     "https://api.ingitdb.com",
	}

	tests := []struct {
		name       string
		setup      func(*Config)
		errContain string
	}{
		{
			name:       "missing client id",
			setup:      func(c *Config) { c.GitHubClientID = "" },
			errContain: "INGITDB_GITHUB_OAUTH_CLIENT_ID is required",
		},
		{
			name:       "missing client secret",
			setup:      func(c *Config) { c.GitHubClientSecret = "" },
			errContain: "INGITDB_GITHUB_OAUTH_CLIENT_SECRET is required",
		},
		{
			name:       "missing callback url",
			setup:      func(c *Config) { c.CallbackURL = "" },
			errContain: "INGITDB_GITHUB_OAUTH_CALLBACK_URL is required",
		},
		{
			name:       "missing cookie domain",
			setup:      func(c *Config) { c.CookieDomain = "" },
			errContain: "INGITDB_AUTH_COOKIE_DOMAIN is required",
		},
		{
			name:       "missing auth api base url",
			setup:      func(c *Config) { c.AuthAPIBaseURL = "" },
			errContain: "INGITDB_AUTH_API_BASE_URL is required",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cfg := validCfg
			tc.setup(&cfg)
			err := cfg.ValidateForHTTPMode()
			if err == nil {
				t.Fatalf("expected error containing %q, got nil", tc.errContain)
			}
			if err.Error() != tc.errContain {
				t.Errorf("expected error %q, got %q", tc.errContain, err.Error())
			}
		})
	}
}
