package tool

// ToolCall represents a call to a tool with specific arguments.
type ToolCall struct {

	// ID is a unique identifier for the tool call (typically UUIDv7 represented as string).
	ID string

	// Function is the name of the tool to be called.
	Function string

	// Arguments is a map of key-value pairs representing the arguments to be passed to the tool.
	Arguments map[string]string
}

type ToolResponse struct {
	// ID is the unique identifier for the tool call (typically UUIDv7 represented as string).
	ID string

	// Error is any error that occurred during the tool execution.
	Error error

	// Result is the result of the tool execution.
	Result map[string]string

	// Done indicates whether the tool execution is complete.
	Done bool
}

// SweTool represents a tool that can be executed by the agent.
// It is responsible for executing the tool and returning the response.
// It can also represent a group of tools, delegating execution to other tools.
type SweTool interface {
	// Name returns the name of the tool.
	Name() string
	// Execute executes the tool with the given arguments and returns the response.
	Execute(args ToolCall) ToolResponse
}
