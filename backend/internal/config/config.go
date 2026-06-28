package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type Config struct {
	SecretKey              string
	Port                   string
	DatabaseURI            string
	SessionDir             string
	DataDir                string
	AuthDisabled           bool
	OIDCWellKnownEndpoint  string
	OIDCClientID           string
	OIDCClientSecret       string
	OIDCScope              string
	OIDCGroupsClaim        string
	AllowedGroups          map[string]struct{}
	DailyVerification      bool
	DailyVerificationCron  string
	AppVersion             string
	MetricsToken           string
	BackupMountRequired    bool
	HealthDegradedHTTPCode int
	HealthDeepBorgCheck    bool
	HealthMaxRunningMin    int
}

func Load() (*Config, error) {
	secret := os.Getenv("SECRET_KEY")
	if secret == "" {
		secret = os.Getenv("FLASK_SECRET_KEY")
	}
	if secret == "" {
		return nil, fmt.Errorf("SECRET_KEY environment variable is not set")
	}

	dataDir := "/app/data"
	if _, err := os.Stat("/app"); os.IsNotExist(err) {
		dataDir = filepath.Join(".", "data")
	}

	dbURI := os.Getenv("DATABASE_URI")
	if dbURI == "" {
		dbURI = "file:" + filepath.Join(dataDir, "backup-tool.db") + "?_pragma=foreign_keys(1)"
	}

	sessionDir := os.Getenv("SESSION_FILE_DIR")
	if sessionDir == "" {
		if _, err := os.Stat("/app"); err == nil {
			sessionDir = "/app/sessions"
		} else {
			sessionDir = "./sessions"
		}
	}

	allowed := map[string]struct{}{}
	for _, g := range strings.Split(os.Getenv("ALLOWED_GROUPS"), ",") {
		g = strings.TrimSpace(strings.ToLower(g))
		if g != "" {
			allowed[g] = struct{}{}
		}
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "5001"
	}

	return &Config{
		SecretKey:              secret,
		Port:                   port,
		DatabaseURI:            dbURI,
		SessionDir:             sessionDir,
		DataDir:                dataDir,
		AuthDisabled:           envBool("AUTH_DISABLED", false),
		OIDCWellKnownEndpoint:  resolveOIDCWellKnown(),
		OIDCClientID:           firstNonEmpty(os.Getenv("OIDC_CLIENT_ID"), os.Getenv("KEYCLOAK_CLIENT_ID")),
		OIDCClientSecret:       firstNonEmpty(os.Getenv("OIDC_CLIENT_SECRET"), os.Getenv("KEYCLOAK_CLIENT_SECRET")),
		OIDCScope:              resolveOIDCScope(),
		OIDCGroupsClaim:        envDefault("OIDC_GROUPS_CLAIM", "groups,roles"),
		AllowedGroups:          allowed,
		DailyVerification:      envBool("DAILY_VERIFICATION_ENABLED", true),
		DailyVerificationCron:  envDefault("DAILY_VERIFICATION_CRON", "0 3 * * *"),
		AppVersion:             envDefault("APP_VERSION", "dev"),
		MetricsToken:           os.Getenv("METRICS_TOKEN"),
		BackupMountRequired:    envBool("BACKUP_MOUNT_REQUIRED", false),
		HealthDegradedHTTPCode: envInt("HEALTH_DEGRADED_HTTP_CODE", 200),
		HealthDeepBorgCheck:    envBool("HEALTH_DEEP_BORG_CHECK", false),
		HealthMaxRunningMin:    envInt("HEALTH_MAX_RUNNING_MINUTES", 0),
	}, nil
}

func resolveOIDCWellKnown() string {
	if v := strings.TrimSpace(os.Getenv("OIDC_WELL_KNOWN_ENDPOINT")); v != "" {
		return v
	}
	if issuer := strings.TrimRight(strings.TrimSpace(os.Getenv("OIDC_ISSUER")), "/"); issuer != "" {
		return issuer + "/.well-known/openid-configuration"
	}
	kcURL := strings.TrimRight(strings.TrimSpace(os.Getenv("KEYCLOAK_URL")), "/")
	if kcURL != "" {
		realm := envDefault("KEYCLOAK_REALM", "master")
		return fmt.Sprintf("%s/realms/%s/.well-known/openid-configuration", kcURL, realm)
	}
	return "https://auth.example.com/.well-known/openid-configuration"
}

func resolveOIDCScope() string {
	scope := envDefault("OIDC_SCOPE", "openid email profile")
	parts := strings.Fields(scope)
	for _, p := range parts {
		if p == "groups" {
			return scope
		}
	}
	return scope + " groups"
}

func envDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func envBool(key string, def bool) bool {
	v := strings.ToLower(strings.TrimSpace(os.Getenv(key)))
	if v == "" {
		return def
	}
	return v == "true" || v == "1" || v == "yes"
}

func envInt(key string, def int) int {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return def
	}
	var n int
	if _, err := fmt.Sscanf(v, "%d", &n); err != nil {
		return def
	}
	return n
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}
