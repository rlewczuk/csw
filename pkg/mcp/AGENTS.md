# Package `pkg/mcp` Overview

Package `pkg/mcp` implements MCP clients and tool bridging. It provides stdio and HTTP transports, a manager for server lifecycle and tool routing, and a prompt generator wrapper that injects MCP tool metadata into prompts.

## Important files

* `mcp.go` - Stdio/HTTP clients, manager, tool bridge.
* `mcp_dto.go` - MCP and JSON-RPC DTOs.
* `mcp_mock.go` - Mock manager for MCP tests.
* `mcp_test.go` - MCP client and manager tests.

## Important public API objects

* `Client` - Stdio JSON-RPC MCP client.
* `HTTPClient` - Streamable HTTP MCP client.
* `Manager` - MCP server lifecycle and tool routing.
* `Tool` - Forwards calls through MCP manager.
* `PromptGenerator` - Injects MCP tool metadata into prompts.
* `MockManager` - In-memory MCP manager mock.
* `NewClient()` - Selects stdio or HTTP transport.
* `NewStdioClient()` - Creates stdio MCP client.
* `NewHTTPClient()` - Creates HTTP MCP client.
* `NewManager()` - Creates manager from config store.
* `NewTool()` - Creates MCP forwarding tool.
* `NewPromptGenerator()` - Wraps prompt generator for MCP.
* `NewMockManager()` - Creates initialized mock manager.
* `RegisterTools()` - Registers MCP tools in registry.
* `BuildQualifiedToolName()` - Builds qualified MCP tool names.
* `WriteMessage()` - Writes framed JSON-RPC message.
* `ReadMessage()` - Reads framed JSON-RPC body.
* `LatestProtocolVersion` - Constant: 2025-11-25.
* `JSONRPCVersion` - Constant: 2.0.
* `JSONRPCRequest` - JSON-RPC request DTO.
* `JSONRPCResponse` - JSON-RPC response DTO.
* `JSONRPCNotification` - JSON-RPC notification DTO.
* `JSONRPCError` - JSON-RPC error DTO.
* `InitializeRequestParams` - Initialize request parameters.
* `InitializeResult` - Initialize response payload.
* `RemoteTool` - Tool metadata from list response.
* `RemoteResource` - Resource metadata from list response.
* `ListToolsResult` - Tool list response payload.
* `ListResourcesResult` - Resource list response payload.
* `ReadResourceRequestParams` - Resource read request parameters.
* `ReadResourceResult` - Resource read response payload.
* `CallToolRequestParams` - Tool call request parameters.
* `CallToolResult` - Tool call response payload.
* `ResourceContent` - Resource content entry payload.
* `MCPContentBlock` - Tool result content block.
* `MCPImplementationInfo` - MCP implementation identity metadata.
