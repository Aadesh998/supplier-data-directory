// Package config loads runtime configuration from environment variables.
package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config holds all runtime settings. Secrets come only from the environment.
type Config struct {
	Env                  string // development | test | production
	HTTPAddr             string
	DatabaseURL          string
	MigrateOnStart       bool
	DataEncryptionKey    string
	PublicSiteURL        string
	CORSAllowedOrigins   []string
	CookieSecure         bool
	SessionTTL           time.Duration
	TrustProxyHeaders    bool
	PrivacyNoticeVersion string
	ReverifyInterval     time.Duration
	MaxUploadBytes       int64

	SMTPHost     string
	SMTPPort     int
	SMTPUsername string
	SMTPPassword string
	MailFrom     string
	AdminNotify  string

	// InternalAPIToken lets the web server's own server-side requests bypass per-IP public rate limits
	// (they all come from one container IP). Never sent by browsers.
	InternalAPIToken string

	// WorkerToken authenticates discovery job workers (Bearer). Empty disables worker endpoints.
	WorkerToken string

	GooglePlacesEnabled  bool
	GooglePlacesAPIKey   string
	GooglePlacesDailyCap int
}

// IsProduction reports whether production safeguards should apply.
func (c Config) IsProduction() bool { return c.Env == "production" }

// Load reads configuration and validates it.
func Load() (Config, error) {
	c := Config{
		Env:                  get("APP_ENV", "development"),
		HTTPAddr:             get("HTTP_ADDR", ":8080"),
		DatabaseURL:          os.Getenv("DATABASE_URL"),
		MigrateOnStart:       getBool("MIGRATE_ON_START", false),
		DataEncryptionKey:    os.Getenv("DATA_ENCRYPTION_KEY"),
		PublicSiteURL:        strings.TrimRight(get("PUBLIC_SITE_URL", "http://localhost:3000"), "/"),
		CORSAllowedOrigins:   split(get("CORS_ALLOWED_ORIGINS", "http://localhost:3000")),
		SessionTTL:           getDuration("SESSION_TTL", 12*time.Hour),
		TrustProxyHeaders:    getBool("TRUST_PROXY_HEADERS", false),
		PrivacyNoticeVersion: get("PRIVACY_NOTICE_VERSION", "2026-11-v1"),
		ReverifyInterval:     getDuration("REVERIFY_INTERVAL", 365*24*time.Hour),
		MaxUploadBytes:       int64(getInt("MAX_UPLOAD_BYTES", 10<<20)),
		SMTPHost:             os.Getenv("SMTP_HOST"),
		SMTPPort:             getInt("SMTP_PORT", 587),
		SMTPUsername:         os.Getenv("SMTP_USERNAME"),
		SMTPPassword:         os.Getenv("SMTP_PASSWORD"),
		MailFrom:             get("MAIL_FROM", "Bharat Manufacturing Directory <no-reply@localhost>"),
		AdminNotify:          os.Getenv("ADMIN_NOTIFY_EMAIL"),
		WorkerToken:          os.Getenv("WORKER_TOKEN"),
		InternalAPIToken:     os.Getenv("INTERNAL_API_TOKEN"),
		GooglePlacesEnabled:  getBool("GOOGLE_PLACES_ENABLED", false),
		GooglePlacesAPIKey:   os.Getenv("GOOGLE_PLACES_API_KEY"),
		GooglePlacesDailyCap: getInt("GOOGLE_PLACES_DAILY_CAP", 100),
	}
	c.CookieSecure = getBool("COOKIE_SECURE", c.IsProduction())

	var errs []error
	if c.DatabaseURL == "" {
		errs = append(errs, errors.New("DATABASE_URL is required"))
	}
	if c.DataEncryptionKey == "" {
		if c.IsProduction() {
			errs = append(errs, errors.New("DATA_ENCRYPTION_KEY is required in production"))
		} else {
			// Fixed development key so local data stays decryptable across restarts. Never used in production.
			c.DataEncryptionKey = "ZGV2ZWxvcG1lbnQta2V5LWRvLW5vdC11c2UtaW4tcHJvZHVjdGlvbiE="
		}
	}
	if c.WorkerToken != "" && len(c.WorkerToken) < 32 {
		errs = append(errs, errors.New("WORKER_TOKEN must be at least 32 characters (openssl rand -hex 32)"))
	}
	if c.InternalAPIToken != "" && len(c.InternalAPIToken) < 32 {
		errs = append(errs, errors.New("INTERNAL_API_TOKEN must be at least 32 characters (openssl rand -hex 32)"))
	}
	if c.IsProduction() && !c.CookieSecure {
		errs = append(errs, errors.New("COOKIE_SECURE must be true in production"))
	}
	switch c.Env {
	case "development", "test", "production":
	default:
		errs = append(errs, fmt.Errorf("APP_ENV must be development, test or production, got %q", c.Env))
	}
	return c, errors.Join(errs...)
}

func get(k, def string) string {
	if v, ok := os.LookupEnv(k); ok && v != "" {
		return v
	}
	return def
}

func getBool(k string, def bool) bool {
	v, err := strconv.ParseBool(os.Getenv(k))
	if err != nil {
		return def
	}
	return v
}

func getInt(k string, def int) int {
	v, err := strconv.Atoi(os.Getenv(k))
	if err != nil {
		return def
	}
	return v
}

func getDuration(k string, def time.Duration) time.Duration {
	v, err := time.ParseDuration(os.Getenv(k))
	if err != nil {
		return def
	}
	return v
}

func split(s string) []string {
	var out []string
	for _, p := range strings.Split(s, ",") {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}
