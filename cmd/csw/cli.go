package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/rlewczuk/csw/pkg/core"
	"github.com/rlewczuk/csw/pkg/logging"
	"github.com/rlewczuk/csw/pkg/presenter"
	"github.com/rlewczuk/csw/pkg/tool"
	"github.com/rlewczuk/csw/pkg/ui"
	"github.com/rlewczuk/csw/pkg/ui/cli"
	"github.com/rlewczuk/csw/pkg/ui/logmd"
	"github.com/rlewczuk/csw/pkg/vfs"
	"github.com/spf13/cobra"
)

// CliCommand creates the cli command.
func CliCommand() *cobra.Command {
	var (
		cliModel          string
		cliRole           string
		cliWorkDir        string
		cliWorktree       string
		cliAllowAllPerms  bool
		cliInteractive    bool
		cliConfigPath     string
		cliSaveSessionTo  string
		cliSaveSession    bool
		cliLogLLMRequests bool
		cliLSPServer      string
		cliThinking       string
		cliCommitMessage  string
		cliMerge          bool
	)

	cmd := &cobra.Command{
		Use:   "cli [--model <model>] [--role <role>] [--workdir <dir>] [--worktree <feature-branch-name>] [--merge] [--commit-message <template>] [--allow-all-permissions] [--interactive] [--save-session-to <file>] [--save-session] \"prompt\"",
		Short: "Start a CLI chat session with an agent",
		Long:  "Start a standard terminal session (no TUI) with a given model and role. The session can be non-interactive or lightly interactive.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			// Suppress usage for runtime errors from command execution.
			// Argument/flag parsing errors happen before RunE and still show usage.
			cmd.SilenceUsage = true

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

			return runCLI(prompt, cliModel, cliRole, cliWorkDir, cliWorktree, cliMerge, cliCommitMessage, cliConfigPath, cliAllowAllPerms, cliInteractive, cliSaveSessionTo, cliSaveSession, cliLogLLMRequests, cliLSPServer, cliThinking)
		},
	}

	// Define flags
	cmd.Flags().StringVar(&cliModel, "model", "", "Model name in provider/model format (if not set, uses default provider)")
	cmd.Flags().StringVar(&cliRole, "role", "developer", "Agent role name")
	cmd.Flags().StringVar(&cliWorkDir, "workdir", "", "Working directory (default: current directory)")
	cmd.Flags().StringVar(&cliWorktree, "worktree", "", "Create and use a git worktree for this session on a feature branch")
	cmd.Flags().BoolVar(&cliMerge, "merge", false, "Merge the feature worktree branch into main after commit")
	cmd.Flags().StringVar(&cliCommitMessage, "commit-message", "", "Custom commit message template, e.g. '[{{ .Branch }}] {{ .Message }}'")
	cmd.Flags().BoolVar(&cliAllowAllPerms, "allow-all-permissions", false, "Allow all permissions without asking")
	cmd.Flags().BoolVar(&cliInteractive, "interactive", false, "Enable interactive mode (allows user to respond to agent questions)")
	cmd.Flags().StringVar(&cliConfigPath, "config-path", "", "Colon-separated list of config directories (optional, added to default hierarchy)")
	cmd.Flags().StringVar(&cliSaveSessionTo, "save-session-to", "", "Save session conversation to specified markdown file")
	cmd.Flags().BoolVar(&cliSaveSession, "save-session", false, "Save session conversation to session.md in session log directory")
	cmd.Flags().BoolVar(&cliLogLLMRequests, "log-llm-requests", false, "Log LLM requests and responses")
	cmd.Flags().StringVar(&cliLSPServer, "lsp-server", "", "Path to LSP server binary (empty to disable LSP)")
	cmd.Flags().StringVar(&cliThinking, "thinking", "", "Thinking/reasoning mode: low, medium, high, xhigh (effort-based) or true/false (boolean)")

	return cmd
}

func runCLI(prompt, modelName, roleName, workDir, worktreeBranch string, merge bool, commitMessageTemplate, configPath string, allowAllPerms, interactive bool, saveSessionTo string, saveSession, logLLMRequests bool, lspServer, thinking string) error {
	startTime := time.Now()
	ctx := context.Background()

	if merge && worktreeBranch == "" {
		return fmt.Errorf("runCLI() [cli.go]: --merge requires --worktree")
	}

	sweSystem, buildResult, err := BuildSystem(BuildSystemParams{
		WorkDir:        workDir,
		ConfigPath:     configPath,
		ModelName:      modelName,
		RoleName:       roleName,
		WorktreeBranch: worktreeBranch,
		LSPServer:      lspServer,
		LogLLMRequests: logLLMRequests,
		Thinking:       thinking,
	})
	if err != nil {
		return err
	}
	defer logging.FlushLogs()

	workDir = buildResult.WorkDir
	modelName = buildResult.ModelName
	logsDir := buildResult.LogsDir

	// Create session thread
	thread := core.NewSessionThread(sweSystem, nil)

	// Start session
	if err := thread.StartSession(modelName); err != nil {
		return fmt.Errorf("runCLI() [cli.go]: failed to start session: %w", err)
	}

	session := thread.GetSession()

	defer finalizeWorktreeSession(ctx, buildResult.VCS, buildResult.WorktreeBranch, merge, commitMessageTemplate, sweSystem, session, os.Stderr)

	// Determine session file path if session saving is enabled
	var sessionFilePath string
	if saveSessionTo != "" {
		sessionFilePath = saveSessionTo
	} else if saveSession {
		// Get session ID and create path
		session := thread.GetSession()
		if session != nil {
			sessionID := session.ID()
			sessionLogDir := filepath.Join(logsDir, "sessions", sessionID)
			sessionFilePath = filepath.Join(sessionLogDir, "session.md")
		}
	}

	// Set role
	session = thread.GetSession()
	if session != nil {
		if err := session.SetRole(roleName); err != nil {
			return fmt.Errorf("runCLI() [cli.go]: failed to set role: %w", err)
		}
		// Set working directory
		session.SetWorkDir(workDir)

	}

	// Create chat presenter
	basePresenter := presenter.NewChatPresenter(sweSystem, thread)
	appView := cli.NewAppView(os.Stdout)
	basePresenter.SetAppView(appView)

	// Create CLI chat view
	baseCliView := cli.NewCliChatView(basePresenter, os.Stdout, os.Stdin, interactive, allowAllPerms)

	// These will be the potentially wrapped versions used for output handling
	var chatPresenter ui.IChatPresenter = basePresenter
	var cliView ui.IChatView = baseCliView

	// Wrap with session logging if enabled
	var sessionFile *os.File
	if sessionFilePath != "" {
		// Ensure directory exists
		sessionDir := filepath.Dir(sessionFilePath)
		if err := os.MkdirAll(sessionDir, 0755); err != nil {
			return fmt.Errorf("runCLI() [cli.go]: failed to create session directory: %w", err)
		}

		// Create session file
		var err error
		sessionFile, err = os.Create(sessionFilePath)
		if err != nil {
			return fmt.Errorf("runCLI() [cli.go]: failed to create session file: %w", err)
		}
		defer sessionFile.Close()

		// Create mutex for thread-safe writes
		mu := &sync.Mutex{}

		// Wrap presenter with logging wrapper
		chatPresenter = logmd.NewLogmdChatPresenter(basePresenter, sessionFile, mu)
	}

	// Set view on presenter
	if err := chatPresenter.SetView(cliView); err != nil {
		return fmt.Errorf("runCLI() [cli.go]: failed to set view: %w", err)
	}

	// If interactive, start reading input
	if interactive {
		baseCliView.StartReadingInput()
	}

	// Create a channel to track when processing is done
	done := make(chan error, 1)

	// Set up a custom output handler to track completion
	// The basePresenter implements SessionThreadOutput
	wrappedHandler := &cliOutputHandler{
		delegate: basePresenter,
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

	appView.ShowMessage(fmt.Sprintf("Session completed in %s", time.Since(startTime).Round(time.Second)), ui.MessageTypeInfo)
	return nil
}

func finalizeWorktreeSession(ctx context.Context, vcs vfs.VCS, worktreeBranch string, merge bool, commitMessageTemplate string, sweSystem *core.SweSystem, session *core.SweSession, stderr io.Writer) {
	if worktreeBranch == "" || vcs == nil {
		return
	}

	commitMessage, err := generateWorktreeCommitMessage(ctx, sweSystem, session, worktreeBranch, commitMessageTemplate)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "worktree commit message generation failed: %v\n", err)
		if dropErr := vcs.DropWorktree(worktreeBranch); dropErr != nil {
			_, _ = fmt.Fprintf(stderr, "worktree cleanup failed: %v\n", dropErr)
		}
		return
	}

	if commitErr := vcs.CommitWorktree(worktreeBranch, commitMessage); commitErr != nil && !errors.Is(commitErr, vfs.ErrNoChangesToCommit) {
		_, _ = fmt.Fprintf(stderr, "worktree commit failed: %v\n", commitErr)
		if merge {
			_, _ = fmt.Fprintln(stderr, "merge skipped because commit failed. Resolve issues and merge manually.")
			return
		}
	}

	if merge {
		mergeErr := vcs.MergeBranches("main", worktreeBranch)
		if mergeErr != nil {
			if errors.Is(mergeErr, vfs.ErrMergeConflict) {
				_, _ = fmt.Fprintf(stderr, "automatic merge failed due to conflicts: %v\n", mergeErr)
				_, _ = fmt.Fprintf(stderr, "resolve conflicts manually and merge branch '%s' into main.\n", worktreeBranch)
				_, _ = fmt.Fprintln(stderr, "worktree and feature branch were kept for manual conflict resolution.")
				return
			}

			_, _ = fmt.Fprintf(stderr, "automatic merge failed: %v\n", mergeErr)
			_, _ = fmt.Fprintln(stderr, "worktree and feature branch were kept for manual investigation.")
			return
		}
	}

	if dropErr := vcs.DropWorktree(worktreeBranch); dropErr != nil {
		_, _ = fmt.Fprintf(stderr, "worktree cleanup failed: %v\n", dropErr)
	}

	if merge {
		if deleteErr := vcs.DeleteBranch(worktreeBranch); deleteErr != nil {
			_, _ = fmt.Fprintf(stderr, "feature branch cleanup failed: %v\n", deleteErr)
		}
	}
}

// cliOutputHandler wraps a SessionThreadOutput to track when processing is done.
type cliOutputHandler struct {
	delegate core.SessionThreadOutput
	done     chan error
}

func (h *cliOutputHandler) AddAssistantMessage(text string, thinking string) {
	h.delegate.AddAssistantMessage(text, thinking)
}

func (h *cliOutputHandler) ShowMessage(message string, messageType string) {
	if h.delegate != nil {
		h.delegate.ShowMessage(message, messageType)
	}
}

func (h *cliOutputHandler) AddToolCall(call *tool.ToolCall) {
	h.delegate.AddToolCall(call)
}

func (h *cliOutputHandler) AddToolCallResult(result *tool.ToolResponse) {
	h.delegate.AddToolCallResult(result)
}

func (h *cliOutputHandler) OnPermissionQuery(query *tool.ToolPermissionsQuery) {
	h.delegate.OnPermissionQuery(query)
}

func (h *cliOutputHandler) OnRateLimitError(retryAfterSeconds int) {
	if h.delegate != nil {
		h.delegate.OnRateLimitError(retryAfterSeconds)
	}
}

func (h *cliOutputHandler) ShouldRetryAfterFailure(message string) bool {
	if h.delegate != nil {
		return h.delegate.ShouldRetryAfterFailure(message)
	}
	return false
}

func (h *cliOutputHandler) RunFinished(err error) {
	h.delegate.RunFinished(err)
	// Signal completion
	select {
	case h.done <- err:
	default:
	}
}
