package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLoadConfigurationInvalid(t *testing.T) {
	_, err := loadConfiguration("nonexistent-file.json", false)
	assert.Error(t, err)
}

func TestLoadConfigurationValid(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "osctrld-test.json")
	configData := []byte(`{
  "osctrld": {
    "secret": "test-secret",
    "secretFile": "/tmp/osquery.secret",
    "flags": "/tmp/osquery.flags",
    "cert": "/tmp/osctrl.crt",
    "environment": "dev",
    "baseurl": "https://localhost:9000",
    "insecure": true,
    "verbose": true,
    "force": true
  }
}`)
	err := os.WriteFile(configPath, configData, 0644)
	assert.NoError(t, err)

	cfg, err := loadConfiguration(configPath, false)
	assert.NoError(t, err)
	assert.Equal(t, "test-secret", cfg.Secret)
	assert.Equal(t, "dev", cfg.Environment)
	assert.Equal(t, "https://localhost:9000", cfg.BaseURL)
	assert.True(t, cfg.Insecure)
	assert.True(t, cfg.Verbose)
	assert.True(t, cfg.Force)
}
