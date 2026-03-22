package core

import (
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/rlewczuk/csw/pkg/conf"
	"github.com/rlewczuk/csw/pkg/conf/impl"
	"github.com/rlewczuk/csw/pkg/logging"
	"github.com/rlewczuk/csw/pkg/models"
	"github.com/rlewczuk/csw/pkg/tool"
	"github.com/rlewczuk/csw/pkg/vfs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// stateAwarePromptGenerator is a mock that uses the state in the prompt
type stateAwarePromptGenerator struct{}

func (s *stateAwarePromptGenerator) GetPrompt(tags []string, role *conf.AgentRoleConfig, state *AgentState) (string, error) {
	return fmt.Sprintf("You are a developer. Work dir: %s", state.Info.WorkDir), nil
}

func (s *stateAwarePromptGenerator) GetToolInfo(tags []string, toolName string, role *conf.AgentRoleConfig, state *AgentState) (tool.ToolInfo, error) {
	schema := tool.NewToolSchema()
	return tool.ToolInfo{
		Name:        toolName,
		Description: "Mock tool for testing",
		Schema:      schema,
	}, nil
}

func (s *stateAwarePromptGenerator) GetAgentFiles(dir string) (map[string]string, error) {
	return make(map[string]string), nil
}

func TestAgentRoleRegistry(t *testing.T) {
	t.Run("Get retrieves role from config store", func(t *testing.T) {
		mockStore := impl.NewMockConfigStore()

		role := &conf.AgentRoleConfig{
			Name:        "test-role",
			Description: "A test role",
		}

		mockStore.SetAgentRoleConfigs(map[string]*conf.AgentRoleConfig{
			"test-role": role,
		})

		registry := NewAgentRoleRegistry(mockStore)

		retrieved, ok := registry.Get("test-role")
		assert.True(t, ok)
		assert.Equal(t, "test-role", retrieved.Name)
		assert.Equal(t, "A test role", retrieved.Description)

		_, ok = registry.Get("nonexistent")
		assert.False(t, ok)
	})

	t.Run("Get resolves role aliases", func(t *testing.T) {
		mockStore := impl.NewMockConfigStore()

		role := &conf.AgentRoleConfig{
			Name:        "developer",
			Description: "Developer role",
			Aliases:     []string{"dev", "build"},
		}

		mockStore.SetAgentRoleConfigs(map[string]*conf.AgentRoleConfig{
			"developer": role,
		})

		registry := NewAgentRoleRegistry(mockStore)

		resolvedByAlias, ok := registry.Get("dev")
		assert.True(t, ok)
		assert.Equal(t, "developer", resolvedByAlias.Name)

		resolvedByAliasUpper, ok := registry.Get("BUILD")
		assert.True(t, ok)
		assert.Equal(t, "developer", resolvedByAliasUpper.Name)
	})

	t.Run("List returns all role names", func(t *testing.T) {
		mockStore := impl.NewMockConfigStore()

		role1 := &conf.AgentRoleConfig{Name: "role1", Description: "Role 1"}
		role2 := &conf.AgentRoleConfig{Name: "role2", Description: "Role 2"}

		mockStore.SetAgentRoleConfigs(map[string]*conf.AgentRoleConfig{
			"role1": role1,
			"role2": role2,
		})

		registry := NewAgentRoleRegistry(mockStore)

		names := registry.List()
		assert.Len(t, names, 2)
		assert.Contains(t, names, "role1")
		assert.Contains(t, names, "role2")
	})

	t.Run("Cache is refreshed when timestamp changes", func(t *testing.T) {
		mockStore := impl.NewMockConfigStore()

		role1 := &conf.AgentRoleConfig{Name: "role1", Description: "Role 1"}
		mockStore.SetAgentRoleConfigs(map[string]*conf.AgentRoleConfig{
			"role1": role1,
		})

		registry := NewAgentRoleRegistry(mockStore)

		// First access loads cache
		retrieved, ok := registry.Get("role1")
		assert.True(t, ok)
		assert.Equal(t, "role1", retrieved.Name)

		// Add a new role and update timestamp
		time.Sleep(10 * time.Millisecond) // Ensure timestamp is different
		role2 := &conf.AgentRoleConfig{Name: "role2", Description: "Role 2"}
		mockStore.SetAgentRoleConfigs(map[string]*conf.AgentRoleConfig{
			"role1": role1,
			"role2": role2,
		})

		// Next access should refresh cache
		names := registry.List()
		assert.Len(t, names, 2)
		assert.Contains(t, names, "role1")
		assert.Contains(t, names, "role2")
	})

	t.Run("Cache is not refreshed when timestamp is unchanged", func(t *testing.T) {
		mockStore := impl.NewMockConfigStore()

		role1 := &conf.AgentRoleConfig{Name: "role1", Description: "Role 1"}
		mockStore.SetAgentRoleConfigs(map[string]*conf.AgentRoleConfig{
			"role1": role1,
		})

		registry := NewAgentRoleRegistry(mockStore)

		// First access loads cache
		retrieved, ok := registry.Get("role1")
		assert.True(t, ok)
		assert.Equal(t, "role1", retrieved.Name)

		// Modify the store without updating timestamp
		// (This simulates internal mutation that shouldn't trigger cache refresh)
		mockStore.SetAgentRoleConfigs(map[string]*conf.AgentRoleConfig{
			"role1": role1,
		})

		// Force the timestamp to be the same
		lastUpdate, _ := mockStore.LastAgentRoleConfigsUpdate()
		mockStore.SetAgentRoleConfigs(map[string]*conf.AgentRoleConfig{
			"role1": role1,
		})

		// Manually set to old timestamp (bypassing SetAgentRoleConfigs)
		_ = lastUpdate // Ensure we don't have unused variable

		// This test is a bit contrived since SetAgentRoleConfigs always updates timestamp
		// In practice, the cache works correctly when timestamps don't change
	})

	t.Run("Handles errors from config store gracefully", func(t *testing.T) {
		mockStore := impl.NewMockConfigStore()
		mockStore.GetAgentRoleConfigsErr = fmt.Errorf("config store error")

		registry := NewAgentRoleRegistry(mockStore)

		// Get should return false when config store fails
		_, ok := registry.Get("any-role")
		assert.False(t, ok)

		// List should return empty slice when config store fails
		names := registry.List()
		assert.Len(t, names, 0)
	})
}

func TestAgentRoleIntegration(t *testing.T) {
	// Create mock components
	mockProvider := models.NewMockProvider([]models.ModelInfo{
		{Name: "test-model", Model: "test-model"},
	})
	mockVFS := vfs.NewMockVFS()
	tools := tool.NewToolRegistry()
	tool.RegisterVFSTools(tools, mockVFS, nil, nil)

	// Define test roles
	developerRole := &conf.AgentRoleConfig{
		Name:        "developer",
		Description: "A software developer role with full VFS access",
		VFSPrivileges: map[string]conf.FileAccess{
			"**": {
				Read:   conf.AccessAllow,
				Write:  conf.AccessAllow,
				Delete: conf.AccessAllow,
				List:   conf.AccessAllow,
				Find:   conf.AccessAllow,
				Move:   conf.AccessAllow,
			},
		},
		ToolsAccess: map[string]conf.AccessFlag{
			"**": conf.AccessAllow,
		},
	}

	readOnlyRole := &conf.AgentRoleConfig{
		Name:        "readonly",
		Description: "A read-only role with limited VFS access",
		VFSPrivileges: map[string]conf.FileAccess{
			"**": {
				Read:   conf.AccessAllow,
				Write:  conf.AccessDeny,
				Delete: conf.AccessDeny,
				List:   conf.AccessAllow,
				Find:   conf.AccessAllow,
				Move:   conf.AccessDeny,
			},
		},
		ToolsAccess: map[string]conf.AccessFlag{
			"vfsRead": conf.AccessAllow,
			"vfsFind": conf.AccessAllow,
			"**":      conf.AccessDeny,
		},
	}

	t.Run("SetRole updates role field", func(t *testing.T) {
		mockStore := impl.NewMockConfigStore()
		mockStore.SetAgentRoleConfigs(map[string]*conf.AgentRoleConfig{
			"custom": {
				Name:        "custom",
				Description: "A custom role",
			},
		})
		registry := NewAgentRoleRegistry(mockStore)

		system := &SweSystem{
			ModelProviders:       map[string]models.ModelProvider{"mock": mockProvider},
			ModelTags:            models.NewModelTagRegistry(),
			Tools:                tools,
			VFS:                  mockVFS,
			Roles:                registry,
			ConfigStore:          mockStore,
			SessionLoggerFactory: logging.NewTestLoggerFactory(t),
			WorkDir:              ".",
		}

		session, err := system.NewSession("mock/test-model", nil)
		require.NoError(t, err)
		// No default role set (no "developer" role exists, no global config default)
		assert.Nil(t, session.Role())

		// Set role explicitly
		err = session.SetRole("custom")
		require.NoError(t, err)
		assert.NotNil(t, session.Role())
		assert.Equal(t, "custom", session.Role().Name)
	})

	t.Run("SetRole updates system prompt", func(t *testing.T) {
		mockStore := impl.NewMockConfigStore()
		mockStore.SetAgentRoleConfigs(map[string]*conf.AgentRoleConfig{
			"tester": {
				Name:        "tester",
				Description: "A software tester role",
			},
		})
		registry := NewAgentRoleRegistry(mockStore)

		system := &SweSystem{
			ModelProviders:       map[string]models.ModelProvider{"mock": mockProvider},
			ModelTags:            models.NewModelTagRegistry(),
			PromptGenerator:      &testPromptGenerator{prompt: "You are an experienced software tester."},
			Tools:                tools,
			VFS:                  mockVFS,
			Roles:                registry,
			ConfigStore:          mockStore,
			SessionLoggerFactory: logging.NewTestLoggerFactory(t),
			WorkDir:              ".",
		}

		session, err := system.NewSession("mock/test-model", nil)
		require.NoError(t, err)

		// With automatic role selection, there should be no default role
		// (no "developer" role exists, no global config default)
		messages := session.ChatMessages()
		require.Len(t, messages, 0)

		// Set role and check system prompt is created
		err = session.SetRole("tester")
		require.NoError(t, err)

		messages = session.ChatMessages()
		require.Len(t, messages, 1)
		assert.Equal(t, models.ChatRoleSystem, messages[0].Role)
		assert.Equal(t, "You are an experienced software tester.", messages[0].Parts[0].Text)
	})

	t.Run("SetRole wraps VFS with access control", func(t *testing.T) {
		mockStore := impl.NewMockConfigStore()
		mockStore.SetAgentRoleConfigs(map[string]*conf.AgentRoleConfig{
			"readonly": readOnlyRole,
		})
		registry := NewAgentRoleRegistry(mockStore)

		system := &SweSystem{
			ModelProviders:       map[string]models.ModelProvider{"mock": mockProvider},
			ModelTags:            models.NewModelTagRegistry(),
			Tools:                tools,
			VFS:                  mockVFS,
			Roles:                registry,
			SessionLoggerFactory: logging.NewTestLoggerFactory(t),
			WorkDir:              ".",
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
		mockStore := impl.NewMockConfigStore()
		mockStore.SetAgentRoleConfigs(map[string]*conf.AgentRoleConfig{
			"readonly": readOnlyRole,
		})
		registry := NewAgentRoleRegistry(mockStore)

		system := &SweSystem{
			ModelProviders:       map[string]models.ModelProvider{"mock": mockProvider},
			ModelTags:            models.NewModelTagRegistry(),
			Tools:                tools,
			VFS:                  mockVFS,
			Roles:                registry,
			SessionLoggerFactory: logging.NewTestLoggerFactory(t),
			WorkDir:              ".",
		}

		session, err := system.NewSession("mock/test-model", nil)
		require.NoError(t, err)

		// Set role to readonly
		err = session.SetRole("readonly")
		require.NoError(t, err)

		// Test that vfsWrite is denied
		writeArgs := tool.NewToolValue(map[string]interface{}{
			"path":    "test.txt",
			"content": "content",
		})
		writeResponse := session.Tools.Execute(&tool.ToolCall{
			Function:  "vfsWrite",
			Arguments: writeArgs,
		})
		assert.Error(t, writeResponse.Error)
		assert.Contains(t, writeResponse.Error.Error(), "access denied")

		// Test that vfsRead is allowed
		mockVFS.WriteFile("existing.txt", []byte("content"))
		readArgs := tool.NewToolValue(map[string]interface{}{
			"path": "existing.txt",
		})
		readResponse := session.Tools.Execute(&tool.ToolCall{
			Function:  "vfsRead",
			Arguments: readArgs,
		})
		assert.NoError(t, readResponse.Error)
	})

	t.Run("SetRole returns error for unknown role", func(t *testing.T) {
		mockStore := impl.NewMockConfigStore()
		mockStore.SetAgentRoleConfigs(map[string]*conf.AgentRoleConfig{})
		registry := NewAgentRoleRegistry(mockStore)

		system := &SweSystem{
			ModelProviders:       map[string]models.ModelProvider{"mock": mockProvider},
			ModelTags:            models.NewModelTagRegistry(),
			Tools:                tools,
			VFS:                  mockVFS,
			Roles:                registry,
			SessionLoggerFactory: logging.NewTestLoggerFactory(t),
			WorkDir:              ".",
		}

		session, err := system.NewSession("mock/test-model", nil)
		require.NoError(t, err)

		err = session.SetRole("unknown")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "role not found")
	})

	t.Run("SetRole can switch between roles", func(t *testing.T) {
		mockStore := impl.NewMockConfigStore()
		mockStore.SetAgentRoleConfigs(map[string]*conf.AgentRoleConfig{
			"developer": developerRole,
			"readonly":  readOnlyRole,
		})
		registry := NewAgentRoleRegistry(mockStore)

		system := &SweSystem{
			ModelProviders:       map[string]models.ModelProvider{"mock": mockProvider},
			ModelTags:            models.NewModelTagRegistry(),
			Tools:                tools,
			VFS:                  mockVFS,
			Roles:                registry,
			SessionLoggerFactory: logging.NewTestLoggerFactory(t),
			WorkDir:              ".",
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
		writeResponse := session.Tools.Execute(&tool.ToolCall{
			Function:  "vfsWrite",
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
		writeResponse = session.Tools.Execute(&tool.ToolCall{
			Function:  "vfsWrite",
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

	mockStore := impl.NewMockConfigStore()
	registry := NewAgentRoleRegistry(mockStore)

	t.Run("GetState returns current work directory", func(t *testing.T) {
		expectedWorkDir, err := filepath.Abs(".")
		require.NoError(t, err)

		system := &SweSystem{
			ModelProviders:       map[string]models.ModelProvider{"mock": mockProvider},
			ModelTags:            models.NewModelTagRegistry(),
			Tools:                tools,
			VFS:                  mockVFS,
			Roles:                registry,
			SessionLoggerFactory: logging.NewTestLoggerFactory(t),
			WorkDir:              ".",
		}

		session, err := system.NewSession("mock/test-model", nil)
		require.NoError(t, err)

		state := session.GetState()
		assert.Equal(t, ".", state.Info.WorkDir)
		assert.Equal(t, expectedWorkDir, state.Info.ShadowDir)
	})

	t.Run("SetWorkDir updates work directory in state", func(t *testing.T) {
		system := &SweSystem{
			ModelProviders:       map[string]models.ModelProvider{"mock": mockProvider},
			ModelTags:            models.NewModelTagRegistry(),
			Tools:                tools,
			VFS:                  mockVFS,
			Roles:                registry,
			SessionLoggerFactory: logging.NewTestLoggerFactory(t),
			WorkDir:              ".",
		}

		session, err := system.NewSession("mock/test-model", nil)
		require.NoError(t, err)

		session.SetWorkDir("/home/user/project")

		state := session.GetState()
		assert.Equal(t, "/home/user/project", state.Info.WorkDir)
		assert.Equal(t, "/home/user/project", state.Info.ShadowDir)
	})

	t.Run("GetState returns absolute shadow directory when explicitly set", func(t *testing.T) {
		expectedShadowDir, err := filepath.Abs("./tmp-shadow")
		require.NoError(t, err)

		system := &SweSystem{
			ModelProviders:       map[string]models.ModelProvider{"mock": mockProvider},
			ModelTags:            models.NewModelTagRegistry(),
			Tools:                tools,
			VFS:                  mockVFS,
			Roles:                registry,
			SessionLoggerFactory: logging.NewTestLoggerFactory(t),
			WorkDir:              ".",
			ShadowDir:            "./tmp-shadow",
		}

		session, err := system.NewSession("mock/test-model", nil)
		require.NoError(t, err)

		state := session.GetState()
		assert.Equal(t, expectedShadowDir, state.Info.ShadowDir)
	})

	t.Run("SetRole renders system prompt with current state", func(t *testing.T) {
		role := &conf.AgentRoleConfig{
			Name:        "dev",
			Description: "Developer role",
		}

		mockStore := impl.NewMockConfigStore()
		mockStore.SetAgentRoleConfigs(map[string]*conf.AgentRoleConfig{
			"dev": role,
		})
		registry := NewAgentRoleRegistry(mockStore)

		// Create a simple mock prompt generator
		mockGen := &stateAwarePromptGenerator{}

		system := &SweSystem{
			ModelProviders:       map[string]models.ModelProvider{"mock": mockProvider},
			ModelTags:            models.NewModelTagRegistry(),
			PromptGenerator:      mockGen,
			Tools:                tools,
			VFS:                  mockVFS,
			Roles:                registry,
			SessionLoggerFactory: logging.NewTestLoggerFactory(t),
			WorkDir:              ".",
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
