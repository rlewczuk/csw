package lsp

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
)

type response struct {
	ID     int64           `json:"id"`
	Result json.RawMessage `json:"result,omitempty"`
	Error  *responseError  `json:"error,omitempty"`
}

type responseError struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data,omitempty"`
}

type request struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      int64       `json:"id"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params,omitempty"`
}

type notification struct {
	JSONRPC string      `json:"jsonrpc"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params,omitempty"`
}

// sendRequest sends a request and waits for the response.
func (c *Client) sendRequest(method string, params interface{}, result interface{}) error {
	id := c.nextID.Add(1) - 1

	req := request{
		JSONRPC: "2.0",
		ID:      id,
		Method:  method,
		Params:  params,
	}

	// Create response channel
	respChan := make(chan *response, 1)
	c.pendingMu.Lock()
	c.pending[id] = respChan
	c.pendingMu.Unlock()

	defer func() {
		c.pendingMu.Lock()
		delete(c.pending, id)
		c.pendingMu.Unlock()
	}()

	// Send request
	if err := c.writeMessage(req); err != nil {
		return fmt.Errorf("Client.sendRequest: failed to write message: %w", err)
	}

	// Wait for response
	select {
	case resp := <-respChan:
		if resp.Error != nil {
			return fmt.Errorf("Client.sendRequest: LSP error %d: %s", resp.Error.Code, resp.Error.Message)
		}

		if result != nil && resp.Result != nil {
			if err := json.Unmarshal(resp.Result, result); err != nil {
				return fmt.Errorf("Client.sendRequest: failed to unmarshal result: %w", err)
			}
		}

		return nil
	case <-c.ctx.Done():
		return fmt.Errorf("Client.sendRequest: context cancelled")
	}
}

// sendNotification sends a notification (no response expected).
func (c *Client) sendNotification(method string, params interface{}) error {
	notif := notification{
		JSONRPC: "2.0",
		Method:  method,
		Params:  params,
	}

	if err := c.writeMessage(notif); err != nil {
		return fmt.Errorf("Client.sendNotification: failed to write message: %w", err)
	}

	return nil
}

// writeMessage writes a JSON-RPC message with LSP headers.
func (c *Client) writeMessage(msg interface{}) error {
	data, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("Client.writeMessage: failed to marshal message: %w", err)
	}

	header := fmt.Sprintf("Content-Length: %d\r\n\r\n", len(data))
	if _, err := c.stdin.Write([]byte(header)); err != nil {
		return fmt.Errorf("Client.writeMessage: failed to write header: %w", err)
	}

	if _, err := c.stdin.Write(data); err != nil {
		return fmt.Errorf("Client.writeMessage: failed to write data: %w", err)
	}

	return nil
}

// readLoop reads messages from the language server.
func (c *Client) readLoop() {
	reader := bufio.NewReader(c.stdout)

	for {
		select {
		case <-c.ctx.Done():
			return
		default:
		}

		// Read headers
		headers := make(map[string]string)
		for {
			line, err := reader.ReadString('\n')
			if err != nil {
				if err != io.EOF {
					fmt.Fprintf(os.Stderr, "Client.readLoop: failed to read header line: %v\n", err)
				}
				return
			}

			line = strings.TrimSpace(line)
			if line == "" {
				break
			}

			parts := strings.SplitN(line, ":", 2)
			if len(parts) == 2 {
				headers[strings.TrimSpace(parts[0])] = strings.TrimSpace(parts[1])
			}
		}

		// Get content length
		contentLengthStr, ok := headers["Content-Length"]
		if !ok {
			fmt.Fprintf(os.Stderr, "Client.readLoop: missing Content-Length header\n")
			continue
		}

		contentLength, err := strconv.Atoi(contentLengthStr)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Client.readLoop: invalid Content-Length: %v\n", err)
			continue
		}

		// Read content
		content := make([]byte, contentLength)
		if _, err := io.ReadFull(reader, content); err != nil {
			fmt.Fprintf(os.Stderr, "Client.readLoop: failed to read content: %v\n", err)
			return
		}

		// Parse message
		c.handleMessage(content)
	}
}

// handleMessage handles an incoming message from the language server.
func (c *Client) handleMessage(data []byte) {
	var baseMsg struct {
		JSONRPC string          `json:"jsonrpc"`
		ID      *int64          `json:"id,omitempty"`
		Method  string          `json:"method,omitempty"`
		Result  json.RawMessage `json:"result,omitempty"`
		Error   *responseError  `json:"error,omitempty"`
		Params  json.RawMessage `json:"params,omitempty"`
	}

	if err := json.Unmarshal(data, &baseMsg); err != nil {
		fmt.Fprintf(os.Stderr, "Client.handleMessage: failed to unmarshal message: %v\n", err)
		return
	}

	// Check if it's a response
	if baseMsg.ID != nil && baseMsg.Method == "" {
		c.pendingMu.RLock()
		respChan, ok := c.pending[*baseMsg.ID]
		c.pendingMu.RUnlock()

		if ok {
			resp := &response{
				ID:     *baseMsg.ID,
				Result: baseMsg.Result,
				Error:  baseMsg.Error,
			}
			select {
			case respChan <- resp:
			default:
			}
		}
		return
	}

	// It's a notification
	if baseMsg.Method != "" {
		c.handleNotification(baseMsg.Method, baseMsg.Params)
	}
}

// handleNotification handles notifications from the language server.
func (c *Client) handleNotification(method string, params json.RawMessage) {
	switch method {
	case "textDocument/publishDiagnostics":
		var p PublishDiagnosticsParams
		if err := json.Unmarshal(params, &p); err != nil {
			fmt.Fprintf(os.Stderr, "Client.handleNotification: failed to unmarshal publishDiagnostics: %v\n", err)
			return
		}

		c.diagnosticsMu.Lock()
		c.diagnostics[string(p.URI)] = p.Diagnostics
		c.diagnosticsMu.Unlock()

	case "window/logMessage":
		// Ignore log messages for now
	}
}

// readStderr reads stderr from the language server (for debugging).
func (c *Client) readStderr() {
	scanner := bufio.NewScanner(c.stderr)
	for scanner.Scan() {
		// Silently consume stderr for now
		// In production, you might want to log this
		_ = scanner.Text()
	}
}
