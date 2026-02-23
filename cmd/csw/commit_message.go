package main

import (
	"context"

	"github.com/rlewczuk/csw/pkg/core"
)

// Default paths to commit prompt template files.
const (
	CommitPromptSystemPath  = "pkg/conf/impl/conf/agent/commit/system.md"
	CommitPromptPromptPath  = "pkg/conf/impl/conf/agent/commit/prompt.md"
	CommitPromptMessagePath = "pkg/conf/impl/conf/agent/commit/message.md"
)

// generateWorktreeCommitMessage generates a short commit message using the active chat model.
// This is a wrapper around core.GenerateCommitMessage with csw-specific default paths.
func generateWorktreeCommitMessage(ctx context.Context, sweSystem *core.SweSystem, session *core.SweSession, branch string, customTemplate string) (string, error) {
	paths := core.CommitPromptPaths{
		SystemPath:  CommitPromptSystemPath,
		PromptPath:  CommitPromptPromptPath,
		MessagePath: CommitPromptMessagePath,
	}
	return core.GenerateCommitMessage(ctx, sweSystem, session, paths, branch, customTemplate)
}
