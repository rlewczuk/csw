package tool

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return f(request)
}

func TestWebFetchTool_Execute(t *testing.T) {
	t.Run("returns error when url is missing", func(t *testing.T) {
		tool := NewWebFetchTool(nil)

		response := tool.Execute(&ToolCall{
			ID:        "test-id",
			Function:  "webFetch",
			Arguments: NewToolValue(map[string]any{"format": "raw"}),
		})

		require.Error(t, response.Error)
		assert.Contains(t, response.Error.Error(), "missing required argument: url")
	})

	t.Run("auto-detects format when not provided", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/plain")
			_, _ = w.Write([]byte("hello"))
		}))
		defer server.Close()

		tool := NewWebFetchTool(nil)

		response := tool.Execute(&ToolCall{
			ID:        "test-id",
			Function:  "webFetch",
			Arguments: NewToolValue(map[string]any{"url": server.URL}),
		})

		// Format is now optional, should auto-detect based on content type
		require.NoError(t, response.Error)
		assert.Equal(t, "raw", response.Result.String("format"))
		assert.Equal(t, "hello", response.Result.String("content"))
	})

	t.Run("returns error on invalid format", func(t *testing.T) {
		tool := NewWebFetchTool(nil)

		response := tool.Execute(&ToolCall{
			ID:       "test-id",
			Function: "webFetch",
			Arguments: NewToolValue(map[string]any{
				"url":    "https://example.com",
				"format": "json",
			}),
		})

		require.Error(t, response.Error)
		assert.Contains(t, response.Error.Error(), "invalid format")
	})

	t.Run("returns error on non-positive timeout", func(t *testing.T) {
		tool := NewWebFetchTool(nil)

		response := tool.Execute(&ToolCall{
			ID:       "test-id",
			Function: "webFetch",
			Arguments: NewToolValue(map[string]any{
				"url":     "https://example.com",
				"format":  "raw",
				"timeout": 0,
			}),
		})

		require.Error(t, response.Error)
		assert.Contains(t, response.Error.Error(), "timeout must be positive")
	})

	t.Run("returns error on invalid url", func(t *testing.T) {
		tool := NewWebFetchTool(nil)

		response := tool.Execute(&ToolCall{
			ID:       "test-id",
			Function: "webFetch",
			Arguments: NewToolValue(map[string]any{
				"url":    "::://bad",
				"format": "raw",
			}),
		})

		require.Error(t, response.Error)
		assert.Contains(t, response.Error.Error(), "invalid url")
	})

	t.Run("returns error on unsupported scheme", func(t *testing.T) {
		tool := NewWebFetchTool(nil)

		response := tool.Execute(&ToolCall{
			ID:       "test-id",
			Function: "webFetch",
			Arguments: NewToolValue(map[string]any{
				"url":    "file:///tmp/test.txt",
				"format": "raw",
			}),
		})

		require.Error(t, response.Error)
		assert.Contains(t, response.Error.Error(), "unsupported url scheme")
	})

	t.Run("fetches raw content", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/plain")
			_, _ = w.Write([]byte("hello raw"))
		}))
		defer server.Close()

		tool := NewWebFetchTool(nil)

		response := tool.Execute(&ToolCall{
			ID:       "test-id",
			Function: "webFetch",
			Arguments: NewToolValue(map[string]any{
				"url":    server.URL,
				"format": "raw",
			}),
		})

		require.NoError(t, response.Error)
		assert.Equal(t, "hello raw", response.Result.String("content"))
		assert.Equal(t, int64(200), response.Result.Int("statusCode"))
		assert.Equal(t, "raw", response.Result.String("format"))
		assert.Contains(t, response.Result.String("contentType"), "text/plain")
	})

	t.Run("converts html to markdown", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			_, _ = w.Write([]byte("<html><body><h1>Title</h1><p>Hello</p></body></html>"))
		}))
		defer server.Close()

		tool := NewWebFetchTool(nil)

		response := tool.Execute(&ToolCall{
			ID:       "test-id",
			Function: "webFetch",
			Arguments: NewToolValue(map[string]any{
				"url":    server.URL,
				"format": "markdown",
			}),
		})

		require.NoError(t, response.Error)
		assert.Contains(t, response.Result.String("content"), "Title")
		assert.Contains(t, response.Result.String("content"), "Hello")
	})

	t.Run("returns error for markdown conversion on non-html content", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"hello":"world"}`))
		}))
		defer server.Close()

		tool := NewWebFetchTool(nil)

		response := tool.Execute(&ToolCall{
			ID:       "test-id",
			Function: "webFetch",
			Arguments: NewToolValue(map[string]any{
				"url":    server.URL,
				"format": "markdown",
			}),
		})

		require.Error(t, response.Error)
		assert.Contains(t, response.Error.Error(), "supports only html content")
	})

	t.Run("returns error on http status error", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte("not found"))
		}))
		defer server.Close()

		tool := NewWebFetchTool(nil)

		response := tool.Execute(&ToolCall{
			ID:       "test-id",
			Function: "webFetch",
			Arguments: NewToolValue(map[string]any{
				"url":    server.URL,
				"format": "raw",
			}),
		})

		require.Error(t, response.Error)
		assert.Contains(t, response.Error.Error(), "request failed with status 404")
	})

	t.Run("uses timeout argument", func(t *testing.T) {
		httpClient := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			deadline, ok := request.Context().Deadline()
			require.True(t, ok)
			assert.WithinDuration(t, time.Now().Add(time.Second), deadline, 100*time.Millisecond)

			return nil, context.DeadlineExceeded
		})}

		tool := NewWebFetchTool(httpClient)

		response := tool.Execute(&ToolCall{
			ID:       "test-id",
			Function: "webFetch",
			Arguments: NewToolValue(map[string]any{
				"url":     "https://example.com",
				"format":  "raw",
				"timeout": 1,
			}),
		})

		require.Error(t, response.Error)
		assert.True(t, errors.Is(response.Error, context.DeadlineExceeded))
		assert.Contains(t, response.Error.Error(), "context deadline exceeded")
	})

	t.Run("defaults to markdown for html content when format not provided", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			_, _ = w.Write([]byte("<html><body><h1>Title</h1><p>Hello</p></body></html>"))
		}))
		defer server.Close()

		tool := NewWebFetchTool(nil)

		response := tool.Execute(&ToolCall{
			ID:       "test-id",
			Function: "webFetch",
			Arguments: NewToolValue(map[string]any{
				"url": server.URL,
				// format not provided
			}),
		})

		require.NoError(t, response.Error)
		assert.Equal(t, "markdown", response.Result.String("format"))
		assert.Contains(t, response.Result.String("content"), "Title")
		assert.Contains(t, response.Result.String("content"), "Hello")
	})

	t.Run("defaults to raw for text content when format not provided", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/plain")
			_, _ = w.Write([]byte("hello raw"))
		}))
		defer server.Close()

		tool := NewWebFetchTool(nil)

		response := tool.Execute(&ToolCall{
			ID:       "test-id",
			Function: "webFetch",
			Arguments: NewToolValue(map[string]any{
				"url": server.URL,
				// format not provided
			}),
		})

		require.NoError(t, response.Error)
		assert.Equal(t, "raw", response.Result.String("format"))
		assert.Equal(t, "hello raw", response.Result.String("content"))
	})

	t.Run("defaults to raw for json content when format not provided", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"hello":"world"}`))
		}))
		defer server.Close()

		tool := NewWebFetchTool(nil)

		response := tool.Execute(&ToolCall{
			ID:       "test-id",
			Function: "webFetch",
			Arguments: NewToolValue(map[string]any{
				"url": server.URL,
				// format not provided
			}),
		})

		require.NoError(t, response.Error)
		assert.Equal(t, "raw", response.Result.String("format"))
		assert.Equal(t, `{"hello":"world"}`, response.Result.String("content"))
	})

	t.Run("returns error for non-textual content when format not provided", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/octet-stream")
			_, _ = w.Write([]byte("binary data"))
		}))
		defer server.Close()

		tool := NewWebFetchTool(nil)

		response := tool.Execute(&ToolCall{
			ID:       "test-id",
			Function: "webFetch",
			Arguments: NewToolValue(map[string]any{
				"url": server.URL,
				// format not provided
			}),
		})

		require.Error(t, response.Error)
		assert.Contains(t, response.Error.Error(), "unable to determine format")
	})
}

func TestWebFetchTool_Render(t *testing.T) {
	tool := NewWebFetchTool(nil)
	call := &ToolCall{
		ID:       "test-id",
		Function: "webFetch",
		Arguments: NewToolValue(map[string]any{
			"url":     "https://example.com",
			"content": "sample",
		}),
	}

	oneLiner, full, _, meta := tool.Render(call)

	assert.Contains(t, oneLiner, "fetch https://example.com")
	assert.Contains(t, full, "sample")
	assert.NotNil(t, meta)
}
