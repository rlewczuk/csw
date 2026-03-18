package system_test

import (
	"context"
	"testing"

	"github.com/rlewczuk/csw/pkg/conf"
	confimpl "github.com/rlewczuk/csw/pkg/conf/impl"
	"github.com/rlewczuk/csw/pkg/core"
	coretestfixture "github.com/rlewczuk/csw/pkg/core/testfixture"
	"github.com/rlewczuk/csw/pkg/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHookEngineExecuteSubAgentHookIntegrationInheritsParentRole(t *testing.T) {
	configStore := confimpl.NewMockConfigStore()
	configStore.SetHookConfigs(map[string]*conf.HookConfig{
		"summary-subagent": {
			Name:         "summary-subagent",
			Hook:         "summary",
			Enabled:      true,
			Type:         conf.HookTypeSubAgent,
			Prompt:       "Prompt: {{.user_prompt}}",
			SystemPrompt: "System: {{.status}}",
			OutputTo:     "summary_out",
		},
	})
	configStore.SetAgentRoleConfigs(map[string]*conf.AgentRoleConfig{
		"developer": {Name: "developer", Description: "dev"},
	})

	roles := core.NewAgentRoleRegistry(configStore)
	fixture := coretestfixture.NewSweSystemFixture(t, coretestfixture.WithConfigStore(configStore), coretestfixture.WithRoles(roles))

	fixture.Server.AddStreamingResponse("/api/chat", "POST", true,
		`{"model":"test-model:latest","created_at":"2024-01-01T00:00:00Z","message":{"role":"assistant","content":"Child completed."},"done":false}`,
		`{"model":"test-model:latest","created_at":"2024-01-01T00:00:01Z","message":{"role":"assistant"},"done":true,"done_reason":"stop"}`,
	)

	parent, err := fixture.System.NewSession("ollama/test-model:latest", testutil.NewMockSessionOutputHandler())
	require.NoError(t, err)
	require.NoError(t, parent.SetRole("developer"))

	engine := core.NewHookEngine(configStore, nil, nil, fixture.System.ModelProviders)
	engine.MergeContext(map[string]string{"user_prompt": "Initial request", "status": "running"})

	result, err := engine.Execute(context.Background(), core.HookExecutionRequest{Name: "summary", Session: parent})
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, "Child completed.", result.Stdout)
	assert.Equal(t, "Child completed.", engine.ContextData()["summary_out"])

	response := core.FindHookResponseRequest(result)
	require.NotNil(t, response)
	assert.Equal(t, "OK", core.HookResponseStatus(response))

	var child *core.SweSession
	for _, session := range fixture.System.ListSessions() {
		if session.ParentID() == parent.ID() {
			child = session
			break
		}
	}
	require.NotNil(t, child)
	require.NotNil(t, child.Role())
	assert.Equal(t, "developer", child.Role().Name)
}

func TestHookEngineExecuteSubAgentHookIntegrationConfiguredRoleAndThinking(t *testing.T) {
	configStore := confimpl.NewMockConfigStore()
	configStore.SetHookConfigs(map[string]*conf.HookConfig{
		"summary-subagent": {
			Name:     "summary-subagent",
			Hook:     "summary",
			Enabled:  true,
			Type:     conf.HookTypeSubAgent,
			Prompt:   "Prompt",
			Role:     "reviewer",
			Thinking: "high",
		},
	})
	configStore.SetAgentRoleConfigs(map[string]*conf.AgentRoleConfig{
		"developer": {Name: "developer", Description: "dev"},
		"reviewer":  {Name: "reviewer", Description: "reviewer"},
	})

	roles := core.NewAgentRoleRegistry(configStore)
	fixture := coretestfixture.NewSweSystemFixture(t, coretestfixture.WithConfigStore(configStore), coretestfixture.WithRoles(roles))

	fixture.Server.AddStreamingResponse("/api/chat", "POST", true,
		`{"model":"test-model:latest","created_at":"2024-01-01T00:00:00Z","message":{"role":"assistant","content":"Done."},"done":false}`,
		`{"model":"test-model:latest","created_at":"2024-01-01T00:00:01Z","message":{"role":"assistant"},"done":true,"done_reason":"stop"}`,
	)

	parent, err := fixture.System.NewSession("ollama/test-model:latest", testutil.NewMockSessionOutputHandler())
	require.NoError(t, err)
	require.NoError(t, parent.SetRole("developer"))

	engine := core.NewHookEngine(configStore, nil, nil, fixture.System.ModelProviders)
	result, err := engine.Execute(context.Background(), core.HookExecutionRequest{Name: "summary", Session: parent})
	require.NoError(t, err)
	require.NotNil(t, result)

	var child *core.SweSession
	for _, session := range fixture.System.ListSessions() {
		if session.ParentID() == parent.ID() {
			child = session
			break
		}
	}
	require.NotNil(t, child)
	require.NotNil(t, child.Role())
	assert.Equal(t, "reviewer", child.Role().Name)
	assert.Equal(t, "high", child.ThinkingLevel())
}
