package config

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadDefaults(t *testing.T) {
	// Clear relevant environment variables
	envVars := []string{
		"SERVER_PORT", "SERVER_ENV", "SERVER_LOG_LEVEL", "SERVER_CORS_ALLOWED_ORIGINS",
		"RATE_LIMIT_RPS", "RATE_LIMIT_BURST", "DATABASE_URL", "DATABASE_MAX_OPEN_CONNS",
		"DATABASE_MAX_IDLE_CONNS", "S3_ENDPOINT", "S3_REGION", "S3_BUCKET",
		"S3_ACCESS_KEY_ID", "S3_SECRET_ACCESS_KEY", "S3_FORCE_PATH_STYLE",
		"S3_PRESIGN_EXPIRY_MINUTES",
	}
	for _, k := range envVars {
		t.Setenv(k, "")
	}

	cfg, err := Load()
	require.NoError(t, err)
	assert.Equal(t, 8080, cfg.Server.Port)
	assert.Equal(t, "development", cfg.Server.Env)
	assert.Equal(t, "info", cfg.Server.LogLevel)
	assert.Equal(t, []string{"*"}, cfg.Server.CORSAllowedOrigins)
	assert.Equal(t, 20, cfg.Server.RateLimitRPS)
	assert.Equal(t, 50, cfg.Server.RateLimitBurst)
	assert.Equal(t, 25, cfg.Database.MaxOpenConns)
	assert.Equal(t, 10, cfg.Database.MaxIdleConns)
	assert.Equal(t, "us-east-1", cfg.S3.Region)
	assert.False(t, cfg.S3.ForcePathStyle)
	assert.Equal(t, 15, cfg.S3.PresignExpiryMinutes)
}

func TestLoadCustomValues(t *testing.T) {
	t.Setenv("SERVER_PORT", "9090")
	t.Setenv("SERVER_ENV", "production")
	t.Setenv("SERVER_LOG_LEVEL", "debug")
	t.Setenv("SERVER_CORS_ALLOWED_ORIGINS", "https://example.com, https://jobs.example.com")
	t.Setenv("RATE_LIMIT_RPS", "100")
	t.Setenv("RATE_LIMIT_BURST", "200")
	t.Setenv("DATABASE_URL", "postgres://user:pass@localhost:5432/testdb")
	t.Setenv("DATABASE_MAX_OPEN_CONNS", "50")
	t.Setenv("DATABASE_MAX_IDLE_CONNS", "20")
	t.Setenv("S3_ENDPOINT", "http://minio:9000")
	t.Setenv("S3_REGION", "auto")
	t.Setenv("S3_BUCKET", "my-bucket")
	t.Setenv("S3_ACCESS_KEY_ID", "key123")
	t.Setenv("S3_SECRET_ACCESS_KEY", "secret123")
	t.Setenv("S3_FORCE_PATH_STYLE", "true")
	t.Setenv("S3_PRESIGN_EXPIRY_MINUTES", "30")

	cfg, err := Load()
	require.NoError(t, err)
	assert.Equal(t, 9090, cfg.Server.Port)
	assert.Equal(t, "production", cfg.Server.Env)
	assert.Equal(t, "debug", cfg.Server.LogLevel)
	assert.Equal(t, []string{"https://example.com", "https://jobs.example.com"}, cfg.Server.CORSAllowedOrigins)
	assert.Equal(t, 100, cfg.Server.RateLimitRPS)
	assert.Equal(t, 200, cfg.Server.RateLimitBurst)
	assert.Equal(t, "postgres://user:pass@localhost:5432/testdb", cfg.Database.URL)
	assert.Equal(t, 50, cfg.Database.MaxOpenConns)
	assert.Equal(t, 20, cfg.Database.MaxIdleConns)
	assert.Equal(t, "http://minio:9000", cfg.S3.Endpoint)
	assert.Equal(t, "auto", cfg.S3.Region)
	assert.Equal(t, "my-bucket", cfg.S3.Bucket)
	assert.Equal(t, "key123", cfg.S3.AccessKeyID)
	assert.Equal(t, "secret123", cfg.S3.SecretAccessKey)
	assert.True(t, cfg.S3.ForcePathStyle)
	assert.Equal(t, 30, cfg.S3.PresignExpiryMinutes)
}

func TestConfigValidation(t *testing.T) {
	tests := []struct {
		name        string
		modify      func(c *Config)
		expectError bool
	}{
		{
			name: "Invalid Port - zero",
			modify: func(c *Config) {
				c.Server.Port = 0
			},
			expectError: true,
		},
		{
			name: "Invalid Port - too high",
			modify: func(c *Config) {
				c.Server.Port = 70000
			},
			expectError: true,
		},
		{
			name: "Invalid Env",
			modify: func(c *Config) {
				c.Server.Env = "invalid_env"
			},
			expectError: true,
		},
		{
			name: "Invalid LogLevel",
			modify: func(c *Config) {
				c.Server.LogLevel = "verbose"
			},
			expectError: true,
		},
		{
			name: "Invalid RateLimitRPS",
			modify: func(c *Config) {
				c.Server.RateLimitRPS = 0
			},
			expectError: true,
		},
		{
			name: "Invalid RateLimitBurst",
			modify: func(c *Config) {
				c.Server.RateLimitBurst = -1
			},
			expectError: true,
		},
		{
			name: "Invalid Database MaxOpenConns",
			modify: func(c *Config) {
				c.Database.MaxOpenConns = 0
			},
			expectError: true,
		},
		{
			name: "Invalid Database MaxIdleConns",
			modify: func(c *Config) {
				c.Database.MaxIdleConns = -1
			},
			expectError: true,
		},
		{
			name: "Invalid S3 PresignExpiryMinutes",
			modify: func(c *Config) {
				c.S3.PresignExpiryMinutes = 0
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{
				Server: ServerConfig{
					Port:               8080,
					Env:                "development",
					LogLevel:           "info",
					CORSAllowedOrigins: []string{"*"},
					RateLimitRPS:       20,
					RateLimitBurst:     50,
				},
				Database: DatabaseConfig{
					MaxOpenConns: 25,
					MaxIdleConns: 10,
				},
				S3: S3Config{
					PresignExpiryMinutes: 15,
				},
			}
			tt.modify(cfg)
			err := cfg.Validate()
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestHelperFunctions(t *testing.T) {
	os.Setenv("TEST_INT_VALID", "42")
	os.Setenv("TEST_INT_INVALID", "not-a-number")
	os.Setenv("TEST_BOOL_VALID", "true")
	os.Setenv("TEST_BOOL_INVALID", "not-a-bool")
	os.Setenv("TEST_SLICE", " a , b, c ")

	defer func() {
		os.Unsetenv("TEST_INT_VALID")
		os.Unsetenv("TEST_INT_INVALID")
		os.Unsetenv("TEST_BOOL_VALID")
		os.Unsetenv("TEST_BOOL_INVALID")
		os.Unsetenv("TEST_SLICE")
	}()

	assert.Equal(t, 42, getEnvInt("TEST_INT_VALID", 10))
	assert.Equal(t, 10, getEnvInt("TEST_INT_INVALID", 10))
	assert.Equal(t, 10, getEnvInt("TEST_INT_NONEXISTENT", 10))

	assert.True(t, getEnvBool("TEST_BOOL_VALID", false))
	assert.False(t, getEnvBool("TEST_BOOL_INVALID", false))
	assert.True(t, getEnvBool("TEST_BOOL_NONEXISTENT", true))

	assert.Equal(t, []string{"a", "b", "c"}, getEnvSlice("TEST_SLICE", []string{"default"}))
	assert.Equal(t, []string{"default"}, getEnvSlice("TEST_SLICE_NONEXISTENT", []string{"default"}))
}
