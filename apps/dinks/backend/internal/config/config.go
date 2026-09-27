// Package config loads the backend configuration from the environment.
package config

import (
	"os"
	"strings"
)

// Config is the effective backend configuration. Secrets are never logged
// directly; see logging.Set.
type Config struct {
	MongoURI           string
	MongoDB            string
	Port               string
	BaseURL            string
	OIDCIssuer         string
	OIDCClientID       string
	OIDCMobileClientID string
	OIDCClientSecret   string
	OIDCRedirectURL    string
	// DevAuth accepts every request as a local development user and disables
	// OIDC. It must never be enabled in a deployed environment.
	DevAuth bool
}

// Load reads the configuration from the environment, applying defaults suited
// to local development.
func Load() Config {
	get := func(k, d string) string {
		if v := os.Getenv(k); v != "" {
			return v
		}
		return d
	}
	return Config{
		MongoURI:           get("MONGO_URI", "mongodb://dinks:dinks@localhost:27017/dinks?replicaSet=rs0"),
		MongoDB:            get("MONGO_DB", "dinks"),
		Port:               get("PORT", "8080"),
		BaseURL:            get("BASE_URL", "http://localhost:8080"),
		OIDCIssuer:         os.Getenv("OIDC_ISSUER"),
		OIDCClientID:       os.Getenv("OIDC_CLIENT_ID"),
		OIDCMobileClientID: get("OIDC_MOBILE_CLIENT_ID", "dinks-mobile"),
		OIDCClientSecret:   os.Getenv("OIDC_CLIENT_SECRET"),
		OIDCRedirectURL:    os.Getenv("OIDC_REDIRECT_URL"),
		DevAuth:            get("DEV_AUTH", "") == "true",
	}
}

// SecureCookies reports whether session cookies should carry the Secure flag,
// which follows whether the app is served over TLS.
func (c Config) SecureCookies() bool {
	return strings.HasPrefix(c.BaseURL, "https://")
}
