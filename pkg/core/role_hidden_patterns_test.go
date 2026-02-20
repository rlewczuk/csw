package core

import (
	"testing"

	"github.com/rlewczuk/csw/pkg/conf"
	"github.com/rlewczuk/csw/pkg/conf/impl"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAgentRoleRegistry_HiddenPatternsMerging(t *testing.T) {
	t.Run("MergesAllRoleHiddenPatternsIntoSpecificRoles", func(t *testing.T) {
		// Create a mock config store with "all" role and a specific role
		mockStore := impl.NewMockConfigStore()
		mockStore.SetAgentRoleConfigs(map[string]*conf.AgentRoleConfig{
			"all": {
				Name:        "all",
				Description: "Common configuration",
				HiddenPatterns: []string{
					"node_modules/",
					".git/",
					"dist/",
				},
			},
			"developer": {
				Name:        "developer",
				Description: "Developer role",
				HiddenPatterns: []string{
					"*.log",
					"tmp/",
				},
			},
		})

		registry := NewAgentRoleRegistry(mockStore)

		// Get the developer role
		developerRole, ok := registry.Get("developer")
		require.True(t, ok, "Developer role should exist")

		// Should have patterns from both "all" and "developer" roles
		// "all" patterns should come first
		expectedPatterns := []string{
			"node_modules/",
			".git/",
			"dist/",
			"*.log",
			"tmp/",
		}
		assert.Equal(t, expectedPatterns, developerRole.HiddenPatterns)
	})

	t.Run("AllRoleItselfIsNotModified", func(t *testing.T) {
		mockStore := impl.NewMockConfigStore()
		mockStore.SetAgentRoleConfigs(map[string]*conf.AgentRoleConfig{
			"all": {
				Name:        "all",
				Description: "Common configuration",
				HiddenPatterns: []string{
					"node_modules/",
					".git/",
				},
			},
		})

		registry := NewAgentRoleRegistry(mockStore)

		// Get the "all" role
		allRole, ok := registry.Get("all")
		require.True(t, ok, "All role should exist")

		// "all" role should not have its own patterns duplicated
		expectedPatterns := []string{
			"node_modules/",
			".git/",
		}
		assert.Equal(t, expectedPatterns, allRole.HiddenPatterns)
	})

	t.Run("RoleWithoutHiddenPatternsGetsAllRolePatterns", func(t *testing.T) {
		mockStore := impl.NewMockConfigStore()
		mockStore.SetAgentRoleConfigs(map[string]*conf.AgentRoleConfig{
			"all": {
				Name:        "all",
				Description: "Common configuration",
				HiddenPatterns: []string{
					"node_modules/",
				},
			},
			"reviewer": {
				Name:           "reviewer",
				Description:    "Reviewer role",
				HiddenPatterns: nil, // No specific patterns
			},
		})

		registry := NewAgentRoleRegistry(mockStore)

		// Get the reviewer role
		reviewerRole, ok := registry.Get("reviewer")
		require.True(t, ok, "Reviewer role should exist")

		// Should only have patterns from "all" role
		expectedPatterns := []string{
			"node_modules/",
		}
		assert.Equal(t, expectedPatterns, reviewerRole.HiddenPatterns)
	})

	t.Run("NoAllRoleDoesNotCrash", func(t *testing.T) {
		mockStore := impl.NewMockConfigStore()
		mockStore.SetAgentRoleConfigs(map[string]*conf.AgentRoleConfig{
			"developer": {
				Name:        "developer",
				Description: "Developer role",
				HiddenPatterns: []string{
					"*.log",
				},
			},
		})

		registry := NewAgentRoleRegistry(mockStore)

		// Get the developer role
		developerRole, ok := registry.Get("developer")
		require.True(t, ok, "Developer role should exist")

		// Should only have its own patterns
		expectedPatterns := []string{
			"*.log",
		}
		assert.Equal(t, expectedPatterns, developerRole.HiddenPatterns)
	})

	t.Run("AllRoleWithNilPatternsDoesNotCrash", func(t *testing.T) {
		mockStore := impl.NewMockConfigStore()
		mockStore.SetAgentRoleConfigs(map[string]*conf.AgentRoleConfig{
			"all": {
				Name:           "all",
				Description:    "Common configuration",
				HiddenPatterns: nil,
			},
			"developer": {
				Name:        "developer",
				Description: "Developer role",
				HiddenPatterns: []string{
					"*.log",
				},
			},
		})

		registry := NewAgentRoleRegistry(mockStore)

		// Get the developer role
		developerRole, ok := registry.Get("developer")
		require.True(t, ok, "Developer role should exist")

		// Should only have its own patterns
		expectedPatterns := []string{
			"*.log",
		}
		assert.Equal(t, expectedPatterns, developerRole.HiddenPatterns)
	})
}
