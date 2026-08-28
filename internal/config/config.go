package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
)

// ServerConfig holds HTTP server configuration options.
type ServerConfig struct {
	Port               int
	Env                string // development, staging, production
	LogLevel           string // debug, info, warn, error
	CORSAllowedOrigins []string
	RateLimitRPS       int
	RateLimitBurst     int
}

// DatabaseConfig holds PostgreSQL connection configuration options.
type DatabaseConfig struct {
	URL          string
	MaxOpenConns int
	MaxIdleConns int
}

// S3Config holds S3-compatible object storage configuration options.
type S3Config struct {
	Endpoint             string
	Region               string
	Bucket               string
	AccessKeyID          string
	SecretAccessKey      string
	ForcePathStyle       bool
	PresignExpiryMinutes int
}

// Config represents the complete application configuration.
type Config struct {
	Server   ServerConfig
	Database DatabaseConfig
	S3       S3Config
}

// Load reads configuration from the environment and optional .env file,
// applies defaults, and validates the configuration.
func Load() (*Config, error) {
	// Attempt to load .env file; ignore if it doesn't exist
	_ = godotenv.Load()

	cfg := &Config{
		Server: ServerConfig{
			Port:               getEnvInt("SERVER_PORT", 8080),
			Env:                getEnv("SERVER_ENV", "development"),
			LogLevel:           getEnv("SERVER_LOG_LEVEL", "info"),
			CORSAllowedOrigins: getEnvSlice("SERVER_CORS_ALLOWED_ORIGINS", []string{"*"}),
			RateLimitRPS:       getEnvInt("RATE_LIMIT_RPS", 20),
			RateLimitBurst:     getEnvInt("RATE_LIMIT_BURST", 50),
		},
		Database: DatabaseConfig{
			URL:          getEnv("DATABASE_URL", ""),
			MaxOpenConns: getEnvInt("DATABASE_MAX_OPEN_CONNS", 25),
			MaxIdleConns: getEnvInt("DATABASE_MAX_IDLE_CONNS", 10),
		},
		S3: S3Config{
			Endpoint:             getEnv("S3_ENDPOINT", ""),
			Region:               getEnv("S3_REGION", "us-east-1"),
			Bucket:               getEnv("S3_BUCKET", ""),
			AccessKeyID:          getEnv("S3_ACCESS_KEY_ID", ""),
			SecretAccessKey:      getEnv("S3_SECRET_ACCESS_KEY", ""),
			ForcePathStyle:       getEnvBool("S3_FORCE_PATH_STYLE", false),
			PresignExpiryMinutes: getEnvInt("S3_PRESIGN_EXPIRY_MINUTES", 15),
		},
	}

	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("config validation failed: %w", err)
	}

	return cfg, nil
}

// Validate checks if the configuration values are valid.
func (c *Config) Validate() error {
	if c.Server.Port <= 0 || c.Server.Port > 65535 {
		return fmt.Errorf("SERVER_PORT must be between 1 and 65535, got %d", c.Server.Port)
	}

	validEnvs := map[string]bool{
		"development": true,
		"staging":     true,
		"production":  true,
		"test":        true,
	}
	if !validEnvs[strings.ToLower(c.Server.Env)] {
		return fmt.Errorf("SERVER_ENV must be one of [development, staging, production, test], got %q", c.Server.Env)
	}

	validLogLevels := map[string]bool{
		"debug": true,
		"info":  true,
		"warn":  true,
		"error": true,
	}
	if !validLogLevels[strings.ToLower(c.Server.LogLevel)] {
		return fmt.Errorf("SERVER_LOG_LEVEL must be one of [debug, info, warn, error], got %q", c.Server.LogLevel)
	}

	if c.Server.RateLimitRPS <= 0 {
		return fmt.Errorf("RATE_LIMIT_RPS must be greater than 0, got %d", c.Server.RateLimitRPS)
	}
	if c.Server.RateLimitBurst <= 0 {
		return fmt.Errorf("RATE_LIMIT_BURST must be greater than 0, got %d", c.Server.RateLimitBurst)
	}

	if c.Database.MaxOpenConns <= 0 {
		return fmt.Errorf("DATABASE_MAX_OPEN_CONNS must be greater than 0, got %d", c.Database.MaxOpenConns)
	}
	if c.Database.MaxIdleConns < 0 {
		return fmt.Errorf("DATABASE_MAX_IDLE_CONNS must be non-negative, got %d", c.Database.MaxIdleConns)
	}

	if c.S3.PresignExpiryMinutes <= 0 {
		return fmt.Errorf("S3_PRESIGN_EXPIRY_MINUTES must be greater than 0, got %d", c.S3.PresignExpiryMinutes)
	}

	return nil
}

func getEnv(key, defaultVal string) string {
	if val, ok := os.LookupEnv(key); ok && val != "" {
		return val
	}
	return defaultVal
}

func getEnvInt(key string, defaultVal int) int {
	if val, ok := os.LookupEnv(key); ok && val != "" {
		if intVal, err := strconv.Atoi(val); err == nil {
			return intVal
		}
	}
	return defaultVal
}

func getEnvBool(key string, defaultVal bool) bool {
	if val, ok := os.LookupEnv(key); ok && val != "" {
		if boolVal, err := strconv.ParseBool(val); err == nil {
			return boolVal
		}
	}
	return defaultVal
}

func getEnvSlice(key string, defaultVal []string) []string {
	if val, ok := os.LookupEnv(key); ok && val != "" {
		parts := strings.Split(val, ",")
		result := make([]string, 0, len(parts))
		for _, p := range parts {
			trimmed := strings.TrimSpace(p)
			if trimmed != "" {
				result = append(result, trimmed)
			}
		}
		if len(result) > 0 {
			return result
		}
	}
	return defaultVal
}
