package main

import (
	"testing"

	"github.com/rlewczuk/csw/pkg/ui"
	"github.com/rlewczuk/csw/pkg/ui/mock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// cli_mock_integ_test.go contains integration tests for mock UI implementations.
// These tests verify that MockChatView's automatic permission response
// functionality works correctly, including proper selection of response options
// and backward compatibility when no auto-response is configured.

// TestMockChatViewAutoPermissionResponse tests the MockChatView's automatic permission response functionality.
func TestMockChatViewAutoPermissionResponse(t *testing.T) {
	tests := []struct {
		name             string
		autoResponse     string
		options          []string
		expectedResponse string
	}{
		{
			name:             "Auto deny selects Deny option",
			autoResponse:     "Deny",
			options:          []string{"Allow", "Ask", "Deny"},
			expectedResponse: "Deny",
		},
		{
			name:             "Auto deny falls back to last option",
			autoResponse:     "Deny",
			options:          []string{"Allow", "Ask", "Reject"},
			expectedResponse: "Reject",
		},
		{
			name:             "Auto accept selects first option",
			autoResponse:     "Accept",
			options:          []string{"Allow", "Ask", "Deny"},
			expectedResponse: "Allow",
		},
		{
			name:             "Custom response",
			autoResponse:     "CustomAnswer",
			options:          []string{"Option1", "Option2"},
			expectedResponse: "CustomAnswer",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Create mock presenter to track responses
			mockPresenter := mock.NewMockChatPresenter()

			// Create mock view with automatic permission response
			mockView := mock.NewMockChatView()
			mockView.AutoPermissionResponse = tc.autoResponse
			mockView.Presenter = mockPresenter

			// Create permission query
			query := &ui.PermissionQueryUI{
				Id:      "test-query-1",
				Title:   "Test Permission",
				Details: "Test details",
				Options: tc.options,
			}

			// Call QueryPermission - should automatically respond
			err := mockView.QueryPermission(query)
			require.NoError(t, err)

			// Verify the query was recorded
			assert.Equal(t, 1, len(mockView.QueryPermissionCalls), "Query should have been recorded")

			// Verify the response was sent
			assert.Equal(t, 1, len(mockPresenter.PermissionResponseCalls), "Permission response should have been sent")
			assert.Equal(t, tc.expectedResponse, mockPresenter.PermissionResponseCalls[0], "Response should match expected")
		})
	}
}

// TestMockChatViewNoAutoResponse tests that MockChatView without auto-response just records the query.
func TestMockChatViewNoAutoResponse(t *testing.T) {
	// Create mock presenter
	mockPresenter := mock.NewMockChatPresenter()

	// Create mock view WITHOUT automatic permission response
	mockView := mock.NewMockChatView()
	// AutoPermissionResponse is empty by default
	mockView.Presenter = mockPresenter

	// Create permission query
	query := &ui.PermissionQueryUI{
		Id:      "test-query-1",
		Title:   "Test Permission",
		Details: "Test details",
		Options: []string{"Allow", "Deny"},
	}

	// Call QueryPermission - should just record without responding
	err := mockView.QueryPermission(query)
	require.NoError(t, err)

	// Verify the query was recorded
	assert.Equal(t, 1, len(mockView.QueryPermissionCalls), "Query should have been recorded")

	// Verify NO response was sent (backward compatibility)
	assert.Equal(t, 0, len(mockPresenter.PermissionResponseCalls), "No permission response should have been sent without auto-response")
}
