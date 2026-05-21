package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestForceFlagDoesNotAffectVerbose(t *testing.T) {
	jsonConfig = JSONConfiguration{}
	app := buildApp()
	err := app.Run([]string{"osctrld", "--force", "--environment", "dev", "--osctrl-url", "http://localhost", "flags"})
	_ = err
	assert.True(t, jsonConfig.Force, "Force should be true when --force flag is set")
	assert.False(t, jsonConfig.Verbose, "Verbose should be false when only --force flag is set")
}

func TestVerboseFlagDoesNotAffectForce(t *testing.T) {
	jsonConfig = JSONConfiguration{}
	app := buildApp()
	err := app.Run([]string{"osctrld", "--verbose", "--environment", "dev", "--osctrl-url", "http://localhost", "flags"})
	_ = err
	assert.True(t, jsonConfig.Verbose, "Verbose should be true when --verbose flag is set")
	assert.False(t, jsonConfig.Force, "Force should be false when only --verbose flag is set")
}
