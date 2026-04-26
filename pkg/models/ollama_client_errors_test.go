package models

import (
	"testing"

	"github.com/rlewczuk/csw/pkg/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOllamaClient_ErrorHandling(t *testing.T) {
	t.Run("handles endpoint not found", func(t *testing.T) {
		mock := testutil.NewMockHTTPServer()
		defer mock.Close()

		mock.AddRestResponseWithStatus("/api/tags", "GET", `{"error":"not found"}`, 404)

		client, err := NewOllamaClientWithHTTPClient(mock.URL(), mock.Client())
		require.NoError(t, err)

		_, err = client.ListModels()
		assert.ErrorIs(t, err, ErrEndpointNotFound)
	})

	t.Run("handles endpoint unavailable", func(t *testing.T) {
		mock := testutil.NewMockHTTPServer()
		defer mock.Close()

		mock.AddRestResponseWithStatus("/api/tags", "GET", `{"error":"unavailable"}`, 503)

		client, err := NewOllamaClientWithHTTPClient(mock.URL(), mock.Client())
		require.NoError(t, err)

		_, err = client.ListModels()
		require.Error(t, err)
		assert.ErrorIs(t, err, ErrEndpointUnavailable)
	})
}
