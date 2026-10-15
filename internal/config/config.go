package config

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Env                string
	HTTPAddr           string
	LogLevel           slog.Level
	DatabaseURL        string
	RedisURL           string
	JWTPublicKeyFile   string
	ShutdownTimeout    time.Duration
	TrustProxy         bool
	EnableDocs         bool
	CORSAllowedOrigins []string
}

// Load reads the service configuration from environment variables.
//
// ENG: Applies defaults (HTTP_ADDR=:8080, REDIS_URL, JWT_PUBLIC_KEY_FILE, ...) and
// collects every problem instead of stopping at the first one. DATABASE_URL is
// required, and in production it must not use sslmode=disable. Docs are enabled
// by default outside production.
// LAO: ອ່ານການຕັ້ງຄ່າຈາກ env ພ້ອມຄ່າເລີ່ມຕົ້ນ ແລະ ກວດສອບ — ລວມຂໍ້ຜິດພາດທັງໝົດສົ່ງຄືນເທື່ອດຽວ.
//
// @returns the config, and a joined error listing every invalid setting
func Load() (Config, error) {
	cfg := Config{
		Env:              getEnv("APP_ENV", "development"),
		HTTPAddr:         getEnv("HTTP_ADDR", ":8080"),
		DatabaseURL:      os.Getenv("DATABASE_URL"),
		RedisURL:         getEnv("REDIS_URL", "redis://localhost:6380/0"),
		JWTPublicKeyFile: getEnv("JWT_PUBLIC_KEY_FILE", "keys/jwt_public.pem"),
	}

	var errs []error
	if cfg.DatabaseURL == "" {
		errs = append(errs, errors.New("DATABASE_URL is required"))
	}

	var err error
	if cfg.ShutdownTimeout, err = time.ParseDuration(getEnv("SHUTDOWN_TIMEOUT", "10s")); err != nil {
		errs = append(errs, fmt.Errorf("SHUTDOWN_TIMEOUT: %w", err))
	}
	if err = cfg.LogLevel.UnmarshalText([]byte(getEnv("LOG_LEVEL", "info"))); err != nil {
		errs = append(errs, fmt.Errorf("LOG_LEVEL: %w", err))
	}
	if cfg.TrustProxy, err = getBool("TRUST_PROXY", false); err != nil {
		errs = append(errs, err)
	}
	if cfg.EnableDocs, err = getBool("ENABLE_DOCS", cfg.Env != "production"); err != nil {
		errs = append(errs, err)
	}
	cfg.CORSAllowedOrigins = splitList(os.Getenv("CORS_ALLOWED_ORIGINS"))

	if cfg.Env == "production" {
		if strings.Contains(cfg.DatabaseURL, "sslmode=disable") {
			errs = append(errs, errors.New("DATABASE_URL must not use sslmode=disable in production"))
		}
	}

	return cfg, errors.Join(errs...)
}

// getEnv reads an environment variable, falling back to a default when it is empty.
//
// ENG: The value is trimmed because make keeps trailing spaces from .env lines.
// LAO: ອ່ານຕົວແປ env (ຕັດຍະຫວ່າງອອກ) — ຖ້າວ່າງຈະໃຊ້ຄ່າເລີ່ມຕົ້ນ.
//
// @param key - environment variable name
// @param fallback - value used when the variable is unset or blank
// @returns the trimmed value or the fallback
func getEnv(key, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return fallback
}

// getBool reads a boolean environment variable.
//
// ENG: Accepts anything strconv.ParseBool accepts (true/false, 1/0, ...).
// Returns the fallback when the variable is unset or blank.
// LAO: ອ່ານຕົວແປ env ແບບ true/false — ຖ້າວ່າງຈະໃຊ້ຄ່າເລີ່ມຕົ້ນ.
//
// @param key - environment variable name
// @param fallback - value used when the variable is unset or blank
// @returns the parsed bool, or an error if the value is not a valid boolean
func getBool(key string, fallback bool) (bool, error) {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return fallback, nil
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		return false, fmt.Errorf("%s must be true or false", key)
	}
	return b, nil
}

// splitList splits a comma-separated string into a list.
//
// ENG: Trims each item and drops empty ones, so "a, ,b," gives ["a", "b"].
// LAO: ແຍກສະຕຣິງທີ່ຂັ້ນດ້ວຍ comma ເປັນລາຍການ ແລະ ຕັດລາຍການທີ່ວ່າງອອກ.
//
// @param s - comma-separated values
// @returns the non-empty, trimmed items (nil when there are none)
func splitList(s string) []string {
	var out []string
	for _, part := range strings.Split(s, ",") {
		if p := strings.TrimSpace(part); p != "" {
			out = append(out, p)
		}
	}
	return out
}
