package core

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/codesnort/codesnort-swe/pkg/models"
	"github.com/codesnort/codesnort-swe/pkg/shared"
	"github.com/codesnort/codesnort-swe/pkg/tool"
	"github.com/codesnort/codesnort-swe/pkg/vfs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// stateAwarePromptGenerator is a mock that uses the state in the prompt
type stateAwarePromptGenerator struct{}

func (s *stateAwarePromptGenerator) GetPrompt(tags []string, role *AgentRole, state *AgentState) (string, error) {
	return fmt.Sprintf("You are a developer. Work dir: %s", state.Info.WorkDir), nil
}

func TestAgentRoleRegistry(t *testing.T) {
	t.Run("Register and Get", func(t *testing.T) {
		registry := NewAgentRoleRegistry()

		role := AgentRole{
			Name:        "test-role",
			Description: "A test role",
		}

		registry.Register(role)

		retrieved, ok := registry.Get("test-role")
		assert.True(t, ok)
		assert.Equal(t, "test-role", retrieved.Name)
		assert.Equal(t, "A test role", retrieved.Description)

		_, ok = registry.Get("nonexistent")
		assert.False(t, ok)
	})

	t.Run("List returns all role names", func(t *testing.T) {
		registry := NewAgentRoleRegistry()

		role1 := AgentRole{Name: "role1", Description: "Role 1"}
		role2 := AgentRole{Name: "role2", Description: "Role 2"}

		registry.Register(role1)
		registry.Register(role2)

		names := registry.List()
		assert.Len(t, names, 2)
		assert.Contains(t, names, "role1")
		assert.Contains(t, names, "role2")
	})

	t.Run("LoadFromDirectory loads roles from config files", func(t *testing.T) {
		registry := NewAgentRoleRegistry()

		// Get the absolute path to the configs/roles directory
		configsDir := filepath.Join("..", "..", "testdata", "conf", "roles")

		err := registry.LoadFromDirectory(configsDir)
		require.NoError(t, err)

		// Check that developer role was loaded
		developer, ok := registry.Get("developer")
		assert.True(t, ok)
		assert.Equal(t, "developer", developer.Name)
		assert.Equal(t, "A software developer role with full VFS access", developer.Description)

		// Check that explorer role was loaded
		explorer, ok := registry.Get("explorer")
		assert.True(t, ok)
		assert.Equal(t, "explorer", explorer.Name)
		assert.Equal(t, "A role for exploring the codebase", explorer.Description)

		// Check VFS privileges
		assert.NotNil(t, developer.VFSPrivileges)
		assert.NotNil(t, explorer.VFSPrivileges)

		// Check tools access
		assert.NotNil(t, developer.ToolsAccess)
		assert.NotNil(t, explorer.ToolsAccess)
	})
}

func TestAgentRoleIntegration(t *testing.T) {
	// Create mock components
	mockProvider := models.NewMockProvider([]models.ModelInfo{
		{Name: "test-model", Model: "test-model"},
	})
	mockVFS := vfs.NewMockVFS()
	tools := tool.NewToolRegistry()
	tool.RegisterVFSTools(tools, mockVFS)

	// Define test roles
	developerRole := AgentRole{
		Name:        "developer",
		Description: "A software developer role with full VFS access",
		VFSPrivileges: map[string]vfs.FileAccess{
			"**": {
				Read:   shared.AccessAllow,
				Write:  shared.AccessAllow,
				Delete: shared.AccessAllow,
				List:   shared.AccessAllow,
				Find:   shared.AccessAllow,
				Move:   shared.AccessAllow,
			},
		},
		ToolsAccess: map[string]shared.AccessFlag{
			"**": shared.AccessAllow,
		},
	}

	readOnlyRole := AgentRole{
		Name:        "readonly",
		Description: "A read-only role with limited VFS access",
		VFSPrivileges: map[string]vfs.FileAccess{
			"**": {
				Read:   shared.AccessAllow,
				Write:  shared.AccessDeny,
				Delete: shared.AccessDeny,
				List:   shared.AccessAllow,
				Find:   shared.AccessAllow,
				Move:   shared.AccessDeny,
			},
		},
		ToolsAccess: map[string]shared.AccessFlag{
			"vfs.read": shared.AccessAllow,
			"vfs.list": shared.AccessAllow,
			"**":       shared.AccessDeny,
		},
	}

	t.Run("SetRole updates role field", func(t *testing.T) {
		registry := NewAgentRoleRegistry()
		registry.Register(developerRole)
		registry.Register(readOnlyRole)

		system := &SweSystem{
			ModelProviders: map[string]models.ModelProvider{"mock": mockProvider},
			Tools:          tools,
			VFS:            mockVFS,
			Roles:          registry,
		}

		session, err := system.NewSession("mock/test-model", nil)
		require.NoError(t, err)
		assert.Nil(t, session.Role())

		err = session.SetRole("developer")
		require.NoError(t, err)
		assert.NotNil(t, session.Role())
		assert.Equal(t, "developer", session.Role().Name)
	})

	t.Run("SetRole updates system prompt", func(t *testing.T) {
		registry := NewAgentRoleRegistry()
		registry.Register(developerRole)

		system := &SweSystem{
			ModelProviders:  map[string]models.ModelProvider{"mock": mockProvider},
			PromptGenerator: newMockPromptGenerator("You are an experienced software developer."),
			Tools:           tools,
			VFS:             mockVFS,
			Roles:           registry,
		}

		session, err := system.NewSession("mock/test-model", nil)
		require.NoError(t, err)

		// Check that initially there is no system prompt (no role set yet)
		messages := session.ChatMessages()
		require.Len(t, messages, 0)

		// Set role and check updated system prompt
		err = session.SetRole("developer")
		require.NoError(t, err)

		messages = session.ChatMessages()
		require.Len(t, messages, 1)
		assert.Equal(t, models.ChatRoleSystem, messages[0].Role)
		assert.Equal(t, "You are an experienced software developer.", messages[0].Parts[0].Text)
	})

	t.Run("SetRole wraps VFS with access control", func(t *testing.T) {
		registry := NewAgentRoleRegistry()
		registry.Register(readOnlyRole)

		system := &SweSystem{
			ModelProviders: map[string]models.ModelProvider{"mock": mockProvider},
			Tools:          tools,
			VFS:            mockVFS,
			Roles:          registry,
		}

		session, err := system.NewSession("mock/test-model", nil)
		require.NoError(t, err)

		// Initially, VFS should be unwrapped
		assert.Equal(t, mockVFS, session.VFS)

		// Set role to readonly
		err = session.SetRole("readonly")
		require.NoError(t, err)

		// VFS should now be wrapped with AccessControlVFS
		assert.NotEqual(t, mockVFS, session.VFS)
		_, ok := session.VFS.(*vfs.AccessControlVFS)
		assert.True(t, ok, "VFS should be wrapped with AccessControlVFS")

		// Test that write is denied
		err = session.VFS.WriteFile("test.txt", []byte("content"))
		assert.Error(t, err)
		assert.Equal(t, vfs.ErrPermissionDenied, err)

		// Test that read is allowed
		mockVFS.WriteFile("existing.txt", []byte("content"))
		_, err = session.VFS.ReadFile("existing.txt")
		assert.NoError(t, err)
	})

	t.Run("SetRole wraps tools with access control", func(t *testing.T) {
		registry := NewAgentRoleRegistry()
		registry.Register(readOnlyRole)

		system := &SweSystem{
			ModelProviders: map[string]models.ModelProvider{"mock": mockProvider},
			Tools:          tools,
			VFS:            mockVFS,
			Roles:          registry,
		}

		session, err := system.NewSession("mock/test-model", nil)
		require.NoError(t, err)

		// Set role to readonly
		err = session.SetRole("readonly")
		require.NoError(t, err)

		// Test that vfs.write is denied
		writeArgs := tool.NewToolValue(map[string]interface{}{
			"path":    "test.txt",
			"content": "content",
		})
		writeResponse := session.Tools.Execute(tool.ToolCall{
			Function:  "vfs.write",
			Arguments: writeArgs,
		})
		assert.Error(t, writeResponse.Error)
		assert.Contains(t, writeResponse.Error.Error(), "access denied")

		// Test that vfs.read is allowed
		mockVFS.WriteFile("existing.txt", []byte("content"))
		readArgs := tool.NewToolValue(map[string]interface{}{
			"path": "existing.txt",
		})
		readResponse := session.Tools.Execute(tool.ToolCall{
			Function:  "vfs.read",
			Arguments: readArgs,
		})
		assert.NoError(t, readResponse.Error)
	})

	t.Run("SetRole returns error for unknown role", func(t *testing.T) {
		registry := NewAgentRoleRegistry()

		system := &SweSystem{
			ModelProviders: map[string]models.ModelProvider{"mock": mockProvider},
			Tools:          tools,
			VFS:            mockVFS,
			Roles:          registry,
		}

		session, err := system.NewSession("mock/test-model", nil)
		require.NoError(t, err)

		err = session.SetRole("unknown")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "role not found")
	})

	t.Run("SetRole can switch between roles", func(t *testing.T) {
		registry := NewAgentRoleRegistry()
		registry.Register(developerRole)
		registry.Register(readOnlyRole)

		system := &SweSystem{
			ModelProviders: map[string]models.ModelProvider{"mock": mockProvider},
			Tools:          tools,
			VFS:            mockVFS,
			Roles:          registry,
		}

		session, err := system.NewSession("mock/test-model", nil)
		require.NoError(t, err)

		// Set to developer role
		err = session.SetRole("developer")
		require.NoError(t, err)
		assert.Equal(t, "developer", session.Role().Name)

		// Write should be allowed
		writeArgs1 := tool.NewToolValue(map[string]interface{}{
			"path":    "test.txt",
			"content": "content",
		})
		writeResponse := session.Tools.Execute(tool.ToolCall{
			Function:  "vfs.write",
			Arguments: writeArgs1,
		})
		assert.NoError(t, writeResponse.Error)

		// Switch to readonly role
		err = session.SetRole("readonly")
		require.NoError(t, err)
		assert.Equal(t, "readonly", session.Role().Name)

		// Write should now be denied
		writeArgs2 := tool.NewToolValue(map[string]interface{}{
			"path":    "test2.txt",
			"content": "content",
		})
		writeResponse = session.Tools.Execute(tool.ToolCall{
			Function:  "vfs.write",
			Arguments: writeArgs2,
		})
		assert.Error(t, writeResponse.Error)
	})
}

func TestSweSessionGetState(t *testing.T) {
	mockProvider := models.NewMockProvider([]models.ModelInfo{
		{Name: "test-model", Model: "test-model"},
	})
	mockVFS := vfs.NewMockVFS()
	tools := tool.NewToolRegistry()
	registry := NewAgentRoleRegistry()

	t.Run("GetState returns current work directory", func(t *testing.T) {
		system := &SweSystem{
			ModelProviders: map[string]models.ModelProvider{"mock": mockProvider},
			Tools:          tools,
			VFS:            mockVFS,
			Roles:          registry,
		}

		session, err := system.NewSession("mock/test-model", nil)
		require.NoError(t, err)

		state := session.GetState()
		assert.Equal(t, ".", state.Info.WorkDir)
	})

	t.Run("SetWorkDir updates work directory in state", func(t *testing.T) {
		system := &SweSystem{
			ModelProviders: map[string]models.ModelProvider{"mock": mockProvider},
			Tools:          tools,
			VFS:            mockVFS,
			Roles:          registry,
		}

		session, err := system.NewSession("mock/test-model", nil)
		require.NoError(t, err)

		session.SetWorkDir("/home/user/project")

		state := session.GetState()
		assert.Equal(t, "/home/user/project", state.Info.WorkDir)
	})

	t.Run("SetRole renders system prompt with current state", func(t *testing.T) {
		role := AgentRole{
			Name:        "dev",
			Description: "Developer role",
		}

		registry := NewAgentRoleRegistry()
		registry.Register(role)

		// Create a simple mock prompt generator
		mockGen := &stateAwarePromptGenerator{}

		system := &SweSystem{
			ModelProviders:  map[string]models.ModelProvider{"mock": mockProvider},
			PromptGenerator: mockGen,
			Tools:           tools,
			VFS:             mockVFS,
			Roles:           registry,
		}

		session, err := system.NewSession("mock/test-model", nil)
		require.NoError(t, err)

		session.SetWorkDir("/test/path")
		err = session.SetRole("dev")
		require.NoError(t, err)

		messages := session.ChatMessages()
		require.Len(t, messages, 1)
		assert.Equal(t, models.ChatRoleSystem, messages[0].Role)
		// The mock generator includes the work dir in the prompt
		assert.Contains(t, messages[0].Parts[0].Text, "/test/path")
	})
}
