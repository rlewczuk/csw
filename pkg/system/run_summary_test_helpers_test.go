package system

import (
	"fmt"
	"os"
	"path/filepath"
)

func findLatestSummaryFile(sessionsDir string) (string, string, error) {
	entries, err := os.ReadDir(sessionsDir)
	if err != nil {
		return "", "", fmt.Errorf("findLatestSummaryFile() [cli_summary_test_helpers_test.go]: failed to read sessions directory: %w", err)
	}

	latestSessionID := ""
	latestPath := ""
	latestTime := int64(0)
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		sessionID := entry.Name()
		summaryPath := filepath.Join(sessionsDir, sessionID, "summary.md")
		info, statErr := os.Stat(summaryPath)
		if statErr != nil {
			continue
		}

		modTime := info.ModTime().UnixNano()
		if latestPath == "" || modTime > latestTime {
			latestTime = modTime
			latestSessionID = sessionID
			latestPath = summaryPath
		}
	}

	if latestPath == "" {
		return "", "", fmt.Errorf("findLatestSummaryFile() [cli_summary_test_helpers_test.go]: no summary.md found in sessions")
	}

	return latestSessionID, latestPath, nil
}
