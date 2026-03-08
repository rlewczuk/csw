package main

import (
	"context"

	"github.com/rlewczuk/csw/pkg/conf"
	"github.com/rlewczuk/csw/pkg/core"
	"github.com/rlewczuk/csw/pkg/models"
)

// generateWorktreeCommitMessage generates a short commit message using the active chat model.
// This is a wrapper around core.GenerateCommitMessage.
func generateWorktreeCommitMessage(ctx context.Context, modelProviders map[string]models.ModelProvider, configStore conf.ConfigStore, session *core.SweSession, branch string, customTemplate string) (string, error) {
	return core.GenerateCommitMessage(ctx, modelProviders, configStore, session, branch, customTemplate)
}
