package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"text/template"
	"time"

	"github.com/rlewczuk/csw/pkg/conf"
	"github.com/rlewczuk/csw/pkg/core"
	"github.com/rlewczuk/csw/pkg/logging"
	"github.com/rlewczuk/csw/pkg/models"
	"github.com/rlewczuk/csw/pkg/presenter"
	"github.com/rlewczuk/csw/pkg/system"
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
	ShadowDir             string
	WorktreeBranch        string
	ContinueWorktree      bool
	GitUserName           string
	GitUserEmail          string
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
	MaxThreads            int
	Verbose               bool
	VFSAllow              []string
	HookOverrides         []string
}

const defaultBashRunTimeout = 120 * time.Second

var runCLIFunc = runCLI
var saveSessionSummaryMarkdownFunc = saveSessionSummaryMarkdown
var saveSessionSummaryJSONFunc = saveSessionSummaryJSON
var resolveCLIDefaultsFunc = system.ResolveCLIDefaults
var resolveWorktreeBranchNameFunc = system.ResolveWorktreeBranchName
var buildSystemFunc = system.BuildSystem
var gitLookPathFunc = exec.LookPath
var gitConfigValueFunc = system.ReadGitConfigValue
var runGitCommandFunc = runGitCommand
var listGitConflictFilesFunc = listGitConflictFiles
var executeConflictSubAgentFunc = executeConflictSubAgentTask

// CliCommand creates the cli command.
func CliCommand() *cobra.Command {
	var (
		cliModel          string
		cliRole           string
		cliWorkDir        string
		cliWorktree       string
		cliShadowDir      string
		cliAllowAllPerms  bool
		cliInteractive    bool
		cliConfigPath     string
		cliProjectConfig  string
		cliSaveSessionTo  string
		cliSaveSession    bool
		cliLogLLMRequests bool
		cliLSPServer      string
		cliThinking       string
		cliGitUser        string
		cliGitEmail       string
		cliCommitMessage  string
		cliMerge          bool
		cliContainerImage string
		cliContainerOn    bool
		cliContainerOff   bool
		cliContainerMount []string
		cliContainerEnv   []string
		cliResume         string
		cliContinue       string
		cliResumeContinue bool
		cliForce          bool
		cliBashRunTimeout string
		cliMaxThreads     int
		cliVerbose        bool
		cliVFSAllow       []string
		cliHooks          []string
	)

	cmd := &cobra.Command{
		Use:   "cli [--model <model>] [--role <role>] [--workdir <dir>] [--shadow-dir <path>] [--worktree <feature-branch-name>] [--continue <feature-branch-name>] [--merge] [--container-image <image>] [--container-enabled|--container-disabled] [--container-mount <host_path>:<container_path>] [--container-env <key>=<value>] [--commit-message <template>] [--allow-all-permissions] [--interactive] [--save-session-to <file>] [--save-session] [--resume <session-id|last>] [--resume-continue] [--force] [\"prompt\"]",
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

			continueWorktreeBranch := strings.TrimSpace(cliContinue)
			if continueWorktreeBranch != "" && cliResumeContinue {
				return fmt.Errorf("CliCommand.RunE() [cli.go]: --continue <branch> cannot be used with --resume-continue")
			}

			if cliResumeContinue && resumeTarget == "" {
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

			if continueWorktreeBranch != "" && cmd.Flags().Changed("worktree") {
				return fmt.Errorf("CliCommand.RunE() [cli.go]: --continue and --worktree cannot be used together")
			}

			if continueWorktreeBranch != "" && resumeTarget != "" {
				return fmt.Errorf("CliCommand.RunE() [cli.go]: --continue <branch> cannot be used with --resume")
			}

			if resumeTarget != "" && !cliResumeContinue && prompt != "" {
				return fmt.Errorf("CliCommand.RunE() [cli.go]: prompt requires --resume-continue when --resume is set")
			}

			if cliResumeContinue && prompt == "" {
				return fmt.Errorf("CliCommand.RunE() [cli.go]: prompt cannot be empty when --resume-continue is set")
			}

			bashRunTimeout, err := parseBashRunTimeout(cliBashRunTimeout)
			if err != nil {
				return err
			}

			if err := applyCLIDefaults(cmd, cliWorkDir, cliShadowDir, cliProjectConfig, cliConfigPath, &cliModel, &cliWorktree, &cliMerge, &cliLogLLMRequests, &cliThinking, &cliLSPServer, &cliGitUser, &cliGitEmail, &cliMaxThreads); err != nil {
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

			// Parse vfs-allow paths, handling both repeated flags and colon-separated values
			vfsAllowPaths := parseVFSAllowPaths(cliVFSAllow)

			return runCLIFunc(&CLIParams{
				Prompt:                prompt,
				ModelName:             cliModel,
				RoleName:              cliRole,
				WorkDir:               cliWorkDir,
				ShadowDir:             cliShadowDir,
				WorktreeBranch:        firstNonEmpty(continueWorktreeBranch, cliWorktree),
				ContinueWorktree:      continueWorktreeBranch != "",
				GitUserName:           resolveGitIdentity(cliGitUser, "user.name"),
				GitUserEmail:          resolveGitIdentity(cliGitEmail, "user.email"),
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
				ContinueSession:       cliResumeContinue,
				ForceResume:           cliForce,
				BashRunTimeout:        bashRunTimeout,
				MaxThreads:            cliMaxThreads,
				Verbose:               cliVerbose,
				VFSAllow:              vfsAllowPaths,
				HookOverrides:         cliHooks,
			})
		},
	}

	// Define flags
	cmd.Flags().StringVar(&cliModel, "model", "", "Model name in provider/model format (if not set, uses default provider)")
	cmd.Flags().StringVar(&cliRole, "role", "developer", "Agent role name")
	cmd.Flags().StringVar(&cliWorkDir, "workdir", "", "Working directory (default: current directory)")
	cmd.Flags().StringVar(&cliShadowDir, "shadow-dir", "", "Shadow directory for agent files overlay (AGENTS.md, .agents*, .csw*, .cswdata)")
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
	cmd.Flags().StringVar(&cliGitUser, "git-user", "", "Git user name for git operations (default: from git config)")
	cmd.Flags().StringVar(&cliGitEmail, "git-email", "", "Git user email for git operations (default: from git config)")
	cmd.Flags().StringVar(&cliResume, "resume", "", "Resume session by id (UUID) or 'last'. If value is omitted, resumes last session")
	cmd.Flags().StringVar(&cliContinue, "continue", "", "Continue work in an existing git worktree branch")
	cmd.Flags().BoolVar(&cliResumeContinue, "resume-continue", false, "Continue resumed session with a new user message")
	cmd.Flags().BoolVar(&cliForce, "force", false, "Force resume even when there is no pending work")
	cmd.Flags().StringVar(&cliBashRunTimeout, "bash-run-timeout", "120", "Default runBash command timeout (duration; plain number means seconds)")
	cmd.Flags().IntVar(&cliMaxThreads, "max-threads", 0, "Maximum number of tool calls executed in parallel")
	cmd.Flags().BoolVar(&cliVerbose, "verbose", false, "Display full tool output instead of one-liners")
	cmd.Flags().StringArrayVar(&cliVFSAllow, "vfs-allow", nil, "Additional path to allow VFS access outside of worktree (repeatable, or use ':' separated list)")
	cmd.Flags().StringArrayVar(&cliHooks, "hook", nil, "Ephemeral hook override: --hook name | --hook name:disable | --hook name:key=value,key2=value2")
	resumeFlag := cmd.Flags().Lookup("resume")
	if resumeFlag != nil {
		resumeFlag.NoOptDefVal = "last"
	}
	return cmd
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}

	return ""
}

func applyCLIDefaults(
	cmd *cobra.Command,
	workDir string,
	shadowDir string,
	projectConfig string,
	configPath string,
	model *string,
	worktree *string,
	merge *bool,
	logLLMRequests *bool,
	thinking *string,
	lspServer *string,
	gitUser *string,
	gitEmail *string,
	maxThreads *int,
) error {
	defaults, err := resolveCLIDefaultsFunc(system.ResolveCLIDefaultsParams{
		WorkDir:       workDir,
		ShadowDir:     shadowDir,
		ProjectConfig: projectConfig,
		ConfigPath:    configPath,
	})
	if err != nil {
		return fmt.Errorf("applyCLIDefaults() [cli.go]: failed to resolve CLI defaults: %w", err)
	}

	if !cmd.Flags().Changed("model") && defaults.Model != "" {
		*model = defaults.Model
	}
	if !cmd.Flags().Changed("worktree") && defaults.Worktree != "" {
		*worktree = defaults.Worktree
	}
	if !cmd.Flags().Changed("merge") && defaults.Merge {
		*merge = true
	}
	if !cmd.Flags().Changed("log-llm-requests") && defaults.LogLLMRequests {
		*logLLMRequests = true
	}
	if !cmd.Flags().Changed("thinking") && defaults.Thinking != "" {
		*thinking = defaults.Thinking
	}
	if !cmd.Flags().Changed("lsp-server") && defaults.LSPServer != "" {
		*lspServer = defaults.LSPServer
	}
	if !cmd.Flags().Changed("git-user") && defaults.GitUserName != "" {
		*gitUser = defaults.GitUserName
	}
	if !cmd.Flags().Changed("git-email") && defaults.GitUserEmail != "" {
		*gitEmail = defaults.GitUserEmail
	}
	if !cmd.Flags().Changed("max-threads") && defaults.MaxThreads > 0 {
		*maxThreads = defaults.MaxThreads
	}

	if *maxThreads < 0 {
		return fmt.Errorf("applyCLIDefaults() [cli.go]: --max-threads must be >= 0")
	}

	return nil
}

func runCLI(params *CLIParams) error {
	startTime := time.Now()
	ctx := context.Background()
	if params.BashRunTimeout <= 0 {
		params.BashRunTimeout = defaultBashRunTimeout
	}

	if params.Merge && params.WorktreeBranch == "" {
		return fmt.Errorf("runCLI() [cli.go]: --merge requires --worktree")
	}

	if (params.ContainerEnabled || len(params.ContainerMounts) > 0 || len(params.ContainerEnv) > 0) && params.ResumeTarget != "" {
		return fmt.Errorf("runCLI() [cli.go]: container mode options are not supported with --resume")
	}

	resolvedWorktreeBranch, err := resolveWorktreeBranchNameFunc(ctx, system.ResolveWorktreeBranchNameParams{
		Prompt:         params.Prompt,
		ModelName:      params.ModelName,
		WorkDir:        params.WorkDir,
		ShadowDir:      params.ShadowDir,
		ProjectConfig:  params.ProjectConfig,
		ConfigPath:     params.ConfigPath,
		WorktreeBranch: params.WorktreeBranch,
	})
	if err != nil {
		return fmt.Errorf("runCLI() [cli.go]: failed to resolve worktree branch: %w", err)
	}
	params.WorktreeBranch = resolvedWorktreeBranch
	if params.WorktreeBranch != "" {
		_, _ = fmt.Fprintf(os.Stdout, "[INFO] Worktree branch: %s\n", params.WorktreeBranch)
	}

	sweSystem, buildResult, err := buildSystemFunc(system.BuildSystemParams{
		WorkDir:           params.WorkDir,
		ShadowDir:         params.ShadowDir,
		ConfigPath:        params.ConfigPath,
		ProjectConfig:     params.ProjectConfig,
		ModelName:         params.ModelName,
		RoleName:          params.RoleName,
		WorktreeBranch:    params.WorktreeBranch,
		ContinueWorktree:  params.ContinueWorktree,
		GitUserName:       params.GitUserName,
		GitUserEmail:      params.GitUserEmail,
		ContainerEnabled:  params.ContainerEnabled,
		ContainerDisabled: params.ContainerDisabled,
		ContainerImage:    params.ContainerImage,
		ContainerMounts:   params.ContainerMounts,
		ContainerEnv:      params.ContainerEnv,
		LSPServer:         params.LSPServer,
		LogLLMRequests:    params.LogLLMRequests,
		Thinking:          params.Thinking,
		BashRunTimeout:    params.BashRunTimeout,
		AllowedPaths:      params.VFSAllow,
		MaxToolThreads:    params.MaxThreads,
	})
	if err != nil {
		return err
	}
	defer buildResult.Cleanup()
	defer logging.FlushLogs()

	params.WorkDir = buildResult.WorkDir
	params.ShadowDir = buildResult.ShadowDir
	params.ModelName = buildResult.ModelName
	cliSlug := strings.TrimSpace(buildResult.WorktreeBranch)
	if cliSlug == "" {
		cliSlug = "main"
	}
	if params.LSPServer != "" {
		lspStatus := "disabled"
		if buildResult.LSPStarted {
			lspStatus = "started"
		}
		_, _ = fmt.Fprintf(os.Stdout, "[INFO] LSP %s (workdir: %s)\n", lspStatus, buildResult.LSPWorkDir)
	}
	if strings.TrimSpace(buildResult.ContainerImage) != "" {
		_, _ = fmt.Fprintln(os.Stdout, buildContainerStartupInfoMessage(buildResult))
	}

	runtimeResult, err := sweSystem.StartCLISession(system.StartCLISessionParams{
		ModelName:       params.ModelName,
		RoleName:        params.RoleName,
		Prompt:          params.Prompt,
		ResumeTarget:    params.ResumeTarget,
		ContinueSession: params.ContinueSession,
		ForceResume:     params.ForceResume,
		Interactive:     params.Interactive,
		AllowAllPerms:   params.AllowAllPerms,
		Verbose:         params.Verbose,
		AppOutput:       os.Stdout,
		ChatOutput:      os.Stdout,
		ChatInput:       os.Stdin,
		AppViewFactory: func(output io.Writer) system.SessionLoggerAppView {
			return cli.NewAppView(output, cliSlug)
		},
		ChatPresenterFactory: func(factory core.SessionFactory, thread *core.SessionThread) system.ChatPresenter {
			return presenter.NewChatPresenter(factory, thread)
		},
		ChatViewFactory: func(chatPresenter ui.IChatPresenter, output io.Writer, input io.Reader, interactive bool, allowAllPerms bool, verbose bool) system.ChatView {
			return cli.NewCliChatView(chatPresenter, output, input, cliSlug, interactive, allowAllPerms, verbose)
		},
	})
	if err != nil {
		return fmt.Errorf("runCLI() [cli.go]: failed to start CLI session runtime: %w", err)
	}
	appView := runtimeResult.AppView
	session := runtimeResult.Session

	sessionID := session.ID()
	defer func() {
		_, _ = fmt.Fprintf(os.Stdout, "Session ID: %s\n", sessionID)
	}()

	baseCommitID := resolveGitCommitID(chooseGitDiffDir(buildResult.WorkDirRoot, buildResult.WorkDir), "HEAD")

	var sessionRunErr error

	// Wait for completion or context cancellation
	select {
	case err := <-runtimeResult.Done:
		if err != nil {
			sessionRunErr = fmt.Errorf("runCLI() [cli.go]: session error: %w", err)
		}
	case <-ctx.Done():
		sessionRunErr = ctx.Err()
	}

	var finalizeResult worktreeFinalizeResult
	hookConfigStore, err := buildRuntimeHookConfigStore(sweSystem.ConfigStore, params.HookOverrides)
	if err != nil {
		return err
	}

	hookEngine := core.NewHookEngine(
		hookConfigStore,
		core.NewDefaultHookRunner(chooseGitDiffDir(buildResult.WorkDirRoot, buildResult.WorkDir)),
		buildResult.ShellRunner,
		sweSystem.ModelProviders,
	)
	hookEngine.MergeContext(map[string]string{
		"branch":  strings.TrimSpace(buildResult.WorktreeBranch),
		"workdir": strings.TrimSpace(buildResult.WorkDir),
		"rootdir": strings.TrimSpace(buildResult.WorkDirRoot),
		"status":  string(core.HookSessionStatusRunning),
		"user_prompt": strings.TrimSpace(params.Prompt),
	})
	finalizeResult, finalizeErr := finalizeWorktreeSession(ctx, buildResult.VCS, buildResult.WorktreeBranch, params.Merge, params.CommitMessageTemplate, sweSystem, session, os.Stderr, buildResult.WorkDirRoot, buildResult.WorkDir, params.Prompt, hookEngine, appView)
	if finalizeErr != nil {
		sessionRunErr = finalizeErr
	}
	endTime := time.Now()
	hookEngine.SetContextValue("summary", strings.TrimSpace(lastAssistantMessageText(session)))
	if sessionRunErr != nil {
		hookEngine.SetSessionStatus(core.HookSessionStatusFailed)
	} else {
		hookEngine.SetSessionStatus(core.HookSessionStatusSuccess)
	}

	if err := emitSessionSummary(startTime, endTime, session, buildResult, appView, sessionRunErr, baseCommitID, finalizeResult.HeadCommitID); err != nil {
		return err
	}

	return nil
}

func emitSessionSummary(startTime time.Time, endTime time.Time, session *core.SweSession, buildResult system.BuildSystemResult, appView ui.IAppView, sessionRunErr error, baseCommitID string, headCommitID string) error {
	duration := endTime.Sub(startTime)
	sessionInfo := buildSessionSummaryMessage(duration, session, buildResult)
	if err := saveSessionSummaryMarkdownFunc(buildResult.LogsDir, session, sessionInfo); err != nil {
		if sessionRunErr == nil {
			return fmt.Errorf("emitSessionSummary() [cli.go]: failed to save session summary: %w", err)
		}

		if appView != nil {
			appView.ShowMessage(fmt.Sprintf("Failed to save session summary: %v", err), ui.MessageTypeWarning)
		}
	}

	if err := saveSessionSummaryJSONFunc(buildResult.LogsDir, session, buildResult, startTime, endTime, baseCommitID, headCommitID); err != nil {
		if sessionRunErr == nil {
			return fmt.Errorf("emitSessionSummary() [cli.go]: failed to save session summary JSON: %w", err)
		}

		if appView != nil {
			appView.ShowMessage(fmt.Sprintf("Failed to save session summary JSON: %v", err), ui.MessageTypeWarning)
		}
	}

	if appView != nil {
		appView.ShowMessage(sessionInfo, ui.MessageTypeInfo)
	}

	return sessionRunErr
}

type sessionSummaryJSON struct {
	BaseCommitID     string                `json:"base_commit_id,omitempty"`
	HeadCommitID     string                `json:"head_commit_id,omitempty"`
	EditedFiles      []string              `json:"edited_files"`
	FinalTodoList    []tool.TodoItem       `json:"final_todo_list"`
	FinalTokenUsage  sessionTokenUsageJSON `json:"final_token_usage"`
	FinalContext     int                   `json:"final_context_length"`
	FinalCompactions int                   `json:"final_compaction_count"`
	FinalDuration    string                `json:"final_session_duration"`
	FinalTimestamp   string                `json:"final_session_timestamp"`
	ToolsUsed        []string              `json:"tools_used"`
	RolesUsed        []string              `json:"roles_used"`
	ModelUsed        string                `json:"model_used"`
	ThinkingLevel    string                `json:"thinking_level"`
	LSPServer        string                `json:"lsp_server,omitempty"`
	ContainerImage   string                `json:"container_image,omitempty"`
	TimeSpentSeconds float64               `json:"time_spent_seconds"`
	SessionID        string                `json:"session_id"`
}

type sessionTokenUsageJSON struct {
	Input  int `json:"input"`
	Output int `json:"output"`
	Total  int `json:"total"`
	Cached int `json:"cached"`
}

func saveSessionSummaryJSON(logsDir string, session *core.SweSession, buildResult system.BuildSystemResult, startTime time.Time, endTime time.Time, baseCommitID string, headCommitID string) error {
	if session == nil {
		return fmt.Errorf("saveSessionSummaryJSON() [cli.go]: session is nil")
	}

	if strings.TrimSpace(logsDir) == "" {
		return fmt.Errorf("saveSessionSummaryJSON() [cli.go]: logsDir is empty")
	}

	sessionLogDir := filepath.Join(logsDir, "sessions", session.ID())
	if err := os.MkdirAll(sessionLogDir, 0755); err != nil {
		return fmt.Errorf("saveSessionSummaryJSON() [cli.go]: failed to create session log directory: %w", err)
	}

	summary := buildSessionSummaryJSON(session, buildResult, startTime, endTime, baseCommitID, headCommitID)
	content, err := json.MarshalIndent(summary, "", "  ")
	if err != nil {
		return fmt.Errorf("saveSessionSummaryJSON() [cli.go]: failed to marshal summary json: %w", err)
	}

	filePath := filepath.Join(sessionLogDir, "summary.json")
	if err := os.WriteFile(filePath, content, 0644); err != nil {
		return fmt.Errorf("saveSessionSummaryJSON() [cli.go]: failed to write summary json file: %w", err)
	}

	return nil
}

func buildSessionSummaryJSON(session *core.SweSession, buildResult system.BuildSystemResult, startTime time.Time, endTime time.Time, baseCommitID string, headCommitID string) sessionSummaryJSON {
	usage := session.TokenUsage()
	duration := endTime.Sub(startTime)
	return sessionSummaryJSON{
		BaseCommitID:     strings.TrimSpace(baseCommitID),
		HeadCommitID:     strings.TrimSpace(headCommitID),
		EditedFiles:      collectEditedFiles(buildResult.WorkDirRoot, buildResult.WorkDir, baseCommitID, headCommitID),
		FinalTodoList:    session.GetTodoList(),
		FinalTokenUsage:  sessionTokenUsageJSON{Input: usage.InputTokens, Output: usage.OutputTokens, Total: usage.TotalTokens, Cached: usage.InputCachedTokens},
		FinalContext:     session.ContextLengthTokens(),
		FinalCompactions: session.CompactionCount(),
		FinalDuration:    duration.String(),
		FinalTimestamp:   endTime.Format(time.RFC3339Nano),
		ToolsUsed:        sortedList(session.UsedTools()),
		RolesUsed:        sortedList(session.UsedRoles()),
		ModelUsed:        strings.TrimSpace(session.ModelWithProvider()),
		ThinkingLevel:    strings.TrimSpace(session.ThinkingLevel()),
		LSPServer:        strings.TrimSpace(buildResult.LSPServer),
		ContainerImage:   strings.TrimSpace(buildResult.ContainerImage),
		TimeSpentSeconds: duration.Seconds(),
		SessionID:        session.ID(),
	}
}

func sortedList(values []string) []string {
	if len(values) == 0 {
		return nil
	}

	copyValues := append([]string(nil), values...)
	sort.Strings(copyValues)

	return copyValues
}

func collectEditedFiles(workDirRoot string, workDir string, baseCommitID string, headCommitID string) []string {
	diffDir := chooseGitDiffDir(workDirRoot, workDir)
	if diffDir == "" {
		return nil
	}

	trimmedBase := strings.TrimSpace(baseCommitID)
	trimmedHead := strings.TrimSpace(headCommitID)
	if trimmedBase != "" && trimmedHead != "" && trimmedBase != trimmedHead {
		files := gitDiffNameOnly(diffDir, trimmedBase+".."+trimmedHead)
		if len(files) > 0 {
			return files
		}
	}

	tracked := gitDiffNameOnly(diffDir, "")
	untracked := gitUntrackedFiles(diffDir)
	combined := append(tracked, untracked...)
	if len(combined) == 0 {
		return nil
	}

	unique := make(map[string]struct{}, len(combined))
	for _, file := range combined {
		if strings.TrimSpace(file) == "" {
			continue
		}
		unique[file] = struct{}{}
	}

	result := make([]string, 0, len(unique))
	for file := range unique {
		result = append(result, file)
	}
	sort.Strings(result)

	return result
}

func gitDiffNameOnly(workDir string, commitRange string) []string {
	args := []string{"diff", "--name-only"}
	if strings.TrimSpace(commitRange) != "" {
		args = append(args, commitRange)
	}

	cmd := exec.Command("git", args...)
	cmd.Dir = workDir
	output, err := cmd.Output()
	if err != nil {
		return nil
	}

	return parseGitFileList(output)
}

func gitUntrackedFiles(workDir string) []string {
	cmd := exec.Command("git", "ls-files", "--others", "--exclude-standard")
	cmd.Dir = workDir
	output, err := cmd.Output()
	if err != nil {
		return nil
	}

	return parseGitFileList(output)
}

func parseGitFileList(output []byte) []string {
	if len(output) == 0 {
		return nil
	}

	scanner := bufio.NewScanner(bytes.NewReader(output))
	result := make([]string, 0)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		result = append(result, line)
	}

	if len(result) == 0 {
		return nil
	}

	sort.Strings(result)
	return result
}

func resolveGitCommitID(workDir string, rev string) string {
	if strings.TrimSpace(workDir) == "" || strings.TrimSpace(rev) == "" {
		return ""
	}

	cmd := exec.Command("git", "rev-parse", rev)
	cmd.Dir = workDir
	output, err := cmd.Output()
	if err != nil {
		return ""
	}

	return strings.TrimSpace(string(output))
}

func saveSessionSummaryMarkdown(logsDir string, session *core.SweSession, sessionInfo string) error {
	if session == nil {
		return fmt.Errorf("saveSessionSummaryMarkdown() [cli.go]: session is nil")
	}

	if strings.TrimSpace(logsDir) == "" {
		return fmt.Errorf("saveSessionSummaryMarkdown() [cli.go]: logsDir is empty")
	}

	sessionLogDir := filepath.Join(logsDir, "sessions", session.ID())
	if err := os.MkdirAll(sessionLogDir, 0755); err != nil {
		return fmt.Errorf("saveSessionSummaryMarkdown() [cli.go]: failed to create session log directory: %w", err)
	}

	filePath := filepath.Join(sessionLogDir, "summary.md")
	content := buildSessionSummaryMarkdown(session, sessionInfo)
	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		return fmt.Errorf("saveSessionSummaryMarkdown() [cli.go]: failed to write summary file: %w", err)
	}

	return nil
}

func buildSessionSummaryMarkdown(session *core.SweSession, sessionInfo string) string {
	summary := strings.TrimSpace(lastAssistantMessageText(session))

	var builder strings.Builder
	builder.WriteString("# Summary\n\n")
	builder.WriteString(summary)
	builder.WriteString("\n\n# Session Info\n\n")
	builder.WriteString(strings.TrimSpace(sessionInfo))
	builder.WriteString("\n")
	builder.WriteString(fmt.Sprintf("Session ID: %s\n", session.ID()))

	return builder.String()
}

func lastAssistantMessageText(session *core.SweSession) string {
	if session == nil {
		return ""
	}

	messages := session.ChatMessages()
	for i := len(messages) - 1; i >= 0; i-- {
		message := messages[i]
		if message == nil || message.Role != models.ChatRoleAssistant {
			continue
		}

		var textBuilder strings.Builder
		for _, part := range message.Parts {
			if part.Text != "" {
				textBuilder.WriteString(part.Text)
			}
		}

		if textBuilder.Len() > 0 {
			return textBuilder.String()
		}

		for _, part := range message.Parts {
			if part.ReasoningContent != "" {
				textBuilder.WriteString(part.ReasoningContent)
			}
		}

		return textBuilder.String()
	}

	return ""
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

// parseVFSAllowPaths parses the --vfs-allow flag values.
// It handles both repeated flags and colon-separated values.
func parseVFSAllowPaths(values []string) []string {
	var result []string
	for _, v := range values {
		// Split by colon to support colon-separated list
		parts := strings.Split(v, ":")
		for _, part := range parts {
			part = strings.TrimSpace(part)
			if part != "" {
				result = append(result, part)
			}
		}
	}
	return result
}

type runtimeHookConfigStore struct {
	base           conf.ConfigStore
	hookConfigs    map[string]*conf.HookConfig
	hookConfigsNow time.Time
}

func (s *runtimeHookConfigStore) GetModelProviderConfigs() (map[string]*conf.ModelProviderConfig, error) {
	return s.base.GetModelProviderConfigs()
}

func (s *runtimeHookConfigStore) LastModelProviderConfigsUpdate() (time.Time, error) {
	return s.base.LastModelProviderConfigsUpdate()
}

func (s *runtimeHookConfigStore) GetAgentRoleConfigs() (map[string]*conf.AgentRoleConfig, error) {
	return s.base.GetAgentRoleConfigs()
}

func (s *runtimeHookConfigStore) LastAgentRoleConfigsUpdate() (time.Time, error) {
	return s.base.LastAgentRoleConfigsUpdate()
}

func (s *runtimeHookConfigStore) GetGlobalConfig() (*conf.GlobalConfig, error) {
	return s.base.GetGlobalConfig()
}

func (s *runtimeHookConfigStore) LastGlobalConfigUpdate() (time.Time, error) {
	return s.base.LastGlobalConfigUpdate()
}

func (s *runtimeHookConfigStore) GetMCPServerConfigs() (map[string]*conf.MCPServerConfig, error) {
	return s.base.GetMCPServerConfigs()
}

func (s *runtimeHookConfigStore) LastMCPServerConfigsUpdate() (time.Time, error) {
	return s.base.LastMCPServerConfigsUpdate()
}

func (s *runtimeHookConfigStore) GetHookConfigs() (map[string]*conf.HookConfig, error) {
	cloned := make(map[string]*conf.HookConfig, len(s.hookConfigs))
	for key, value := range s.hookConfigs {
		cloned[key] = value.Clone()
	}

	return cloned, nil
}

func (s *runtimeHookConfigStore) LastHookConfigsUpdate() (time.Time, error) {
	return s.hookConfigsNow, nil
}

func (s *runtimeHookConfigStore) GetAgentConfigFile(subdir, filename string) ([]byte, error) {
	return s.base.GetAgentConfigFile(subdir, filename)
}

func buildRuntimeHookConfigStore(base conf.ConfigStore, overrides []string) (conf.ConfigStore, error) {
	if base == nil {
		return nil, fmt.Errorf("buildRuntimeHookConfigStore() [cli.go]: base config store is nil")
	}

	if len(overrides) == 0 {
		return base, nil
	}

	configs, err := base.GetHookConfigs()
	if err != nil {
		return nil, fmt.Errorf("buildRuntimeHookConfigStore() [cli.go]: failed to load hook configs: %w", err)
	}

	adjusted, err := applyHookOverridesToConfigs(configs, overrides)
	if err != nil {
		return nil, err
	}

	return &runtimeHookConfigStore{
		base:           base,
		hookConfigs:    adjusted,
		hookConfigsNow: time.Now(),
	}, nil
}

type hookOverride struct {
	Name     string
	Disable  bool
	Settings map[string]string
}

func applyHookOverridesToConfigs(configs map[string]*conf.HookConfig, overrides []string) (map[string]*conf.HookConfig, error) {
	cloned := make(map[string]*conf.HookConfig, len(configs))
	for key, value := range configs {
		cloned[key] = value.Clone()
	}

	for _, rawOverride := range overrides {
		override, err := parseHookOverride(rawOverride)
		if err != nil {
			return nil, err
		}

		current, exists := cloned[override.Name]
		if !exists {
			if len(override.Settings) == 0 || override.Disable {
				return nil, fmt.Errorf("applyHookOverridesToConfigs() [cli.go]: hook %q is not configured", override.Name)
			}

			created, createErr := buildNewHookConfig(override.Name, override.Settings)
			if createErr != nil {
				return nil, createErr
			}
			cloned[override.Name] = created
			continue
		}

		wasDisabled := !current.Enabled
		if override.Disable {
			current.Enabled = false
			continue
		}

		if len(override.Settings) == 0 {
			if !current.Enabled {
				current.Enabled = true
			}
			applyHookDefaults(current)
			continue
		}

		if err := applyHookSettings(current, override.Name, override.Settings); err != nil {
			return nil, err
		}
		if wasDisabled {
			current.Enabled = true
		}
		applyHookDefaults(current)
	}

	return cloned, nil
}

func parseHookOverride(value string) (*hookOverride, error) {
	trimmed := strings.TrimSpace(value)
	trimmed = strings.TrimPrefix(trimmed, ":")
	if trimmed == "" {
		return nil, fmt.Errorf("parseHookOverride() [cli.go]: hook override cannot be empty")
	}

	parts := strings.SplitN(trimmed, ":", 2)
	name := strings.TrimSpace(parts[0])
	if name == "" {
		return nil, fmt.Errorf("parseHookOverride() [cli.go]: hook name is required")
	}

	if len(parts) == 1 {
		return &hookOverride{Name: name}, nil
	}

	action := strings.TrimSpace(parts[1])
	if action == "" {
		return nil, fmt.Errorf("parseHookOverride() [cli.go]: hook action for %q is empty", name)
	}

	if strings.EqualFold(action, "disable") {
		return &hookOverride{Name: name, Disable: true}, nil
	}

	settings := make(map[string]string)
	entries := strings.Split(action, ",")
	for _, entry := range entries {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}

		keyValue := strings.SplitN(entry, "=", 2)
		if len(keyValue) != 2 {
			return nil, fmt.Errorf("parseHookOverride() [cli.go]: invalid hook setting %q for %q", entry, name)
		}

		key := strings.TrimSpace(keyValue[0])
		val := strings.TrimSpace(keyValue[1])
		if key == "" {
			return nil, fmt.Errorf("parseHookOverride() [cli.go]: empty setting key in %q", entry)
		}
		settings[key] = val
	}

	if len(settings) == 0 {
		return nil, fmt.Errorf("parseHookOverride() [cli.go]: no hook settings provided for %q", name)
	}

	return &hookOverride{Name: name, Settings: settings}, nil
}

func buildNewHookConfig(name string, settings map[string]string) (*conf.HookConfig, error) {
	created := &conf.HookConfig{Name: name, Enabled: true}
	if err := applyHookSettings(created, name, settings); err != nil {
		return nil, err
	}
	if strings.TrimSpace(created.Hook) == "" {
		return nil, fmt.Errorf("buildNewHookConfig() [cli.go]: hook %q requires setting \"hook\"", name)
	}
	if created.Type == conf.HookTypeLLM {
		if strings.TrimSpace(created.Prompt) == "" {
			return nil, fmt.Errorf("buildNewHookConfig() [cli.go]: hook %q requires setting \"prompt\"", name)
		}
	} else {
		if strings.TrimSpace(created.Command) == "" {
			return nil, fmt.Errorf("buildNewHookConfig() [cli.go]: hook %q requires setting \"command\"", name)
		}
	}
	applyHookDefaults(created)

	return created, nil
}

func applyHookSettings(target *conf.HookConfig, name string, settings map[string]string) error {
	if target == nil {
		return fmt.Errorf("applyHookSettings() [cli.go]: target hook is nil")
	}

	for key, value := range settings {
		switch strings.ToLower(strings.TrimSpace(key)) {
		case "enabled":
			parsed, err := strconv.ParseBool(value)
			if err != nil {
				return fmt.Errorf("applyHookSettings() [cli.go]: invalid enabled value for hook %q: %w", name, err)
			}
			target.Enabled = parsed
		case "hook":
			target.Hook = strings.TrimSpace(value)
		case "name":
			if strings.TrimSpace(value) != "" && strings.TrimSpace(value) != name {
				return fmt.Errorf("applyHookSettings() [cli.go]: name override must match hook selector %q", name)
			}
			target.Name = name
		case "type":
			hookType := conf.HookType(strings.TrimSpace(value))
			switch hookType {
			case conf.HookTypeShell, conf.HookTypeLLM, conf.HookTypeSubAgent:
				target.Type = hookType
			default:
				return fmt.Errorf("applyHookSettings() [cli.go]: unsupported hook type %q for %q", value, name)
			}
		case "command":
			target.Command = value
		case "prompt":
			target.Prompt = value
		case "system_prompt", "system-prompt":
			target.SystemPrompt = value
		case "model":
			target.Model = value
		case "thinking":
			target.Thinking = value
		case "to_field", "to-field", "tofield":
			target.ToField = value
		case "timeout":
			parsed, err := parseHookTimeout(value)
			if err != nil {
				return fmt.Errorf("applyHookSettings() [cli.go]: invalid timeout for hook %q: %w", name, err)
			}
			target.Timeout = parsed
		case "run-on", "runon":
			runOn := conf.HookRunOn(strings.TrimSpace(value))
			switch runOn {
			case conf.HookRunOnHost, conf.HookRunOnSandbox:
				target.RunOn = runOn
			default:
				return fmt.Errorf("applyHookSettings() [cli.go]: unsupported run-on value %q for %q", value, name)
			}
		default:
			return fmt.Errorf("applyHookSettings() [cli.go]: unsupported hook setting %q for %q", key, name)
		}
	}

	if strings.TrimSpace(target.Name) == "" {
		target.Name = name
	}

	return nil
}

func parseHookTimeout(value string) (time.Duration, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return 0, nil
	}
	if _, err := strconv.ParseInt(trimmed, 10, 64); err == nil {
		trimmed += "s"
	}
	parsed, err := time.ParseDuration(trimmed)
	if err != nil {
		return 0, err
	}
	if parsed < 0 {
		return 0, fmt.Errorf("duration must not be negative")
	}

	return parsed, nil
}

func applyHookDefaults(target *conf.HookConfig) {
	if target == nil {
		return
	}
	if target.Type == "" {
		target.Type = conf.HookTypeShell
	}
	if target.RunOn == "" {
		target.RunOn = conf.HookRunOnSandbox
	}
	if target.Type == conf.HookTypeLLM && strings.TrimSpace(target.ToField) == "" {
		target.ToField = "result"
	}
}

func buildSessionSummaryMessage(duration time.Duration, session *core.SweSession, buildResult system.BuildSystemResult) string {
	base := fmt.Sprintf("Session completed in %s", duration.Round(time.Second))
	if session == nil {
		return base
	}

	usage := session.TokenUsage()
	primary := fmt.Sprintf(
		"%s | tokens(input=%d[cached=%d,noncached=%d], output=%d, total=%d) | context=%d",
		base,
		usage.InputTokens,
		usage.InputCachedTokens,
		usage.InputNonCachedTokens,
		usage.OutputTokens,
		usage.TotalTokens,
		session.ContextLengthTokens(),
	)

	lines := []string{primary}
	lines = append(lines, fmt.Sprintf("Model: %s", nullValue(session.ModelWithProvider())))
	lines = append(lines, fmt.Sprintf("Thinking: %s", nullValue(session.ThinkingLevel())))
	lines = append(lines, fmt.Sprintf("LSP server: %s", nullValue(strings.TrimSpace(buildResult.LSPServer))))
	lines = append(lines, fmt.Sprintf("Container image: %s", nullValue(strings.TrimSpace(buildResult.ContainerImage))))
	lines = append(lines, fmt.Sprintf("Roles used: %s", formatList(session.UsedRoles())))
	lines = append(lines, fmt.Sprintf("Tools used: %s", formatList(session.UsedTools())))
	lines = append(lines, "")
	lines = append(lines, "Edited files:")
	lines = append(lines, formatEditedFilesSummary(buildResult.WorkDirRoot, buildResult.WorkDir))

	return strings.Join(lines, "\n")
}

func nullValue(value string) string {
	if strings.TrimSpace(value) == "" {
		return "-"
	}

	return value
}

func buildContainerStartupInfoMessage(buildResult system.BuildSystemResult) string {
	identity := buildResult.ContainerIdentity
	return fmt.Sprintf(
		"[INFO] Container: image=%s tag=%s version=%s user=%s(uid=%d) group=%s(gid=%d)",
		nullValue(strings.TrimSpace(buildResult.ContainerImageName)),
		nullValue(strings.TrimSpace(buildResult.ContainerImageTag)),
		nullValue(strings.TrimSpace(buildResult.ContainerImageVersion)),
		nullValue(strings.TrimSpace(identity.UserName)),
		identity.UID,
		nullValue(strings.TrimSpace(identity.GroupName)),
		identity.GID,
	)
}

func formatList(values []string) string {
	if len(values) == 0 {
		return "-"
	}

	copyValues := append([]string(nil), values...)
	sort.Strings(copyValues)
	return strings.Join(copyValues, ", ")
}

func formatEditedFilesSummary(workDirRoot string, workDir string) string {
	diffDir := chooseGitDiffDir(workDirRoot, workDir)
	cmd := exec.Command("git", "diff", "--numstat")
	cmd.Dir = diffDir

	output, err := cmd.Output()
	if err != nil {
		return "-"
	}

	scanner := bufio.NewScanner(bytes.NewReader(output))
	lines := make([]string, 0)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		parts := strings.Split(line, "\t")
		if len(parts) != 3 {
			continue
		}

		lines = append(lines, fmt.Sprintf("- %s (+%s/-%s)", parts[2], parts[0], parts[1]))
	}

	untrackedCmd := exec.Command("git", "ls-files", "--others", "--exclude-standard")
	untrackedCmd.Dir = diffDir
	untrackedOutput, untrackedErr := untrackedCmd.Output()
	if untrackedErr == nil {
		untrackedScanner := bufio.NewScanner(bytes.NewReader(untrackedOutput))
		for untrackedScanner.Scan() {
			path := strings.TrimSpace(untrackedScanner.Text())
			if path == "" {
				continue
			}
			lines = append(lines, fmt.Sprintf("- %s (new file)", path))
		}
	}

	if len(lines) == 0 {
		return "-"
	}

	return strings.Join(lines, "\n")
}

func chooseGitDiffDir(workDirRoot string, workDir string) string {
	if strings.TrimSpace(workDir) != "" {
		return workDir
	}
	if strings.TrimSpace(workDirRoot) != "" {
		return workDirRoot
	}

	return "."
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

func resolveHostGitConfigValue(key string) string {
	if _, err := gitLookPathFunc("git"); err != nil {
		return ""
	}

	value, err := gitConfigValueFunc(key)
	if err != nil {
		return ""
	}

	return strings.TrimSpace(value)
}

// resolveGitIdentity returns the provided value if non-empty, otherwise falls back to host git config.
func resolveGitIdentity(value, gitConfigKey string) string {
	if value != "" {
		return value
	}
	return resolveHostGitConfigValue(gitConfigKey)
}

type worktreeFinalizeResult struct {
	HeadCommitID string
}

func finalizeWorktreeSession(ctx context.Context, vcs vfs.VCS, worktreeBranch string, merge bool, commitMessageTemplate string, sweSystem *system.SweSystem, session *core.SweSession, stderr io.Writer, repoDir string, worktreeDir string, originalPrompt string, hookEngine *core.HookEngine, appView ui.IAppView) (worktreeFinalizeResult, error) {
	result := worktreeFinalizeResult{}
	if worktreeBranch == "" || vcs == nil {
		return result, nil
	}

	baseBranch := detectMergeBaseBranch(repoDir)
	commitMessage := ""
	commitHandledByHook := false

	hookCommitMessage, skipBuiltInCommit, commitHookErr := handleCommitHookResponse(ctx, hookEngine, vcs, worktreeBranch, repoDir, worktreeDir, session, appView)
	if commitHookErr != nil {
		_, _ = fmt.Fprintf(stderr, "worktree commit hook failed: %v\n", commitHookErr)
		return result, fmt.Errorf("finalizeWorktreeSession() [cli.go]: commit hook failed: %w", commitHookErr)
	}
	if strings.TrimSpace(hookCommitMessage) != "" {
		commitMessage = hookCommitMessage
	}
	if skipBuiltInCommit {
		commitHandledByHook = true
	}

	if !commitHandledByHook {
		if strings.TrimSpace(commitMessage) == "" {
			generatedMessage, err := core.GenerateCommitMessage(ctx, sweSystem.ModelProviders, sweSystem.ConfigStore, session, worktreeBranch, commitMessageTemplate)
			if err != nil {
				_, _ = fmt.Fprintf(stderr, "worktree commit message generation failed: %v\n", err)
				if dropErr := vcs.DropWorktree(worktreeBranch); dropErr != nil {
					_, _ = fmt.Fprintf(stderr, "worktree cleanup failed: %v\n", dropErr)
				}
				return result, nil
			}
			commitMessage = generatedMessage
		}

		if commitErr := vcs.CommitWorktree(worktreeBranch, commitMessage); commitErr != nil && !errors.Is(commitErr, vfs.ErrNoChangesToCommit) {
			_, _ = fmt.Fprintf(stderr, "worktree commit failed: %v\n", commitErr)
			if merge {
				_, _ = fmt.Fprintln(stderr, "merge skipped because commit failed. Resolve issues and merge manually.")
				return result, nil
			}
		}
	}

	if merge {
		mergeHookExecuted := false
		if hookEngine != nil {
			hookEngine.MergeContext(map[string]string{
				"branch":  strings.TrimSpace(worktreeBranch),
				"workdir": strings.TrimSpace(firstNonEmpty(worktreeDir, repoDir)),
				"rootdir": strings.TrimSpace(repoDir),
			})
			hookResult, hookErr := hookEngine.Execute(ctx, core.HookExecutionRequest{Name: "merge", View: appView, VCS: vcs, Session: session})
			if hookResult != nil {
				mergeHookExecuted = true
			}
			if hookErr != nil {
				_, _ = fmt.Fprintf(stderr, "automatic merge failed: %v\n", hookErr)
				_, _ = fmt.Fprintln(stderr, "worktree and feature branch were kept for manual investigation.")
				return result, nil
			}
		}

		if mergeHookExecuted {
			result.HeadCommitID = resolveGitCommitID(repoDir, baseBranch)
		} else if strings.TrimSpace(repoDir) == "" || strings.TrimSpace(worktreeDir) == "" || sweSystem == nil || session == nil {
			mergeErr := vcs.MergeBranches(baseBranch, worktreeBranch)
			if mergeErr != nil {
				if errors.Is(mergeErr, vfs.ErrMergeConflict) {
					_, _ = fmt.Fprintf(stderr, "automatic merge failed due to conflicts: %v\n", mergeErr)
					_, _ = fmt.Fprintf(stderr, "resolve conflicts manually and merge branch '%s' into %s.\n", worktreeBranch, baseBranch)
					_, _ = fmt.Fprintln(stderr, "worktree and feature branch were kept for manual conflict resolution.")
					return result, nil
				}

				_, _ = fmt.Fprintf(stderr, "automatic merge failed: %v\n", mergeErr)
				_, _ = fmt.Fprintln(stderr, "worktree and feature branch were kept for manual investigation.")
				return result, nil
			}

			result.HeadCommitID = resolveGitCommitID(repoDir, baseBranch)
		} else {
			headCommitID, mergeErr := mergeWorktreeWithConflictResolution(ctx, repoDir, worktreeDir, worktreeBranch, baseBranch, originalPrompt, sweSystem, session, stderr)
			if mergeErr != nil {
				_, _ = fmt.Fprintf(stderr, "automatic merge failed: %v\n", mergeErr)
				_, _ = fmt.Fprintln(stderr, "worktree and feature branch were kept for manual investigation.")
				return result, nil
			}
			result.HeadCommitID = headCommitID
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

	return result, nil
}

func handleCommitHookResponse(ctx context.Context, hookEngine *core.HookEngine, vcs vfs.VCS, worktreeBranch string, repoDir string, worktreeDir string, session *core.SweSession, appView ui.IAppView) (string, bool, error) {
	if hookEngine == nil {
		return "", false, nil
	}

	hookEngine.MergeContext(map[string]string{
		"branch":  strings.TrimSpace(worktreeBranch),
		"workdir": strings.TrimSpace(firstNonEmpty(worktreeDir, repoDir)),
		"rootdir": strings.TrimSpace(repoDir),
	})
	hookResult, hookErr := hookEngine.Execute(ctx, core.HookExecutionRequest{Name: "commit", View: appView, VCS: vcs, Session: session})
	if hookResult == nil {
		if hookErr != nil {
			return "", false, fmt.Errorf("handleCommitHookResponse() [cli.go]: commit hook execution failed: %w", hookErr)
		}
		return "", false, nil
	}

	response := core.FindHookResponseRequest(hookResult)
	if response == nil {
		if hookErr != nil {
			return "", false, fmt.Errorf("handleCommitHookResponse() [cli.go]: commit hook execution failed: %w", hookErr)
		}
		return "", false, nil
	}

	status := strings.ToUpper(strings.TrimSpace(core.HookResponseStatus(response)))
	if status == "" {
		status = "OK"
	}

	switch status {
	case "TIMEOUT", "ERROR":
		return "", false, fmt.Errorf("handleCommitHookResponse() [cli.go]: commit hook returned status %s", status)
	case "COMMITED":
		resetDir := strings.TrimSpace(firstNonEmpty(worktreeDir, repoDir))
		if err := hardResetWorktree(resetDir); err != nil {
			return "", false, err
		}
		return "", true, nil
	default:
		return core.HookResponseArgString(response, "commit-message"), false, nil
	}
}

func hardResetWorktree(workDir string) error {
	trimmedWorkDir := strings.TrimSpace(workDir)
	if trimmedWorkDir == "" {
		return fmt.Errorf("hardResetWorktree() [cli.go]: workDir is empty")
	}

	if _, err := runGitCommandFunc(trimmedWorkDir, "reset", "--hard", "HEAD"); err != nil {
		return fmt.Errorf("hardResetWorktree() [cli.go]: failed to reset worktree: %w", err)
	}
	if _, err := runGitCommandFunc(trimmedWorkDir, "clean", "-fd"); err != nil {
		return fmt.Errorf("hardResetWorktree() [cli.go]: failed to clean worktree: %w", err)
	}

	return nil
}

// detectMergeBaseBranch resolves the branch currently checked out in repoDir.
func detectMergeBaseBranch(repoDir string) string {
	if strings.TrimSpace(repoDir) == "" {
		return "main"
	}

	branch, err := runGitCommandFunc(repoDir, "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		return "main"
	}

	trimmedBranch := strings.TrimSpace(branch)
	if trimmedBranch == "" || trimmedBranch == "HEAD" {
		return "main"
	}

	return trimmedBranch
}

type conflictResolutionPromptData struct {
	Branch         string
	OriginalPrompt string
	ConflictFiles  string
	ConflictOutput string
}

func mergeWorktreeWithConflictResolution(ctx context.Context, repoDir string, worktreeDir string, worktreeBranch string, baseBranch string, originalPrompt string, sweSystem *system.SweSystem, session *core.SweSession, stderr io.Writer) (string, error) {
	if sweSystem == nil || sweSystem.ConfigStore == nil {
		return "", fmt.Errorf("mergeWorktreeWithConflictResolution() [cli.go]: system config store is not available")
	}
	if session == nil {
		return "", fmt.Errorf("mergeWorktreeWithConflictResolution() [cli.go]: session is nil")
	}
	if strings.TrimSpace(baseBranch) == "" {
		return "", fmt.Errorf("mergeWorktreeWithConflictResolution() [cli.go]: baseBranch is empty")
	}

	attempt := 1
	failedRebaseOutput := ""
	conflictFiles := []string{}

	for {
		command := []string{"rebase", baseBranch}
		_, rebaseErr := runGitCommandFunc(worktreeDir, command...)
		if rebaseErr == nil {
			_, _ = fmt.Fprintf(stderr, "[conflict] rebase step succeeded (attempt %d, command: git %s)\n", attempt, strings.Join(command, " "))
			break
		}

		rebaseOutput := rebaseErr.Error()
		if !isMergeConflictError(rebaseOutput) {
			return "", fmt.Errorf("mergeWorktreeWithConflictResolution() [cli.go]: rebase failed: %w", rebaseErr)
		}

		failedRebaseOutput = rebaseOutput
		conflictFiles = listGitConflictFilesFunc(worktreeDir)
		if len(conflictFiles) == 0 {
			conflictFiles = extractConflictFilesFromOutput(rebaseOutput)
		}

		_, _ = fmt.Fprintf(stderr, "[conflict] merge/rebase conflict detected on attempt %d\n", attempt)
		if len(conflictFiles) > 0 {
			_, _ = fmt.Fprintf(stderr, "[conflict] conflicted files: %s\n", strings.Join(conflictFiles, ", "))
		}

		prompt, err := buildConflictResolutionPrompt(sweSystem.ConfigStore, conflictResolutionPromptData{
			Branch:         worktreeBranch,
			OriginalPrompt: originalPrompt,
			ConflictFiles:  strings.Join(conflictFiles, "\n"),
			ConflictOutput: failedRebaseOutput,
		})
		if err != nil {
			return "", err
		}

		request := tool.SubAgentTaskRequest{
			Slug:   fmt.Sprintf("conflict-resolution-%d", attempt),
			Title:  fmt.Sprintf("Resolve merge conflicts (%d)", attempt),
			Role:   "developer",
			Prompt: prompt,
		}
		_, _ = fmt.Fprintf(stderr, "[conflict] starting conflict-resolution sub-session: %s\n", request.Slug)
		subAgentResult, subAgentErr := executeConflictSubAgentFunc(session, request)
		if subAgentErr != nil {
			return "", fmt.Errorf("mergeWorktreeWithConflictResolution() [cli.go]: sub-session failed: %w", subAgentErr)
		}
		if subAgentResult.Status != "completed" {
			return "", fmt.Errorf("mergeWorktreeWithConflictResolution() [cli.go]: conflict-resolution sub-session ended with status %q: %s", subAgentResult.Status, strings.TrimSpace(subAgentResult.Summary))
		}
		_, _ = fmt.Fprintf(stderr, "[conflict] conflict-resolution sub-session completed: %s\n", request.Slug)
		attempt++
	}

	mergeWorktreePath, cleanup, err := createMergeWorktree(repoDir, baseBranch)
	if err != nil {
		return "", err
	}
	defer cleanup()

	if _, err := runGitCommandFunc(mergeWorktreePath, "merge", "--ff-only", worktreeBranch); err != nil {
		return "", fmt.Errorf("mergeWorktreeWithConflictResolution() [cli.go]: fast-forward merge into %q failed: %w", baseBranch, err)
	}

	headCommitID, err := runGitCommandFunc(mergeWorktreePath, "rev-parse", "HEAD")
	if err != nil {
		return "", fmt.Errorf("mergeWorktreeWithConflictResolution() [cli.go]: failed to resolve %q HEAD commit: %w", baseBranch, err)
	}

	if err := syncCheckedOutBranchWorktrees(repoDir, baseBranch, mergeWorktreePath); err != nil {
		return "", fmt.Errorf("mergeWorktreeWithConflictResolution() [cli.go]: %w", err)
	}

	return strings.TrimSpace(headCommitID), nil
}

func syncCheckedOutBranchWorktrees(repoDir string, branch string, skipPath string) error {
	worktreesOutput, err := runGitCommandFunc(repoDir, "worktree", "list", "--porcelain")
	if err != nil {
		return fmt.Errorf("syncCheckedOutBranchWorktrees() [cli.go]: failed to list git worktrees: %w", err)
	}

	branchRef := "refs/heads/" + branch
	skipPath = filepath.Clean(strings.TrimSpace(skipPath))

	resetWorktree := func(worktreePath string, worktreeBranchRef string) error {
		trimmedPath := filepath.Clean(strings.TrimSpace(worktreePath))
		if trimmedPath == "" || worktreeBranchRef != branchRef || trimmedPath == skipPath {
			return nil
		}

		if _, resetErr := runGitCommandFunc(trimmedPath, "reset", "--hard", branch); resetErr != nil {
			return fmt.Errorf("syncCheckedOutBranchWorktrees() [cli.go]: failed to update checked-out %q worktree at %q: %w", branch, trimmedPath, resetErr)
		}

		return nil
	}

	currentPath := ""
	currentBranchRef := ""
	for _, line := range strings.Split(worktreesOutput, "\n") {
		trimmedLine := strings.TrimSpace(line)
		if trimmedLine == "" {
			if err := resetWorktree(currentPath, currentBranchRef); err != nil {
				return err
			}
			currentPath = ""
			currentBranchRef = ""
			continue
		}

		if strings.HasPrefix(trimmedLine, "worktree ") {
			currentPath = strings.TrimPrefix(trimmedLine, "worktree ")
			continue
		}

		if strings.HasPrefix(trimmedLine, "branch ") {
			currentBranchRef = strings.TrimPrefix(trimmedLine, "branch ")
		}
	}

	if err := resetWorktree(currentPath, currentBranchRef); err != nil {
		return err
	}

	return nil
}

func createMergeWorktree(repoDir string, branch string) (string, func(), error) {
	if strings.TrimSpace(branch) == "" {
		return "", func() {}, fmt.Errorf("createMergeWorktree() [cli.go]: branch is empty")
	}

	workRoot := filepath.Join(repoDir, ".cswdata", "work")
	if err := os.MkdirAll(workRoot, 0755); err != nil {
		return "", func() {}, fmt.Errorf("createMergeWorktree() [cli.go]: failed to create worktree root directory: %w", err)
	}

	mergeWorktreePath, err := os.MkdirTemp(workRoot, ".merge-"+strings.ReplaceAll(branch, "/", "-")+"-")
	if err != nil {
		return "", func() {}, fmt.Errorf("createMergeWorktree() [cli.go]: failed to allocate temporary merge worktree path: %w", err)
	}

	if err := os.RemoveAll(mergeWorktreePath); err != nil {
		return "", func() {}, fmt.Errorf("createMergeWorktree() [cli.go]: failed to prepare temporary merge worktree path: %w", err)
	}

	if _, err := runGitCommandFunc(repoDir, "worktree", "add", "--force", mergeWorktreePath, branch); err != nil {
		_ = os.RemoveAll(mergeWorktreePath)
		return "", func() {}, fmt.Errorf("createMergeWorktree() [cli.go]: failed to create temporary %q worktree: %w", branch, err)
	}

	cleanup := func() {
		_, _ = runGitCommandFunc(repoDir, "worktree", "remove", "--force", mergeWorktreePath)
		_ = os.RemoveAll(mergeWorktreePath)
	}

	return mergeWorktreePath, cleanup, nil
}

func buildConflictResolutionPrompt(configStore interface {
	GetAgentConfigFile(subdir, filename string) ([]byte, error)
}, data conflictResolutionPromptData) (string, error) {
	templateBytes, err := configStore.GetAgentConfigFile("conflict", "prompt.md")
	if err != nil {
		return "", fmt.Errorf("buildConflictResolutionPrompt() [cli.go]: failed to read conflict/prompt.md: %w", err)
	}

	tmpl, err := template.New("conflict-prompt").Parse(string(templateBytes))
	if err != nil {
		return "", fmt.Errorf("buildConflictResolutionPrompt() [cli.go]: failed to parse prompt template: %w", err)
	}

	var promptBuffer bytes.Buffer
	if err := tmpl.Execute(&promptBuffer, data); err != nil {
		return "", fmt.Errorf("buildConflictResolutionPrompt() [cli.go]: failed to render prompt template: %w", err)
	}

	return promptBuffer.String(), nil
}

func executeConflictSubAgentTask(session *core.SweSession, request tool.SubAgentTaskRequest) (tool.SubAgentTaskResult, error) {
	if session == nil {
		return tool.SubAgentTaskResult{}, fmt.Errorf("executeConflictSubAgentTask() [cli.go]: session is nil")
	}

	return session.ExecuteSubAgentTask(request)
}

func runGitCommand(workDir string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = workDir
	output, err := cmd.CombinedOutput()
	trimmedOutput := strings.TrimSpace(string(output))
	if err != nil {
		return "", fmt.Errorf("runGitCommand() [cli.go]: git %s failed: %w: %s", strings.Join(args, " "), err, trimmedOutput)
	}

	return trimmedOutput, nil
}

func listGitConflictFiles(workDir string) []string {
	output, err := runGitCommandFunc(workDir, "diff", "--name-only", "--diff-filter=U")
	if err != nil {
		return nil
	}

	lines := strings.Split(strings.TrimSpace(output), "\n")
	conflicts := make([]string, 0, len(lines))
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		conflicts = append(conflicts, trimmed)
	}

	return conflicts
}

func extractConflictFilesFromOutput(output string) []string {
	if strings.TrimSpace(output) == "" {
		return nil
	}

	pattern := regexp.MustCompile(`(?m)CONFLICT \([^\)]*\): .* in (.+)$`)
	matches := pattern.FindAllStringSubmatch(output, -1)
	if len(matches) == 0 {
		return nil
	}

	seen := make(map[string]struct{}, len(matches))
	files := make([]string, 0, len(matches))
	for _, match := range matches {
		if len(match) < 2 {
			continue
		}
		name := strings.TrimSpace(match[1])
		if name == "" {
			continue
		}
		if _, exists := seen[name]; exists {
			continue
		}
		seen[name] = struct{}{}
		files = append(files, name)
	}

	return files
}

func isMergeConflictError(output string) bool {
	trimmedOutput := strings.TrimSpace(output)
	if trimmedOutput == "" {
		return false
	}

	upperOutput := strings.ToUpper(trimmedOutput)
	if strings.Contains(upperOutput, "CONFLICT") {
		return true
	}

	return strings.Contains(trimmedOutput, "could not apply") ||
		strings.Contains(trimmedOutput, "Resolve all conflicts manually") ||
		strings.Contains(trimmedOutput, "rebase-merge")
}
