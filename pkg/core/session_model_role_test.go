package core

import (
	"testing"

	"github.com/rlewczuk/csw/pkg/apis"
	"github.com/rlewczuk/csw/pkg/conf"
	"github.com/rlewczuk/csw/pkg/conf/impl"
	"github.com/rlewczuk/csw/pkg/logging"
	"github.com/rlewczuk/csw/pkg/models"
	"github.com/rlewczuk/csw/pkg/tool"
	"github.com/rlewczuk/csw/pkg/vfs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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
		assert.Nil(t, session.Role())

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

		messages := session.ChatMessages()
		require.Len(t, messages, 0)

		err = session.SetRole("tester")
		require.NoError(t, err)

		messages = session.ChatMessages()
		require.Len(t, messages, 1)
		assert.Equal(t, models.ChatRoleSystem, messages[0].Role)
		assert.Equal(t, "You are an experienced software tester.", messages[0].Parts[0].Text)
	})

	t.Run("SetRole wraps VFS with access control", func(t *testing.T) {
		mockStore := impl.NewMockConfigStore()
		mockStore.SetAgentRoleConfigs(map[string]*conf.AgentRoleConfig{"readonly": readOnlyRole})
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

		assert.Equal(t, mockVFS, session.VFS)

		err = session.SetRole("readonly")
		require.NoError(t, err)

		assert.NotEqual(t, mockVFS, session.VFS)
		_, ok := session.VFS.(*vfs.AccessControlVFS)
		assert.True(t, ok, "VFS should be wrapped with AccessControlVFS")

		err = session.VFS.WriteFile("test.txt", []byte("content"))
		assert.Error(t, err)
		assert.Equal(t, apis.ErrPermissionDenied, err)

		mockVFS.WriteFile("existing.txt", []byte("content"))
		_, err = session.VFS.ReadFile("existing.txt")
		assert.NoError(t, err)
	})

	t.Run("SetRole keeps base VFS when allow all permissions enabled", func(t *testing.T) {
		mockStore := impl.NewMockConfigStore()
		mockStore.SetAgentRoleConfigs(map[string]*conf.AgentRoleConfig{"readonly": readOnlyRole})
		registry := NewAgentRoleRegistry(mockStore)

		session := NewSweSession(&SweSessionParams{
			Provider:            mockProvider,
			ProviderName:        "mock",
			Model:               "test-model",
			VFS:                 mockVFS,
			BaseVFS:             mockVFS,
			SystemTools:         tools,
			ModelProviders:      map[string]models.ModelProvider{"mock": mockProvider},
			ModelTags:           models.NewModelTagRegistry(),
			Roles:               registry,
			AllowAllPermissions: true,
		})

		err := session.SetRole("readonly")
		require.NoError(t, err)

		assert.Equal(t, mockVFS, session.VFS)
		_, ok := session.VFS.(*vfs.AccessControlVFS)
		assert.False(t, ok)

		err = session.VFS.WriteFile("test.txt", []byte("content"))
		assert.NoError(t, err)
	})

	t.Run("SetRole wraps tools with access control", func(t *testing.T) {
		mockStore := impl.NewMockConfigStore()
		mockStore.SetAgentRoleConfigs(map[string]*conf.AgentRoleConfig{"readonly": readOnlyRole})
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

		err = session.SetRole("readonly")
		require.NoError(t, err)

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

		mockVFS.WriteFile("existing.txt", []byte("content"))
		readArgs := tool.NewToolValue(map[string]interface{}{"path": "existing.txt"})
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

		err = session.SetRole("developer")
		require.NoError(t, err)
		assert.Equal(t, "developer", session.Role().Name)

		writeArgs1 := tool.NewToolValue(map[string]interface{}{
			"path":    "test.txt",
			"content": "content",
		})
		writeResponse := session.Tools.Execute(&tool.ToolCall{
			Function:  "vfsWrite",
			Arguments: writeArgs1,
		})
		assert.NoError(t, writeResponse.Error)

		err = session.SetRole("readonly")
		require.NoError(t, err)
		assert.Equal(t, "readonly", session.Role().Name)

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
