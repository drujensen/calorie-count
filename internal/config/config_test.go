package config_test

import (
	"os"
	"testing"

	"github.com/drujensen/calorie-count/internal/config"
)

func TestLoad_Defaults(t *testing.T) {
	// Ensure env vars are unset so we test defaults.
	for _, key := range []string{"PORT", "ENV", "DATABASE_PATH", "OLLAMA_API_URL", "OLLAMA_MODEL"} {
		t.Setenv(key, "")
		os.Unsetenv(key) //nolint:errcheck
	}

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Load() returned unexpected error: %v", err)
	}

	tests := []struct {
		name string
		got  string
		want string
	}{
		{"Port", cfg.Port, "8080"},
		{"Env", cfg.Env, "development"},
		{"DatabasePath", cfg.DatabasePath, "./calorie-count.db"},
		{"OllamaAPIURL", cfg.OllamaAPIURL, "http://localhost:11434"},
		{"OllamaModel", cfg.OllamaModel, "llama-vision:latest"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.got != tt.want {
				t.Errorf("got %q, want %q", tt.got, tt.want)
			}
		})
	}
}

func TestLoad_EnvOverrides(t *testing.T) {
	t.Setenv("PORT", "9090")
	t.Setenv("ENV", "production")
	t.Setenv("DATABASE_PATH", "/data/app.db")
	t.Setenv("OLLAMA_API_URL", "http://remote:11434")
	t.Setenv("OLLAMA_MODEL", "llama3-custom")

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Load() returned unexpected error: %v", err)
	}

	tests := []struct {
		name string
		got  string
		want string
	}{
		{"Port", cfg.Port, "9090"},
		{"Env", cfg.Env, "production"},
		{"DatabasePath", cfg.DatabasePath, "/data/app.db"},
		{"OllamaAPIURL", cfg.OllamaAPIURL, "http://remote:11434"},
		{"OllamaModel", cfg.OllamaModel, "llama3-custom"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.got != tt.want {
				t.Errorf("got %q, want %q", tt.got, tt.want)
			}
		})
	}
}
