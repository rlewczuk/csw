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
	"github.com/codesnort/codesnort-swe/pkg/models"
	"github.com/codesnort/codesnort-swe/pkg/presenter"
	"github.com/codesnort/codesnort-swe/pkg/tool"
	"github.com/codesnort/codesnort-swe/pkg/ui/tui"
	"github.com/codesnort/codesnort-swe/pkg/vfs"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	workDir   string
	modelName string
	configDir string
	roleName  string
)

func main() {
	rootCmd := &cobra.Command{
		Use:   "csw",
		Short: "Codesnort SWE - AI-powered software engineering assistant",
		Long:  `Codesnort SWE is an AI-powered software engineering assistant that helps you write, review, and maintain code.`,
		RunE:  run,
	}

	// Define flags
	rootCmd.Flags().StringVar(&workDir, "work-dir", "", "Working directory (default: current directory)")
	rootCmd.Flags().StringVar(&modelName, "model", "ollama/devstral-small-2:latest", "Model name in provider/model format")
	rootCmd.Flags().StringVar(&configDir, "config-dir", "", "Config directory (default: ~/.config/codesnort-swe)")
	rootCmd.Flags().StringVar(&roleName, "role", "developer", "Agent role name")

	// Bind flags to viper
	viper.BindPFlag("work-dir", rootCmd.Flags().Lookup("work-dir"))
	viper.BindPFlag("model", rootCmd.Flags().Lookup("model"))
	viper.BindPFlag("config-dir", rootCmd.Flags().Lookup("config-dir"))
	viper.BindPFlag("role", rootCmd.Flags().Lookup("role"))

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func run(cmd *cobra.Command, args []string) error {
	// Resolve working directory
	if workDir == "" {
		wd, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("failed to get current working directory: %w", err)
		}
		workDir = wd
	}

	// Resolve config directory
	if configDir == "" {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return fmt.Errorf("failed to get user home directory: %w", err)
		}
		configDir = filepath.Join(homeDir, ".config", "codesnort-swe")
	}

	// Check if config directory exists
	if _, err := os.Stat(configDir); os.IsNotExist(err) {
		return fmt.Errorf("config directory does not exist: %s", configDir)
	}

	// Create config store
	configStore, err := impl.NewLocalConfigStore(configDir)
	if err != nil {
		return fmt.Errorf("failed to create config store: %w", err)
	}

	// Create provider registry using config store
	providerRegistry := models.NewProviderRegistry(configStore)

	// Check if any providers were loaded
	if len(providerRegistry.List()) == 0 {
		return fmt.Errorf("no model providers found in config")
	}

	// Create model provider map for SweSystem
	modelProviders := make(map[string]models.ModelProvider)
	for _, name := range providerRegistry.List() {
		provider, err := providerRegistry.Get(name)
		if err != nil {
			return fmt.Errorf("failed to get provider %s: %w", name, err)
		}
		modelProviders[name] = provider
	}

	// Create role registry using config store
	roleRegistry := core.NewAgentRoleRegistry(configStore)

	// Check if any roles were loaded
	if len(roleRegistry.List()) == 0 {
		return fmt.Errorf("no roles found in config")
	}

	// Check if the requested role exists
	if _, ok := roleRegistry.Get(roleName); !ok {
		return fmt.Errorf("role not found: %s (available: %v)", roleName, roleRegistry.List())
	}

	// Create VFS for the working directory
	localVFS, err := vfs.NewLocalVFS(workDir)
	if err != nil {
		return fmt.Errorf("failed to create VFS: %w", err)
	}

	// Create tool registry and register VFS tools
	toolRegistry := tool.NewToolRegistry()
	tool.RegisterVFSTools(toolRegistry, localVFS)

	// Create prompt generator
	promptGenerator, err := core.NewConfPromptGenerator(configStore)
	if err != nil {
		return fmt.Errorf("failed to create prompt generator: %w", err)
	}

	// Create SweSystem
	sweSystem := &core.SweSystem{
		ModelProviders:  modelProviders,
		PromptGenerator: promptGenerator,
		Tools:           toolRegistry,
		VFS:             localVFS,
		Roles:           roleRegistry,
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
		return fmt.Errorf("failed to set app view: %w", err)
	}

	// Create a new session to start with
	if err := appPresenter.NewSession(); err != nil {
		return fmt.Errorf("failed to create initial session: %w", err)
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
			return fmt.Errorf("TUI error: %w", err)
		}
	case <-ctx.Done():
		app.Quit()
		return nil
	}

	return nil
}
