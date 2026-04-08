package system_test

import (
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"github.com/rlewczuk/csw/pkg/conf"
	"github.com/rlewczuk/csw/pkg/conf/impl"
	"github.com/rlewczuk/csw/pkg/core"
	coretestfixture "github.com/rlewczuk/csw/pkg/core/testfixture"
	"github.com/rlewczuk/csw/pkg/shared"
	"github.com/rlewczuk/csw/pkg/system"
	"github.com/rlewczuk/csw/pkg/testutil"
	"github.com/rlewczuk/csw/pkg/tool"
	"github.com/rlewczuk/csw/pkg/ui"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type runtimeTestChatView struct{}

func (v *runtimeTestChatView) Init(_ *ui.ChatSessionUI) error                { return nil }
func (v *runtimeTestChatView) AddMessage(_ *ui.ChatMessageUI) error          { return nil }
func (v *runtimeTestChatView) UpdateMessage(_ *ui.ChatMessageUI) error       { return nil }
func (v *runtimeTestChatView) UpdateTool(_ *ui.ToolUI) error                 { return nil }
func (v *runtimeTestChatView) MoveToBottom() error                           { return nil }
func (v *runtimeTestChatView) QueryPermission(_ *ui.PermissionQueryUI) error { return nil }
func (v *runtimeTestChatView) ShowMessage(_ string, _ shared.MessageType)    {}
func (v *runtimeTestChatView) SetSessionLogger(_ *slog.Logger)               {}
func (v *runtimeTestChatView) StartReadingInput()                            {}

type runtimeTestChatPresenter struct{}

func (p *runtimeTestChatPresenter) SetView(_ ui.IChatView) error                   { return nil }
func (p *runtimeTestChatPresenter) SendUserMessage(_ *ui.ChatMessageUI) error      { return nil }
func (p *runtimeTestChatPresenter) SaveUserMessage(_ *ui.ChatMessageUI) error      { return nil }
func (p *runtimeTestChatPresenter) Pause() error                                   { return nil }
func (p *runtimeTestChatPresenter) Resume() error                                  { return nil }
func (p *runtimeTestChatPresenter) PermissionResponse(_ string) error              { return nil }
func (p *runtimeTestChatPresenter) SetModel(_ string) error                        { return nil }
func (p *runtimeTestChatPresenter) AddAssistantMessage(_ string, _ string)         {}
func (p *runtimeTestChatPresenter) ShowMessage(_ string, _ string)                 {}
func (p *runtimeTestChatPresenter) AddToolCall(_ *tool.ToolCall)                   {}
func (p *runtimeTestChatPresenter) AddToolCallResult(_ *tool.ToolResponse)         {}
func (p *runtimeTestChatPresenter) OnPermissionQuery(_ *tool.ToolPermissionsQuery) {}
func (p *runtimeTestChatPresenter) OnRateLimitError(_ int)                         {}
func (p *runtimeTestChatPresenter) ShouldRetryAfterFailure(_ string) bool          { return false }
func (p *runtimeTestChatPresenter) RunFinished(_ error)                            {}

func TestStartCLISessionResumeAppliesOverridesAndForceCompact(t *testing.T) {
	tmpDir := filepath.Join("../../tmp", "runtime_resume", t.Name())
	require.NoError(t, os.MkdirAll(tmpDir, 0755))
	defer os.RemoveAll(tmpDir)

	configStore := impl.NewMockConfigStore()
	configStore.SetAgentRoleConfigs(map[string]*conf.AgentRoleConfig{
		"developer": {Name: "developer", Description: "Developer"},
		"reviewer":  {Name: "reviewer", Description: "Reviewer"},
	})
	roles := core.NewAgentRoleRegistry(configStore)

	fixture := coretestfixture.NewSweSystemFixture(
		t,
		coretestfixture.WithLogBaseDir(tmpDir),
		coretestfixture.WithRoles(roles),
		coretestfixture.WithConfigStore(configStore),
	)
	sweSystem := fixture.System

	original, err := sweSystem.NewSession("ollama/test-model:latest", testutil.NewMockSessionOutputHandler())
	require.NoError(t, err)
	require.NoError(t, original.SetRole("developer"))
	original.SetThinkingLevel("low")
	original.SetTodoList([]tool.TodoItem{{
		ID:       "0195d6da-4ca1-7a57-a17a-f00000000012",
		Content:  "keep this todo",
		Status:   "pending",
		Priority: "high",
	}})
	for i := 0; i < 20; i++ {
		require.NoError(t, original.UserPrompt("message to compact"))
	}
	initialCompactions := original.CompactionCount()
	sessionID := original.ID()

	sweSystem.Shutdown()

	result, err := sweSystem.StartCLISession(system.StartCLISessionParams{
		ModelName:          "ollama/override-model",
		RoleName:           "reviewer",
		Thinking:           "high",
		ModelOverridden:    true,
		RoleOverridden:     true,
		ThinkingOverridden: true,
		Prompt:             "continue",
		ResumeTarget:       sessionID,
		ContinueSession:    true,
		ForceCompact:       true,
		ChatOutput:         io.Discard,
		ChatInput:          nil,
		ChatPresenterFactory: func(_ core.SessionFactory, _ *core.SessionThread) system.ChatPresenter {
			return &runtimeTestChatPresenter{}
		},
		ChatViewFactory: func(_ ui.IChatPresenter, _ io.Writer, _ io.Reader, _ bool, _ bool, _ string) system.ChatView {
			return &runtimeTestChatView{}
		},
	})
	require.NoError(t, err)
	require.NotNil(t, result.Session)

	loaded := result.Session
	assert.Equal(t, "ollama", loaded.ProviderName())
	assert.Equal(t, "override-model", loaded.Model())
	require.NotNil(t, loaded.Role())
	assert.Equal(t, "reviewer", loaded.Role().Name)
	assert.Equal(t, "high", loaded.ThinkingLevel())
	assert.Equal(t, "keep this todo", loaded.GetTodoList()[0].Content)
	assert.Greater(t, loaded.CompactionCount(), initialCompactions)
}
