package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/rlewczuk/csw/pkg/conf/impl"
	"github.com/rlewczuk/csw/pkg/core"
	"github.com/rlewczuk/csw/pkg/logging"
	"github.com/rlewczuk/csw/pkg/models"
	"github.com/rlewczuk/csw/pkg/presenter"
	"github.com/rlewczuk/csw/pkg/tool"
	"github.com/rlewczuk/csw/pkg/ui"
	"github.com/rlewczuk/csw/pkg/ui/cli"
	"github.com/rlewczuk/csw/pkg/vfs"
	"github.com/spf13/cobra"
)

// CLIParams holds all parameters for runCLI.
type CLIParams struct {
	Prompt                string
	ModelName             string
	RoleName              string
	WorkDir               string
	WorktreeBranch        string
	Merge                 bool
	ContainerEnabled      bool
	ContainerDisabled     bool
	ContainerImage        string
	ContainerMounts       []string
	ContainerEnv          []string
	CommitMessageTemplate string
	ConfigPath            string
	ProjectConfig         string
	AllowAllPerms         bool
	Interactive           bool
	SaveSessionTo         string
	SaveSession           bool
	LogLLMRequests        bool
	LSPServer             string
	Thinking              string
	ResumeTarget          string
	ContinueSession       bool
	ForceResume           bool
	BashRunTimeout        time.Duration
}

const defaultBashRunTimeout = 120 * time.Second

var runCLIFunc = runCLI

var newCompositeConfigStoreFunc = impl.NewCompositeConfigStore
var resolveModelNameFunc = ResolveModelName
var createProviderMapFunc = CreateProviderMap
var generateWorktreeBranchNameFunc = core.GenerateWorktreeBranchName

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
		cliProjectConfig  string
		cliSaveSessionTo  string
		cliSaveSession    bool
		cliLogLLMRequests bool
		cliLSPServer      string
		cliThinking       string
		cliCommitMessage  string
		cliMerge          bool
		cliContainerImage string
		cliContainerOn    bool
		cliContainerOff   bool
		cliContainerMount []string
		cliContainerEnv   []string
		cliResume         string
		cliContinue       bool
		cliForce          bool
		cliBashRunTimeout string
	)

	cmd := &cobra.Command{
		Use:   "cli [--model <model>] [--role <role>] [--workdir <dir>] [--worktree <feature-branch-name>] [--merge] [--container-image <image>] [--container-enabled|--container-disabled] [--container-mount <host_path>:<container_path>] [--container-env <key>=<value>] [--commit-message <template>] [--allow-all-permissions] [--interactive] [--save-session-to <file>] [--save-session] [--resume <session-id|last>] [--continue] [--force] [\"prompt\"]",
		Short: "Start a CLI chat session with an agent",
		Long:  "Start a standard terminal session (no TUI) with a given model and role. The session can be non-interactive or lightly interactive.",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			// Suppress usage for runtime errors from command execution.
			// Argument/flag parsing errors happen before RunE and still show usage.
			cmd.SilenceUsage = true

			resumeTarget, err := normalizeResumeTarget(cliResume)
			if err != nil {
				return err
			}

			if cliContinue && resumeTarget == "" {
				resumeTarget = "last"
			}

			prompt := ""
			if len(args) == 1 {
				prompt = args[0]
			}

			// Read prompt from file if it starts with @
			if prompt != "" && strings.HasPrefix(prompt, "@") {
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

			if prompt != "" {
				prompt = strings.TrimSpace(prompt)
			}

			if resumeTarget == "" {
				if prompt == "" {
					return fmt.Errorf("CliCommand.RunE() [cli.go]: prompt cannot be empty")
				}
			}

			if resumeTarget != "" && !cliContinue && prompt != "" {
				return fmt.Errorf("CliCommand.RunE() [cli.go]: prompt requires --continue when --resume is set")
			}

			if cliContinue && prompt == "" {
				return fmt.Errorf("CliCommand.RunE() [cli.go]: prompt cannot be empty when --continue is set")
			}

			bashRunTimeout, err := parseBashRunTimeout(cliBashRunTimeout)
			if err != nil {
				return err
			}

			if err := applyCLIDefaults(cmd, cliWorkDir, cliProjectConfig, cliConfigPath, &cliModel, &cliWorktree, &cliMerge, &cliLogLLMRequests, &cliThinking, &cliLSPServer); err != nil {
				return err
			}

			containerEnabledChanged := cmd.Flags().Changed("container-enabled")
			containerDisabledChanged := cmd.Flags().Changed("container-disabled")
			if containerEnabledChanged && containerDisabledChanged {
				return fmt.Errorf("CliCommand.RunE() [cli.go]: --container-enabled and --container-disabled cannot be used together")
			}

			containerRequested := (containerEnabledChanged && cliContainerOn) || len(cliContainerMount) > 0 || len(cliContainerEnv) > 0
			if containerRequested && resumeTarget != "" {
				return fmt.Errorf("CliCommand.RunE() [cli.go]: container mode options are not supported with --resume")
			}

			return runCLIFunc(&CLIParams{
				Prompt:                prompt,
				ModelName:             cliModel,
				RoleName:              cliRole,
				WorkDir:               cliWorkDir,
				WorktreeBranch:        cliWorktree,
				Merge:                 cliMerge,
				ContainerEnabled:      containerRequested,
				ContainerDisabled:     containerDisabledChanged && cliContainerOff,
				ContainerImage:        cliContainerImage,
				ContainerMounts:       cliContainerMount,
				ContainerEnv:          cliContainerEnv,
				CommitMessageTemplate: cliCommitMessage,
				ConfigPath:            cliConfigPath,
				ProjectConfig:         cliProjectConfig,
				AllowAllPerms:         cliAllowAllPerms,
				Interactive:           cliInteractive,
				SaveSessionTo:         cliSaveSessionTo,
				SaveSession:           cliSaveSession,
				LogLLMRequests:        cliLogLLMRequests,
				LSPServer:             cliLSPServer,
				Thinking:              cliThinking,
				ResumeTarget:          resumeTarget,
				ContinueSession:       cliContinue,
				ForceResume:           cliForce,
				BashRunTimeout:        bashRunTimeout,
			})
		},
	}

	// Define flags
	cmd.Flags().StringVar(&cliModel, "model", "", "Model name in provider/model format (if not set, uses default provider)")
	cmd.Flags().StringVar(&cliRole, "role", "developer", "Agent role name")
	cmd.Flags().StringVar(&cliWorkDir, "workdir", "", "Working directory (default: current directory)")
	cmd.Flags().StringVar(&cliWorktree, "worktree", "", "Create and use a git worktree for this session on a feature branch")
	cmd.Flags().BoolVar(&cliMerge, "merge", false, "Merge the feature worktree branch into main after commit")
	cmd.Flags().StringVar(&cliContainerImage, "container-image", "", "Container image for running bash commands in container mode")
	cmd.Flags().BoolVar(&cliContainerOn, "container-enabled", false, "Enable running bash commands in container mode")
	cmd.Flags().BoolVar(&cliContainerOff, "container-disabled", false, "Disable running bash commands in container mode")
	cmd.Flags().StringArrayVar(&cliContainerMount, "container-mount", nil, "Additional container mount in host_path:container_path format (repeatable)")
	cmd.Flags().StringArrayVar(&cliContainerEnv, "container-env", nil, "Additional container env var in KEY=VALUE format (repeatable)")
	cmd.Flags().StringVar(&cliCommitMessage, "commit-message", "", "Custom commit message template, e.g. '[{{ .Branch }}] {{ .Message }}'")
	cmd.Flags().BoolVar(&cliAllowAllPerms, "allow-all-permissions", false, "Allow all permissions without asking")
	cmd.Flags().BoolVar(&cliInteractive, "interactive", false, "Enable interactive mode (allows user to respond to agent questions)")
	cmd.Flags().StringVar(&cliConfigPath, "config-path", "", "Colon-separated list of config directories (optional, added to default hierarchy)")
	cmd.Flags().StringVar(&cliProjectConfig, "project-config", "", "Custom project config directory (default: .csw/config)")
	cmd.Flags().StringVar(&cliSaveSessionTo, "save-session-to", "", "Save session conversation to specified markdown file")
	cmd.Flags().BoolVar(&cliSaveSession, "save-session", false, "Save session conversation")
	cmd.Flags().BoolVar(&cliLogLLMRequests, "log-llm-requests", false, "Log LLM requests and responses")
	cmd.Flags().StringVar(&cliLSPServer, "lsp-server", "", "Path to LSP server binary (empty to disable LSP)")
	cmd.Flags().StringVar(&cliThinking, "thinking", "", "Thinking/reasoning mode: low, medium, high, xhigh (effort-based) or true/false (boolean)")
	cmd.Flags().StringVar(&cliResume, "resume", "", "Resume session by id (UUID) or 'last'. If value is omitted, resumes last session")
	cmd.Flags().BoolVar(&cliContinue, "continue", false, "Continue resumed session with a new user message")
	cmd.Flags().BoolVar(&cliForce, "force", false, "Force resume even when there is no pending work")
	cmd.Flags().StringVar(&cliBashRunTimeout, "bash-run-timeout", "120", "Default runBash command timeout (duration; plain number means seconds)")
	resumeFlag := cmd.Flags().Lookup("resume")
	if resumeFlag != nil {
		resumeFlag.NoOptDefVal = "last"
	}
	return cmd
}

func applyCLIDefaults(
	cmd *cobra.Command,
	workDir string,
	projectConfig string,
	configPath string,
	model *string,
	worktree *string,
	merge *bool,
	logLLMRequests *bool,
	thinking *string,
	lspServer *string,
) error {
	resolvedWorkDir, err := ResolveWorkDir(workDir)
	if err != nil {
		return fmt.Errorf("applyCLIDefaults() [cli.go]: failed to resolve work directory: %w", err)
	}

	configPathStr, err := BuildConfigPath(projectConfig, configPath)
	if err != nil {
		return fmt.Errorf("applyCLIDefaults() [cli.go]: failed to build config path: %w", err)
	}

	configStore, err := newCompositeConfigStoreFunc(resolvedWorkDir, configPathStr)
	if err != nil {
		return fmt.Errorf("applyCLIDefaults() [cli.go]: failed to create config store: %w", err)
	}

	globalConfig, err := configStore.GetGlobalConfig()
	if err != nil {
		return fmt.Errorf("applyCLIDefaults() [cli.go]: failed to load global config: %w", err)
	}

	if !cmd.Flags().Changed("model") && globalConfig.Defaults.Model != "" {
		*model = globalConfig.Defaults.Model
	}
	if !cmd.Flags().Changed("worktree") && globalConfig.Defaults.Worktree != "" {
		*worktree = globalConfig.Defaults.Worktree
	}
	if !cmd.Flags().Changed("merge") && globalConfig.Defaults.Merge {
		*merge = true
	}
	if !cmd.Flags().Changed("log-llm-requests") && globalConfig.Defaults.LogLLMRequests {
		*logLLMRequests = true
	}
	if !cmd.Flags().Changed("thinking") && globalConfig.Defaults.Thinking != "" {
		*thinking = globalConfig.Defaults.Thinking
	}
	if !cmd.Flags().Changed("lsp-server") && globalConfig.Defaults.LSPServer != "" {
		*lspServer = globalConfig.Defaults.LSPServer
	}

	return nil
}

func runCLI(params *CLIParams) error {
	startTime := time.Now()
	ctx := context.Background()
	appView := cli.NewAppView(os.Stdout)
	if params.BashRunTimeout <= 0 {
		params.BashRunTimeout = defaultBashRunTimeout
	}

	if params.Merge && params.WorktreeBranch == "" {
		return fmt.Errorf("runCLI() [cli.go]: --merge requires --worktree")
	}

	if (params.ContainerEnabled || len(params.ContainerMounts) > 0 || len(params.ContainerEnv) > 0) && params.ResumeTarget != "" {
		return fmt.Errorf("runCLI() [cli.go]: container mode options are not supported with --resume")
	}

	resolvedWorktreeBranch, err := resolveWorktreeBranchName(ctx, params.Prompt, params.ModelName, params.WorkDir, params.ProjectConfig, params.ConfigPath, params.WorktreeBranch)
	if err != nil {
		return fmt.Errorf("runCLI() [cli.go]: failed to resolve worktree branch: %w", err)
	}
	params.WorktreeBranch = resolvedWorktreeBranch
	if params.WorktreeBranch != "" {
		appView.ShowMessage(fmt.Sprintf("Worktree branch: %s", params.WorktreeBranch), ui.MessageTypeInfo)
	}

	sweSystem, buildResult, err := BuildSystem(BuildSystemParams{
		WorkDir:           params.WorkDir,
		ConfigPath:        params.ConfigPath,
		ProjectConfig:     params.ProjectConfig,
		ModelName:         params.ModelName,
		RoleName:          params.RoleName,
		WorktreeBranch:    params.WorktreeBranch,
		ContainerEnabled:  params.ContainerEnabled,
		ContainerDisabled: params.ContainerDisabled,
		ContainerImage:    params.ContainerImage,
		ContainerMounts:   params.ContainerMounts,
		ContainerEnv:      params.ContainerEnv,
		LSPServer:         params.LSPServer,
		LogLLMRequests:    params.LogLLMRequests,
		Thinking:          params.Thinking,
		BashRunTimeout:    params.BashRunTimeout,
	})
	if err != nil {
		return err
	}
	defer buildResult.Cleanup()
	defer logging.FlushLogs()

	params.WorkDir = buildResult.WorkDir
	params.ModelName = buildResult.ModelName
	if params.LSPServer != "" {
		lspStatus := "disabled"
		if buildResult.LSPStarted {
			lspStatus = "started"
		}
		appView.ShowMessage(fmt.Sprintf("LSP %s (workdir: %s)", lspStatus, buildResult.LSPWorkDir), ui.MessageTypeInfo)
	}

	var (
		thread  *core.SessionThread
		session *core.SweSession
	)

	if params.ResumeTarget != "" {
		if params.ResumeTarget == "last" {
			session, err = sweSystem.LoadLastSession(nil)
			if err != nil {
				return fmt.Errorf("runCLI() [cli.go]: failed to load last session: %w", err)
			}
		} else {
			session, err = sweSystem.LoadSession(params.ResumeTarget, nil)
			if err != nil {
				return fmt.Errorf("runCLI() [cli.go]: failed to load session: %w", err)
			}
		}
		thread = core.NewSessionThreadWithSession(sweSystem, session, nil)
	} else {
		thread = core.NewSessionThread(sweSystem, nil)
		if err := thread.StartSession(params.ModelName); err != nil {
			return fmt.Errorf("runCLI() [cli.go]: failed to start session: %w", err)
		}
		session = thread.GetSession()
	}

	if session == nil {
		return fmt.Errorf("runCLI() [cli.go]: session is not available")
	}
	appView.SetSessionLogger(logging.GetSessionLogger(session.ID(), logging.LogTypeSession))

	sessionID := session.ID()
	defer func() {
		_, _ = fmt.Fprintf(os.Stdout, "Session ID: %s\n", sessionID)
	}()

	defer finalizeWorktreeSession(ctx, buildResult.VCS, buildResult.WorktreeBranch, params.Merge, params.CommitMessageTemplate, sweSystem, session, os.Stderr)

	// Set role
	if params.ResumeTarget == "" {
		if err := session.SetRole(params.RoleName); err != nil {
			return fmt.Errorf("runCLI() [cli.go]: failed to set role: %w", err)
		}
		// Set working directory
		session.SetWorkDir(params.WorkDir)
	}

	// Create chat presenter
	basePresenter := presenter.NewChatPresenter(sweSystem, thread)
	basePresenter.SetAppView(appView)

	// Create CLI chat view
	baseCliView := cli.NewCliChatView(basePresenter, os.Stdout, os.Stdin, params.Interactive, params.AllowAllPerms)

	// Set view on presenter
	if err := basePresenter.SetView(baseCliView); err != nil {
		return fmt.Errorf("runCLI() [cli.go]: failed to set view: %w", err)
	}

	// If interactive, start reading input
	if params.Interactive {
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

	if params.ResumeTarget != "" {
		if params.ContinueSession {
			userMsg := &ui.ChatMessageUI{
				Role: ui.ChatRoleUser,
				Text: params.Prompt,
			}
			if err := basePresenter.SendUserMessage(userMsg); err != nil {
				return fmt.Errorf("runCLI() [cli.go]: failed to send continue message: %w", err)
			}
		} else {
			if !params.ForceResume && !session.HasPendingWork() {
				return fmt.Errorf("runCLI() [cli.go]: resumed session has no pending work (use --continue to add a prompt or --force to run anyway)")
			}
			if err := thread.ResumePending(); err != nil {
				return fmt.Errorf("runCLI() [cli.go]: failed to resume pending work: %w", err)
			}
		}
	} else {
		userMsg := &ui.ChatMessageUI{
			Role: ui.ChatRoleUser,
			Text: params.Prompt,
		}
		if err := basePresenter.SendUserMessage(userMsg); err != nil {
			return fmt.Errorf("runCLI() [cli.go]: failed to send initial message: %w", err)
		}
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

	appView.ShowMessage(buildSessionSummaryMessage(time.Since(startTime), session), ui.MessageTypeInfo)
	return nil
}

func parseBashRunTimeout(value string) (time.Duration, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return defaultBashRunTimeout, nil
	}

	if _, err := strconv.ParseInt(trimmed, 10, 64); err == nil {
		trimmed += "s"
	}

	parsed, err := time.ParseDuration(trimmed)
	if err != nil {
		return 0, fmt.Errorf("parseBashRunTimeout() [cli.go]: invalid --bash-run-timeout value %q: %w", value, err)
	}

	if parsed <= 0 {
		return 0, fmt.Errorf("parseBashRunTimeout() [cli.go]: --bash-run-timeout must be positive, got %q", value)
	}

	return parsed, nil
}

func buildSessionSummaryMessage(duration time.Duration, session *core.SweSession) string {
	base := fmt.Sprintf("Session completed in %s", duration.Round(time.Second))
	if session == nil {
		return base
	}

	usage := session.TokenUsage()
	return fmt.Sprintf(
		"%s | tokens(input=%d[cached=%d,noncached=%d], output=%d, total=%d) | context=%d",
		base,
		usage.InputTokens,
		usage.InputCachedTokens,
		usage.InputNonCachedTokens,
		usage.OutputTokens,
		usage.TotalTokens,
		session.ContextLengthTokens(),
	)
}

func normalizeResumeTarget(raw string) (string, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return "", nil
	}
	value = strings.ToLower(value)

	if value == "last" {
		return value, nil
	}

	uuidPattern := regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)
	if !uuidPattern.MatchString(value) {
		return "", fmt.Errorf("normalizeResumeTarget() [cli.go]: invalid --resume value: %q (expected UUID or 'last')", raw)
	}

	return value, nil
}

func resolveWorktreeBranchName(ctx context.Context, prompt, modelName, workDir, projectConfig, configPath, worktreeBranch string) (string, error) {
	if worktreeBranch == "" || !strings.HasSuffix(worktreeBranch, "%") {
		return worktreeBranch, nil
	}

	if strings.TrimSpace(prompt) == "" {
		return "", fmt.Errorf("resolveWorktreeBranchName() [cli.go]: --worktree ending with %% requires non-empty prompt")
	}

	prefix := strings.TrimSuffix(worktreeBranch, "%")
	resolvedWorkDir, err := ResolveWorkDir(workDir)
	if err != nil {
		return "", fmt.Errorf("resolveWorktreeBranchName() [cli.go]: failed to resolve work directory: %w", err)
	}

	configPathStr, err := BuildConfigPath(projectConfig, configPath)
	if err != nil {
		return "", fmt.Errorf("resolveWorktreeBranchName() [cli.go]: failed to build config path: %w", err)
	}

	configStore, err := newCompositeConfigStoreFunc(resolvedWorkDir, configPathStr)
	if err != nil {
		return "", fmt.Errorf("resolveWorktreeBranchName() [cli.go]: failed to create config store: %w", err)
	}

	providerRegistry := models.NewProviderRegistry(configStore)
	if len(providerRegistry.List()) == 0 {
		return "", fmt.Errorf("resolveWorktreeBranchName() [cli.go]: no model providers found in config")
	}

	resolvedModelName, err := resolveModelNameFunc(modelName, configStore, providerRegistry)
	if err != nil {
		return "", fmt.Errorf("resolveWorktreeBranchName() [cli.go]: failed to resolve model name: %w", err)
	}

	modelProviders, err := createProviderMapFunc(providerRegistry)
	if err != nil {
		return "", fmt.Errorf("resolveWorktreeBranchName() [cli.go]: failed to create provider map: %w", err)
	}

	sweSystem := &core.SweSystem{
		ModelProviders: modelProviders,
		ConfigStore:    configStore,
	}

	branchSuffix, err := generateWorktreeBranchNameFunc(ctx, sweSystem, resolvedModelName, prompt)
	if err != nil {
		return "", fmt.Errorf("resolveWorktreeBranchName() [cli.go]: failed to generate branch name: %w", err)
	}

	return prefix + branchSuffix, nil
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
