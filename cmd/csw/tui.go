package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/rlewczuk/csw/pkg/gtv"
	"github.com/rlewczuk/csw/pkg/gtv/tio"
	gtvtui "github.com/rlewczuk/csw/pkg/gtv/tui"
	"github.com/rlewczuk/csw/pkg/logging"
	"github.com/rlewczuk/csw/pkg/presenter"
	"github.com/rlewczuk/csw/pkg/ui/tui"
)

func runTUI(workDir, configPath, modelName, roleName, lspServer, saveSessionTo string, saveSession, logLLMRequests bool) error {
	sweSystem, buildResult, err := BuildSystem(BuildSystemParams{
		WorkDir:        workDir,
		ConfigPath:     configPath,
		ModelName:      modelName,
		RoleName:       roleName,
		LSPServer:      lspServer,
		LogLLMRequests: logLLMRequests,
	})
	if err != nil {
		return err
	}
	defer logging.FlushLogs()

	modelName = buildResult.ModelName

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
