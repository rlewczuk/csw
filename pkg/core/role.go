package core

import (
	"github.com/codesnort/codesnort-swe/pkg/shared"
	"github.com/codesnort/codesnort-swe/pkg/vfs"
)

// AccessFlag represents access flag for a file or directory.

// Contains all information
type AgentRole struct {
	// Name of the role (short name, used to select role and identify it in logs etc.)
	Name string `json:"name"`

	// Description of the role (longer text, used in UI to describe role to user)
	Description string `json:"description"`

	// System prompt template (markdown format, parsed by text/template)
	SystemPrompt string `json:"system-prompt"`

	// Privileges for VFS and runtime
	VFSPrivileges map[string]vfs.FileAccess `json:"vfs-privileges"`

	// Tools available
	ToolsAccess map[string]shared.AccessFlag `json:"tools-access"`
}
