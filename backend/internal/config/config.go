package config

import (
	"os"
	"strconv"
	"time"

	"github.com/joho/godotenv"
)

// Config holds all runtime configuration, populated from environment variables.
// Everything has a sensible default so the service boots with zero configuration
// in development.
type Config struct {
	Port         string
	DatabaseURL  string
	JWTSecret    string
	JWTTTL       time.Duration
	StorageDir   string        // where uploaded resume files are written
	VectorDir    string        // where chromem-go persists embeddings
	MigrationDir string        // path to .sql migration files
	SeedFile     string        // roles seed file
	LLMProvider  string        // "mock" | "anthropic" | "openai"
	LLMModel     string        // model name passed to the provider
	LLMAPIKey    string        // provider API key (empty -> mock)
	LLMTimeout   time.Duration // per-request timeout for LLM calls
}

// Load reads configuration from the environment (and an optional .env file).
func Load() Config {
	// Best-effort load of a local .env file; ignored if absent.
	_ = godotenv.Load()

	cfg := Config{
		Port:         env("PORT", "8080"),
		DatabaseURL:  env("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/career?sslmode=disable"),
		JWTSecret:    env("JWT_SECRET", "dev-secret-change-me"),
		JWTTTL:       time.Duration(envInt("JWT_TTL_HOURS", 72)) * time.Hour,
		StorageDir:   env("STORAGE_DIR", "./data/uploads"),
		VectorDir:    env("VECTOR_DIR", "./data/vectors"),
		MigrationDir: env("MIGRATION_DIR", "./migrations"),
		SeedFile:     env("SEED_FILE", "./seed/roles.json"),
		LLMProvider:  env("LLM_PROVIDER", "mock"),
		LLMModel:     env("LLM_MODEL", "claude-sonnet-4-6"),
		LLMAPIKey:    env("LLM_API_KEY", ""),
		LLMTimeout:   time.Duration(envInt("LLM_TIMEOUT_SECONDS", 60)) * time.Second,
	}

	// If a provider was requested but no key supplied, fall back to mock so the
	// app always runs.
	if cfg.LLMProvider != "mock" && cfg.LLMAPIKey == "" {
		cfg.LLMProvider = "mock"
	}
	return cfg
}

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func envInt(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return fallback
}
