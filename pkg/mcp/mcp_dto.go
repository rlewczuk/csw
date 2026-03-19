package mcp

// LatestProtocolVersion is the MCP protocol version used by client initialization.
const LatestProtocolVersion = "2025-11-25"

// JSONRPCVersion is the JSON-RPC protocol version.
const JSONRPCVersion = "2.0"

// JSONRPCError represents JSON-RPC error payload.
type JSONRPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

// JSONRPCRequest represents JSON-RPC request message.
type JSONRPCRequest struct {
	JSONRPC string `json:"jsonrpc"`
	ID      int64  `json:"id"`
	Method  string `json:"method"`
	Params  any    `json:"params,omitempty"`
}

// JSONRPCNotification represents JSON-RPC notification message.
type JSONRPCNotification struct {
	JSONRPC string `json:"jsonrpc"`
	Method  string `json:"method"`
	Params  any    `json:"params,omitempty"`
}

// JSONRPCResponse represents JSON-RPC response payload.
type JSONRPCResponse struct {
	JSONRPC string         `json:"jsonrpc"`
	ID      int64          `json:"id,omitempty"`
	Result  map[string]any `json:"result,omitempty"`
	Error   *JSONRPCError  `json:"error,omitempty"`
}

// InitializeRequestParams are parameters for MCP initialize request.
type InitializeRequestParams struct {
	ProtocolVersion string                `json:"protocolVersion"`
	Capabilities    map[string]any        `json:"capabilities"`
	ClientInfo      MCPImplementationInfo `json:"clientInfo"`
}

// MCPImplementationInfo identifies MCP endpoint implementation.
type MCPImplementationInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// InitializeResult contains MCP server initialization response.
type InitializeResult struct {
	ProtocolVersion string                `json:"protocolVersion"`
	Capabilities    map[string]any        `json:"capabilities"`
	ServerInfo      MCPImplementationInfo `json:"serverInfo"`
	Instructions    string                `json:"instructions,omitempty"`
}

// RemoteTool defines MCP tool metadata returned by tools/list.
type RemoteTool struct {
	Name        string         `json:"name"`
	Description string         `json:"description,omitempty"`
	InputSchema map[string]any `json:"inputSchema"`
	Annotations map[string]any `json:"annotations,omitempty"`
	Meta        map[string]any `json:"_meta,omitempty"`
}

// ListToolsResult is result payload for tools/list request.
type ListToolsResult struct {
	Tools []RemoteTool `json:"tools"`
}

// RemoteResource defines MCP resource metadata returned by resources/list.
type RemoteResource struct {
	URI         string         `json:"uri"`
	Name        string         `json:"name,omitempty"`
	Description string         `json:"description,omitempty"`
	MimeType    string         `json:"mimeType,omitempty"`
	Meta        map[string]any `json:"_meta,omitempty"`
}

// ListResourcesResult is result payload for resources/list request.
type ListResourcesResult struct {
	Resources []RemoteResource `json:"resources"`
}

// ReadResourceRequestParams are parameters for resources/read request.
type ReadResourceRequestParams struct {
	URI string `json:"uri"`
}

// ResourceContent is a single resources/read response content entry.
type ResourceContent struct {
	URI      string `json:"uri,omitempty"`
	MimeType string `json:"mimeType,omitempty"`
	Text     string `json:"text,omitempty"`
	Blob     string `json:"blob,omitempty"`
}

// ReadResourceResult is result payload for resources/read request.
type ReadResourceResult struct {
	Contents []ResourceContent `json:"contents"`
}

// CallToolRequestParams are parameters for tools/call request.
type CallToolRequestParams struct {
	Name      string         `json:"name"`
	Arguments map[string]any `json:"arguments,omitempty"`
}

// MCPContentBlock is tool result content block.
type MCPContentBlock struct {
	Type string `json:"type,omitempty"`
	Text string `json:"text,omitempty"`
}

// CallToolResult is result payload for tools/call request.
type CallToolResult struct {
	Content           []MCPContentBlock `json:"content"`
	StructuredContent map[string]any    `json:"structuredContent,omitempty"`
	IsError           bool              `json:"isError,omitempty"`
}
