package config

import "os"

// DefaultJWTSecret is the fallback signing key. It is intentionally obvious so
// a deployment that forgets to set JWT_SECRET can be detected at boot instead
// of silently issuing forgeable session cookies.
const DefaultJWTSecret = "change-me-in-production"

type Config struct {
	MongoURI         string
	DBName           string
	JWTSecret        string
	Port             string
	SMTPHost         string
	SMTPPort         string
	SMTPUser         string
	SMTPPassword     string
	SMTPFrom         string
	AdminEmail       string
	BaseURL          string
	OIDCIssuer       string
	OIDCClientID     string
	OIDCClientSecret string
	OIDCRedirectURL  string
}

func Load() *Config {
	return &Config{
		MongoURI:         getEnv("MONGO_URI", "mongodb://localhost:27017"),
		DBName:           getEnv("DB_NAME", "kommande"),
		JWTSecret:        getEnv("JWT_SECRET", DefaultJWTSecret),
		Port:             getEnv("PORT", "8080"),
		SMTPHost:         getEnv("SMTP_HOST", ""),
		SMTPPort:         getEnv("SMTP_PORT", "587"),
		SMTPUser:         getEnv("SMTP_USER", ""),
		SMTPPassword:     getEnv("SMTP_PASSWORD", ""),
		SMTPFrom:         getEnv("SMTP_FROM", "kommande@kommande.local"),
		AdminEmail:       getEnv("ADMIN_EMAIL", ""),
		BaseURL:          getEnv("BASE_URL", "https://kommande.internal.rayq.app"),
		OIDCIssuer:       getEnv("OIDC_ISSUER", ""),
		OIDCClientID:     getEnv("OIDC_CLIENT_ID", "kommande"),
		OIDCClientSecret: getEnv("OIDC_CLIENT_SECRET", ""),
		OIDCRedirectURL:  getEnv("OIDC_REDIRECT_URL", "https://kommande.internal.rayq.app/auth/callback"),
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
