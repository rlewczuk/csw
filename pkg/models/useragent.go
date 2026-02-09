package models

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
)

// defaultVersion is used when VERSION file cannot be read.
const defaultVersion = "dev"

// userAgentVersion holds the agent version, set at build time or read from VERSION file.
// This is initialized lazily by getUserAgent().
var userAgentVersion string
var userAgentOnce sync.Once

// readVersion reads the version from the VERSION file at the project root.
// It returns defaultVersion if the file cannot be read.
func readVersion() string {
	// Try to find VERSION file from project root
	// Start from current working directory and traverse up
	cwd, err := os.Getwd()
	if err != nil {
		return defaultVersion
	}

	// Try current directory and parent directories
	dir := cwd
	for {
		versionPath := filepath.Join(dir, "VERSION")
		data, err := os.ReadFile(versionPath)
		if err == nil {
			return strings.TrimSpace(string(data))
		}

		// Move to parent directory
		parent := filepath.Dir(dir)
		if parent == dir {
			// Reached root
			break
		}
		dir = parent
	}

	return defaultVersion
}

// getUserAgent returns the User-Agent string for the Codesnort SWE agent.
// Format: Codesnort SWE/<version> (<os>/<arch>) Go/<goversion>
func getUserAgent() string {
	userAgentOnce.Do(func() {
		userAgentVersion = readVersion()
	})
	return fmt.Sprintf("Codesnort SWE/%s (%s/%s) Go/%s",
		userAgentVersion,
		runtime.GOOS,
		runtime.GOARCH,
		runtime.Version(),
	)
}

// setUserAgentHeader sets the User-Agent header on the given request.
func setUserAgentHeader(req *http.Request) {
	req.Header.Set("User-Agent", getUserAgent())
}
