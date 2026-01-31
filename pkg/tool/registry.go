package tool

import (
	"fmt"

	"github.com/codesnort/codesnort-swe/pkg/conf"
	"github.com/codesnort/codesnort-swe/pkg/runner"
	"github.com/codesnort/codesnort-swe/pkg/vfs"
)

// ToolRegistry implements a registry for tools that can be registered and retrieved by name.
// It implements the Tool interface and delegates execution to the appropriate tool.
type ToolRegistry struct {
	tools map[string]Tool
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

// Execute executes the tool with the given function name and arguments.
// It delegates execution to the appropriate tool based on the function name.
func (r *ToolRegistry) Execute(args ToolCall) ToolResponse {
	tool, err := r.Get(args.Function)
	if err != nil {
		return ToolResponse{
			Call:  &args,
			Error: err,
			Done:  true,
		}
	}

	return tool.Execute(args)
}

// RegisterVFSTools registers all VFS tools with the given VFS implementation.
// Line numbers are enabled by default for the vfs.read tool.
func RegisterVFSTools(registry *ToolRegistry, vfsImpl vfs.VFS) {
	registry.Register("vfs.read", NewVFSReadTool(vfsImpl, true))
	registry.Register("vfs.write", NewVFSWriteTool(vfsImpl, nil))
	registry.Register("vfs.edit", NewVFSEditTool(vfsImpl, nil))
	registry.Register("vfs.delete", NewVFSDeleteTool(vfsImpl))
	registry.Register("vfs.ls", NewVFSListTool(vfsImpl))
	registry.Register("vfs.move", NewVFSMoveTool(vfsImpl))
	registry.Register("vfs.find", NewVFSFindTool(vfsImpl))
	registry.Register("vfs.grep", NewVFSGrepTool(vfsImpl))
}

// RegisterRunBashTool registers the run.bash tool with the given CommandRunner and privileges.
func RegisterRunBashTool(registry *ToolRegistry, r runner.CommandRunner, privileges map[string]conf.AccessFlag) {
	registry.Register("run.bash", NewRunBashTool(r, privileges))
}
