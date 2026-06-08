package main

import (
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRunCommandDoesNotExposeTaskFlag(t *testing.T) {
	command := RunCommand()
	assert.Nil(t, command.Flags().Lookup("task"))
}

func TestRunCommandExposesVFSReadLimitFlag(t *testing.T) {
	command := RunCommand()
	flag := command.Flags().Lookup("vfs-read-limit")
	assert.NotNil(t, flag)
	assert.Equal(t, "384", flag.DefValue)
}

func TestResolveShadowDir(t *testing.T) {
	testCases := []struct {
		name            string
		globalShadowDir string
		envShadowDir    string
		envSet          bool
		setShadowFlag   bool
		shadowFlagValue string
		expected        string
	}{
		{
			name:            "uses CLI value when provided without flag change",
			globalShadowDir: "cli-shadow",
			envShadowDir:    "env-shadow",
			envSet:          true,
			expected:        "cli-shadow",
		},
		{
			name:         "uses env value when CLI value empty",
			envShadowDir: "env-shadow",
			envSet:       true,
			expected:     "env-shadow",
		},
		{
			name:         "trims env value",
			envShadowDir: "  env-shadow  ",
			envSet:       true,
			expected:     "env-shadow",
		},
		{
			name:            "shadow-dir flag overrides env",
			envShadowDir:    "env-shadow",
			envSet:          true,
			setShadowFlag:   true,
			shadowFlagValue: "flag-shadow",
			expected:        "flag-shadow",
		},
		{
			name:            "empty shadow-dir flag overrides env",
			envShadowDir:    "env-shadow",
			envSet:          true,
			setShadowFlag:   true,
			shadowFlagValue: "",
			expected:        "",
		},
		{
			name:     "returns empty when no CLI value and env not set",
			envSet:   false,
			expected: "",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			shadowDir = tc.globalShadowDir
			if tc.envSet {
				t.Setenv(shadowDirEnvVar, tc.envShadowDir)
			} else {
				t.Setenv(shadowDirEnvVar, "")
			}

			cmd := &cobra.Command{Use: "run"}
			cmd.Flags().String("shadow-dir", "", "")
			if tc.setShadowFlag {
				err := cmd.Flags().Set("shadow-dir", tc.shadowFlagValue)
				assert.NoError(t, err)
				shadowDir = tc.shadowFlagValue
			}

			resolved := resolveShadowDir(cmd)
			assert.Equal(t, tc.expected, resolved)
		})
	}
}

func TestResolveShadowDirUsesEnvForRunSubcommandWithRootPersistentFlag(t *testing.T) {
	originalShadowDir := shadowDir
	t.Cleanup(func() {
		shadowDir = originalShadowDir
	})

	shadowDir = ""
	t.Setenv(shadowDirEnvVar, "env-shadow")

	rootCmd := &cobra.Command{Use: "csw"}
	rootCmd.PersistentFlags().StringVar(&shadowDir, "shadow-dir", "", "")
	runCmd := &cobra.Command{Use: "run"}
	rootCmd.AddCommand(runCmd)

	resolved := resolveShadowDir(runCmd)

	assert.Equal(t, "env-shadow", resolved)
}

func TestRunCommandExposesInheritedShadowDirFlag(t *testing.T) {
	originalShadowDir := shadowDir
	t.Cleanup(func() {
		shadowDir = originalShadowDir
	})

	shadowDir = ""
	rootCmd := &cobra.Command{Use: "csw"}
	rootCmd.PersistentFlags().StringVar(&shadowDir, "shadow-dir", "", "")
	runCmd := RunCommand()
	rootCmd.AddCommand(runCmd)

	flag := runCmd.Flag("shadow-dir")

	require.NotNil(t, flag)
}

func TestShouldApplyRunShadowDirUsesEnvResolvedValue(t *testing.T) {
	originalShadowDir := shadowDir
	t.Cleanup(func() {
		shadowDir = originalShadowDir
	})

	rootCmd := &cobra.Command{Use: "csw"}
	rootCmd.PersistentFlags().StringVar(&shadowDir, "shadow-dir", "", "")
	runCmd := RunCommand()
	rootCmd.AddCommand(runCmd)
	shadowDir = "env-shadow"

	apply := shouldApplyRunShadowDir(runCmd)

	assert.True(t, apply)
}
