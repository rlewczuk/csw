package tui

import (
	"strings"
	"testing"
	"time"

	"github.com/codesnort/codesnort-swe/pkg/conf"
	"github.com/codesnort/codesnort-swe/pkg/core"
	"github.com/codesnort/codesnort-swe/pkg/gtv"
	"github.com/codesnort/codesnort-swe/pkg/gtv/tio"
	"github.com/codesnort/codesnort-swe/pkg/gtv/tui"
	"github.com/codesnort/codesnort-swe/pkg/logging"
	"github.com/codesnort/codesnort-swe/pkg/models"
	"github.com/codesnort/codesnort-swe/pkg/presenter"
	"github.com/codesnort/codesnort-swe/pkg/testutil"
	"github.com/codesnort/codesnort-swe/pkg/tool"
	"github.com/codesnort/codesnort-swe/pkg/vfs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Mock prompt generator for TAppView tests
type appViewMockPromptGen struct{}

func (m *appViewMockPromptGen) GetPrompt(tags []string, role *conf.AgentRoleConfig, state *core.AgentState) (string, error) {
	return "You are a helpful assistant.", nil
}

func TestAppViewWithChatIntegration(t *testing.T) {
	t.Run("chat response appears in app view without user interaction", func(t *testing.T) {
		// Setup mock LLM server
		mockServer := testutil.NewMockHTTPServer()
		defer mockServer.Close()

		client, err := models.NewOllamaClientWithHTTPClient(mockServer.URL(), mockServer.Client())
		require.NoError(t, err)

		vfsInstance := vfs.NewMockVFS()
		tools := tool.NewToolRegistry()
		tool.RegisterVFSTools(tools, vfsInstance)

		system := &core.SweSystem{
			ModelProviders:  map[string]models.ModelProvider{"ollama": client},
			PromptGenerator: &appViewMockPromptGen{},
			Tools:           tools,
			VFS:             vfsInstance,
			SessionLoggerFactory: logging.NewTestLoggerFactory(t),
		}

		// Create session thread
		thread := core.NewSessionThread(system, nil)
		err = thread.StartSession("ollama/test-model:latest")
		require.NoError(t, err)

		// Create presenters
		appPresenter := presenter.NewAppPresenter(system, "ollama/test-model:latest", "")
		chatPresenter := presenter.NewChatPresenter(system, thread)

		// Create TAppView
		screen := tio.NewScreenBuffer(80, 24, 0)
		appView := NewAppView(nil, gtv.TRect{X: 0, Y: 0, W: 80, H: 24}, appPresenter)

		// Create application
		app := tui.NewApplication(appView, screen)
		require.NotNil(t, app)

		// Show chat view through app view
		chatView := appView.ShowChat(chatPresenter)
		require.NotNil(t, chatView)

		// Connect chat presenter to view
		err = chatPresenter.SetView(chatView)
		require.NoError(t, err)

		// Setup LLM response
		mockServer.AddStreamingResponse("/api/chat", "POST", true,
			`{"model":"test-model:latest","created_at":"2024-01-01T00:00:00Z","message":{"role":"assistant","content":"Hello from LLM!"},"done":false}`,
			`{"model":"test-model:latest","created_at":"2024-01-01T00:00:01Z","message":{"role":"assistant"},"done":true,"done_reason":"stop"}`,
		)

		// Create mock input reader
		mockInput := tio.NewMockInputEventReader(app)

		// Draw initial frame
		app.ExecuteOnUiThread(func() {
			appView.Draw(screen)
		})

		// Type a message and submit
		mockInput.TypeKeys("Hello, assistant!")
		mockInput.TypeKeysByName("Alt+Enter")

		// Wait a bit for async processing
		//time.Sleep(200 * time.Millisecond)

		// Verify user message appeared
		app.ExecuteOnUiThread(func() {
			appView.Draw(screen)
		})

		width, height, content := screen.GetContent()
		verifier := gtv.NewScreenVerifier(width, height, content)

		// Check for user message
		fullText := verifier.GetText(0, 0, width, height)
		assert.Contains(t, fullText, "Hello, assistant!", "Should show user message")

		// Wait for assistant response WITHOUT any user interaction
		timeout := time.After(5 * time.Second)
		ticker := time.NewTicker(10 * time.Millisecond)
		defer ticker.Stop()

		assistantResponseFound := false
		for !assistantResponseFound {
			select {
			case <-timeout:
				t.Fatal("Timeout waiting for assistant response")
			case <-ticker.C:
				app.ExecuteOnUiThread(func() {
					appView.Draw(screen)
				})

				width, height, content := screen.GetContent()
				verifier := gtv.NewScreenVerifier(width, height, content)
				fullText := verifier.GetText(0, 0, width, height)

				if strings.Contains(fullText, "Hello from LLM!") {
					assistantResponseFound = true
				}
			}
		}

		assert.True(t, assistantResponseFound, "Should show assistant response without requiring user interaction")
	})

	t.Run("streaming response updates appear automatically in app view", func(t *testing.T) {
		// Setup mock LLM server
		mockServer := testutil.NewMockHTTPServer()
		defer mockServer.Close()

		client, err := models.NewOllamaClientWithHTTPClient(mockServer.URL(), mockServer.Client())
		require.NoError(t, err)

		vfsInstance := vfs.NewMockVFS()
		tools := tool.NewToolRegistry()
		tool.RegisterVFSTools(tools, vfsInstance)

		system := &core.SweSystem{
			ModelProviders:  map[string]models.ModelProvider{"ollama": client},
			PromptGenerator: &appViewMockPromptGen{},
			Tools:           tools,
			VFS:             vfsInstance,
			SessionLoggerFactory: logging.NewTestLoggerFactory(t),
		}

		// Create session thread
		thread := core.NewSessionThread(system, nil)
		err = thread.StartSession("ollama/test-model:latest")
		require.NoError(t, err)

		// Create presenters
		appPresenter := presenter.NewAppPresenter(system, "ollama/test-model:latest", "")
		chatPresenter := presenter.NewChatPresenter(system, thread)

		// Create TAppView
		screen := tio.NewScreenBuffer(80, 24, 0)
		appView := NewAppView(nil, gtv.TRect{X: 0, Y: 0, W: 80, H: 24}, appPresenter)

		// Create application
		app := tui.NewApplication(appView, screen)
		require.NotNil(t, app)

		// Show chat view through app view
		chatView := appView.ShowChat(chatPresenter)
		require.NotNil(t, chatView)

		// Connect chat presenter to view
		err = chatPresenter.SetView(chatView)
		require.NoError(t, err)

		// Setup LLM response with multiple chunks
		mockServer.AddStreamingResponse("/api/chat", "POST", true,
			`{"model":"test-model:latest","created_at":"2024-01-01T00:00:00Z","message":{"role":"assistant","content":"First chunk "},"done":false}`,
			`{"model":"test-model:latest","created_at":"2024-01-01T00:00:00Z","message":{"role":"assistant","content":"second chunk "},"done":false}`,
			`{"model":"test-model:latest","created_at":"2024-01-01T00:00:00Z","message":{"role":"assistant","content":"third chunk."},"done":false}`,
			`{"model":"test-model:latest","created_at":"2024-01-01T00:00:01Z","message":{"role":"assistant"},"done":true,"done_reason":"stop"}`,
		)

		// Create mock input reader
		mockInput := tio.NewMockInputEventReader(app)

		// Draw initial frame
		app.ExecuteOnUiThread(func() {
			appView.Draw(screen)
		})

		// Type a message and submit
		mockInput.TypeKeys("Tell me something")
		mockInput.TypeKeysByName("Alt+Enter")

		// Wait for complete streaming response without user interaction
		timeout := time.After(5 * time.Second)
		ticker := time.NewTicker(10 * time.Millisecond)
		defer ticker.Stop()

		completeResponseFound := false
		for !completeResponseFound {
			select {
			case <-timeout:
				t.Fatal("Timeout waiting for complete streamed response")
			case <-ticker.C:
				app.ExecuteOnUiThread(func() {
					appView.Draw(screen)
				})

				width, height, content := screen.GetContent()
				verifier := gtv.NewScreenVerifier(width, height, content)
				fullText := verifier.GetText(0, 0, width, height)

				if strings.Contains(fullText, "First chunk second chunk third chunk.") {
					completeResponseFound = true
				}
			}
		}

		assert.True(t, completeResponseFound, "Should show complete streamed response")
	})

	t.Run("user message should not be duplicated in app view", func(t *testing.T) {
		// Setup mock LLM server
		mockServer := testutil.NewMockHTTPServer()
		defer mockServer.Close()

		client, err := models.NewOllamaClientWithHTTPClient(mockServer.URL(), mockServer.Client())
		require.NoError(t, err)

		vfsInstance := vfs.NewMockVFS()
		tools := tool.NewToolRegistry()
		tool.RegisterVFSTools(tools, vfsInstance)

		system := &core.SweSystem{
			ModelProviders:  map[string]models.ModelProvider{"ollama": client},
			PromptGenerator: &appViewMockPromptGen{},
			Tools:           tools,
			VFS:             vfsInstance,
			SessionLoggerFactory: logging.NewTestLoggerFactory(t),
		}

		// Create session thread
		thread := core.NewSessionThread(system, nil)
		err = thread.StartSession("ollama/test-model:latest")
		require.NoError(t, err)

		// Create presenters
		appPresenter := presenter.NewAppPresenter(system, "ollama/test-model:latest", "")
		chatPresenter := presenter.NewChatPresenter(system, thread)

		// Create TAppView
		screen := tio.NewScreenBuffer(80, 24, 0)
		appView := NewAppView(nil, gtv.TRect{X: 0, Y: 0, W: 80, H: 24}, appPresenter)

		// Create application
		app := tui.NewApplication(appView, screen)
		require.NotNil(t, app)

		// Show chat view through app view
		chatView := appView.ShowChat(chatPresenter)
		require.NotNil(t, chatView)

		// Connect chat presenter to view
		err = chatPresenter.SetView(chatView)
		require.NoError(t, err)

		// Setup LLM response
		mockServer.AddStreamingResponse("/api/chat", "POST", true,
			`{"model":"test-model:latest","created_at":"2024-01-01T00:00:00Z","message":{"role":"assistant","content":"Response received"},"done":true,"done_reason":"stop"}`,
		)

		// Create mock input reader
		mockInput := tio.NewMockInputEventReader(app)

		// Draw initial frame
		app.ExecuteOnUiThread(func() {
			appView.Draw(screen)
		})

		// Type a unique message
		uniqueUserMessage := "This is my unique app view test message"
		mockInput.TypeKeys(uniqueUserMessage)
		mockInput.TypeKeysByName("Alt+Enter")

		// Wait for assistant response
		//time.Sleep(200 * time.Millisecond)

		// Redraw and check for duplication
		app.ExecuteOnUiThread(func() {
			appView.Draw(screen)
		})

		width, height, content := screen.GetContent()
		verifier := gtv.NewScreenVerifier(width, height, content)
		fullText := verifier.GetText(0, 0, width, height)

		// Count occurrences of the user message
		count := strings.Count(fullText, uniqueUserMessage)

		// The message should appear exactly once
		assert.Equal(t, 1, count, "User message should appear exactly once, but appeared %d times", count)
	})
}

func TestAppViewMenuInteraction(t *testing.T) {
	t.Run("Ctrl+P shows menu", func(t *testing.T) {
		// Setup mock LLM server
		mockServer := testutil.NewMockHTTPServer()
		defer mockServer.Close()

		client, err := models.NewOllamaClientWithHTTPClient(mockServer.URL(), mockServer.Client())
		require.NoError(t, err)

		vfsInstance := vfs.NewMockVFS()
		tools := tool.NewToolRegistry()
		tool.RegisterVFSTools(tools, vfsInstance)

		system := &core.SweSystem{
			ModelProviders:  map[string]models.ModelProvider{"ollama": client},
			PromptGenerator: &appViewMockPromptGen{},
			Tools:           tools,
			VFS:             vfsInstance,
			SessionLoggerFactory: logging.NewTestLoggerFactory(t),
		}

		// Create session thread
		thread := core.NewSessionThread(system, nil)
		err = thread.StartSession("ollama/test-model:latest")
		require.NoError(t, err)

		// Create presenters
		appPresenter := presenter.NewAppPresenter(system, "ollama/test-model:latest", "")
		chatPresenter := presenter.NewChatPresenter(system, thread)

		// Create TAppView
		screen := tio.NewScreenBuffer(80, 24, 0)
		appView := NewAppView(nil, gtv.TRect{X: 0, Y: 0, W: 80, H: 24}, appPresenter)

		// Create application
		app := tui.NewApplication(appView, screen)
		require.NotNil(t, app)

		// Show chat view through app view
		chatView := appView.ShowChat(chatPresenter)
		require.NotNil(t, chatView)

		// Connect chat presenter to view
		err = chatPresenter.SetView(chatView)
		require.NoError(t, err)

		// Create mock input reader
		mockInput := tio.NewMockInputEventReader(app)

		// Draw initial frame
		app.ExecuteOnUiThread(func() {
			appView.Draw(screen)
		})

		// Press Ctrl+P to show menu
		mockInput.TypeKeysByName("Ctrl+P")

		// Redraw
		app.ExecuteOnUiThread(func() {
			appView.Draw(screen)
		})

		width, height, content := screen.GetContent()
		verifier := gtv.NewScreenVerifier(width, height, content)
		fullText := verifier.GetText(0, 0, width, height)

		assert.Contains(t, fullText, "Main Menu", "Menu should appear")
		assert.Contains(t, fullText, "New Session", "Menu should show New Session option")
		assert.Contains(t, fullText, "Exit", "Menu should show Exit option")
		assert.True(t, appView.showingMenu, "showingMenu flag should be true")
	})

	t.Run("Esc key should open menu", func(t *testing.T) {
		// Setup mock LLM server
		mockServer := testutil.NewMockHTTPServer()
		defer mockServer.Close()

		client, err := models.NewOllamaClientWithHTTPClient(mockServer.URL(), mockServer.Client())
		require.NoError(t, err)

		vfsInstance := vfs.NewMockVFS()
		tools := tool.NewToolRegistry()
		tool.RegisterVFSTools(tools, vfsInstance)

		system := &core.SweSystem{
			ModelProviders:  map[string]models.ModelProvider{"ollama": client},
			PromptGenerator: &appViewMockPromptGen{},
			Tools:           tools,
			VFS:             vfsInstance,
			SessionLoggerFactory: logging.NewTestLoggerFactory(t),
		}

		// Create session thread
		thread := core.NewSessionThread(system, nil)
		err = thread.StartSession("ollama/test-model:latest")
		require.NoError(t, err)

		// Create presenters
		appPresenter := presenter.NewAppPresenter(system, "ollama/test-model:latest", "")
		chatPresenter := presenter.NewChatPresenter(system, thread)

		// Create TAppView
		screen := tio.NewScreenBuffer(80, 24, 0)
		appView := NewAppView(nil, gtv.TRect{X: 0, Y: 0, W: 80, H: 24}, appPresenter)

		// Create application
		app := tui.NewApplication(appView, screen)
		require.NotNil(t, app)

		// Show chat view through app view
		chatView := appView.ShowChat(chatPresenter)
		require.NotNil(t, chatView)

		// Connect chat presenter to view
		err = chatPresenter.SetView(chatView)
		require.NoError(t, err)

		// Create mock input reader
		mockInput := tio.NewMockInputEventReader(app)

		// Draw initial frame
		app.ExecuteOnUiThread(func() {
			appView.Draw(screen)
		})

		// Press Esc to show menu
		mockInput.TypeKeysByName("Escape")

		// Redraw
		app.ExecuteOnUiThread(func() {
			appView.Draw(screen)
		})

		width, height, content := screen.GetContent()
		verifier := gtv.NewScreenVerifier(width, height, content)
		fullText := verifier.GetText(0, 0, width, height)

		assert.Contains(t, fullText, "Main Menu", "Esc should open main menu")
		assert.True(t, appView.showingMenu, "showingMenu flag should be true")
	})
}
