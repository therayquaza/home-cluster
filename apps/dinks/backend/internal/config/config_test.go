package config_test

import (
	"testing"

	"dinks/internal/config"
)

func TestLoadDefaults(t *testing.T) {
	cfg := config.Load()

	if cfg.Port != "8080" {
		t.Errorf("Port = %q, want 8080", cfg.Port)
	}
	if cfg.MongoDB != "dinks" {
		t.Errorf("MongoDB = %q, want dinks", cfg.MongoDB)
	}
	if cfg.OIDCMobileClientID != "dinks-mobile" {
		t.Errorf("OIDCMobileClientID = %q, want dinks-mobile", cfg.OIDCMobileClientID)
	}
	if cfg.DevAuth {
		t.Error("DevAuth defaulted to true, want false")
	}
}

func TestLoadReadsEnvironment(t *testing.T) {
	t.Setenv("PORT", "9090")
	t.Setenv("MONGO_DB", "other")
	t.Setenv("OIDC_ISSUER", "https://keycloak.internal/realms/home")
	t.Setenv("OIDC_CLIENT_ID", "dinks-web")
	t.Setenv("OIDC_MOBILE_CLIENT_ID", "custom-mobile")
	t.Setenv("OIDC_CLIENT_SECRET", "s3cr3t")
	t.Setenv("OIDC_REDIRECT_URL", "https://dinks.internal/auth/callback")
	t.Setenv("DEV_AUTH", "true")

	cfg := config.Load()

	if cfg.Port != "9090" || cfg.MongoDB != "other" {
		t.Errorf("port/db = %q/%q", cfg.Port, cfg.MongoDB)
	}
	if cfg.OIDCClientID != "dinks-web" || cfg.OIDCMobileClientID != "custom-mobile" {
		t.Errorf("client ids = %q/%q", cfg.OIDCClientID, cfg.OIDCMobileClientID)
	}
	if cfg.OIDCClientSecret != "s3cr3t" {
		t.Errorf("OIDCClientSecret was not read")
	}
	if !cfg.DevAuth {
		t.Error("DevAuth = false, want true")
	}
}

// An unset variable must fall back to its default rather than an empty string,
// so a typo in the ConfigMap cannot silently blank a required setting.
func TestEmptyEnvironmentValueFallsBackToDefault(t *testing.T) {
	t.Setenv("PORT", "")
	t.Setenv("OIDC_MOBILE_CLIENT_ID", "")

	cfg := config.Load()
	if cfg.Port != "8080" {
		t.Errorf("Port = %q, want the default", cfg.Port)
	}
	if cfg.OIDCMobileClientID != "dinks-mobile" {
		t.Errorf("OIDCMobileClientID = %q, want the default", cfg.OIDCMobileClientID)
	}
}

func TestDevAuthOnlyTrueForExactValue(t *testing.T) {
	for _, v := range []string{"1", "yes", "TRUE", "false", ""} {
		t.Setenv("DEV_AUTH", v)
		if got := config.Load().DevAuth; got != (v == "true") {
			t.Errorf("DEV_AUTH=%q -> DevAuth=%v", v, got)
		}
	}
}

func TestSecureCookiesFollowsBaseURL(t *testing.T) {
	for _, tc := range []struct {
		base string
		want bool
	}{
		{"https://dinks.internal.rayq.app", true},
		{"http://localhost:8080", false},
		{"", false},
	} {
		t.Setenv("BASE_URL", tc.base)
		if got := config.Load().SecureCookies(); got != tc.want {
			t.Errorf("BaseURL %q -> SecureCookies = %v, want %v", tc.base, got, tc.want)
		}
	}
}
