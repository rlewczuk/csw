// Package testutil provides testing utilities for LLM clients.
package testutil

import (
	"net/http"
	"net/http/httptest"
	"sync"
)

// MockHTTPServer provides a mock HTTP server for testing REST endpoints.
type MockHTTPServer struct {
	Server    *httptest.Server
	mu        sync.Mutex
	responses map[string]map[string]*responseQueue // path -> method -> response queue
}

type responseQueue struct {
	queue []*mockResponse
}

type mockResponse struct {
	isStreaming bool
	body        string        // for non-streaming responses
	chunks      []string      // for streaming responses
	closeAfter  bool          // whether to close stream after current chunks
	waitCh      chan struct{} // channel to signal new chunks available
}

// NewMockHTTPServer creates a new mock HTTP server.
func NewMockHTTPServer() *MockHTTPServer {
	m := &MockHTTPServer{
		responses: make(map[string]map[string]*responseQueue),
	}

	m.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		m.HandleRequest(w, r)
	}))

	return m
}

// Client returns an HTTP client configured to use the mock server.
func (m *MockHTTPServer) Client() *http.Client {
	return m.Server.Client()
}

// URL returns the base URL of the mock server.
func (m *MockHTTPServer) URL() string {
	return m.Server.URL
}

// Close stops the mock server.
func (m *MockHTTPServer) Close() {
	m.Server.Close()
}

// AddRestResponse adds a response for a specific REST endpoint.
// Multiple calls append responses to a FIFO queue.
func (m *MockHTTPServer) AddRestResponse(path, method, response string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.responses[path] == nil {
		m.responses[path] = make(map[string]*responseQueue)
	}

	if m.responses[path][method] == nil {
		m.responses[path][method] = &responseQueue{
			queue: []*mockResponse{},
		}
	}

	// Append new non-streaming response to queue
	m.responses[path][method].queue = append(m.responses[path][method].queue, &mockResponse{
		isStreaming: false,
		body:        response,
	})
}

// AddStreamingResponse adds a streaming response for a specific REST endpoint.
// If called again on the same path and method, it appends new chunks to the existing stream.
func (m *MockHTTPServer) AddStreamingResponse(path, method string, closeAfter bool, responses ...string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.responses[path] == nil {
		m.responses[path] = make(map[string]*responseQueue)
	}

	if m.responses[path][method] == nil {
		m.responses[path][method] = &responseQueue{
			queue: []*mockResponse{},
		}
	}

	// Append new streaming response to queue
	m.responses[path][method].queue = append(m.responses[path][method].queue, &mockResponse{
		isStreaming: true,
		chunks:      responses,
		closeAfter:  closeAfter,
		waitCh:      make(chan struct{}),
	})
}

// HandleRequest handles incoming HTTP requests by matching against configured responses.
func (m *MockHTTPServer) HandleRequest(w http.ResponseWriter, r *http.Request) {
	m.mu.Lock()
	pathResponses := m.responses[r.URL.Path]
	var respQueue *responseQueue
	if pathResponses != nil {
		respQueue = pathResponses[r.Method]
	}

	if respQueue == nil || len(respQueue.queue) == 0 {
		m.mu.Unlock()
		http.NotFound(w, r)
		return
	}

	// Dequeue the first response
	resp := respQueue.queue[0]
	respQueue.queue = respQueue.queue[1:]
	m.mu.Unlock()

	if !resp.isStreaming {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(resp.body))
		return
	}

	// Streaming response
	w.Header().Set("Content-Type", "application/x-ndjson")
	w.Header().Set("Transfer-Encoding", "chunked")
	w.WriteHeader(http.StatusOK)

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming not supported", http.StatusInternalServerError)
		return
	}

	// Write all chunks
	for _, chunk := range resp.chunks {
		_, err := w.Write([]byte(chunk + "\n"))
		if err != nil {
			return
		}
		flusher.Flush()
	}
}
