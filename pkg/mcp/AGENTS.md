# Package `pkg/mcp` Overview

Package `pkg/mcp` contains MCP (Model Context Protocol) client implementation for connecting to external MCP servers via stdio and HTTP transports, managing server lifecycle, and exposing remote tools to the agent's tool registry.

## Important files

* `mcp.go` - MCP clients, manager, and tool integration
* `mcp_dto.go` - JSON-RPC DTOs and protocol constants
* `mcp_mock.go` - Mock manager for testing
* `mcp_test.go` - Unit tests for MCP functionality

## Important public API objects

* `Client` - stdio JSON-RPC client for MCP server
* `HTTPClient` - streamable HTTP JSON-RPC client for MCP server
* `Manager` - manages MCP server lifecycle and exposes MCP tools
* `Tool` - wraps manager tool forwarding in tool.Tool interface
* `PromptGenerator` - wraps prompt generator and injects MCP tool infos
* `MockManager` - reusable MCP manager mock for tests
* `NewClient()` - creates MCP client based on configured transport
* `NewStdioClient()` - creates new MCP stdio client
* `NewHTTPClient()` - creates new MCP streamable HTTP client
* `NewManager()` - creates MCP manager from configuration store
* `NewTool()` - creates MCP forwarding tool
* `NewPromptGenerator()` - creates prompt generator wrapper for MCP tools
* `NewMockManager()` - creates mock manager with initialized maps
* `RegisterTools()` - registers all manager MCP tools into tool registry
* `BuildQualifiedToolName()` - creates unique public tool name for MCP tool
* `WriteMessage()` - writes framed JSON-RPC message to writer
* `ReadMessage()` - reads framed JSON-RPC payload bytes from reader
* `LatestProtocolVersion` - MCP protocol version constant ("2025-11-25")
* `JSONRPCVersion` - JSON-RPC protocol version constant ("2.0")
* `JSONRPCRequest` - JSON-RPC request message DTO
* `JSONRPCResponse` - JSON-RPC response payload DTO
* `JSONRPCNotification` - JSON-RPC notification message DTO
* `JSONRPCError` - JSON-RPC error payload DTO
* `InitializeRequestParams` - parameters for MCP initialize request
* `InitializeResult` - MCP server initialization response DTO
* `RemoteTool` - MCP tool metadata returned by tools/list
* `RemoteResource` - MCP resource metadata returned by resources/list
* `ListToolsResult` - result payload for tools/list request
* `ListResourcesResult` - result payload for resources/list request
* `ReadResourceRequestParams` - parameters for resources/read request
* `ReadResourceResult` - result payload for resources/read request
* `CallToolRequestParams` - parameters for tools/call request
* `CallToolResult` - result payload for tools/call request
* `ResourceContent` - resources/read response content entry
* `MCPContentBlock` - tool result content block
* `MCPImplementationInfo` - identifies MCP endpoint implementation
