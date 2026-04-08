# Package `pkg/mcp` Overview

Package `pkg/mcp` implements MCP clients and tool bridging in `pkg/mcp`.

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
* `NewMockManager()` - Creates initialized `MockManager`.
* `RegisterTools()` - Registers MCP tools in registry.
* `BuildQualifiedToolName()` - Builds `mcp.server.tool` names.
* `WriteMessage()` - Writes framed JSON-RPC message.
* `ReadMessage()` - Reads framed JSON-RPC body.
* `LatestProtocolVersion` - Constant: `2025-11-25`.
* `JSONRPCVersion` - Constant: `2.0`.
* `JSONRPCRequest` - JSON-RPC request DTO.
* `JSONRPCResponse` - JSON-RPC response DTO.
* `JSONRPCNotification` - JSON-RPC notification DTO.
* `JSONRPCError` - JSON-RPC error DTO.
* `InitializeRequestParams` - Initialize request parameters.
* `InitializeResult` - Initialize response payload.
* `RemoteTool` - Tool metadata from `tools/list`.
* `RemoteResource` - Resource metadata from `resources/list`.
* `ListToolsResult` - `tools/list` response payload.
* `ListResourcesResult` - `resources/list` response payload.
* `ReadResourceRequestParams` - `resources/read` request parameters.
* `ReadResourceResult` - `resources/read` response payload.
* `CallToolRequestParams` - `tools/call` request parameters.
* `CallToolResult` - `tools/call` response payload.
* `ResourceContent` - Resource content entry payload.
* `MCPContentBlock` - Tool result content block.
* `MCPImplementationInfo` - MCP implementation identity metadata.
