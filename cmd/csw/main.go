package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/codesnort/codesnort-swe/pkg/core"
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

	// Load model providers from config
	modelsDir := filepath.Join(configDir, "models")
	providerRegistry := models.NewProviderRegistry()
	if err := providerRegistry.LoadFromDirectory(modelsDir); err != nil {
		return fmt.Errorf("failed to load model providers: %w", err)
	}

	// Check if any providers were loaded
	if len(providerRegistry.List()) == 0 {
		return fmt.Errorf("no model providers found in %s", modelsDir)
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

	// Load roles from config
	rolesDir := filepath.Join(configDir, "roles")
	roleRegistry := core.NewAgentRoleRegistry()
	if err := roleRegistry.LoadFromDirectory(rolesDir); err != nil {
		return fmt.Errorf("failed to load roles: %w", err)
	}

	// Check if any roles were loaded
	if len(roleRegistry.List()) == 0 {
		return fmt.Errorf("no roles found in %s", rolesDir)
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

	// Create SweSystem
	sweSystem := &core.SweSystem{
		ModelProviders: modelProviders,
		SystemPrompt:   "",
		Tools:          toolRegistry,
		VFS:            localVFS,
		Roles:          roleRegistry,
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

	// Create TuiAppView with the presenter
	tuiAppView, err := tui.NewTuiAppView(appPresenter)
	if err != nil {
		return fmt.Errorf("failed to create app view: %w", err)
	}

	// Set the view on the presenter
	if err := appPresenter.SetView(tuiAppView); err != nil {
		return fmt.Errorf("failed to set app view: %w", err)
	}

	// Create a new session to start with
	if err := appPresenter.NewSession(); err != nil {
		return fmt.Errorf("failed to create initial session: %w", err)
	}

	// Create the bubbletea program with the app view's model
	p := tea.NewProgram(tuiAppView.Model(), tea.WithAltScreen())

	// Run the program in a goroutine
	done := make(chan error, 1)
	go func() {
		if _, err := p.Run(); err != nil {
			done <- err
		} else {
			done <- nil
		}
	}()

	// Wait for either the program to finish or context cancellation
	select {
	case err := <-done:
		if err != nil {
			return fmt.Errorf("TUI error: %w", err)
		}
	case <-ctx.Done():
		p.Quit()
		return nil
	}

	return nil
}
