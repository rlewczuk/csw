package core

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"text/template"

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

	// Load system.md
	systemPath := filepath.Join(dir, "system.md")
	systemData, err := os.ReadFile(systemPath)
	if err != nil {
		return role, fmt.Errorf("loadRole() [role.go]: failed to read system.md: %w", err)
	}

	role.SystemPrompt = string(systemData)

	return role, nil
}

// RenderSystemPrompt renders the system prompt template with the given agent state.
func (r *AgentRole) RenderSystemPrompt(state AgentState) (string, error) {
	tmpl, err := template.New("system").Parse(r.SystemPrompt)
	if err != nil {
		return "", fmt.Errorf("AgentRole.RenderSystemPrompt() [role.go]: failed to parse system prompt template: %w", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, state); err != nil {
		return "", fmt.Errorf("AgentRole.RenderSystemPrompt() [role.go]: failed to execute system prompt template: %w", err)
	}

	return buf.String(), nil
}
