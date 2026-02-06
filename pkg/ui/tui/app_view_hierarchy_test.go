package tui

import (
	"fmt"
	"testing"

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

// hierarchyMockPromptGen is a mock prompt generator for hierarchy tests
type hierarchyMockPromptGen struct{}

func (m *hierarchyMockPromptGen) GetPrompt(tags []string, role *conf.AgentRoleConfig, state *core.AgentState) (string, error) {
	return "You are a helpful assistant.", nil
}

func (m *hierarchyMockPromptGen) GetToolInfo(tags []string, toolName string, role *conf.AgentRoleConfig, state *core.AgentState) (tool.ToolInfo, error) {
	schema := tool.NewToolSchema()
	return tool.ToolInfo{
		Name:        toolName,
		Description: "Mock tool for testing",
		Schema:      schema,
	}, nil
}

func (m *hierarchyMockPromptGen) GetAgentFiles(dir string) (map[string]string, error) {
	return make(map[string]string), nil
}

// TestAppViewWidgetHierarchy verifies that all widgets in the TAppView hierarchy
// have their Parent fields set correctly.
func TestAppViewWidgetHierarchy(t *testing.T) {
	// Setup mock LLM server
	mockServer := testutil.NewMockHTTPServer()
	defer mockServer.Close()

	client, err := models.NewOllamaClientWithHTTPClient(mockServer.URL(), mockServer.Client())
	require.NoError(t, err)

	vfsInstance := vfs.NewMockVFS()
	tools := tool.NewToolRegistry()
	tool.RegisterVFSTools(tools, vfsInstance, nil)

	system := &core.SweSystem{
		ModelProviders:       map[string]models.ModelProvider{"ollama": client},
		ModelTags:            models.NewModelTagRegistry(),
		PromptGenerator:      &hierarchyMockPromptGen{},
		Tools:                tools,
		VFS:                  vfsInstance,
		SessionLoggerFactory: logging.NewTestLoggerFactory(t),
		WorkDir:              ".",
	}

	// Create session thread
	thread := core.NewSessionThread(system, nil)
	err = thread.StartSession("ollama/test-model:latest")
	require.NoError(t, err)

	// Create presenters
	appPresenter := presenter.NewAppPresenter(system, "ollama/test-model:latest", "")
	chatPresenter := presenter.NewChatPresenter(system, thread)

	// Create TAppView (similar to main.go)
	screen := tio.NewScreenBuffer(80, 24, 0)
	appView := NewAppView(nil, gtv.TRect{X: 0, Y: 0, W: 80, H: 24}, appPresenter)

	// Create application
	app := tui.NewApplication(appView, screen)
	require.NotNil(t, app)

	// Set the view on the presenter (similar to main.go)
	err = appPresenter.SetView(appView)
	require.NoError(t, err)

	// Show chat view through app view (similar to what happens in integration tests)
	chatView := appView.ShowChat(chatPresenter)
	require.NotNil(t, chatView)

	// Connect chat presenter to view
	err = chatPresenter.SetView(chatView)
	require.NoError(t, err)

	// Now verify the entire widget hierarchy has correct Parent fields
	t.Run("verify widget hierarchy parents", func(t *testing.T) {
		// Collect all widgets in the hierarchy
		var widgets []tui.IWidget
		var collectWidgets func(widget tui.IWidget)
		collectWidgets = func(widget tui.IWidget) {
			if widget == nil {
				return
			}
			widgets = append(widgets, widget)

			// Use type assertion to get children if the widget is a TWidget
			if appViewWidget, ok := widget.(*TAppView); ok {
				for _, child := range appViewWidget.Children {
					collectWidgets(child)
				}
			} else if chatViewWidget, ok := widget.(*TChatView); ok {
				for _, child := range chatViewWidget.Children {
					collectWidgets(child)
				}
			} else if baseWidget, ok := widget.(*tui.TWidget); ok {
				for _, child := range baseWidget.Children {
					collectWidgets(child)
				}
			}
		}

		// Start collecting from appView
		collectWidgets(appView)

		t.Logf("Found %d widgets in hierarchy", len(widgets))

		// Verify that all widgets have correct parents
		for i, widget := range widgets {
			parent := widget.GetParent()

			// appView was added to the application, so its parent is the application
			if widget == appView {
				assert.NotNil(t, parent, "appView should have application as parent")
				assert.IsType(t, &tui.TApplication{}, parent, "appView's parent should be TApplication")
				continue
			}

			// All other widgets should have a non-nil parent
			if parent == nil {
				// Try to get widget type for better error message
				widgetType := "unknown"
				switch widget.(type) {
				case *TAppView:
					widgetType = "TAppView"
				case *TChatView:
					widgetType = "TChatView"
				case *tui.TFlexLayout:
					widgetType = "TFlexLayout"
				case *tui.TZAxisLayout:
					widgetType = "TZAxisLayout"
				case *tui.TLabel:
					widgetType = "TLabel"
				case *tui.TTextArea:
					widgetType = "TTextArea"
				default:
					widgetType = "widget (unknown type)"
				}

				t.Errorf("Widget #%d (%s) has nil parent, but should have a parent", i, widgetType)
			} else {
				assert.NotNil(t, parent, "Widget #%d should have a non-nil parent", i)
			}
		}
	})

	t.Run("verify specific widget parents in TAppView", func(t *testing.T) {
		// Note: Due to Go's struct embedding model, GetParent() returns *TWidget
		// even when the actual parent is a derived type like *TAppView or *TFlexLayout.
		// This is because the embedded TWidget.AddChild() method stores its receiver
		// (which has static type *TWidget) in the Parent field.
		//
		// However, the memory addresses are correct - the stored *TWidget points to
		// the same memory as the outer struct. We verify this by comparing addresses.
		//
		// For type-safe traversal to TApplication, use GetApplication() instead.

		// Verify mainLayout's parent points to appView's memory
		if appView.mainLayout != nil {
			parent := appView.mainLayout.GetParent()
			assert.NotNil(t, parent, "mainLayout should have a parent")
			// Compare memory addresses using %p format
			parentAddr := fmt.Sprintf("%p", parent)
			appViewAddr := fmt.Sprintf("%p", &appView.TWidget)
			assert.Equal(t, appViewAddr, parentAddr, "mainLayout's parent should point to appView's TWidget")
		}

		// Verify contentLayout's parent points to mainLayout's memory
		if appView.contentLayout != nil {
			parent := appView.contentLayout.GetParent()
			assert.NotNil(t, parent, "contentLayout should have a parent")
			parentAddr := fmt.Sprintf("%p", parent)
			mainLayoutAddr := fmt.Sprintf("%p", &appView.mainLayout.TLayout.TResizable.TWidget)
			assert.Equal(t, mainLayoutAddr, parentAddr, "contentLayout's parent should point to mainLayout's TWidget")
		}

		// Verify statusBar's parent points to mainLayout's memory
		if appView.statusBar != nil {
			parent := appView.statusBar.GetParent()
			assert.NotNil(t, parent, "statusBar should have a parent")
			parentAddr := fmt.Sprintf("%p", parent)
			mainLayoutAddr := fmt.Sprintf("%p", &appView.mainLayout.TLayout.TResizable.TWidget)
			assert.Equal(t, mainLayoutAddr, parentAddr, "statusBar's parent should point to mainLayout's TWidget")
		}

		// Verify chatView's parent points to contentLayout's memory
		if appView.chatView != nil {
			parent := appView.chatView.GetParent()
			assert.NotNil(t, parent, "chatView should have a parent")
			parentAddr := fmt.Sprintf("%p", parent)
			contentLayoutAddr := fmt.Sprintf("%p", &appView.contentLayout.TLayout.TResizable.TWidget)
			assert.Equal(t, contentLayoutAddr, parentAddr, "chatView's parent should point to contentLayout's TWidget")
		}
	})

	t.Run("verify chatView internal hierarchy", func(t *testing.T) {
		if appView.chatView == nil {
			t.Skip("chatView not initialized")
		}

		chatView := appView.chatView

		// Note: Same as above - GetParent() returns *TWidget due to Go's embedding.
		// We verify correct parent relationships by comparing memory addresses.

		// Verify chatView's layout parent points to chatView's memory
		if chatView.layout != nil {
			parent := chatView.layout.GetParent()
			assert.NotNil(t, parent, "chatView.layout should have a parent")
			parentAddr := fmt.Sprintf("%p", parent)
			chatViewAddr := fmt.Sprintf("%p", &chatView.TWidget)
			assert.Equal(t, chatViewAddr, parentAddr, "chatView.layout's parent should point to chatView's TWidget")
		}

		// Verify markdownView parent points to chatView.layout's memory
		if chatView.markdownView != nil {
			parent := chatView.markdownView.GetParent()
			assert.NotNil(t, parent, "chatView.markdownView should have a parent")
			parentAddr := fmt.Sprintf("%p", parent)
			layoutAddr := fmt.Sprintf("%p", &chatView.layout.TLayout.TResizable.TWidget)
			assert.Equal(t, layoutAddr, parentAddr, "chatView.markdownView's parent should point to chatView.layout's TWidget")
		}

		// Verify textArea parent points to chatView.layout's memory
		if chatView.textArea != nil {
			parent := chatView.textArea.GetParent()
			assert.NotNil(t, parent, "chatView.textArea should have a parent")
			parentAddr := fmt.Sprintf("%p", parent)
			layoutAddr := fmt.Sprintf("%p", &chatView.layout.TLayout.TResizable.TWidget)
			assert.Equal(t, layoutAddr, parentAddr, "chatView.textArea's parent should point to chatView.layout's TWidget")
		}
	})

	t.Run("verify GetApplication works from all levels", func(t *testing.T) {
		// GetApplication() uses recursive dispatch through the parent chain,
		// which correctly finds TApplication even though GetParent() returns *TWidget.
		// This is the recommended way to find the application from any widget.

		// Verify GetApplication from appView
		gotApp := appView.GetApplication()
		assert.Equal(t, app, gotApp, "appView.GetApplication() should return the application")

		// Verify GetApplication from mainLayout
		if appView.mainLayout != nil {
			gotApp = appView.mainLayout.GetApplication()
			assert.Equal(t, app, gotApp, "mainLayout.GetApplication() should return the application")
		}

		// Verify GetApplication from contentLayout (nested one level deeper)
		if appView.contentLayout != nil {
			gotApp = appView.contentLayout.GetApplication()
			assert.Equal(t, app, gotApp, "contentLayout.GetApplication() should return the application")
		}

		// Verify GetApplication from chatView
		if appView.chatView != nil {
			gotApp = appView.chatView.GetApplication()
			assert.Equal(t, app, gotApp, "chatView.GetApplication() should return the application")

			// Verify from even deeper nested widgets
			if appView.chatView.layout != nil {
				gotApp = appView.chatView.layout.GetApplication()
				assert.Equal(t, app, gotApp, "chatView.layout.GetApplication() should return the application")
			}
			if appView.chatView.textArea != nil {
				gotApp = appView.chatView.textArea.GetApplication()
				assert.Equal(t, app, gotApp, "chatView.textArea.GetApplication() should return the application")
			}
		}
	})
}
