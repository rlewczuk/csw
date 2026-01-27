package tui

import (
	"fmt"
	"strings"

	"github.com/codesnort/codesnort-swe/pkg/gtv"
	"github.com/codesnort/codesnort-swe/pkg/gtv/tui"
	"github.com/codesnort/codesnort-swe/pkg/ui"
)

const (
	appViewName    = "Codesnort SWE"
	appViewVersion = "0.1.0"
)

// TAppView implements ui.IAppView for a terminal user interface using gtv package.
type TAppView struct {
	tui.TWidget

	presenter   ui.IAppPresenter
	chatView    *TChatView
	menu        *tui.TMenuWidget
	showingMenu bool

	// Main layout for organizing content and status bar vertically
	mainLayout *tui.TFlexLayout

	// Z-axis layout for stacking chat view and menu
	contentLayout *tui.TZAxisLayout

	// Status bar label
	statusBar *tui.TLabel

	// Dimensions
	width  int
	height int
}

// NewAppView creates a new TUI app view with the given presenter.
func NewAppView(parent tui.IWidget, rect gtv.TRect, presenter ui.IAppPresenter) *TAppView {
	view := &TAppView{
		TWidget: tui.TWidget{
			Position: rect,
			Parent:   parent,
		},
		presenter:   presenter,
		showingMenu: false,
		width:       int(rect.W),
		height:      int(rect.H),
	}

	// Create main flex layout (vertical) to hold content and status bar
	view.mainLayout = tui.NewFlexLayout(
		view,
		gtv.TRect{X: 0, Y: 0, W: rect.W, H: rect.H},
		tui.FlexDirectionColumn,
	)

	// Create Z-axis layout for content (chat view and menu)
	view.contentLayout = tui.NewZAxisLayout(
		view.mainLayout,
		gtv.TRect{X: 0, Y: 0, W: rect.W, H: rect.H - 1}, // Initial size, will be managed by flex
		nil,                                             // No background (transparent)
	)

	// Set content layout to grow and fill available space
	view.mainLayout.SetItemProperties(view.contentLayout, tui.FlexItemProperties{
		FlexGrow:   1.0,
		FlexShrink: 1.0,
		MinSize:    1,
	})

	// Create status bar
	statusBarAttrs := gtv.CellTag("app-view-status-bar")
	view.statusBar = tui.NewLabel(
		view.mainLayout,
		view.renderStatusBarText(),
		gtv.TRect{X: 0, Y: 0, W: rect.W, H: 1},
		statusBarAttrs,
	)

	// Set status bar to fixed height of 1
	view.mainLayout.SetItemProperties(view.statusBar, tui.FlexItemProperties{
		FlexGrow:    0.0,
		FlexShrink:  0.0,
		FixedHeight: 1,
	})

	// Register with parent if provided
	if parent != nil {
		parent.AddChild(view)
	}

	return view
}

// SetApp sets the application for the app view.
// This allows the view to propagate the app to child views for redraw requests.
func (v *TAppView) SetApp(app tui.IRedrawRequester) {
	// Propagate to existing chat view if any
	if v.chatView != nil {
		v.chatView.SetApp(app)
	}
}

// ShowChat switches to the chat view with the given presenter.
func (v *TAppView) ShowChat(presenter ui.IChatPresenter) ui.IChatView {
	// Create or reuse chat view
	if v.chatView == nil {
		// Get the content layout size for initial chat view size
		contentRect := v.contentLayout.GetPos()
		chatRect := gtv.TRect{X: 0, Y: 0, W: contentRect.W, H: contentRect.H}
		// Create chat view without parent (we'll add it manually to Z-axis layout)
		v.chatView = NewChatView(nil, chatRect, presenter)
		// Set app for redraw requests if available
		// Add to Z-axis layout with z-index 0 (bottom layer)
		v.contentLayout.AddZWidget(v.chatView, 0)
		// Set chat view as the active child to receive keyboard events
		v.contentLayout.ActiveChild = v.chatView
	}
	return v.chatView
}

// ShowSettings shows the settings view (not yet implemented).
func (v *TAppView) ShowSettings() {
	// TODO: Implement settings view
}

// Draw draws the app view on the screen.
func (v *TAppView) Draw(screen gtv.IScreenOutput) {
	// Don't draw if hidden
	if v.Flags&tui.WidgetFlagHidden != 0 {
		return
	}

	// Draw main layout (contains content layout and status bar)
	v.mainLayout.Draw(screen)
}

// HandleEvent handles events for the app view.
func (v *TAppView) HandleEvent(event *tui.TEvent) {
	// Handle resize events
	if event.Type == tui.TEventTypeResize {
		v.Position = event.Rect
		v.width = int(event.Rect.W)
		v.height = int(event.Rect.H)

		// Propagate resize to main layout - it will automatically handle resizing children
		layoutEvent := &tui.TEvent{
			Type: tui.TEventTypeResize,
			Rect: gtv.TRect{X: 0, Y: 0, W: event.Rect.W, H: event.Rect.H},
		}
		v.mainLayout.HandleEvent(layoutEvent)

		// Update status bar text to match new width
		v.statusBar.SetText(v.renderStatusBarText())

		return
	}

	// Handle input events
	if event.Type == tui.TEventTypeInput && event.InputEvent != nil {
		inputEvent := event.InputEvent

		// If menu is showing, handle menu input
		if v.showingMenu && v.menu != nil {
			v.menu.HandleEvent(event)
			return
		}

		// Handle global hotkeys
		if inputEvent.Type == gtv.InputEventKey {
			// Check for Ctrl+P or Esc to show menu
			if (inputEvent.Modifiers&gtv.ModCtrl != 0 && inputEvent.Key == 'p') ||
				inputEvent.Key == 0x1B { // ESC key
				v.showMenu()
				return
			}
		}

		// Pass through to content layout (which contains chat view)
		v.contentLayout.HandleEvent(event)
		return
	}

	// Delegate other events to content layout
	v.contentLayout.HandleEvent(event)
}

// renderStatusBarText renders the status bar text.
func (v *TAppView) renderStatusBarText() string {
	leftText := "Ctrl+P/Esc: Menu"
	rightText := fmt.Sprintf("%s v%s", appViewName, appViewVersion)

	// Calculate spacing to fill the width
	leftWidth := len(leftText)
	rightWidth := len(rightText)
	spacingWidth := v.width - leftWidth - rightWidth

	if spacingWidth < 0 {
		spacingWidth = 0
	}

	spacing := strings.Repeat(" ", spacingWidth)

	return leftText + spacing + rightText
}

// showMenu displays the main menu.
func (v *TAppView) showMenu() {
	// Create menu items
	items := []tui.MenuItem{
		{
			Label: "New Session",
			Handler: func(text string) {
				v.hideMenu()
				v.presenter.NewSession()
			},
		},
		{
			Label: "Exit",
			Handler: func(text string) {
				v.hideMenu()
				v.presenter.Exit()
			},
		},
	}

	// Calculate menu size (centered)
	menuWidth := uint16(40)
	if menuWidth > uint16(v.width) {
		menuWidth = uint16(v.width)
	}
	menuHeight := uint16(len(items) + 2) // +2 for border

	menuX := (uint16(v.width) - menuWidth) / 2
	menuY := (uint16(v.height) - menuHeight) / 2

	// Create menu widget without parent
	v.menu = tui.NewMenuWidget(
		nil,
		tui.WithRectangle(int(menuX), int(menuY), int(menuWidth), int(menuHeight)),
	)

	v.menu.SetTitle("Main Menu")
	v.menu.SetItems(items)

	// Set cancel handler
	v.menu.SetOnCancel(func() {
		v.hideMenu()
	})

	// Add menu to content Z-axis layout with z-index 100 (top layer)
	v.contentLayout.AddZWidget(v.menu, 100, tui.WithZBehavior(tui.ZWidgetBehaviorDim))

	// Set menu as active child to receive keyboard events
	v.contentLayout.ActiveChild = v.menu

	// Focus the menu
	v.menu.Focus()

	v.showingMenu = true
}

// hideMenu hides the main menu.
func (v *TAppView) hideMenu() {
	if v.menu != nil {
		// Remove menu from Z-axis layout
		v.contentLayout.RemoveZWidget(v.menu)
		v.menu = nil
	}

	v.showingMenu = false

	// Restore chat view as active child
	if v.chatView != nil {
		v.contentLayout.ActiveChild = v.chatView
		// Restore focus to text area
		if v.chatView.textArea != nil {
			v.chatView.textArea.Focus()
		}
	}
}

// Ensure TAppView implements ui.IAppView
var _ ui.IAppView = (*TAppView)(nil)
var _ tui.IWidget = (*TAppView)(nil)
