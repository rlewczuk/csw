package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/codesnort/codesnort-swe/pkg/conf"
	"github.com/codesnort/codesnort-swe/pkg/conf/impl"
	"github.com/codesnort/codesnort-swe/pkg/core"
	"github.com/codesnort/codesnort-swe/pkg/logging"
	"github.com/codesnort/codesnort-swe/pkg/models"
	"github.com/codesnort/codesnort-swe/pkg/presenter"
	"github.com/codesnort/codesnort-swe/pkg/tool"
	"github.com/codesnort/codesnort-swe/pkg/ui"
	"github.com/codesnort/codesnort-swe/pkg/ui/cli"
	"github.com/codesnort/codesnort-swe/pkg/vfs"
	"github.com/spf13/cobra"
)

// CliCommand creates the cli command.
func CliCommand() *cobra.Command {
	var (
		cliModel         string
		cliRole          string
		cliWorkDir       string
		cliAllowAllPerms bool
		cliInteractive   bool
		cliConfigPath    string
	)

	cmd := &cobra.Command{
		Use:   "cli [--model <model>] [--role <role>] [--workdir <dir>] [--allow-all-permissions] [--interactive] \"prompt\"",
		Short: "Start a CLI chat session with an agent",
		Long:  "Start a standard terminal session (no TUI) with a given model and role. The session can be non-interactive or lightly interactive.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			prompt := args[0]

			// Read prompt from file if it starts with @
			if strings.HasPrefix(prompt, "@") {
				promptFile := strings.TrimPrefix(prompt, "@")
				data, err := os.ReadFile(promptFile)
				if err != nil {
					return fmt.Errorf("CliCommand.RunE() [cli.go]: failed to read prompt file: %w", err)
				}
				prompt = string(data)
			} else if prompt == "-" {
				// Read prompt from stdin
				data, err := io.ReadAll(os.Stdin)
				if err != nil {
					return fmt.Errorf("CliCommand.RunE() [cli.go]: failed to read prompt from stdin: %w", err)
				}
				prompt = string(data)
			}

			// Trim whitespace from prompt
			prompt = strings.TrimSpace(prompt)
			if prompt == "" {
				return fmt.Errorf("CliCommand.RunE() [cli.go]: prompt cannot be empty")
			}

			return runCLI(prompt, cliModel, cliRole, cliWorkDir, cliConfigPath, cliAllowAllPerms, cliInteractive)
		},
	}

	// Define flags
	cmd.Flags().StringVar(&cliModel, "model", "", "Model name in provider/model format (if not set, uses default provider)")
	cmd.Flags().StringVar(&cliRole, "role", "developer", "Agent role name")
	cmd.Flags().StringVar(&cliWorkDir, "workdir", "", "Working directory (default: current directory)")
	cmd.Flags().BoolVar(&cliAllowAllPerms, "allow-all-permissions", false, "Allow all permissions without asking")
	cmd.Flags().BoolVar(&cliInteractive, "interactive", false, "Enable interactive mode (allows user to respond to agent questions)")
	cmd.Flags().StringVar(&cliConfigPath, "config-path", "", "Colon-separated list of config directories (optional, added to default hierarchy)")

	return cmd
}

func runCLI(prompt, modelName, roleName, workDir, configPath string, allowAllPerms, interactive bool) error {
	ctx := context.Background()

	// Resolve working directory
	if workDir == "" {
		wd, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("runCLI() [cli.go]: failed to get current working directory: %w", err)
		}
		workDir = wd
	} else {
		absPath, err := filepath.Abs(workDir)
		if err != nil {
			return fmt.Errorf("runCLI() [cli.go]: failed to resolve directory path: %w", err)
		}
		info, err := os.Stat(absPath)
		if err != nil {
			return fmt.Errorf("runCLI() [cli.go]: failed to access directory: %w", err)
		}
		if !info.IsDir() {
			return fmt.Errorf("runCLI() [cli.go]: path is not a directory: %s", workDir)
		}
		workDir = absPath
	}

	// Initialize logging infrastructure
	logsDir := filepath.Join(workDir, ".cswdata", "logs")
	if err := logging.SetLogsDirectory(logsDir, true); err != nil {
		return fmt.Errorf("runCLI() [cli.go]: failed to initialize logging: %w", err)
	}
	defer logging.FlushLogs()

	// Build config path hierarchy
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("runCLI() [cli.go]: failed to get user home directory: %w", err)
	}

	configPathStr := "@DEFAULTS:./.csw/config:" + filepath.Join(homeDir, ".config", "csw")

	// Validate and append custom config paths if provided
	if configPath != "" {
		pathComponents := filepath.SplitList(configPath)
		for _, pathComponent := range pathComponents {
			if pathComponent == "" {
				continue
			}
			info, err := os.Stat(pathComponent)
			if err != nil {
				if os.IsNotExist(err) {
					return fmt.Errorf("runCLI() [cli.go]: config path does not exist: %s", pathComponent)
				}
				return fmt.Errorf("runCLI() [cli.go]: failed to access config path %s: %w", pathComponent, err)
			}
			if !info.IsDir() {
				return fmt.Errorf("runCLI() [cli.go]: config path is not a directory: %s", pathComponent)
			}
		}
		configPathStr = configPathStr + ":" + configPath
	}

	// Create composite config store
	configStore, err := impl.NewCompositeConfigStore(workDir, configPathStr)
	if err != nil {
		return fmt.Errorf("runCLI() [cli.go]: failed to create config store: %w", err)
	}

	// Create provider registry
	providerRegistry := models.NewProviderRegistry(configStore)
	if len(providerRegistry.List()) == 0 {
		return fmt.Errorf("runCLI() [cli.go]: no model providers found in config")
	}

	// Determine model to use
	if modelName == "" {
		globalConfig, err := configStore.GetGlobalConfig()
		if err != nil {
			return fmt.Errorf("runCLI() [cli.go]: failed to get global config: %w", err)
		}
		if globalConfig.DefaultProvider != "" {
			modelName = globalConfig.DefaultProvider + "/default"
		} else {
			providers := providerRegistry.List()
			if len(providers) > 0 {
				modelName = providers[0] + "/default"
			} else {
				return fmt.Errorf("runCLI() [cli.go]: no default provider configured and no providers available")
			}
		}
	}

	// Create model provider map
	modelProviders := make(map[string]models.ModelProvider)
	for _, name := range providerRegistry.List() {
		provider, err := providerRegistry.Get(name)
		if err != nil {
			return fmt.Errorf("runCLI() [cli.go]: failed to get provider %s: %w", name, err)
		}
		modelProviders[name] = provider
	}

	// Create role registry
	roleRegistry := core.NewAgentRoleRegistry(configStore)
	if len(roleRegistry.List()) == 0 {
		return fmt.Errorf("runCLI() [cli.go]: no roles found in config")
	}

	// Get role configuration
	roleConfig, ok := roleRegistry.Get(roleName)
	if !ok {
		return fmt.Errorf("runCLI() [cli.go]: role not found: %s (available: %v)", roleName, roleRegistry.List())
	}

	// Build hide patterns
	hidePatterns, err := vfs.BuildHidePatterns(workDir, roleConfig.HiddenPatterns)
	if err != nil {
		return fmt.Errorf("runCLI() [cli.go]: failed to build hide patterns: %w", err)
	}

	// Create VFS
	localVFS, err := vfs.NewLocalVFS(workDir, hidePatterns)
	if err != nil {
		return fmt.Errorf("runCLI() [cli.go]: failed to create VFS: %w", err)
	}

	// Create tool registry and register VFS tools
	toolRegistry := tool.NewToolRegistry()
	tool.RegisterVFSTools(toolRegistry, localVFS)

	// Create prompt generator
	promptGenerator, err := core.NewConfPromptGenerator(configStore)
	if err != nil {
		return fmt.Errorf("runCLI() [cli.go]: failed to create prompt generator: %w", err)
	}

	// Create model tag registry and populate from config
	modelTagRegistry := models.NewModelTagRegistry()

	// Load global config model tags
	globalConfig, err := configStore.GetGlobalConfig()
	if err == nil && globalConfig != nil && len(globalConfig.ModelTags) > 0 {
		if err := modelTagRegistry.SetGlobalMappings(globalConfig.ModelTags); err != nil {
			return fmt.Errorf("runCLI() [cli.go]: failed to set global model tags: %w", err)
		}
	}

	// Load provider-specific model tags
	for _, providerName := range providerRegistry.List() {
		provider, err := providerRegistry.Get(providerName)
		if err != nil {
			continue
		}
		if chatProvider, ok := provider.(interface{ GetConfig() interface{} }); ok {
			config := chatProvider.GetConfig()
			if providerConfig, ok := config.(*conf.ModelProviderConfig); ok && len(providerConfig.ModelTags) > 0 {
				if err := modelTagRegistry.SetProviderMappings(providerName, providerConfig.ModelTags); err != nil {
					return fmt.Errorf("runCLI() [cli.go]: failed to set provider model tags: %w", err)
				}
			}
		}
	}

	// Create SweSystem
	sweSystem := &core.SweSystem{
		ModelProviders:  modelProviders,
		ModelTags:       modelTagRegistry,
		PromptGenerator: promptGenerator,
		Tools:           toolRegistry,
		VFS:             localVFS,
		Roles:           roleRegistry,
		LogBaseDir:      logsDir,
	}

	// Create session thread
	thread := core.NewSessionThread(sweSystem, nil)

	// Start session
	if err := thread.StartSession(modelName); err != nil {
		return fmt.Errorf("runCLI() [cli.go]: failed to start session: %w", err)
	}

	// Set role
	session := thread.GetSession()
	if session != nil {
		if err := session.SetRole(roleName); err != nil {
			return fmt.Errorf("runCLI() [cli.go]: failed to set role: %w", err)
		}
		// Set working directory
		session.SetWorkDir(workDir)

		// If allow-all-permissions is set, override the access controls
		if allowAllPerms {
			// Replace VFS with one that allows all operations
			allAccessVFS := vfs.NewAccessControlVFS(localVFS, map[string]conf.FileAccess{
				"*": {
					Read:   conf.AccessAllow,
					Write:  conf.AccessAllow,
					Delete: conf.AccessAllow,
					List:   conf.AccessAllow,
					Find:   conf.AccessAllow,
					Move:   conf.AccessAllow,
				},
			})
			session.VFS = allAccessVFS

			// Re-create tool registry without access control wrappers
			session.Tools = tool.NewToolRegistry()
			// Register system tools
			for _, name := range sweSystem.Tools.List() {
				t, _ := sweSystem.Tools.Get(name)
				session.Tools.Register(name, t)
			}
			// Re-register VFS tools with the all-access VFS
			session.Tools.Register("vfs.read", tool.NewVFSReadTool(allAccessVFS, true))
			session.Tools.Register("vfs.write", tool.NewVFSWriteTool(allAccessVFS))
			session.Tools.Register("vfs.edit", tool.NewVFSEditTool(allAccessVFS))
			session.Tools.Register("vfs.delete", tool.NewVFSDeleteTool(allAccessVFS))
			session.Tools.Register("vfs.ls", tool.NewVFSListTool(allAccessVFS))
			session.Tools.Register("vfs.move", tool.NewVFSMoveTool(allAccessVFS))
			session.Tools.Register("vfs.find", tool.NewVFSFindTool(allAccessVFS))
			session.Tools.Register("vfs.grep", tool.NewVFSGrepTool(allAccessVFS))
			// Re-register session-specific tools (like todo tools)
			session.Tools.Register("todo.read", tool.NewTodoReadTool(session))
			session.Tools.Register("todo.write", tool.NewTodoWriteTool(session))
		}
	}

	// Create chat presenter
	chatPresenter := presenter.NewChatPresenter(sweSystem, thread)

	// Create CLI chat view
	cliView := cli.NewCliChatView(chatPresenter, os.Stdout, os.Stdin, interactive, allowAllPerms)

	// Set view on presenter
	if err := chatPresenter.SetView(cliView); err != nil {
		return fmt.Errorf("runCLI() [cli.go]: failed to set view: %w", err)
	}

	// If interactive, start reading input
	if interactive {
		cliView.StartReadingInput()
	}

	// Create a channel to track when processing is done
	done := make(chan error, 1)

	// Set up a custom output handler to track completion
	originalHandler := chatPresenter
	wrappedHandler := &cliOutputHandler{
		delegate: originalHandler,
		done:     done,
	}
	thread.SetOutputHandler(wrappedHandler)

	// Send initial prompt
	userMsg := &ui.ChatMessageUI{
		Role: ui.ChatRoleUser,
		Text: prompt,
	}
	if err := chatPresenter.SendUserMessage(userMsg); err != nil {
		return fmt.Errorf("runCLI() [cli.go]: failed to send initial message: %w", err)
	}

	// Wait for completion or context cancellation
	select {
	case err := <-done:
		if err != nil {
			return fmt.Errorf("runCLI() [cli.go]: session error: %w", err)
		}
	case <-ctx.Done():
		return ctx.Err()
	}

	return nil
}

// cliOutputHandler wraps a SessionThreadOutput to track when processing is done.
type cliOutputHandler struct {
	delegate core.SessionThreadOutput
	done     chan error
}

func (h *cliOutputHandler) AddMarkdownChunk(markdown string) {
	h.delegate.AddMarkdownChunk(markdown)
}

func (h *cliOutputHandler) AddToolCallStart(call *tool.ToolCall) {
	h.delegate.AddToolCallStart(call)
}

func (h *cliOutputHandler) AddToolCallDetails(call *tool.ToolCall) {
	h.delegate.AddToolCallDetails(call)
}

func (h *cliOutputHandler) AddToolCallResult(result *tool.ToolResponse) {
	h.delegate.AddToolCallResult(result)
}

func (h *cliOutputHandler) OnPermissionQuery(query *tool.ToolPermissionsQuery) {
	h.delegate.OnPermissionQuery(query)
}

func (h *cliOutputHandler) RunFinished(err error) {
	h.delegate.RunFinished(err)
	// Signal completion
	select {
	case h.done <- err:
	default:
	}
}
