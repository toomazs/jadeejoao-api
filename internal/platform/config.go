package platform

import (
	"fmt"
	"os"
	"strings"
)

// Config holds every runtime setting. Configuration comes from environment
// variables only (see .env.example for the documented inventory).
type Config struct {
	Port            string
	DatabaseURL     string
	SupabaseURL     string
	SupabaseJWKSURL string
	// SupabaseSecretKey reaches the Auth Admin API. Only the server holds it:
	// it can rewrite any account in the project, so it must never be shipped
	// to a browser. Empty disables admin password changes (loudly).
	SupabaseSecretKey string

	StorageS3Endpoint  string
	StorageS3Region    string
	StorageS3AccessKey string
	StorageS3SecretKey string
	StorageBucket      string

	AdminEmails []string

	PIXKey          string
	PIXMerchantName string
	PIXMerchantCity string

	// RSVP email notifications via Resend. Optional: empty API key or empty
	// recipient list disables the feature entirely.
	ResendAPIKey string
	NotifyFrom   string
	NotifyEmails []string

	CORSOrigins []string
}

// LoadConfig reads and validates the environment. Every missing required
// variable is reported at once so a broken deploy fails loudly and completely.
func LoadConfig() (*Config, error) {
	cfg := &Config{
		Port:               getEnv("PORT", "8080"),
		DatabaseURL:        os.Getenv("DATABASE_URL"),
		SupabaseURL:        strings.TrimRight(os.Getenv("SUPABASE_URL"), "/"),
		SupabaseJWKSURL:    os.Getenv("SUPABASE_JWKS_URL"),
		SupabaseSecretKey:  os.Getenv("SUPABASE_SECRET_KEY"),
		StorageS3Endpoint:  os.Getenv("STORAGE_S3_ENDPOINT"),
		StorageS3Region:    os.Getenv("STORAGE_S3_REGION"),
		StorageS3AccessKey: os.Getenv("STORAGE_S3_ACCESS_KEY"),
		StorageS3SecretKey: os.Getenv("STORAGE_S3_SECRET_KEY"),
		StorageBucket:      getEnv("STORAGE_BUCKET", "site-media"),
		AdminEmails:        splitList(os.Getenv("ADMIN_EMAILS")),
		PIXKey:             os.Getenv("PIX_KEY"),
		PIXMerchantName:    os.Getenv("PIX_MERCHANT_NAME"),
		PIXMerchantCity:    os.Getenv("PIX_MERCHANT_CITY"),
		ResendAPIKey:       os.Getenv("RESEND_API_KEY"),
		NotifyFrom:         getEnv("RESEND_FROM", "Casamento Jade & João <onboarding@resend.dev>"),
		NotifyEmails:       splitList(os.Getenv("RSVP_NOTIFY_EMAILS")),
		CORSOrigins:        splitList(getEnv("CORS_ORIGINS", "http://localhost:5173,http://localhost:5174")),
	}

	if cfg.SupabaseJWKSURL == "" && cfg.SupabaseURL != "" {
		cfg.SupabaseJWKSURL = cfg.SupabaseURL + "/auth/v1/.well-known/jwks.json"
	}
	if cfg.StorageS3Endpoint == "" && cfg.SupabaseURL != "" {
		cfg.StorageS3Endpoint = cfg.SupabaseURL + "/storage/v1/s3"
	}

	var missing []string
	require := func(name, value string) {
		if value == "" {
			missing = append(missing, name)
		}
	}
	require("DATABASE_URL", cfg.DatabaseURL)
	require("SUPABASE_URL", cfg.SupabaseURL)
	require("STORAGE_S3_REGION", cfg.StorageS3Region)
	require("STORAGE_S3_ACCESS_KEY", cfg.StorageS3AccessKey)
	require("STORAGE_S3_SECRET_KEY", cfg.StorageS3SecretKey)
	require("PIX_KEY", cfg.PIXKey)
	require("PIX_MERCHANT_NAME", cfg.PIXMerchantName)
	require("PIX_MERCHANT_CITY", cfg.PIXMerchantCity)
	// Not required: the API serves the whole site without it, and refusing to
	// start would take the wedding site down over a feature only the two of
	// them use. It is warned about loudly at boot instead, and the password
	// change now says plainly when it is missing rather than blaming the
	// admin's password.
	if len(cfg.AdminEmails) == 0 {
		missing = append(missing, "ADMIN_EMAILS")
	}
	if len(missing) > 0 {
		return nil, fmt.Errorf("missing required environment variables: %s", strings.Join(missing, ", "))
	}
	return cfg, nil
}

// LoadDotEnv is a best-effort loader for local development: it reads KEY=VALUE
// lines from the given file and sets any variable not already present in the
// environment. Missing file is not an error; real deployments (Railway) inject
// env vars directly and never rely on this.
func LoadDotEnv(path string) {
	data, err := os.ReadFile(path)
	if err != nil {
		return
	}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		value = strings.Trim(strings.TrimSpace(value), `"'`)
		if key == "" || os.Getenv(key) != "" {
			continue
		}
		_ = os.Setenv(key, value)
	}
}

func getEnv(name, fallback string) string {
	if v := os.Getenv(name); v != "" {
		return v
	}
	return fallback
}

func splitList(raw string) []string {
	var out []string
	for _, part := range strings.Split(raw, ",") {
		if part = strings.TrimSpace(part); part != "" {
			out = append(out, part)
		}
	}
	return out
}
