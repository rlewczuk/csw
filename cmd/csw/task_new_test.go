package main

import (
	"context"
	"testing"

	"github.com/rlewczuk/csw/pkg/conf"
	"github.com/rlewczuk/csw/pkg/system"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResolveTaskCreateParamsGeneratesBranchAndDescription(t *testing.T) {
	originalDefaults := resolveTaskRunDefaultsFunc
	originalBranchResolver := resolveTaskWorktreeBranchNameFunc
	originalDescriptionGenerator := generateTaskDescriptionFunc
	t.Cleanup(func() {
		resolveTaskRunDefaultsFunc = originalDefaults
		resolveTaskWorktreeBranchNameFunc = originalBranchResolver
		generateTaskDescriptionFunc = originalDescriptionGenerator
	})

	resolveTaskRunDefaultsFunc = func(params system.ResolveRunDefaultsParams) (conf.RunDefaultsConfig, error) {
		_ = params
		return conf.RunDefaultsConfig{Model: "provider/model", Worktree: "feature/%"}, nil
	}

	resolveTaskWorktreeBranchNameFunc = func(ctx context.Context, params system.ResolveWorktreeBranchNameParams) (string, error) {
		_ = ctx
		assert.Equal(t, "do this task", params.Prompt)
		assert.Equal(t, "provider/model", params.ModelName)
		assert.Equal(t, "feature/%", params.WorktreeBranch)
		return "feature/generated", nil
	}

	generateTaskDescriptionFunc = func(ctx context.Context, params taskCreateResolveParams) (string, error) {
		_ = ctx
		assert.Equal(t, "do this task", params.Prompt)
		assert.Equal(t, "feature/generated", params.Branch)
		assert.Equal(t, "provider/model", params.ModelName)
		return "generated description", nil
	}

	resolved, err := resolveTaskCreateParams(context.Background(), taskCreateResolveParams{Prompt: "do this task"})
	require.NoError(t, err)
	assert.Equal(t, "feature/generated", resolved.FeatureBranch)
	assert.Equal(t, "feature/generated", resolved.Name)
	assert.Equal(t, "generated description", resolved.Description)
	assert.Equal(t, "do this task", resolved.Prompt)
}

func TestTaskNewCommandPromptFlagIsOptional(t *testing.T) {
	command := taskNewCommand()
	promptFlag := command.Flags().Lookup("prompt")
	require.NotNil(t, promptFlag)
	_, required := promptFlag.Annotations[cobra.BashCompOneRequiredFlag]
	assert.False(t, required)
	assert.Nil(t, command.Flags().Lookup("run"))
}
