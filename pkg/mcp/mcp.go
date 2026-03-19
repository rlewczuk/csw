package mcp

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/rlewczuk/csw/pkg/conf"
	"github.com/rlewczuk/csw/pkg/core"
	"github.com/rlewczuk/csw/pkg/tool"
)

type pendingResponse struct {
	ch chan JSONRPCResponse
}

type client interface {
	Start() error
	Close() error
	ListTools() ([]RemoteTool, error)
	ListResources() ([]RemoteResource, error)
	ReadResource(uri string) (*ReadResourceResult, error)
	CallTool(name string, arguments map[string]any) (*CallToolResult, error)
}

var newClientFunc = func(name string, cfg *conf.MCPServerConfig) (client, error) {
	return NewClient(name, cfg)
}

const mcpSessionIDHeader = "Mcp-Session-Id"

const mcpProtocolVersionHeader = "MCP-Protocol-Version"

// Client is stdio JSON-RPC client for MCP server.
type Client struct {
	name    string
	command string
	args    []string
	env     map[string]string

	cmd    *exec.Cmd
	stdin  io.WriteCloser
	stdout io.ReadCloser
	stderr io.ReadCloser

	nextID  atomic.Int64
	pending map[int64]*pendingResponse
	mu      sync.RWMutex

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// NewClient creates a new MCP client based on configured transport.
func NewClient(name string, cfg *conf.MCPServerConfig) (client, error) {
	transport := resolveMCPTransportType(cfg)
	switch transport {
	case conf.MCPTransportTypeHTTP, conf.MCPTransportTypeHTTPS:
		return NewHTTPClient(name, cfg)
	default:
		return NewStdioClient(name, cfg)
	}
}

// NewStdioClient creates a new MCP stdio client.
func NewStdioClient(name string, cfg *conf.MCPServerConfig) (*Client, error) {
	if strings.TrimSpace(name) == "" {
		return nil, fmt.Errorf("NewStdioClient() [mcp.go]: name cannot be empty")
	}
	if cfg == nil {
		return nil, fmt.Errorf("NewStdioClient() [mcp.go]: config cannot be nil")
	}
	if strings.TrimSpace(cfg.Cmd) == "" {
		return nil, fmt.Errorf("NewStdioClient() [mcp.go]: cmd cannot be empty for server %s", name)
	}

	ctx, cancel := context.WithCancel(context.Background())

	client := &Client{
		name:    name,
		command: cfg.Cmd,
		args:    append([]string(nil), cfg.Args...),
		env:     cloneEnv(cfg.Env),
		pending: make(map[int64]*pendingResponse),
		ctx:     ctx,
		cancel:  cancel,
	}
	client.nextID.Store(1)

	return client, nil
}

// Start starts MCP server process and initializes protocol.
func (c *Client) Start() error {
	cmdName, cmdArgs, err := splitCommand(c.command)
	if err != nil {
		return fmt.Errorf("Client.Start() [mcp.go]: failed to parse command for %s: %w", c.name, err)
	}
	cmdArgs = append(cmdArgs, c.args...)

	c.cmd = exec.CommandContext(c.ctx, cmdName, cmdArgs...)
	c.cmd.Env = mergeEnv(os.Environ(), c.env)

	c.stdin, err = c.cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("Client.Start() [mcp.go]: failed to create stdin pipe for %s: %w", c.name, err)
	}
	c.stdout, err = c.cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("Client.Start() [mcp.go]: failed to create stdout pipe for %s: %w", c.name, err)
	}
	c.stderr, err = c.cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("Client.Start() [mcp.go]: failed to create stderr pipe for %s: %w", c.name, err)
	}

	if err := c.cmd.Start(); err != nil {
		return fmt.Errorf("Client.Start() [mcp.go]: failed to start server %s: %w", c.name, err)
	}

	c.wg.Add(2)
	go c.readLoop()
	go c.readStderr()

	if _, err := c.Initialize(); err != nil {
		_ = c.Close()
		return fmt.Errorf("Client.Start() [mcp.go]: initialize failed for %s: %w", c.name, err)
	}

	if err := c.sendNotification("notifications/initialized", map[string]any{}); err != nil {
		_ = c.Close()
		return fmt.Errorf("Client.Start() [mcp.go]: initialized notification failed for %s: %w", c.name, err)
	}

	return nil
}

// Close terminates server process and closes resources.
func (c *Client) Close() error {
	c.cancel()
	if c.stdin != nil {
		_ = c.stdin.Close()
	}
	if c.cmd != nil && c.cmd.Process != nil {
		_ = c.cmd.Process.Kill()
		_ = c.cmd.Wait()
	}
	c.wg.Wait()
	return nil
}

// Initialize performs MCP protocol initialization handshake.
func (c *Client) Initialize() (*InitializeResult, error) {
	params := InitializeRequestParams{
		ProtocolVersion: LatestProtocolVersion,
		Capabilities:    map[string]any{},
		ClientInfo: MCPImplementationInfo{
			Name:    "csw",
			Version: "dev",
		},
	}

	resultMap, err := c.request("initialize", params)
	if err != nil {
		return nil, err
	}

	data, err := json.Marshal(resultMap)
	if err != nil {
		return nil, fmt.Errorf("Client.Initialize() [mcp.go]: failed to marshal initialize result for %s: %w", c.name, err)
	}

	var result InitializeResult
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("Client.Initialize() [mcp.go]: failed to parse initialize result for %s: %w", c.name, err)
	}

	return &result, nil
}

// ListTools lists tools exposed by MCP server.
func (c *Client) ListTools() ([]RemoteTool, error) {
	resultMap, err := c.request("tools/list", map[string]any{})
	if err != nil {
		return nil, err
	}

	data, err := json.Marshal(resultMap)
	if err != nil {
		return nil, fmt.Errorf("Client.ListTools() [mcp.go]: failed to marshal tools/list result for %s: %w", c.name, err)
	}

	var result ListToolsResult
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("Client.ListTools() [mcp.go]: failed to parse tools/list result for %s: %w", c.name, err)
	}

	return result.Tools, nil
}

// ListResources lists resources exposed by MCP server.
func (c *Client) ListResources() ([]RemoteResource, error) {
	resultMap, err := c.request("resources/list", map[string]any{})
	if err != nil {
		return nil, err
	}

	data, err := json.Marshal(resultMap)
	if err != nil {
		return nil, fmt.Errorf("Client.ListResources() [mcp.go]: failed to marshal resources/list result for %s: %w", c.name, err)
	}

	var result ListResourcesResult
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("Client.ListResources() [mcp.go]: failed to parse resources/list result for %s: %w", c.name, err)
	}

	return result.Resources, nil
}

// ReadResource reads one MCP resource by URI.
func (c *Client) ReadResource(uri string) (*ReadResourceResult, error) {
	params := ReadResourceRequestParams{URI: uri}
	resultMap, err := c.request("resources/read", params)
	if err != nil {
		return nil, err
	}

	data, err := json.Marshal(resultMap)
	if err != nil {
		return nil, fmt.Errorf("Client.ReadResource() [mcp.go]: failed to marshal resources/read result for %s: %w", c.name, err)
	}

	var result ReadResourceResult
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("Client.ReadResource() [mcp.go]: failed to parse resources/read result for %s: %w", c.name, err)
	}

	return &result, nil
}

// CallTool invokes a specific MCP tool.
func (c *Client) CallTool(name string, arguments map[string]any) (*CallToolResult, error) {
	params := CallToolRequestParams{Name: name, Arguments: arguments}
	resultMap, err := c.request("tools/call", params)
	if err != nil {
		return nil, err
	}

	data, err := json.Marshal(resultMap)
	if err != nil {
		return nil, fmt.Errorf("Client.CallTool() [mcp.go]: failed to marshal tools/call result for %s: %w", c.name, err)
	}

	var result CallToolResult
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("Client.CallTool() [mcp.go]: failed to parse tools/call result for %s: %w", c.name, err)
	}

	return &result, nil
}

func (c *Client) request(method string, params any) (map[string]any, error) {
	id := c.nextID.Add(1)
	pending := &pendingResponse{ch: make(chan JSONRPCResponse, 1)}

	c.mu.Lock()
	c.pending[id] = pending
	c.mu.Unlock()

	req := JSONRPCRequest{JSONRPC: JSONRPCVersion, ID: id, Method: method, Params: params}
	if err := c.writeMessage(req); err != nil {
		c.mu.Lock()
		delete(c.pending, id)
		c.mu.Unlock()
		return nil, fmt.Errorf("Client.request() [mcp.go]: failed to write request %s for %s: %w", method, c.name, err)
	}

	resp := <-pending.ch
	if resp.Error != nil {
		return nil, fmt.Errorf("Client.request() [mcp.go]: mcp error from %s: code=%d message=%s", c.name, resp.Error.Code, resp.Error.Message)
	}

	return resp.Result, nil
}

func (c *Client) sendNotification(method string, params any) error {
	notif := JSONRPCNotification{JSONRPC: JSONRPCVersion, Method: method, Params: params}
	if err := c.writeMessage(notif); err != nil {
		return fmt.Errorf("Client.sendNotification() [mcp.go]: failed to send notification %s for %s: %w", method, c.name, err)
	}
	return nil
}

func (c *Client) writeMessage(msg any) error {
	if err := WriteMessage(c.stdin, msg); err != nil {
		return fmt.Errorf("Client.writeMessage() [mcp.go]: failed to write message for %s: %w", c.name, err)
	}

	return nil
}

func (c *Client) readLoop() {
	defer c.wg.Done()
	reader := bufio.NewReader(c.stdout)

	for {
		content, err := ReadMessage(reader)
		if err != nil {
			return
		}

		c.handleMessage(content)
	}
}

func (c *Client) handleMessage(data []byte) {
	var response JSONRPCResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return
	}
	if response.ID == 0 {
		return
	}

	c.mu.Lock()
	pending, ok := c.pending[response.ID]
	if ok {
		delete(c.pending, response.ID)
	}
	c.mu.Unlock()

	if ok {
		pending.ch <- response
	}
}

func (c *Client) readStderr() {
	defer c.wg.Done()
	scanner := bufio.NewScanner(c.stderr)
	for scanner.Scan() {
		_ = scanner.Text()
	}
}

// HTTPClient is streamable HTTP JSON-RPC client for MCP server.
type HTTPClient struct {
	name      string
	endpoint  string
	apiKey    string
	http      *http.Client
	nextID    atomic.Int64
	protocol  string
	started   bool
	sessionID string
	mu        sync.RWMutex
}

// NewHTTPClient creates a new MCP streamable HTTP client.
func NewHTTPClient(name string, cfg *conf.MCPServerConfig) (*HTTPClient, error) {
	if strings.TrimSpace(name) == "" {
		return nil, fmt.Errorf("NewHTTPClient() [mcp.go]: name cannot be empty")
	}
	if cfg == nil {
		return nil, fmt.Errorf("NewHTTPClient() [mcp.go]: config cannot be nil")
	}

	endpoint := strings.TrimSpace(cfg.URL)
	if endpoint == "" {
		return nil, fmt.Errorf("NewHTTPClient() [mcp.go]: url cannot be empty for server %s", name)
	}

	parsedURL, err := url.Parse(endpoint)
	if err != nil {
		return nil, fmt.Errorf("NewHTTPClient() [mcp.go]: invalid url for server %s: %w", name, err)
	}
	if strings.TrimSpace(parsedURL.Scheme) == "" || strings.TrimSpace(parsedURL.Host) == "" {
		return nil, fmt.Errorf("NewHTTPClient() [mcp.go]: url must include scheme and host for server %s", name)
	}

	transport := resolveMCPTransportType(cfg)
	if transport == conf.MCPTransportTypeHTTPS && !strings.EqualFold(parsedURL.Scheme, "https") {
		return nil, fmt.Errorf("NewHTTPClient() [mcp.go]: https transport requires https url for server %s", name)
	}
	if transport == conf.MCPTransportTypeHTTP && !strings.EqualFold(parsedURL.Scheme, "http") {
		return nil, fmt.Errorf("NewHTTPClient() [mcp.go]: http transport requires http url for server %s", name)
	}

	client := &HTTPClient{
		name:     name,
		endpoint: endpoint,
		apiKey:   strings.TrimSpace(cfg.APIKey),
		http:     http.DefaultClient,
		protocol: LatestProtocolVersion,
	}
	client.nextID.Store(1)

	return client, nil
}

// Start initializes MCP protocol over HTTP transport.
func (c *HTTPClient) Start() error {
	if _, err := c.Initialize(); err != nil {
		return fmt.Errorf("HTTPClient.Start() [mcp.go]: initialize failed for %s: %w", c.name, err)
	}

	if err := c.sendNotification("notifications/initialized", map[string]any{}); err != nil {
		return fmt.Errorf("HTTPClient.Start() [mcp.go]: initialized notification failed for %s: %w", c.name, err)
	}

	c.mu.Lock()
	c.started = true
	c.mu.Unlock()

	return nil
}

// Close closes HTTP client session when server supports explicit session termination.
func (c *HTTPClient) Close() error {
	c.mu.Lock()
	c.started = false
	sessionID := c.sessionID
	c.mu.Unlock()

	if strings.TrimSpace(sessionID) == "" {
		return nil
	}

	req, err := http.NewRequest(http.MethodDelete, c.endpoint, nil)
	if err != nil {
		return fmt.Errorf("HTTPClient.Close() [mcp.go]: failed to create delete request for %s: %w", c.name, err)
	}
	req.Header.Set(mcpSessionIDHeader, sessionID)
	req.Header.Set(mcpProtocolVersionHeader, c.protocol)

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("HTTPClient.Close() [mcp.go]: delete request failed for %s: %w", c.name, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusMethodNotAllowed || resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusNoContent || resp.StatusCode == http.StatusOK {
		return nil
	}

	if resp.StatusCode >= http.StatusBadRequest {
		return fmt.Errorf("HTTPClient.Close() [mcp.go]: unexpected delete status for %s: %s", c.name, resp.Status)
	}

	return nil
}

// Initialize performs MCP protocol initialization handshake.
func (c *HTTPClient) Initialize() (*InitializeResult, error) {
	params := InitializeRequestParams{
		ProtocolVersion: LatestProtocolVersion,
		Capabilities:    map[string]any{},
		ClientInfo: MCPImplementationInfo{
			Name:    "csw",
			Version: "dev",
		},
	}

	resultMap, header, err := c.requestWithHeaders("initialize", params)
	if err != nil {
		return nil, err
	}

	if sessionID := strings.TrimSpace(header.Get(mcpSessionIDHeader)); sessionID != "" {
		c.mu.Lock()
		c.sessionID = sessionID
		c.mu.Unlock()
	}

	data, err := json.Marshal(resultMap)
	if err != nil {
		return nil, fmt.Errorf("HTTPClient.Initialize() [mcp.go]: failed to marshal initialize result for %s: %w", c.name, err)
	}

	var result InitializeResult
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("HTTPClient.Initialize() [mcp.go]: failed to parse initialize result for %s: %w", c.name, err)
	}

	return &result, nil
}

// ListTools lists tools exposed by MCP server.
func (c *HTTPClient) ListTools() ([]RemoteTool, error) {
	resultMap, _, err := c.requestWithHeaders("tools/list", map[string]any{})
	if err != nil {
		return nil, err
	}

	data, err := json.Marshal(resultMap)
	if err != nil {
		return nil, fmt.Errorf("HTTPClient.ListTools() [mcp.go]: failed to marshal tools/list result for %s: %w", c.name, err)
	}

	var result ListToolsResult
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("HTTPClient.ListTools() [mcp.go]: failed to parse tools/list result for %s: %w", c.name, err)
	}

	return result.Tools, nil
}

// ListResources lists resources exposed by MCP server.
func (c *HTTPClient) ListResources() ([]RemoteResource, error) {
	resultMap, _, err := c.requestWithHeaders("resources/list", map[string]any{})
	if err != nil {
		return nil, err
	}

	data, err := json.Marshal(resultMap)
	if err != nil {
		return nil, fmt.Errorf("HTTPClient.ListResources() [mcp.go]: failed to marshal resources/list result for %s: %w", c.name, err)
	}

	var result ListResourcesResult
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("HTTPClient.ListResources() [mcp.go]: failed to parse resources/list result for %s: %w", c.name, err)
	}

	return result.Resources, nil
}

// ReadResource reads one MCP resource by URI.
func (c *HTTPClient) ReadResource(uri string) (*ReadResourceResult, error) {
	params := ReadResourceRequestParams{URI: uri}
	resultMap, _, err := c.requestWithHeaders("resources/read", params)
	if err != nil {
		return nil, err
	}

	data, err := json.Marshal(resultMap)
	if err != nil {
		return nil, fmt.Errorf("HTTPClient.ReadResource() [mcp.go]: failed to marshal resources/read result for %s: %w", c.name, err)
	}

	var result ReadResourceResult
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("HTTPClient.ReadResource() [mcp.go]: failed to parse resources/read result for %s: %w", c.name, err)
	}

	return &result, nil
}

// CallTool invokes a specific MCP tool.
func (c *HTTPClient) CallTool(name string, arguments map[string]any) (*CallToolResult, error) {
	params := CallToolRequestParams{Name: name, Arguments: arguments}
	resultMap, _, err := c.requestWithHeaders("tools/call", params)
	if err != nil {
		return nil, err
	}

	data, err := json.Marshal(resultMap)
	if err != nil {
		return nil, fmt.Errorf("HTTPClient.CallTool() [mcp.go]: failed to marshal tools/call result for %s: %w", c.name, err)
	}

	var result CallToolResult
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("HTTPClient.CallTool() [mcp.go]: failed to parse tools/call result for %s: %w", c.name, err)
	}

	return &result, nil
}

func (c *HTTPClient) requestWithHeaders(method string, params any) (map[string]any, http.Header, error) {
	id := c.nextID.Add(1)
	req := JSONRPCRequest{JSONRPC: JSONRPCVersion, ID: id, Method: method, Params: params}
	response, header, err := c.postJSONRPC(req, true, id)
	if err != nil {
		return nil, nil, fmt.Errorf("HTTPClient.requestWithHeaders() [mcp.go]: request %s failed for %s: %w", method, c.name, err)
	}
	if response.Error != nil {
		return nil, nil, fmt.Errorf("HTTPClient.requestWithHeaders() [mcp.go]: mcp error from %s: code=%d message=%s", c.name, response.Error.Code, response.Error.Message)
	}

	return response.Result, header, nil
}

func (c *HTTPClient) sendNotification(method string, params any) error {
	notification := JSONRPCNotification{JSONRPC: JSONRPCVersion, Method: method, Params: params}
	_, _, err := c.postJSONRPC(notification, false, 0)
	if err != nil {
		return fmt.Errorf("HTTPClient.sendNotification() [mcp.go]: notification %s failed for %s: %w", method, c.name, err)
	}

	return nil
}

func (c *HTTPClient) postJSONRPC(message any, expectResponse bool, expectedID int64) (*JSONRPCResponse, http.Header, error) {
	body, err := json.Marshal(message)
	if err != nil {
		return nil, nil, fmt.Errorf("HTTPClient.postJSONRPC() [mcp.go]: failed to marshal request for %s: %w", c.name, err)
	}

	req, err := http.NewRequest(http.MethodPost, c.endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, nil, fmt.Errorf("HTTPClient.postJSONRPC() [mcp.go]: failed to create request for %s: %w", c.name, err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	req.Header.Set(mcpProtocolVersionHeader, c.protocol)
	if strings.TrimSpace(c.apiKey) != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	}

	c.mu.RLock()
	sessionID := c.sessionID
	c.mu.RUnlock()
	if strings.TrimSpace(sessionID) != "" {
		req.Header.Set(mcpSessionIDHeader, sessionID)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, nil, fmt.Errorf("HTTPClient.postJSONRPC() [mcp.go]: request failed for %s: %w", c.name, err)
	}
	defer resp.Body.Close()

	if !expectResponse {
		if resp.StatusCode != http.StatusAccepted {
			return nil, resp.Header, fmt.Errorf("HTTPClient.postJSONRPC() [mcp.go]: expected 202 Accepted for notification from %s, got %s", c.name, resp.Status)
		}
		return nil, resp.Header, nil
	}

	if resp.StatusCode >= http.StatusBadRequest {
		return nil, resp.Header, fmt.Errorf("HTTPClient.postJSONRPC() [mcp.go]: request failed for %s with status %s", c.name, resp.Status)
	}

	contentType := strings.ToLower(strings.TrimSpace(strings.Split(resp.Header.Get("Content-Type"), ";")[0]))
	switch contentType {
	case "application/json":
		decoder := json.NewDecoder(resp.Body)
		var response JSONRPCResponse
		if err := decoder.Decode(&response); err != nil {
			return nil, resp.Header, fmt.Errorf("HTTPClient.postJSONRPC() [mcp.go]: failed to decode json response from %s: %w", c.name, err)
		}
		if response.ID != expectedID {
			return nil, resp.Header, fmt.Errorf("HTTPClient.postJSONRPC() [mcp.go]: response id mismatch for %s: expected %d got %d", c.name, expectedID, response.ID)
		}
		return &response, resp.Header, nil
	case "text/event-stream":
		response, err := readSSEJSONRPCResponse(resp.Body, expectedID)
		if err != nil {
			return nil, resp.Header, fmt.Errorf("HTTPClient.postJSONRPC() [mcp.go]: failed to parse event stream response from %s: %w", c.name, err)
		}
		return response, resp.Header, nil
	default:
		return nil, resp.Header, fmt.Errorf("HTTPClient.postJSONRPC() [mcp.go]: unsupported content type from %s: %s", c.name, contentType)
	}
}

func readSSEJSONRPCResponse(reader io.Reader, expectedID int64) (*JSONRPCResponse, error) {
	if reader == nil {
		return nil, fmt.Errorf("readSSEJSONRPCResponse() [mcp.go]: reader cannot be nil")
	}

	scanner := bufio.NewScanner(reader)
	dataLines := make([]string, 0)

	processEvent := func(lines []string) (*JSONRPCResponse, bool, error) {
		if len(lines) == 0 {
			return nil, false, nil
		}
		payload := strings.TrimSpace(strings.Join(lines, "\n"))
		if payload == "" || payload == "[DONE]" {
			return nil, false, nil
		}

		var response JSONRPCResponse
		if err := json.Unmarshal([]byte(payload), &response); err != nil {
			return nil, false, nil
		}
		if response.ID == expectedID {
			return &response, true, nil
		}

		return nil, false, nil
	}

	for scanner.Scan() {
		line := scanner.Text()
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			response, found, err := processEvent(dataLines)
			if err != nil {
				return nil, err
			}
			if found {
				return response, nil
			}
			dataLines = dataLines[:0]
			continue
		}
		if strings.HasPrefix(line, "data:") {
			dataLines = append(dataLines, strings.TrimSpace(strings.TrimPrefix(line, "data:")))
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("readSSEJSONRPCResponse() [mcp.go]: failed to scan event stream: %w", err)
	}

	response, found, err := processEvent(dataLines)
	if err != nil {
		return nil, err
	}
	if found {
		return response, nil
	}

	return nil, fmt.Errorf("readSSEJSONRPCResponse() [mcp.go]: matching response not found")
}

func resolveMCPTransportType(cfg *conf.MCPServerConfig) conf.MCPTransportType {
	if cfg == nil {
		return conf.MCPTransportTypeStdio
	}
	transport := conf.MCPTransportType(strings.ToLower(strings.TrimSpace(string(cfg.Transport))))
	switch transport {
	case conf.MCPTransportTypeHTTP, conf.MCPTransportTypeHTTPS, conf.MCPTransportTypeStdio:
		return transport
	default:
		return conf.MCPTransportTypeStdio
	}
}

// Manager manages MCP server lifecycle and exposes MCP tools.
type Manager struct {
	clients  map[string]client
	mcpTools map[string]tool.ToolInfo
	toolMap  map[string]mcpToolTarget
	mutex    sync.RWMutex
}

type mcpToolTarget struct {
	serverName string
	toolName   string
}

// NewManager creates MCP manager from configuration store.
func NewManager(store conf.ConfigStore) (*Manager, error) {
	configs, err := store.GetMCPServerConfigs()
	if err != nil {
		return nil, fmt.Errorf("NewManager() [mcp.go]: failed to load mcp server configs: %w", err)
	}

	manager := &Manager{
		clients:  make(map[string]client),
		mcpTools: make(map[string]tool.ToolInfo),
		toolMap:  make(map[string]mcpToolTarget),
	}

	for serverName, cfg := range configs {
		if cfg == nil || !cfg.Enabled {
			continue
		}

		client, clientErr := newClientFunc(serverName, cfg)
		if clientErr != nil {
			return nil, fmt.Errorf("NewManager() [mcp.go]: failed to create client for %s: %w", serverName, clientErr)
		}
		if startErr := client.Start(); startErr != nil {
			return nil, fmt.Errorf("NewManager() [mcp.go]: failed to start client for %s: %w", serverName, startErr)
		}

		tools, toolsErr := client.ListTools()
		if toolsErr != nil {
			_ = client.Close()
			return nil, fmt.Errorf("NewManager() [mcp.go]: failed to list tools for %s: %w", serverName, toolsErr)
		}

		compiledMatchers, matcherErr := compileToolMatchers(cfg.Tools)
		if matcherErr != nil {
			_ = client.Close()
			return nil, fmt.Errorf("NewManager() [mcp.go]: invalid tool pattern in %s: %w", serverName, matcherErr)
		}

		for _, remoteTool := range tools {
			if !isToolEnabled(remoteTool.Name, cfg.Tools, compiledMatchers) {
				continue
			}

			qualifiedName := BuildQualifiedToolName(serverName, remoteTool.Name)
			toolInfo, convErr := convertMCPToolInfo(qualifiedName, remoteTool)
			if convErr != nil {
				_ = client.Close()
				return nil, convErr
			}
			manager.mcpTools[qualifiedName] = toolInfo
			manager.toolMap[qualifiedName] = mcpToolTarget{serverName: serverName, toolName: remoteTool.Name}
		}

		manager.clients[serverName] = client
	}

	return manager, nil
}

// Close closes all MCP clients.
func (m *Manager) Close() error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	var firstErr error
	for name, client := range m.clients {
		if err := client.Close(); err != nil && firstErr == nil {
			firstErr = fmt.Errorf("Manager.Close() [mcp.go]: failed to close %s: %w", name, err)
		}
	}

	m.clients = make(map[string]client)
	return firstErr
}

// ToolInfos returns MCP tool infos keyed by qualified names.
func (m *Manager) ToolInfos() map[string]tool.ToolInfo {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	result := make(map[string]tool.ToolInfo, len(m.mcpTools))
	for key, info := range m.mcpTools {
		result[key] = info
	}
	return result
}

// ExecuteTool executes qualified MCP tool by forwarding request to target server.
func (m *Manager) ExecuteTool(call *tool.ToolCall) *tool.ToolResponse {
	if call == nil {
		return &tool.ToolResponse{Call: call, Error: fmt.Errorf("Manager.ExecuteTool() [mcp.go]: call cannot be nil"), Done: true}
	}

	m.mutex.RLock()
	target, ok := m.toolMap[call.Function]
	if !ok {
		m.mutex.RUnlock()
		return &tool.ToolResponse{Call: call, Error: fmt.Errorf("Manager.ExecuteTool() [mcp.go]: mcp tool not found: %s", call.Function), Done: true}
	}
	client, ok := m.clients[target.serverName]
	m.mutex.RUnlock()
	if !ok {
		return &tool.ToolResponse{Call: call, Error: fmt.Errorf("Manager.ExecuteTool() [mcp.go]: mcp server not running: %s", target.serverName), Done: true}
	}

	arguments := map[string]any{}
	if call.Arguments.Raw() != nil {
		if obj, ok := call.Arguments.ObjectOK(); ok {
			for key, value := range obj {
				arguments[key] = value.Raw()
			}
		}
	}

	result, err := client.CallTool(target.toolName, arguments)
	if err != nil {
		return &tool.ToolResponse{Call: call, Error: fmt.Errorf("Manager.ExecuteTool() [mcp.go]: tools/call failed for %s: %w", call.Function, err), Done: true}
	}

	payload := map[string]any{
		"isError": result.IsError,
	}
	if result.StructuredContent != nil {
		payload["structuredContent"] = result.StructuredContent
	}
	if len(result.Content) > 0 {
		contentBlocks := make([]map[string]any, 0, len(result.Content))
		var textParts []string
		for _, block := range result.Content {
			blockMap := map[string]any{}
			if strings.TrimSpace(block.Type) != "" {
				blockMap["type"] = block.Type
			}
			if strings.TrimSpace(block.Text) != "" {
				blockMap["text"] = block.Text
				textParts = append(textParts, block.Text)
			}
			contentBlocks = append(contentBlocks, blockMap)
		}
		payload["content"] = contentBlocks
		if len(textParts) > 0 {
			payload["text"] = strings.Join(textParts, "\n")
		}
	}

	if result.IsError {
		message := "mcp tool returned error"
		if text, ok := payload["text"].(string); ok && strings.TrimSpace(text) != "" {
			message = text
		}
		return &tool.ToolResponse{Call: call, Error: fmt.Errorf("Manager.ExecuteTool() [mcp.go]: %s", message), Result: tool.NewToolValue(payload), Done: true}
	}

	return &tool.ToolResponse{Call: call, Result: tool.NewToolValue(payload), Done: true}
}

// Tool wraps manager tool forwarding in tool.Tool interface.
type Tool struct {
	manager interface {
		ExecuteTool(call *tool.ToolCall) *tool.ToolResponse
	}
	name       string
	serverName string
	original   string
}

// NewTool creates MCP forwarding tool.
func NewTool(manager interface {
	ExecuteTool(call *tool.ToolCall) *tool.ToolResponse
}, name string, serverName string, originalName string) *Tool {
	return &Tool{manager: manager, name: name, serverName: serverName, original: originalName}
}

// Execute forwards tool call to MCP manager.
func (t *Tool) Execute(args *tool.ToolCall) *tool.ToolResponse {
	if t.manager == nil {
		return &tool.ToolResponse{Call: args, Error: fmt.Errorf("Tool.Execute() [mcp.go]: manager is nil"), Done: true}
	}
	return t.manager.ExecuteTool(args)
}

// Render renders MCP tool call summary.
func (t *Tool) Render(call *tool.ToolCall) (string, string, string, map[string]string) {
	oneLine := fmt.Sprintf("MCP %s/%s", t.serverName, t.original)
	full := fmt.Sprintf("MCP tool %s/%s", t.serverName, t.original)
	jsonl := tool.NewToolValue(map[string]any{})
	jsonl.Set("tool", strings.TrimSpace(t.name))
	jsonl.Set("status", "success")
	jsonl.Set("server", t.serverName)
	jsonl.Set("mcp_tool", t.original)
	jsonBytes, err := json.Marshal(jsonl.Raw())
	if err != nil {
		jsonBytes = []byte(`{"tool":"mcp","status":"success"}`)
	}
	return oneLine, full, string(jsonBytes), map[string]string{"server": t.serverName, "tool": t.original}
}

// GetDescription returns empty dynamic description (static description comes from MCP schema).
func (t *Tool) GetDescription() (string, bool) {
	return "", false
}

// BuildQualifiedToolName creates unique public tool name for MCP tool.
func BuildQualifiedToolName(serverName string, toolName string) string {
	return "mcp." + strings.TrimSpace(serverName) + "." + strings.TrimSpace(toolName)
}

type toolRegistrar interface {
	ToolInfos() map[string]tool.ToolInfo
	ExecuteTool(call *tool.ToolCall) *tool.ToolResponse
}

// RegisterTools registers all manager MCP tools into tool registry.
func RegisterTools(registry *tool.ToolRegistry, manager toolRegistrar) error {
	if registry == nil {
		return fmt.Errorf("RegisterTools() [mcp.go]: registry cannot be nil")
	}
	if manager == nil {
		return nil
	}

	infos := manager.ToolInfos()
	for qualified := range infos {
		serverName, originalName, err := parseQualifiedToolName(qualified)
		if err != nil {
			return err
		}
		registry.Register(qualified, NewTool(manager, qualified, serverName, originalName))
	}

	return nil
}

// GetToolInfo returns MCP tool info for prompt generator override.
func (m *Manager) GetToolInfo(toolName string) (tool.ToolInfo, bool) {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	info, ok := m.mcpTools[toolName]
	return info, ok
}

func convertMCPToolInfo(qualifiedName string, mcpTool RemoteTool) (tool.ToolInfo, error) {
	schema := tool.NewToolSchema()
	if inputSchema, err := convertSchemaMap(mcpTool.InputSchema); err == nil {
		schema = inputSchema
	} else {
		return tool.ToolInfo{}, fmt.Errorf("convertMCPToolInfo() [mcp.go]: failed to convert input schema for %s: %w", qualifiedName, err)
	}

	description := strings.TrimSpace(mcpTool.Description)
	if description == "" {
		description = "MCP tool"
	}

	return tool.ToolInfo{Name: qualifiedName, Description: description, Schema: schema}, nil
}

func convertSchemaMap(input map[string]any) (tool.ToolSchema, error) {
	schema := tool.NewToolSchema()
	if input == nil {
		return schema, nil
	}

	if schemaValue, ok := input["$schema"].(string); ok && strings.TrimSpace(schemaValue) != "" {
		schema.Schema = schemaValue
	}
	if typeValue, ok := input["type"].(string); ok && strings.TrimSpace(typeValue) != "" {
		schema.Type = tool.SchemaType(typeValue)
	}
	if additional, ok := input["additionalProperties"].(bool); ok {
		schema.AdditionalProperties = additional
	}

	requiredSet := make(map[string]struct{})
	if requiredRaw, ok := input["required"].([]any); ok {
		for _, item := range requiredRaw {
			if name, ok := item.(string); ok {
				requiredSet[name] = struct{}{}
			}
		}
	}

	propertiesRaw, ok := input["properties"].(map[string]any)
	if !ok {
		return schema, nil
	}

	for name, raw := range propertiesRaw {
		propertyMap, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		prop, err := convertProperty(propertyMap)
		if err != nil {
			return schema, err
		}
		_, required := requiredSet[name]
		schema.AddProperty(name, prop, required)
	}

	return schema, nil
}

func convertProperty(input map[string]any) (tool.PropertySchema, error) {
	result := tool.PropertySchema{}

	if typeValue, ok := input["type"].(string); ok {
		result.Type = tool.SchemaType(typeValue)
	}
	if description, ok := input["description"].(string); ok {
		result.Description = description
	}
	if enumRaw, ok := input["enum"].([]any); ok {
		for _, item := range enumRaw {
			if value, ok := item.(string); ok {
				result.Enum = append(result.Enum, value)
			}
		}
	}

	if itemsRaw, ok := input["items"].(map[string]any); ok {
		items, err := convertProperty(itemsRaw)
		if err != nil {
			return result, err
		}
		result.Items = &items
	}

	if propertiesRaw, ok := input["properties"].(map[string]any); ok {
		result.Properties = make(map[string]tool.PropertySchema)
		for key, value := range propertiesRaw {
			nestedMap, ok := value.(map[string]any)
			if !ok {
				continue
			}
			nested, err := convertProperty(nestedMap)
			if err != nil {
				return result, err
			}
			result.Properties[key] = nested
		}
	}

	if requiredRaw, ok := input["required"].([]any); ok {
		for _, item := range requiredRaw {
			if value, ok := item.(string); ok {
				result.Required = append(result.Required, value)
			}
		}
	}

	if additional, ok := input["additionalProperties"].(bool); ok {
		result.AdditionalProperties = &additional
	}

	return result, nil
}

func parseQualifiedToolName(qualified string) (string, string, error) {
	parts := strings.SplitN(qualified, ".", 3)
	if len(parts) != 3 || parts[0] != "mcp" {
		return "", "", fmt.Errorf("parseQualifiedToolName() [mcp.go]: invalid qualified tool name: %s", qualified)
	}
	return parts[1], parts[2], nil
}

type toolMatcher struct {
	exact string
	glob  string
}

func compileToolMatchers(patterns []string) ([]toolMatcher, error) {
	if len(patterns) == 0 {
		return nil, nil
	}

	matchers := make([]toolMatcher, 0, len(patterns))
	for _, pattern := range patterns {
		trimmed := strings.TrimSpace(pattern)
		if trimmed == "" {
			continue
		}

		if isGlobPattern(trimmed) {
			if _, err := filepath.Match(trimmed, ""); err != nil {
				return nil, fmt.Errorf("compileToolMatchers() [mcp.go]: invalid glob %q: %w", trimmed, err)
			}
			matchers = append(matchers, toolMatcher{glob: trimmed})
			continue
		}

		matchers = append(matchers, toolMatcher{exact: trimmed})
	}

	return matchers, nil
}

func isToolEnabled(toolName string, patterns []string, matchers []toolMatcher) bool {
	if len(patterns) == 0 {
		return true
	}

	for _, matcher := range matchers {
		if matcher.exact != "" && matcher.exact == toolName {
			return true
		}
		if matcher.glob == "" {
			continue
		}
		if ok, _ := filepath.Match(matcher.glob, toolName); ok {
			return true
		}
	}

	return false
}

func isGlobPattern(pattern string) bool {
	return strings.ContainsAny(pattern, "*?[")
}

func cloneEnv(env map[string]string) map[string]string {
	if env == nil {
		return nil
	}
	cloned := make(map[string]string, len(env))
	for key, value := range env {
		cloned[key] = value
	}
	return cloned
}

func mergeEnv(base []string, override map[string]string) []string {
	if len(override) == 0 {
		return append([]string(nil), base...)
	}

	envMap := make(map[string]string, len(base)+len(override))
	for _, item := range base {
		parts := strings.SplitN(item, "=", 2)
		if len(parts) == 2 {
			envMap[parts[0]] = parts[1]
		}
	}
	for key, value := range override {
		envMap[key] = value
	}

	result := make([]string, 0, len(envMap))
	for key, value := range envMap {
		result = append(result, key+"="+value)
	}
	return result
}

func splitCommand(command string) (string, []string, error) {
	parts := strings.Fields(strings.TrimSpace(command))
	if len(parts) == 0 {
		return "", nil, fmt.Errorf("splitCommand() [mcp.go]: empty command")
	}
	return parts[0], parts[1:], nil
}

// WriteMessage writes framed JSON-RPC message to writer.
func WriteMessage(writer io.Writer, msg any) error {
	if writer == nil {
		return fmt.Errorf("WriteMessage() [mcp.go]: writer cannot be nil")
	}

	data, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("WriteMessage() [mcp.go]: failed to marshal message: %w", err)
	}

	header := fmt.Sprintf("Content-Length: %d\r\n\r\n", len(data))
	if _, err := writer.Write([]byte(header)); err != nil {
		return fmt.Errorf("WriteMessage() [mcp.go]: failed to write header: %w", err)
	}
	if _, err := writer.Write(data); err != nil {
		return fmt.Errorf("WriteMessage() [mcp.go]: failed to write body: %w", err)
	}

	return nil
}

// ReadMessage reads framed JSON-RPC payload bytes from reader.
func ReadMessage(reader *bufio.Reader) ([]byte, error) {
	if reader == nil {
		return nil, fmt.Errorf("ReadMessage() [mcp.go]: reader cannot be nil")
	}

	headers := make(map[string]string)
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			return nil, fmt.Errorf("ReadMessage() [mcp.go]: failed to read header line: %w", err)
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

	contentLengthStr, ok := headers["Content-Length"]
	if !ok {
		return nil, fmt.Errorf("ReadMessage() [mcp.go]: missing Content-Length header")
	}

	contentLength, err := strconv.Atoi(contentLengthStr)
	if err != nil {
		return nil, fmt.Errorf("ReadMessage() [mcp.go]: invalid Content-Length value: %w", err)
	}

	content := make([]byte, contentLength)
	if _, err := io.ReadFull(reader, content); err != nil {
		return nil, fmt.Errorf("ReadMessage() [mcp.go]: failed to read message body: %w", err)
	}

	return content, nil
}

// PromptGenerator wraps existing prompt generator and injects MCP tool infos.
type PromptGenerator struct {
	base    core.PromptGenerator
	manager interface {
		GetToolInfo(toolName string) (tool.ToolInfo, bool)
	}
}

// NewPromptGenerator creates prompt generator wrapper for MCP tools.
func NewPromptGenerator(base core.PromptGenerator, manager interface {
	GetToolInfo(toolName string) (tool.ToolInfo, bool)
}) core.PromptGenerator {
	if manager == nil {
		return base
	}
	return &PromptGenerator{base: base, manager: manager}
}

// GetPrompt delegates to base prompt generator.
func (g *PromptGenerator) GetPrompt(tags []string, role *conf.AgentRoleConfig, state *core.AgentState) (string, error) {
	return g.base.GetPrompt(tags, role, state)
}

// GetToolInfo returns MCP tool info when available, otherwise delegates to base generator.
func (g *PromptGenerator) GetToolInfo(tags []string, toolName string, role *conf.AgentRoleConfig, state *core.AgentState) (tool.ToolInfo, error) {
	if g.manager != nil {
		if info, ok := g.manager.GetToolInfo(toolName); ok {
			return info, nil
		}
	}
	return g.base.GetToolInfo(tags, toolName, role, state)
}

// GetAgentFiles delegates to base prompt generator.
func (g *PromptGenerator) GetAgentFiles(dir string) (map[string]string, error) {
	return g.base.GetAgentFiles(dir)
}
