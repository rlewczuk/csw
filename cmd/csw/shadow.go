package main

import (
	"os"
	"strings"

	"github.com/spf13/cobra"
)

const shadowDirEnvVar = "CSW_SHADOW_DIR"

// resolveShadowDir resolves shadow directory from global flag and environment.
func resolveShadowDir(cmd *cobra.Command) string {
	trimmedFlagValue := strings.TrimSpace(shadowDir)
	if cmd != nil {
		shadowFlag := cmd.Flag("shadow-dir")
		if shadowFlag != nil && shadowFlag.Changed {
			return trimmedFlagValue
		}
	}

	if trimmedFlagValue != "" {
		return trimmedFlagValue
	}

	shadowDirFromEnv, envExists := os.LookupEnv(shadowDirEnvVar)
	if !envExists {
		return ""
	}

	return strings.TrimSpace(shadowDirFromEnv)
}
