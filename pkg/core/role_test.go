package core

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/rlewczuk/csw/pkg/conf"
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

func testCswConfigWithRoles(roles map[string]*conf.AgentRoleConfig) *conf.CswConfig {
	return &conf.CswConfig{AgentRoleConfigs: roles}
}

func TestAgentRoleRegistry(t *testing.T) {
	t.Run("Get retrieves role from config", func(t *testing.T) {
		role := &conf.AgentRoleConfig{
			Name:        "test-role",
			Description: "A test role",
		}

		cfg := testCswConfigWithRoles(map[string]*conf.AgentRoleConfig{
			"test-role": role,
		})

		registry := NewAgentRoleRegistry(cfg)

		retrieved, ok := registry.Get("test-role")
		assert.True(t, ok)
		assert.Equal(t, "test-role", retrieved.Name)
		assert.Equal(t, "A test role", retrieved.Description)

		_, ok = registry.Get("nonexistent")
		assert.False(t, ok)
	})

	t.Run("Get resolves role aliases", func(t *testing.T) {
		role := &conf.AgentRoleConfig{
			Name:        "developer",
			Description: "Developer role",
			Aliases:     []string{"dev", "build"},
		}

		cfg := testCswConfigWithRoles(map[string]*conf.AgentRoleConfig{
			"developer": role,
		})

		registry := NewAgentRoleRegistry(cfg)

		resolvedByAlias, ok := registry.Get("dev")
		assert.True(t, ok)
		assert.Equal(t, "developer", resolvedByAlias.Name)

		resolvedByAliasUpper, ok := registry.Get("BUILD")
		assert.True(t, ok)
		assert.Equal(t, "developer", resolvedByAliasUpper.Name)
	})

	t.Run("List returns all role names", func(t *testing.T) {
		role1 := &conf.AgentRoleConfig{Name: "role1", Description: "Role 1"}
		role2 := &conf.AgentRoleConfig{Name: "role2", Description: "Role 2"}

		cfg := testCswConfigWithRoles(map[string]*conf.AgentRoleConfig{
			"role1": role1,
			"role2": role2,
		})

		registry := NewAgentRoleRegistry(cfg)

		names := registry.List()
		assert.Len(t, names, 2)
		assert.Contains(t, names, "role1")
		assert.Contains(t, names, "role2")
	})

	t.Run("Handles missing role config gracefully", func(t *testing.T) {
		registry := NewAgentRoleRegistry(&conf.CswConfig{})

		_, ok := registry.Get("any-role")
		assert.False(t, ok)

		names := registry.List()
		assert.Len(t, names, 0)
	})
}

func TestSweSessionGetState(t *testing.T) {
	mockProvider := models.NewMockProvider([]models.ModelInfo{
		{Name: "test-model", Model: "test-model"},
	})
	mockVFS := vfs.NewMockVFS()
	tools := tool.NewToolRegistry()

	registry := NewAgentRoleRegistry(&conf.CswConfig{})

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

		cfg := testCswConfigWithRoles(map[string]*conf.AgentRoleConfig{
			"dev": role,
		})
		registry := NewAgentRoleRegistry(cfg)

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
		assert.Contains(t, messages[0].Parts[0].Text, "/test/path")
	})
}
