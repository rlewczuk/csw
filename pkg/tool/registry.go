package tool

import (
	"fmt"
	"log/slog"

	"github.com/codesnort/codesnort-swe/pkg/conf"
	"github.com/codesnort/codesnort-swe/pkg/lsp"
	"github.com/codesnort/codesnort-swe/pkg/runner"
	"github.com/codesnort/codesnort-swe/pkg/vfs"
)

// ToolRegistry implements a registry for tools that can be registered and retrieved by name.
// It implements the Tool interface and delegates execution to the appropriate tool.
type ToolRegistry struct {
	tools map[string]Tool
}

// LoggerSetter is implemented by tools that accept a logger for structured output.
// SetLogger assigns the logger instance used by the tool.
type LoggerSetter interface {
	// SetLogger assigns the logger instance used by the tool.
	SetLogger(logger *slog.Logger)
}

// NewToolRegistry creates a new ToolRegistry instance.
func NewToolRegistry() *ToolRegistry {
	return &ToolRegistry{
		tools: make(map[string]Tool),
	}
}

// Register registers a tool under the given name(s).
// A tool can be registered under multiple names.
func (r *ToolRegistry) Register(name string, tool Tool) {
	r.tools[name] = tool
}

// Get retrieves a tool by name.
// Returns an error if the tool is not found.
func (r *ToolRegistry) Get(name string) (Tool, error) {
	tool, exists := r.tools[name]
	if !exists {
		return nil, fmt.Errorf("tool not found: %s", name)
	}
	return tool, nil
}

// List returns a list of all registered tool names.
func (r *ToolRegistry) List() []string {
	var names []string
	for name := range r.tools {
		names = append(names, name)
	}
	return names
}

// ApplyLogger assigns a logger to all tools that support LoggerSetter.
func (r *ToolRegistry) ApplyLogger(logger *slog.Logger) {
	if logger == nil {
		return
	}

	for _, name := range r.List() {
		toolInstance, err := r.Get(name)
		if err != nil {
			continue
		}
		if setter, ok := toolInstance.(LoggerSetter); ok {
			setter.SetLogger(logger)
		}
	}
}

// Execute executes the tool with the given function name and arguments.
// It delegates execution to the appropriate tool based on the function name.
func (r *ToolRegistry) Execute(args *ToolCall) *ToolResponse {
	tool, err := r.Get(args.Function)
	if err != nil {
		return &ToolResponse{
			Call:  args,
			Error: err,
			Done:  true,
		}
	}

	return tool.Execute(args)
}

// RegisterVFSTools registers all VFS tools with the given VFS implementation.
// Line numbers are enabled by default for the vfsRead tool.
// lspClient is optional and can be nil.
// logger is optional and can be nil.
func RegisterVFSTools(registry *ToolRegistry, vfsImpl vfs.VFS, lspClient lsp.LSP, logger *slog.Logger) {
	registry.Register("vfsRead", NewVFSReadTool(vfsImpl, true))

	writeTool := NewVFSWriteTool(vfsImpl, lspClient)
	if logger != nil {
		writeTool.SetLogger(logger)
	}
	registry.Register("vfsWrite", writeTool)

	editTool := NewVFSEditTool(vfsImpl, lspClient)
	if logger != nil {
		editTool.SetLogger(logger)
	}
	registry.Register("vfsEdit", editTool)

	//registry.Register("vfsDelete", NewVFSDeleteTool(vfsImpl))
	registry.Register("vfsList", NewVFSListTool(vfsImpl))
	//registry.Register("vfsMode", NewVFSMoveTool(vfsImpl))
	registry.Register("vfsFind", NewVFSFindTool(vfsImpl))
	registry.Register("vfsGrep", NewVFSGrepTool(vfsImpl))
}

// RegisterRunBashTool registers the runBash tool with the given CommandRunner and privileges.
func RegisterRunBashTool(registry *ToolRegistry, r runner.CommandRunner, privileges map[string]conf.AccessFlag) {
	registry.Register("runBash", NewRunBashTool(r, privileges))
}

// Render returns a string representation of the tool call.
func (r *ToolRegistry) Render(call *ToolCall) (string, string, map[string]string) {
	return "ToolRegistry", "ToolRegistry", make(map[string]string)
}
