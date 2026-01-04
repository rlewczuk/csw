package core

import (
	"testing"

	"github.com/codesnort/codesnort-swe/pkg/models"
	"github.com/codesnort/codesnort-swe/pkg/models/mock"
	"github.com/codesnort/codesnort-swe/pkg/shared"
	"github.com/codesnort/codesnort-swe/pkg/tool"
	"github.com/codesnort/codesnort-swe/pkg/vfs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAgentRoleIntegration(t *testing.T) {
	// Create mock components
	mockProvider := mock.NewMockProvider([]models.ModelInfo{
		{Name: "test-model", Model: "test-model"},
	})
	mockVFS := vfs.NewMockVFS()
	tools := tool.NewToolRegistry()
	tool.RegisterVFSTools(tools, mockVFS)

	// Define test roles
	developerRole := AgentRole{
		Name:         "developer",
		Description:  "A software developer role with full VFS access",
		SystemPrompt: "You are an experienced software developer.",
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
		Name:         "readonly",
		Description:  "A read-only role with limited VFS access",
		SystemPrompt: "You are a code reviewer with read-only access.",
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
		system := &SweSystem{
			ModelProviders: map[string]models.ModelProvider{"mock": mockProvider},
			Tools:          tools,
			VFS:            mockVFS,
			Roles: map[string]AgentRole{
				"developer": developerRole,
				"readonly":  readOnlyRole,
			},
		}

		session, err := system.NewSession("mock/test-model")
		require.NoError(t, err)
		assert.Nil(t, session.Role())

		err = session.SetRole("developer")
		require.NoError(t, err)
		assert.NotNil(t, session.Role())
		assert.Equal(t, "developer", session.Role().Name)
	})

	t.Run("SetRole updates system prompt", func(t *testing.T) {
		system := &SweSystem{
			ModelProviders: map[string]models.ModelProvider{"mock": mockProvider},
			SystemPrompt:   "Initial system prompt",
			Tools:          tools,
			VFS:            mockVFS,
			Roles: map[string]AgentRole{
				"developer": developerRole,
			},
		}

		session, err := system.NewSession("mock/test-model")
		require.NoError(t, err)

		// Check initial system prompt
		messages := session.ChatMessages()
		require.Len(t, messages, 1)
		assert.Equal(t, models.ChatRoleSystem, messages[0].Role)
		assert.Equal(t, "Initial system prompt", messages[0].Parts[0].Text)

		// Set role and check updated system prompt
		err = session.SetRole("developer")
		require.NoError(t, err)

		messages = session.ChatMessages()
		require.Len(t, messages, 1)
		assert.Equal(t, models.ChatRoleSystem, messages[0].Role)
		assert.Equal(t, "You are an experienced software developer.", messages[0].Parts[0].Text)
	})

	t.Run("SetRole wraps VFS with access control", func(t *testing.T) {
		system := &SweSystem{
			ModelProviders: map[string]models.ModelProvider{"mock": mockProvider},
			Tools:          tools,
			VFS:            mockVFS,
			Roles: map[string]AgentRole{
				"readonly": readOnlyRole,
			},
		}

		session, err := system.NewSession("mock/test-model")
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
		system := &SweSystem{
			ModelProviders: map[string]models.ModelProvider{"mock": mockProvider},
			Tools:          tools,
			VFS:            mockVFS,
			Roles: map[string]AgentRole{
				"readonly": readOnlyRole,
			},
		}

		session, err := system.NewSession("mock/test-model")
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
		system := &SweSystem{
			ModelProviders: map[string]models.ModelProvider{"mock": mockProvider},
			Tools:          tools,
			VFS:            mockVFS,
			Roles:          map[string]AgentRole{},
		}

		session, err := system.NewSession("mock/test-model")
		require.NoError(t, err)

		err = session.SetRole("unknown")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "role not found")
	})

	t.Run("SetRole can switch between roles", func(t *testing.T) {
		system := &SweSystem{
			ModelProviders: map[string]models.ModelProvider{"mock": mockProvider},
			Tools:          tools,
			VFS:            mockVFS,
			Roles: map[string]AgentRole{
				"developer": developerRole,
				"readonly":  readOnlyRole,
			},
		}

		session, err := system.NewSession("mock/test-model")
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
