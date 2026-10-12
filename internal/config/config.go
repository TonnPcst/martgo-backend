package config

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"time"
)

type Config struct {
	Env             string
	HTTPAddr        string
	LogLevel        slog.Level
	DatabaseURL     string
	JWTSecret       string
	JWTTTL          time.Duration
	ShutdownTimeout time.Duration
	TrustProxy      bool
	EnableDocs      bool
}

func Load() (Config, error) {
	cfg := Config{
		Env:         getEnv("APP_ENV", "development"),
		HTTPAddr:    getEnv("HTTP_ADDR", ":8080"),
		DatabaseURL: os.Getenv("DATABASE_URL"),
		JWTSecret:   os.Getenv("JWT_SECRET"),
	}

	var errs []error
	if cfg.DatabaseURL == "" {
		errs = append(errs, errors.New("DATABASE_URL is required"))
	}
	if len(cfg.JWTSecret) < 32 {
		errs = append(errs, errors.New("JWT_SECRET must be at least 32 characters"))
	}

	var err error
	if cfg.JWTTTL, err = time.ParseDuration(getEnv("JWT_TTL", "24h")); err != nil {
		errs = append(errs, fmt.Errorf("JWT_TTL: %w", err))
	}
	if cfg.ShutdownTimeout, err = time.ParseDuration(getEnv("SHUTDOWN_TIMEOUT", "10s")); err != nil {
		errs = append(errs, fmt.Errorf("SHUTDOWN_TIMEOUT: %w", err))
	}
	if err = cfg.LogLevel.UnmarshalText([]byte(getEnv("LOG_LEVEL", "info"))); err != nil {
		errs = append(errs, fmt.Errorf("LOG_LEVEL: %w", err))
	}
	if cfg.TrustProxy, err = getBool("TRUST_PROXY", false); err != nil {
		errs = append(errs, err)
	}
	if cfg.EnableDocs, err = getBool("ENABLE_DOCS", true); err != nil {
		errs = append(errs, err)
	}

	return cfg, errors.Join(errs...)
}

func getEnv(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return fallback
}

func getBool(key string, fallback bool) (bool, error) {
	v, ok := os.LookupEnv(key)
	if !ok || v == "" {
		return fallback, nil
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		return false, fmt.Errorf("%s must be true or false", key)
	}
	return b, nil
}
