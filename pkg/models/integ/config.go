// Package integ provides integration test configuration helpers.
package integ

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// integDir returns the absolute path to the _integ directory at project root.
func integDir() string {
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		return "_integ"
	}
	// This file is at pkg/models/integ/config.go, so project root is 4 levels up
	projectRoot := filepath.Join(filepath.Dir(thisFile), "..", "..", "..")
	return filepath.Join(projectRoot, "_integ")
}

// readFileContent reads a file from the _integ directory and returns its trimmed content.
// Returns empty string if file doesn't exist or is empty.
func readFileContent(filename string) string {
	path := filepath.Join(integDir(), filename)
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

// isEnabled checks if a feature is enabled by reading the corresponding .enabled file.
// Returns true if the file exists and contains "yes" (case insensitive).
func isEnabled(name string) bool {
	content := readFileContent(name + ".enabled")
	return strings.EqualFold(content, "yes")
}

// IsAllEnabled returns true if all integration tests should run.
func IsAllEnabled() bool {
	return isEnabled("all")
}

// IsOllamaEnabled returns true if ollama integration tests should run.
func IsOllamaEnabled() bool {
	return IsAllEnabled() || isEnabled("ollama")
}

// IsOpenAIEnabled returns true if openai integration tests should run.
func IsOpenAIEnabled() bool {
	return IsAllEnabled() || isEnabled("openai")
}

// IsAnthropicEnabled returns true if anthropic integration tests should run.
func IsAnthropicEnabled() bool {
	return IsAllEnabled() || isEnabled("anthropic")
}

// GetOllamaURL returns the URL for ollama from the config file.
func GetOllamaURL() string {
	return readFileContent("ollama.url")
}

// GetOpenAIURL returns the URL for openai from the config file.
func GetOpenAIURL() string {
	return readFileContent("openai.url")
}

// GetOllamaAPIKey returns the optional API key for ollama from the config file.
// Returns empty string if file doesn't exist.
func GetOllamaAPIKey() string {
	return readFileContent("ollama.key")
}

// GetOpenAIAPIKey returns the optional API key for openai from the config file.
// Returns empty string if file doesn't exist.
func GetOpenAIAPIKey() string {
	return readFileContent("openai.key")
}

// GetAnthropicAPIKey returns the API key for anthropic from the config file.
func GetAnthropicAPIKey() string {
	return readFileContent("anthropic.key")
}

// SkipIfOllamaDisabled skips the test if ollama integration tests are disabled.
// Returns the URL to use for ollama if tests should run.
func SkipIfOllamaDisabled(t *testing.T) string {
	t.Helper()
	if !IsOllamaEnabled() {
		t.Skip("Skipping test: ollama integration tests not enabled (set _integ/ollama.enabled or _integ/all.enabled to 'yes')")
	}
	url := GetOllamaURL()
	if url == "" {
		t.Skip("Skipping test: _integ/ollama.url not configured")
	}
	return url
}

// SkipIfOpenAIDisabled skips the test if openai integration tests are disabled.
// Returns the URL to use for openai if tests should run.
func SkipIfOpenAIDisabled(t *testing.T) string {
	t.Helper()
	if !IsOpenAIEnabled() {
		t.Skip("Skipping test: openai integration tests not enabled (set _integ/openai.enabled or _integ/all.enabled to 'yes')")
	}
	url := GetOpenAIURL()
	if url == "" {
		t.Skip("Skipping test: _integ/openai.url not configured")
	}
	return url
}

// SkipIfAnthropicDisabled skips the test if anthropic integration tests are disabled.
// Returns the API key to use for anthropic if tests should run.
func SkipIfAnthropicDisabled(t *testing.T) string {
	t.Helper()
	if !IsAnthropicEnabled() {
		t.Skip("Skipping test: anthropic integration tests not enabled (set _integ/anthropic.enabled or _integ/all.enabled to 'yes')")
	}
	apiKey := GetAnthropicAPIKey()
	if apiKey == "" {
		t.Skip("Skipping test: _integ/anthropic.key not configured")
	}
	return apiKey
}
