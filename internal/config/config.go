package config

import (
	"os"
)

// Config holds application configuration
type Config struct {
	Port         string
	Env          string
	DatabasePath string
	OllamaAPIURL string
	OllamaModel  string
}

// Load loads configuration from environment variables
func Load() (*Config, error) {
	return &Config{
		Port:         getEnv("PORT", "8080"),
		Env:          getEnv("ENV", "development"),
		DatabasePath: getEnv("DATABASE_PATH", "./calorie-count.db"),
		OllamaAPIURL: getEnv("OLLAMA_API_URL", "http://localhost:11434"),
		OllamaModel:  getEnv("OLLAMA_MODEL", "llama-vision:latest"),
	}, nil
}

func getEnv(key, defaultValue string) string {
	value, exists := os.LookupEnv(key)
	if !exists {
		return defaultValue
	}
	return value
}
