package mock

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestMockAppView_ShowChat tests that ShowChat records calls correctly.
func TestMockAppView_ShowChat(t *testing.T) {
	view := NewMockAppView()
	chatPresenter1 := NewMockChatPresenter()
	chatPresenter2 := NewMockChatPresenter()

	chatView1 := view.ShowChat(chatPresenter1)
	chatView2 := view.ShowChat(chatPresenter2)

	assert.Len(t, view.ShowChatCalls, 2, "expected 2 ShowChat calls")
	assert.Equal(t, chatPresenter1, view.ShowChatCalls[0], "first call should record chatPresenter1")
	assert.Equal(t, chatPresenter2, view.ShowChatCalls[1], "second call should record chatPresenter2")
	assert.NotNil(t, chatView1, "ShowChat should return a chat view")
	assert.NotNil(t, chatView2, "ShowChat should return a chat view")
	assert.Equal(t, chatView1, chatView2, "ShowChat should return the same view instance")
}

// TestMockAppView_ShowSettings tests that ShowSettings counts calls correctly.
func TestMockAppView_ShowSettings(t *testing.T) {
	view := NewMockAppView()

	view.ShowSettings()
	view.ShowSettings()
	view.ShowSettings()

	assert.Equal(t, 3, view.ShowSettingsCalls, "expected 3 ShowSettings calls")
}

// TestMockAppView_Reset tests that Reset clears all recorded data.
func TestMockAppView_Reset(t *testing.T) {
	view := NewMockAppView()
	chatPresenter := NewMockChatPresenter()

	view.ShowChat(chatPresenter)
	view.ShowSettings()

	assert.Len(t, view.ShowChatCalls, 1, "should have 1 ShowChat call before reset")
	assert.Equal(t, 1, view.ShowSettingsCalls, "should have 1 ShowSettings call before reset")

	view.Reset()

	assert.Len(t, view.ShowChatCalls, 0, "ShowChatCalls should be empty after reset")
	assert.Equal(t, 0, view.ShowSettingsCalls, "ShowSettingsCalls should be 0 after reset")
}

// TestMockAppPresenter_NewSession tests that NewSession works correctly.
func TestMockAppPresenter_NewSession(t *testing.T) {
	presenter := NewMockAppPresenter()

	err := presenter.NewSession()

	assert.NoError(t, err, "NewSession should not return error by default")
	assert.Equal(t, 1, presenter.NewSessionCalls, "expected 1 NewSession call")
}

// TestMockAppPresenter_NewSession_WithError tests that NewSession returns configured error.
func TestMockAppPresenter_NewSession_WithError(t *testing.T) {
	presenter := NewMockAppPresenter()
	expectedErr := errors.New("new session error")
	presenter.NewSessionErr = expectedErr

	err := presenter.NewSession()

	assert.Equal(t, expectedErr, err, "NewSession should return configured error")
	assert.Equal(t, 1, presenter.NewSessionCalls, "expected 1 NewSession call")
}

// TestMockAppPresenter_OpenSession tests that OpenSession records calls correctly.
func TestMockAppPresenter_OpenSession(t *testing.T) {
	presenter := NewMockAppPresenter()

	err := presenter.OpenSession("session-123")
	assert.NoError(t, err, "OpenSession should not return error by default")

	err = presenter.OpenSession("session-456")
	assert.NoError(t, err, "OpenSession should not return error by default")

	assert.Len(t, presenter.OpenSessionCalls, 2, "expected 2 OpenSession calls")
	assert.Equal(t, "session-123", presenter.OpenSessionCalls[0], "first call should record session-123")
	assert.Equal(t, "session-456", presenter.OpenSessionCalls[1], "second call should record session-456")
}

// TestMockAppPresenter_OpenSession_WithError tests that OpenSession returns configured error.
func TestMockAppPresenter_OpenSession_WithError(t *testing.T) {
	presenter := NewMockAppPresenter()
	expectedErr := errors.New("open session error")
	presenter.OpenSessionErr = expectedErr

	err := presenter.OpenSession("session-789")

	assert.Equal(t, expectedErr, err, "OpenSession should return configured error")
	assert.Len(t, presenter.OpenSessionCalls, 1, "expected 1 OpenSession call")
	assert.Equal(t, "session-789", presenter.OpenSessionCalls[0], "should record session ID even with error")
}

// TestMockAppPresenter_Exit tests that Exit works correctly.
func TestMockAppPresenter_Exit(t *testing.T) {
	presenter := NewMockAppPresenter()

	err := presenter.Exit()
	assert.NoError(t, err, "Exit should not return error by default")

	err = presenter.Exit()
	assert.NoError(t, err, "Exit should not return error by default")

	assert.Equal(t, 2, presenter.ExitCalls, "expected 2 Exit calls")
}

// TestMockAppPresenter_Exit_WithError tests that Exit returns configured error.
func TestMockAppPresenter_Exit_WithError(t *testing.T) {
	presenter := NewMockAppPresenter()
	expectedErr := errors.New("exit error")
	presenter.ExitErr = expectedErr

	err := presenter.Exit()

	assert.Equal(t, expectedErr, err, "Exit should return configured error")
	assert.Equal(t, 1, presenter.ExitCalls, "expected 1 Exit call")
}

// TestMockAppPresenter_Reset tests that Reset clears all recorded data and errors.
func TestMockAppPresenter_Reset(t *testing.T) {
	presenter := NewMockAppPresenter()

	// Set up some errors
	presenter.NewSessionErr = errors.New("new session error")
	presenter.OpenSessionErr = errors.New("open session error")
	presenter.ExitErr = errors.New("exit error")

	// Make some calls
	presenter.NewSession()
	presenter.OpenSession("session-1")
	presenter.OpenSession("session-2")
	presenter.Exit()

	// Verify calls were recorded
	assert.Equal(t, 1, presenter.NewSessionCalls, "should have 1 NewSession call before reset")
	assert.Len(t, presenter.OpenSessionCalls, 2, "should have 2 OpenSession calls before reset")
	assert.Equal(t, 1, presenter.ExitCalls, "should have 1 Exit call before reset")

	// Reset
	presenter.Reset()

	// Verify everything is cleared
	assert.Equal(t, 0, presenter.NewSessionCalls, "NewSessionCalls should be 0 after reset")
	assert.Len(t, presenter.OpenSessionCalls, 0, "OpenSessionCalls should be empty after reset")
	assert.Equal(t, 0, presenter.ExitCalls, "ExitCalls should be 0 after reset")

	// Verify errors are cleared
	assert.Nil(t, presenter.NewSessionErr, "NewSessionErr should be nil after reset")
	assert.Nil(t, presenter.OpenSessionErr, "OpenSessionErr should be nil after reset")
	assert.Nil(t, presenter.ExitErr, "ExitErr should be nil after reset")
}

// TestMockAppPresenter_MultipleCalls tests that all methods work correctly with multiple calls.
func TestMockAppPresenter_MultipleCalls(t *testing.T) {
	presenter := NewMockAppPresenter()

	// Make multiple calls to each method
	presenter.NewSession()
	presenter.NewSession()
	presenter.OpenSession("session-1")
	presenter.OpenSession("session-2")
	presenter.OpenSession("session-3")
	presenter.Exit()

	// Verify all calls were recorded
	assert.Equal(t, 2, presenter.NewSessionCalls, "expected 2 NewSession calls")
	assert.Len(t, presenter.OpenSessionCalls, 3, "expected 3 OpenSession calls")
	assert.Equal(t, []string{"session-1", "session-2", "session-3"}, presenter.OpenSessionCalls, "OpenSession should record all IDs in order")
	assert.Equal(t, 1, presenter.ExitCalls, "expected 1 Exit call")
}
