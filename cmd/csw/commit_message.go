package main

import (
	"context"

	"github.com/rlewczuk/csw/pkg/core"
)

// generateWorktreeCommitMessage generates a short commit message using the active chat model.
// This is a wrapper around core.GenerateCommitMessage.
func generateWorktreeCommitMessage(ctx context.Context, sweSystem *core.SweSystem, session *core.SweSession, branch string, customTemplate string) (string, error) {
	return core.GenerateCommitMessage(ctx, sweSystem, session, branch, customTemplate)
}
