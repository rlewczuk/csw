package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/codesnort/codesnort-swe/pkg/conf/impl"
	"github.com/codesnort/codesnort-swe/pkg/core"
	"github.com/codesnort/codesnort-swe/pkg/gtv"
	"github.com/codesnort/codesnort-swe/pkg/gtv/tio"
	gtvtui "github.com/codesnort/codesnort-swe/pkg/gtv/tui"
	"github.com/codesnort/codesnort-swe/pkg/logging"
	"github.com/codesnort/codesnort-swe/pkg/lsp"
	"github.com/codesnort/codesnort-swe/pkg/models"
	"github.com/codesnort/codesnort-swe/pkg/presenter"
	"github.com/codesnort/codesnort-swe/pkg/tool"
	"github.com/codesnort/codesnort-swe/pkg/ui/tui"
	"github.com/codesnort/codesnort-swe/pkg/vfs"
	"github.com/spf13/cobra"
)

var (
	modelName  string
	configPath string
	roleName   string
	lspServer  string
)

func main() {
	rootCmd := &cobra.Command{
		Use:   "csw [directory]",
		Short: "Codesnort SWE - AI-powered software engineering assistant",
		Long:  `Codesnort SWE is an AI-powered software engineering assistant that helps you write, review, and maintain code.`,
		Args:  cobra.MaximumNArgs(1),
		RunE:  runTUI,
	}

	// Define flags
	rootCmd.Flags().StringVar(&modelName, "model", "", "Model name in provider/model format (if not set, uses default provider)")
	rootCmd.Flags().StringVar(&configPath, "config-path", "", "Colon-separated list of config directories (optional, added to default hierarchy)")
	rootCmd.Flags().StringVar(&roleName, "role", "developer", "Agent role name")
	rootCmd.Flags().StringVar(&lspServer, "lsp-server", "gopls", "Path to LSP server binary (empty to disable LSP)")

	// Add subcommands
	rootCmd.AddCommand(ConfCommand())
	rootCmd.AddCommand(CliCommand())

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func runTUI(cmd *cobra.Command, args []string) error {
	// Resolve working directory from argument or use current directory
	var workDir string
	if len(args) > 0 {
		// Directory provided as argument
		dirPath := args[0]
		absPath, err := filepath.Abs(dirPath)
		if err != nil {
			return fmt.Errorf("runTUI() [main.go]: failed to resolve directory path: %w", err)
		}
		info, err := os.Stat(absPath)
		if err != nil {
			return fmt.Errorf("runTUI() [main.go]: failed to access directory: %w", err)
		}
		if !info.IsDir() {
			return fmt.Errorf("runTUI() [main.go]: path is not a directory: %s", dirPath)
		}
		workDir = absPath
	} else {
		// Use current directory
		wd, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("runTUI() [main.go]: failed to get current working directory: %w", err)
		}
		workDir = wd
	}

	// Initialize logging infrastructure
	// Logs directory is placed in project dir at .cswdata/logs
	// Logging is set to debug level by default
	// Logging is synchronous by default (sync=true)
	logsDir := filepath.Join(workDir, ".cswdata", "logs")
	if err := logging.SetLogsDirectory(logsDir, true); err != nil {
		return fmt.Errorf("runTUI() [main.go]: failed to initialize logging: %w", err)
	}
	defer logging.FlushLogs()

	// Build config path hierarchy:
	// 1. @DEFAULTS (embedded)
	// 2. ./.csw/config (local project config)
	// 3. ~/.config/csw (user config)
	// 4. --config-path components (if provided)

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("runTUI() [main.go]: failed to get user home directory: %w", err)
	}

	// Start with default hierarchy
	configPathStr := "@DEFAULTS:./.csw/config:" + filepath.Join(homeDir, ".config", "csw")

	// If --config-path is provided, validate and append it
	if configPath != "" {
		// Split by colon to get individual paths
		pathComponents := filepath.SplitList(configPath)

		// Validate each path component
		for _, pathComponent := range pathComponents {
			if pathComponent == "" {
				continue
			}

			// Check if path exists
			info, err := os.Stat(pathComponent)
			if err != nil {
				if os.IsNotExist(err) {
					return fmt.Errorf("runTUI() [main.go]: config path does not exist: %s", pathComponent)
				}
				return fmt.Errorf("runTUI() [main.go]: failed to access config path %s: %w", pathComponent, err)
			}

			// Check if it's a directory
			if !info.IsDir() {
				return fmt.Errorf("runTUI() [main.go]: config path is not a directory: %s", pathComponent)
			}
		}

		// Append validated path to hierarchy
		configPathStr = configPathStr + ":" + configPath
	}

	// Create composite config store
	configStore, err := impl.NewCompositeConfigStore(workDir, configPathStr)
	if err != nil {
		return fmt.Errorf("runTUI() [main.go]: failed to create config store: %w", err)
	}

	// Create provider registry using config store
	providerRegistry := models.NewProviderRegistry(configStore)

	// Check if any providers were loaded
	if len(providerRegistry.List()) == 0 {
		return fmt.Errorf("runTUI() [main.go]: no model providers found in config")
	}

	// If no model specified, try to use default provider from global config
	if modelName == "" {
		globalConfig, err := configStore.GetGlobalConfig()
		if err != nil {
			return fmt.Errorf("runTUI() [main.go]: failed to get global config: %w", err)
		}
		if globalConfig.DefaultProvider != "" {
			// Use default provider with a default model
			modelName = globalConfig.DefaultProvider + "/default"
		} else {
			// Fallback to first available provider
			providers := providerRegistry.List()
			if len(providers) > 0 {
				modelName = providers[0] + "/default"
			} else {
				return fmt.Errorf("runTUI() [main.go]: no default provider configured and no providers available")
			}
		}
	}

	// Create model provider map for SweSystem
	modelProviders := make(map[string]models.ModelProvider)
	for _, name := range providerRegistry.List() {
		provider, err := providerRegistry.Get(name)
		if err != nil {
			return fmt.Errorf("runTUI() [main.go]: failed to get provider %s: %w", name, err)
		}
		modelProviders[name] = provider
	}

	// Create role registry using config store
	roleRegistry := core.NewAgentRoleRegistry(configStore)

	// Check if any roles were loaded
	if len(roleRegistry.List()) == 0 {
		return fmt.Errorf("runTUI() [main.go]: no roles found in config")
	}

	// Check if the requested role exists and get its configuration
	roleConfig, ok := roleRegistry.Get(roleName)
	if !ok {
		return fmt.Errorf("runTUI() [main.go]: role not found: %s (available: %v)", roleName, roleRegistry.List())
	}

	// Build hide patterns from role configuration and .cswignore/.gitignore
	hidePatterns, err := vfs.BuildHidePatterns(workDir, roleConfig.HiddenPatterns)
	if err != nil {
		return fmt.Errorf("runTUI() [main.go]: failed to build hide patterns: %w", err)
	}

	// Create VFS for the working directory
	localVFS, err := vfs.NewLocalVFS(workDir, hidePatterns)
	if err != nil {
		return fmt.Errorf("runTUI() [main.go]: failed to create VFS: %w", err)
	}

	// Create tool registry and register VFS tools
	toolRegistry := tool.NewToolRegistry()
	tool.RegisterVFSTools(toolRegistry, localVFS)

	// Create prompt generator
	promptGenerator, err := core.NewConfPromptGenerator(configStore)
	if err != nil {
		return fmt.Errorf("runTUI() [main.go]: failed to create prompt generator: %w", err)
	}

	// Create LSP client if lsp-server is specified
	var lspClient lsp.LSP
	logger := logging.GetGlobalLogger()
	if lspServer != "" {
		logger.Debug("lsp_initialization", "enabled", true, "server", lspServer)
		client, err := lsp.NewClient(lspServer, workDir)
		if err != nil {
			logger.Warn("failed to create LSP client, continuing without LSP", "error", err)
		} else {
			// Initialize LSP client asynchronously
			if err := client.Init(false); err != nil {
				logger.Warn("failed to initialize LSP client, continuing without LSP", "error", err)
			} else {
				lspClient = client
				logger.Debug("lsp_initialized", "server", lspServer)
			}
		}
	} else {
		logger.Debug("lsp_initialization", "enabled", false)
	}

	// Create SweSystem
	sweSystem := &core.SweSystem{
		ModelProviders:  modelProviders,
		PromptGenerator: promptGenerator,
		Tools:           toolRegistry,
		VFS:             localVFS,
		Roles:           roleRegistry,
		LSP:             lspClient,
	}

	// Create a context that can be cancelled on interrupt
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle interrupt signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigChan
		cancel()
	}()

	// Create AppPresenter with the system, default model, and role
	appPresenter := presenter.NewAppPresenter(sweSystem, modelName, roleName)

	// Create screen buffer (80x24 is initial size, will be resized to terminal size)
	screen := tio.NewScreenBuffer(80, 24, 0)

	// Create TAppView with the presenter
	appView := tui.NewAppView(nil, gtv.TRect{X: 0, Y: 0, W: 80, H: 24}, appPresenter)

	// Set the view on the presenter
	if err := appPresenter.SetView(appView); err != nil {
		return fmt.Errorf("runTUI() [main.go]: failed to set app view: %w", err)
	}

	// Create a new session to start with
	if err := appPresenter.NewSession(); err != nil {
		return fmt.Errorf("runTUI() [main.go]: failed to create initial session: %w", err)
	}

	// Create the gtv application
	app := gtvtui.NewApplication(appView, screen)

	// Run the application in a goroutine
	done := make(chan error, 1)
	go func() {
		if err := app.Run(os.Stdin, os.Stdout); err != nil {
			done <- err
		} else {
			done <- nil
		}
	}()

	// Wait for either the application to finish or context cancellation
	select {
	case err := <-done:
		if err != nil {
			return fmt.Errorf("runTUI() [main.go]: TUI error: %w", err)
		}
	case <-ctx.Done():
		app.Quit()
		return nil
	}

	return nil
}
