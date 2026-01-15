package core

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

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

	// Privileges for VFS and runtime
	VFSPrivileges map[string]vfs.FileAccess `json:"vfs-privileges"`

	// Tools available
	ToolsAccess map[string]shared.AccessFlag `json:"tools-access"`

	// Run privileges maps command regex patterns to access flags
	RunPrivileges map[string]shared.AccessFlag `json:"run-privileges"`
}

type AgentRoleRegistry struct {
	roles map[string]AgentRole
}

func NewAgentRoleRegistry() *AgentRoleRegistry {
	return &AgentRoleRegistry{
		roles: make(map[string]AgentRole),
	}
}

// Get returns a role by name and a boolean indicating if it was found.
func (r *AgentRoleRegistry) Get(name string) (AgentRole, bool) {
	role, ok := r.roles[name]
	return role, ok
}

// Register adds a role to the registry.
func (r *AgentRoleRegistry) Register(role AgentRole) {
	r.roles[role.Name] = role
}

// List returns all role names in the registry.
func (r *AgentRoleRegistry) List() []string {
	names := make([]string, 0, len(r.roles))
	for name := range r.roles {
		names = append(names, name)
	}
	return names
}

// LoadFromDirectory loads all roles from a directory structure.
// Each subdirectory is expected to contain:
// - config.json: JSON file with AgentRole fields (description, vfs-privileges, tools-access)
// - system.md: Markdown file with system prompt template
// The role name is derived from the subdirectory name.
func (r *AgentRoleRegistry) LoadFromDirectory(dir string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("AgentRoleRegistry.LoadFromDirectory() [role.go]: failed to read roles directory: %w", err)
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		roleName := entry.Name()
		rolePath := filepath.Join(dir, roleName)

		role, err := loadRole(roleName, rolePath)
		if err != nil {
			return fmt.Errorf("AgentRoleRegistry.LoadFromDirectory() [role.go]: failed to load role %s: %w", roleName, err)
		}

		r.Register(role)
	}

	return nil
}

// loadRole loads a single role from a directory.
func loadRole(name, dir string) (AgentRole, error) {
	role := AgentRole{
		Name: name,
	}

	// Load config.json
	configPath := filepath.Join(dir, "config.json")
	configData, err := os.ReadFile(configPath)
	if err != nil {
		return role, fmt.Errorf("loadRole() [role.go]: failed to read config.json: %w", err)
	}

	if err := json.Unmarshal(configData, &role); err != nil {
		return role, fmt.Errorf("loadRole() [role.go]: failed to parse config.json: %w", err)
	}

	return role, nil
}
