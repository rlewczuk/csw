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
)

func runTUI(workDir, configPath, modelName, roleName, lspServer, saveSessionTo string, saveSession, logLLMRequests bool) error {

	// Resolve working directory
	workDir, err := ResolveWorkDir(workDir)
	if err != nil {
		return err
	}

	// Initialize logging infrastructure
	// Logs directory is placed in project dir at .cswdata/logs
	// Logging is set to debug level by default
	// Logging is synchronous by default (sync=true)
	logsDir := filepath.Join(workDir, ".cswdata", "logs")
	if err := logging.SetLogsDirectory(logsDir, true); err != nil {
		return fmt.Errorf("runTUI() [tui.go]: failed to initialize logging: %w", err)
	}
	defer logging.FlushLogs()

	// Build config path hierarchy:
	// 1. @DEFAULTS (embedded)
	// 2. ./.csw/config (local project config)
	// 3. ~/.config/csw (user config)
	// 4. --config-path components (if provided)
	configPathStr, err := BuildConfigPath(configPath)
	if err != nil {
		return err
	}

	// Create composite config store
	configStore, err := impl.NewCompositeConfigStore(workDir, configPathStr)
	if err != nil {
		return fmt.Errorf("runTUI() [tui.go]: failed to create config store: %w", err)
	}

	// Create provider registry using config store
	providerRegistry := models.NewProviderRegistry(configStore)

	// Check if any providers were loaded
	if len(providerRegistry.List()) == 0 {
		return fmt.Errorf("runTUI() [tui.go]: no model providers found in config")
	}

	// Determine model to use
	modelName, err = ResolveModelName(modelName, configStore, providerRegistry)
	if err != nil {
		return err
	}

	// Create model provider map for SweSystem
	modelProviders, err := CreateProviderMap(providerRegistry)
	if err != nil {
		return err
	}

	// Create role registry using config store
	roleRegistry := core.NewAgentRoleRegistry(configStore)

	// Check if any roles were loaded
	if len(roleRegistry.List()) == 0 {
		return fmt.Errorf("runTUI() [tui.go]: no roles found in config")
	}

	// Check if the requested role exists and get its configuration
	roleConfig, ok := roleRegistry.Get(roleName)
	if !ok {
		return fmt.Errorf("runTUI() [tui.go]: role not found: %s (available: %v)", roleName, roleRegistry.List())
	}

	// Build hide patterns from role configuration and .cswignore/.gitignore
	hidePatterns, err := vfs.BuildHidePatterns(workDir, roleConfig.HiddenPatterns)
	if err != nil {
		return fmt.Errorf("runTUI() [tui.go]: failed to build hide patterns: %w", err)
	}

	// Create VFS for the working directory
	localVFS, err := vfs.NewLocalVFS(workDir, hidePatterns)
	if err != nil {
		return fmt.Errorf("runTUI() [tui.go]: failed to create VFS: %w", err)
	}

	// Create tool registry and register VFS tools
	toolRegistry := tool.NewToolRegistry()
	tool.RegisterVFSTools(toolRegistry, localVFS)

	// Create prompt generator
	promptGenerator, err := core.NewConfPromptGenerator(configStore, localVFS)
	if err != nil {
		return fmt.Errorf("runTUI() [tui.go]: failed to create prompt generator: %w", err)
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
		LogBaseDir:      logsDir,
		WorkDir:         workDir,
		LogLLMRequests:  logLLMRequests,
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
		return fmt.Errorf("runTUI() [tui.go]: failed to set app view: %w", err)
	}

	// Create a new session to start with
	if err := appPresenter.NewSession(); err != nil {
		return fmt.Errorf("runTUI() [tui.go]: failed to create initial session: %w", err)
	}

	// Note: Session saving for TUI is not yet implemented
	// The flags are accepted for compatibility but session saving only works in CLI mode
	if saveSessionTo != "" || saveSession {
		logger := logging.GetGlobalLogger()
		logger.Warn("session saving is not yet implemented for TUI mode, use 'csw cli' command for session saving")
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
			return fmt.Errorf("runTUI() [tui.go]: TUI error: %w", err)
		}
	case <-ctx.Done():
		app.Quit()
		return nil
	}

	return nil
}
