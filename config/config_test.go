package config

import (
	"os"
	"testing"
)

func TestLoad(t *testing.T) {
	// 1. Set environment variable
	os.Setenv("API_KEY", "test-api-key")
	defer os.Unsetenv("API_KEY")

	// 2. Call Load (mocking godotenv by ensuring environment is set)
	cfg := Load()

	// 3. Assertions
	if cfg.APIKey != "test-api-key" {
		t.Errorf("Expected APIKey 'test-api-key', got '%s'", cfg.APIKey)
	}
}
