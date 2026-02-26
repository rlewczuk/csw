package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

	"github.com/rlewczuk/csw/pkg/conf"
	"github.com/rlewczuk/csw/pkg/conf/impl"
	"github.com/rlewczuk/csw/pkg/core"
	"github.com/rlewczuk/csw/pkg/logging"
	"github.com/rlewczuk/csw/pkg/lsp"
	"github.com/rlewczuk/csw/pkg/models"
	"github.com/rlewczuk/csw/pkg/runner"
	"github.com/rlewczuk/csw/pkg/tool"
	"github.com/rlewczuk/csw/pkg/vfs"
)

const (
	// defaultGitAuthorName is used in container mode when git identity cannot be resolved.
	defaultGitAuthorName = "CSW"
	// defaultGitAuthorEmail is used in container mode when git identity cannot be resolved.
	defaultGitAuthorEmail = "csw@example.com"
)

// gitLookPathFunc resolves executable path for git and can be overridden in tests.
var gitLookPathFunc = exec.LookPath

// gitConfigValueFunc resolves git config values and can be overridden in tests.
var gitConfigValueFunc = readGitConfigValue

// BuildSystemParams contains inputs for constructing a SweSystem.
type BuildSystemParams struct {
	WorkDir        string
	ConfigPath     string
	ModelName      string
	RoleName       string
	WorktreeBranch string
	ContainerImage string
	LSPServer      string
	LogLLMRequests bool
	// Thinking controls the thinking/reasoning mode for LLM requests.
	// Values like "low", "medium", "high", "xhigh" for effort-based thinking,
	// or "true"/"false" for boolean thinking modes.
	Thinking string
}

// BuildSystemResult contains outputs from building a SweSystem.
type BuildSystemResult struct {
	WorkDir          string
	RoleConfig       conf.AgentRoleConfig
	ModelName        string
	ConfigStore      conf.ConfigStore
	ProviderRegistry *models.ProviderRegistry
	LogsDir          string
	VCS              vfs.VCS
	WorktreeBranch   string
	LSPStarted       bool
	LSPWorkDir       string
	Cleanup          func()
}

func prepareSessionVFS(workDir string, worktreeBranch string, hidePatterns []string) (vfs.VCS, vfs.VFS, error) {
	localVFS, err := vfs.NewLocalVFS(workDir, hidePatterns)
	if err != nil {
		return nil, nil, fmt.Errorf("prepareSessionVFS() [bootstrap.go]: failed to create local VFS: %w", err)
	}

	nullVCS, err := vfs.NewNullVFS(localVFS)
	if err != nil {
		return nil, nil, fmt.Errorf("prepareSessionVFS() [bootstrap.go]: failed to create NullVCS: %w", err)
	}

	var selectedVCS vfs.VCS = nullVCS

	if worktreeBranch != "" {
		worktreesRoot := filepath.Join(workDir, ".cswdata", "work")
		gitRepo, err := vfs.NewGitRepo(workDir, worktreesRoot, hidePatterns)
		if err != nil {
			return nil, nil, fmt.Errorf("prepareSessionVFS() [bootstrap.go]: failed to create GitVCS: %w", err)
		}

		if err := os.RemoveAll(filepath.Join(worktreesRoot, worktreeBranch)); err != nil {
			return nil, nil, fmt.Errorf("prepareSessionVFS() [bootstrap.go]: failed to remove existing worktree path: %w", err)
		}

		if err := gitRepo.DropWorktree(worktreeBranch); err != nil && !errors.Is(err, vfs.ErrFileNotFound) {
			return nil, nil, fmt.Errorf("prepareSessionVFS() [bootstrap.go]: failed to drop existing worktree: %w", err)
		}

		if err := gitRepo.DeleteBranch(worktreeBranch); err != nil && !errors.Is(err, vfs.ErrFileNotFound) {
			return nil, nil, fmt.Errorf("prepareSessionVFS() [bootstrap.go]: failed to delete existing worktree branch: %w", err)
		}

		if err := gitRepo.NewBranch(worktreeBranch, "HEAD"); err != nil {
			return nil, nil, fmt.Errorf("prepareSessionVFS() [bootstrap.go]: failed to create worktree branch: %w", err)
		}

		selectedVCS = gitRepo
	}

	selectedVFS, err := selectedVCS.GetWorktree(worktreeBranch)
	if err != nil {
		return nil, nil, fmt.Errorf("prepareSessionVFS() [bootstrap.go]: failed to get selected worktree: %w", err)
	}

	return selectedVCS, selectedVFS, nil
}

// BuildSystem builds a SweSystem and related setup for CLI and TUI.
func BuildSystem(params BuildSystemParams) (*core.SweSystem, BuildSystemResult, error) {
	var result BuildSystemResult

	workDir, err := ResolveWorkDir(params.WorkDir)
	if err != nil {
		return nil, result, fmt.Errorf("BuildSystem() [bootstrap.go]: %w", err)
	}

	logsDir := filepath.Join(workDir, ".cswdata", "logs")
	if err := logging.SetLogsDirectory(logsDir, true); err != nil {
		return nil, result, fmt.Errorf("BuildSystem() [bootstrap.go]: failed to initialize logging: %w", err)
	}

	configPathStr, err := BuildConfigPath(params.ConfigPath)
	if err != nil {
		logging.FlushLogs()
		return nil, result, fmt.Errorf("BuildSystem() [bootstrap.go]: %w", err)
	}

	configStore, err := impl.NewCompositeConfigStore(workDir, configPathStr)
	if err != nil {
		logging.FlushLogs()
		return nil, result, fmt.Errorf("BuildSystem() [bootstrap.go]: failed to create config store: %w", err)
	}

	providerRegistry := models.NewProviderRegistry(configStore)
	if len(providerRegistry.List()) == 0 {
		logging.FlushLogs()
		return nil, result, fmt.Errorf("BuildSystem() [bootstrap.go]: no model providers found in config")
	}

	modelName, err := ResolveModelName(params.ModelName, configStore, providerRegistry)
	if err != nil {
		logging.FlushLogs()
		return nil, result, fmt.Errorf("BuildSystem() [bootstrap.go]: %w", err)
	}

	modelProviders, err := CreateProviderMap(providerRegistry)
	if err != nil {
		logging.FlushLogs()
		return nil, result, fmt.Errorf("BuildSystem() [bootstrap.go]: %w", err)
	}

	roleRegistry := core.NewAgentRoleRegistry(configStore)
	if len(roleRegistry.List()) == 0 {
		logging.FlushLogs()
		return nil, result, fmt.Errorf("BuildSystem() [bootstrap.go]: no roles found in config")
	}

	roleConfig, ok := roleRegistry.Get(params.RoleName)
	if !ok {
		logging.FlushLogs()
		return nil, result, fmt.Errorf("BuildSystem() [bootstrap.go]: role not found: %s (available: %v)", params.RoleName, roleRegistry.List())
	}

	hidePatterns, err := vfs.BuildHidePatterns(workDir, roleConfig.HiddenPatterns)
	if err != nil {
		logging.FlushLogs()
		return nil, result, fmt.Errorf("BuildSystem() [bootstrap.go]: failed to build hide patterns: %w", err)
	}

	selectedVCS, selectedVFS, err := prepareSessionVFS(workDir, params.WorktreeBranch, hidePatterns)
	if err != nil {
		logging.FlushLogs()
		return nil, result, fmt.Errorf("BuildSystem() [bootstrap.go]: %w", err)
	}

	effectiveWorkDir := selectedVFS.WorktreePath()

	var lspClient lsp.LSP
	if params.LSPServer != "" {
		logger := logging.GetGlobalLogger()
		logger.Debug("lsp_initialization", "enabled", true, "server", params.LSPServer)
		// Check if the LSP server binary exists
		if _, err := os.Stat(params.LSPServer); err != nil {
			logger.Warn("LSP server binary not found, continuing without LSP", "server", params.LSPServer, "error", err)
		} else {
			client, err := lsp.NewClient(params.LSPServer, effectiveWorkDir)
			if err != nil {
				logger.Warn("failed to create LSP client, continuing without LSP", "error", err)
			} else if err := client.Init(false); err != nil {
				logger.Warn("failed to initialize LSP client, continuing without LSP", "error", err)
			} else {
				lspClient = client
				logger.Debug("lsp_initialized", "server", params.LSPServer)
			}
		}
	}

	toolRegistry := tool.NewToolRegistry()
	tool.RegisterVFSTools(toolRegistry, selectedVFS, lspClient, nil)

	bashRunner := runner.CommandRunner(runner.NewBashRunner(effectiveWorkDir, 0))
	cleanupFn := func() {}

	if params.ContainerImage != "" {
		uid, gid, err := resolveCurrentUserIDs(effectiveWorkDir)
		if err != nil {
			logging.FlushLogs()
			return nil, result, fmt.Errorf("BuildSystem() [bootstrap.go]: failed to resolve current user ids: %w", err)
		}

		gitAuthorName, gitAuthorEmail := resolveContainerGitAuthorIdentity()

		containerRunner, err := runner.NewContainerRunner(runner.ContainerConfig{
			ImageName:      params.ContainerImage,
			Workdir:        effectiveWorkDir,
			MountDirs:      map[string]string{effectiveWorkDir: effectiveWorkDir},
			UID:            uid,
			GID:            gid,
			Env:            map[string]string{"GIT_AUTHOR_NAME": gitAuthorName, "GIT_AUTHOR_EMAIL": gitAuthorEmail},
			ReadOnlyMounts: false,
		})
		if err != nil {
			logging.FlushLogs()
			return nil, result, fmt.Errorf("BuildSystem() [bootstrap.go]: failed to create container runner: %w", err)
		}

		bashRunner = containerRunner
		cleanupFn = func() {
			if closeErr := containerRunner.Close(); closeErr != nil {
				logger := logging.GetGlobalLogger()
				logger.Warn("failed to close container runner", "error", closeErr)
			}
		}
	}

	tool.RegisterRunBashTool(toolRegistry, bashRunner, roleConfig.RunPrivileges)

	promptGenerator, err := core.NewConfPromptGenerator(configStore, selectedVFS)
	if err != nil {
		logging.FlushLogs()
		return nil, result, fmt.Errorf("BuildSystem() [bootstrap.go]: failed to create prompt generator: %w", err)
	}

	modelTagRegistry, err := CreateModelTagRegistry(configStore, providerRegistry)
	if err != nil {
		logging.FlushLogs()
		return nil, result, fmt.Errorf("BuildSystem() [bootstrap.go]: %w", err)
	}

	globalConfig, err := configStore.GetGlobalConfig()
	if err != nil {
		logging.FlushLogs()
		return nil, result, fmt.Errorf("BuildSystem() [bootstrap.go]: failed to load global config: %w", err)
	}

	sweSystem := &core.SweSystem{
		ModelProviders:  modelProviders,
		ModelTags:       modelTagRegistry,
		ToolSelection:   globalConfig.ToolSelection,
		PromptGenerator: promptGenerator,
		Tools:           toolRegistry,
		VFS:             selectedVFS,
		Roles:           roleRegistry,
		LSP:             lspClient,
		ConfigStore:     configStore,
		LogBaseDir:      logsDir,
		WorkDir:         effectiveWorkDir,
		LogLLMRequests:  params.LogLLMRequests,
		Thinking:        params.Thinking,
	}

	result = BuildSystemResult{
		WorkDir:          effectiveWorkDir,
		RoleConfig:       roleConfig,
		ModelName:        modelName,
		ConfigStore:      configStore,
		ProviderRegistry: providerRegistry,
		LogsDir:          logsDir,
		VCS:              selectedVCS,
		WorktreeBranch:   params.WorktreeBranch,
		LSPStarted:       lspClient != nil,
		LSPWorkDir:       effectiveWorkDir,
		Cleanup:          cleanupFn,
	}

	return sweSystem, result, nil
}

func resolveCurrentUserIDs(workDir string) (int, int, error) {
	if workDir != "" {
		fileInfo, err := os.Stat(workDir)
		if err == nil {
			stat, ok := fileInfo.Sys().(*syscall.Stat_t)
			if !ok {
				return 0, 0, fmt.Errorf("resolveCurrentUserIDs() [bootstrap.go]: failed to read stat info for workdir: %s", workDir)
			}

			return int(stat.Uid), int(stat.Gid), nil
		}
	}

	currentUser, err := user.Current()
	if err != nil {
		return 0, 0, fmt.Errorf("resolveCurrentUserIDs() [bootstrap.go]: failed to get current user: %w", err)
	}

	uid, err := strconv.Atoi(currentUser.Uid)
	if err != nil {
		return 0, 0, fmt.Errorf("resolveCurrentUserIDs() [bootstrap.go]: failed to parse uid: %w", err)
	}

	gid, err := strconv.Atoi(currentUser.Gid)
	if err != nil {
		return 0, 0, fmt.Errorf("resolveCurrentUserIDs() [bootstrap.go]: failed to parse gid: %w", err)
	}

	return uid, gid, nil
}

// resolveContainerGitAuthorIdentity returns git author identity for container mode.
// It uses host git config values when git is available, otherwise default fallback values.
func resolveContainerGitAuthorIdentity() (string, string) {
	name := defaultGitAuthorName
	email := defaultGitAuthorEmail

	if _, err := gitLookPathFunc("git"); err != nil {
		return name, email
	}

	resolvedName, err := gitConfigValueFunc("user.name")
	if err == nil && resolvedName != "" {
		name = resolvedName
	}

	resolvedEmail, err := gitConfigValueFunc("user.email")
	if err == nil && resolvedEmail != "" {
		email = resolvedEmail
	}

	return name, email
}

// readGitConfigValue reads a single git configuration key from host git config.
func readGitConfigValue(key string) (string, error) {
	cmd := exec.Command("git", "config", "--get", key)
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("readGitConfigValue() [bootstrap.go]: failed to read git config key %q: %w", key, err)
	}

	return strings.TrimSpace(string(output)), nil
}
