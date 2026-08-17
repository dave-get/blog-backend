package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/joho/godotenv"
)

type Config struct {
	Environment      string
	HTTPAddress      string
	DatabaseURL      string
	JWTSecret        string
	AccessTokenTTL   time.Duration
	ShutdownTimeout  time.Duration
	DBMaxConnections int32
	DBMinConnections int32
}

func Load() (Config, error) {
	if err := godotenv.Load(); err != nil && !errors.Is(err, os.ErrNotExist) {
		return Config{}, fmt.Errorf("load .env: %w", err)
	}

	c := Config{Environment: get("APP_ENV", "development"), HTTPAddress: get("HTTP_ADDRESS", ":8080"), DatabaseURL: os.Getenv("DATABASE_URL"), JWTSecret: os.Getenv("JWT_SECRET"), AccessTokenTTL: 15 * time.Minute, ShutdownTimeout: 10 * time.Second, DBMaxConnections: 20, DBMinConnections: 2}
	var err error
	if c.AccessTokenTTL, err = duration("ACCESS_TOKEN_TTL", c.AccessTokenTTL); err != nil {
		return Config{}, err
	}
	if c.ShutdownTimeout, err = duration("SHUTDOWN_TIMEOUT", c.ShutdownTimeout); err != nil {
		return Config{}, err
	}
	if c.DBMaxConnections, err = integer("DB_MAX_CONNECTIONS", c.DBMaxConnections); err != nil {
		return Config{}, err
	}
	if c.DBMinConnections, err = integer("DB_MIN_CONNECTIONS", c.DBMinConnections); err != nil {
		return Config{}, err
	}
	if c.DatabaseURL == "" {
		return Config{}, errors.New("DATABASE_URL is required")
	}
	if len(c.JWTSecret) < 32 {
		return Config{}, errors.New("JWT_SECRET must contain at least 32 characters")
	}
	if c.DBMinConnections > c.DBMaxConnections {
		return Config{}, errors.New("DB_MIN_CONNECTIONS cannot exceed DB_MAX_CONNECTIONS")
	}
	return c, nil
}

func get(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
func duration(key string, fallback time.Duration) (time.Duration, error) {
	value := os.Getenv(key)
	if value == "" {
		return fallback, nil
	}
	v, err := time.ParseDuration(value)
	if err != nil {
		return 0, fmt.Errorf("parse %s: %w", key, err)
	}
	return v, nil
}
func integer(key string, fallback int32) (int32, error) {
	value := os.Getenv(key)
	if value == "" {
		return fallback, nil
	}
	v, err := strconv.ParseInt(value, 10, 32)
	if err != nil {
		return 0, fmt.Errorf("parse %s: %w", key, err)
	}
	return int32(v), nil
}
