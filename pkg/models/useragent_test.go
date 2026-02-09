package models

import (
	"net/http"
	"runtime"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetUserAgent(t *testing.T) {
	// Reset the version to ensure we read from VERSION file
	userAgentVersion = ""
	userAgentOnce = sync.Once{}

	userAgent := getUserAgent()

	// Verify the format: Codesnort SWE/<version> (<os>/<arch>) Go/<goversion>
	assert.True(t, strings.HasPrefix(userAgent, "Codesnort SWE/"), "User-Agent should start with 'Codesnort SWE/'")
	assert.Contains(t, userAgent, runtime.GOOS, "User-Agent should contain OS")
	assert.Contains(t, userAgent, runtime.GOARCH, "User-Agent should contain architecture")
	assert.Contains(t, userAgent, runtime.Version(), "User-Agent should contain Go version")

	// Verify format parts - the User-Agent format is:
	// Codesnort SWE/<version> (<os>/<arch>) Go/<goversion>
	// Note: Go version can contain spaces (e.g., "go1.23.4 linux/amd64")
	assert.True(t, strings.HasPrefix(userAgent, "Codesnort SWE/"), "User-Agent should start with 'Codesnort SWE/'")

	// Check for the OS/arch parentheses section
	osArchPattern := "(" + runtime.GOOS + "/" + runtime.GOARCH + ")"
	assert.Contains(t, userAgent, osArchPattern, "User-Agent should contain OS/arch in parentheses")

	// Check for Go version prefix
	assert.True(t, strings.Contains(userAgent, "Go/"), "User-Agent should contain 'Go/' prefix")
}

func TestSetUserAgentHeader(t *testing.T) {
	req, err := http.NewRequest(http.MethodGet, "http://example.com", nil)
	assert.NoError(t, err)

	setUserAgentHeader(req)

	userAgent := req.Header.Get("User-Agent")
	assert.NotEmpty(t, userAgent, "User-Agent header should be set")
	assert.True(t, strings.HasPrefix(userAgent, "Codesnort SWE/"), "User-Agent should start with 'Codesnort SWE/'")
}

func TestReadVersion(t *testing.T) {
	version := readVersion()

	// Version should not be empty and should not be the default "dev" when VERSION file exists
	assert.NotEmpty(t, version, "Version should not be empty")

	// In test environment, we should either read from VERSION file or get "dev"
	// The VERSION file exists at project root, so we should get the actual version
	assert.NotEqual(t, "", version, "Version should be read from VERSION file or default to 'dev'")
}

func TestGetUserAgent_Caching(t *testing.T) {
	// Reset the version
	userAgentVersion = ""
	userAgentOnce = sync.Once{}

	// First call should initialize
	ua1 := getUserAgent()

	// Second call should return cached value
	ua2 := getUserAgent()

	assert.Equal(t, ua1, ua2, "Subsequent calls should return the same User-Agent")
}
