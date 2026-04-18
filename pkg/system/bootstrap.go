package system

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/rlewczuk/csw/pkg/apis"
	"github.com/rlewczuk/csw/pkg/conf"
	"github.com/rlewczuk/csw/pkg/conf/impl"
	"github.com/rlewczuk/csw/pkg/core"
	"github.com/rlewczuk/csw/pkg/logging"
	"github.com/rlewczuk/csw/pkg/lsp"
	"github.com/rlewczuk/csw/pkg/models"
	"github.com/rlewczuk/csw/pkg/runner"
	"github.com/rlewczuk/csw/pkg/tool"
	"github.com/rlewczuk/csw/pkg/vcs"
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
var gitConfigValueFunc = vcs.ReadGitConfigValue

var newCompositeConfigStoreFunc = impl.NewCompositeConfigStore
var resolveModelNameFunc = ResolveModelName
var createProviderMapFunc = CreateProviderMap
var generateWorktreeBranchNameFunc = core.GenerateWorktreeBranchName
var createConfigUpdaterFunc = models.NewConfigUpdater

// SetNewCompositeConfigStoreFuncForTest overrides composite store constructor in tests.
func SetNewCompositeConfigStoreFuncForTest(fn func(projectRoot string, configPath string) (conf.ConfigStore, error)) {
	newCompositeConfigStoreFunc = fn
}

// NewCompositeConfigStoreFuncForTest returns current composite store constructor.
func NewCompositeConfigStoreFuncForTest() func(projectRoot string, configPath string) (conf.ConfigStore, error) {
	return newCompositeConfigStoreFunc
}

// SetResolveModelNameFuncForTest overrides model name resolver in tests.
func SetResolveModelNameFuncForTest(fn func(modelName string, configStore conf.ConfigStore, providerRegistry *models.ProviderRegistry) (string, error)) {
	resolveModelNameFunc = fn
}

// ResolveModelNameFuncForTest returns current model name resolver.
func ResolveModelNameFuncForTest() func(modelName string, configStore conf.ConfigStore, providerRegistry *models.ProviderRegistry) (string, error) {
	return resolveModelNameFunc
}

// SetCreateProviderMapFuncForTest overrides provider map builder in tests.
func SetCreateProviderMapFuncForTest(fn func(providerRegistry *models.ProviderRegistry) (map[string]models.ModelProvider, error)) {
	createProviderMapFunc = fn
}

// CreateProviderMapFuncForTest returns current provider map builder.
func CreateProviderMapFuncForTest() func(providerRegistry *models.ProviderRegistry) (map[string]models.ModelProvider, error) {
	return createProviderMapFunc
}

// SetGenerateWorktreeBranchNameFuncForTest overrides branch name generator in tests.
func SetGenerateWorktreeBranchNameFuncForTest(fn func(ctx context.Context, modelProviders map[string]models.ModelProvider, configStore conf.ConfigStore, model string, inputPrompt string) (string, error)) {
	generateWorktreeBranchNameFunc = fn
}

// GenerateWorktreeBranchNameFuncForTest returns current branch name generator.
func GenerateWorktreeBranchNameFuncForTest() func(ctx context.Context, modelProviders map[string]models.ModelProvider, configStore conf.ConfigStore, model string, inputPrompt string) (string, error) {
	return generateWorktreeBranchNameFunc
}

// BuildSystemParams contains inputs for constructing a SweSystem.
type BuildSystemParams struct {
	WorkDir           string
	ShadowDir         string
	ConfigPath        string
	ProjectConfig     string
	ModelName         string
	RoleName          string
	WorktreeBranch    string
	GitUserName       string
	GitUserEmail      string
	ContainerEnabled  bool
	ContainerDisabled bool
	ContainerImage    string
	ContainerMounts   []string
	ContainerEnv      []string
	LSPServer         string
	LogLLMRequests    bool
	LogLLMRequestsRaw bool
	NoRefresh         bool
	// Thinking controls the thinking/reasoning mode for LLM requests.
	// Values like "low", "medium", "high", "xhigh" for effort-based thinking,
	// or "true"/"false" for boolean thinking modes.
	Thinking string
	// BashRunTimeout sets default timeout for runBash tool command execution.
	BashRunTimeout time.Duration
	// AllowedPaths specifies additional absolute paths outside of workDir that VFS can access.
	// Paths must be absolute. When accessing files via allowedPaths, the path must be absolute
	// and point within one of these directories.
	AllowedPaths []string
	// MaxToolThreads overrides max parallel tool executions. When <=0, value from config is used.
	MaxToolThreads int
	// AllowAllPermissions forces all tool permission checks to allow mode for this run.
	AllowAllPermissions bool
}

// BuildSystemResult contains outputs from building a SweSystem.
type BuildSystemResult struct {
	WorkDir               string
	WorkDirRoot           string
	ShadowDir             string
	RoleConfig            conf.AgentRoleConfig
	ModelName             string
	ConfigStore           conf.ConfigStore
	ProviderRegistry      *models.ProviderRegistry
	LogsDir               string
	VCS                   apis.VCS
	WorktreeBranch        string
	LSPServer             string
	ShellRunner           runner.CommandRunner
	HostShellRunner       runner.CommandRunner
	ContainerImage        string
	ContainerImageName    string
	ContainerImageTag     string
	ContainerImageVersion string
	ContainerIdentity     runner.ContainerIdentity
	LSPStarted            bool
	LSPWorkDir            string
	Cleanup               func()
}

// ResolveRunDefaultsParams contains inputs for resolving run command defaults.
type ResolveRunDefaultsParams struct {
	WorkDir       string
	ShadowDir     string
	ProjectConfig string
	ConfigPath    string
}

// ResolveWorktreeBranchNameParams contains inputs for resolving dynamic worktree branch names.
type ResolveWorktreeBranchNameParams struct {
	Prompt         string
	ModelName      string
	WorkDir        string
	ShadowDir      string
	ProjectConfig  string
	ConfigPath     string
	WorktreeBranch string
}

// ResolveRunDefaults resolves run command defaults from effective global config.
func ResolveRunDefaults(params ResolveRunDefaultsParams) (conf.RunDefaultsConfig, error) {
	var defaults conf.RunDefaultsConfig

	resolvedWorkDir, err := ResolveWorkDir(params.WorkDir)
	if err != nil {
		return defaults, fmt.Errorf("ResolveRunDefaults() [bootstrap.go]: failed to resolve work directory: %w", err)
	}

	configPathStr, err := BuildConfigPath(params.ProjectConfig, params.ConfigPath)
	if err != nil {
		return defaults, fmt.Errorf("ResolveRunDefaults() [bootstrap.go]: failed to build config path: %w", err)
	}

	configRoot := resolvedWorkDir
	if strings.TrimSpace(params.ShadowDir) != "" {
		resolvedShadowDir, shadowErr := ResolveWorkDir(params.ShadowDir)
		if shadowErr != nil {
			return defaults, fmt.Errorf("ResolveRunDefaults() [bootstrap.go]: failed to resolve shadow directory: %w", shadowErr)
		}
		configRoot = resolvedShadowDir
	}

	configStore, err := newCompositeConfigStoreFunc(configRoot, configPathStr)
	if err != nil {
		return defaults, fmt.Errorf("ResolveRunDefaults() [bootstrap.go]: failed to create config store: %w", err)
	}

	globalConfig, err := configStore.GetGlobalConfig()
	if err != nil {
		return defaults, fmt.Errorf("ResolveRunDefaults() [bootstrap.go]: failed to load global config: %w", err)
	}

	if globalConfig == nil {
		return defaults, nil
	}

	return globalConfig.Defaults, nil
}

// ResolveWorktreeBranchName resolves a worktree branch placeholder that ends with '%'.
func ResolveWorktreeBranchName(ctx context.Context, params ResolveWorktreeBranchNameParams) (string, error) {
	if params.WorktreeBranch == "" || !strings.HasSuffix(params.WorktreeBranch, "%") {
		return params.WorktreeBranch, nil
	}

	if strings.TrimSpace(params.Prompt) == "" {
		return "", fmt.Errorf("ResolveWorktreeBranchName() [bootstrap.go]: --worktree ending with %% requires non-empty prompt")
	}

	prefix := strings.TrimSuffix(params.WorktreeBranch, "%")
	resolvedWorkDir, err := ResolveWorkDir(params.WorkDir)
	if err != nil {
		return "", fmt.Errorf("ResolveWorktreeBranchName() [bootstrap.go]: failed to resolve work directory: %w", err)
	}

	configPathStr, err := BuildConfigPath(params.ProjectConfig, params.ConfigPath)
	if err != nil {
		return "", fmt.Errorf("ResolveWorktreeBranchName() [bootstrap.go]: failed to build config path: %w", err)
	}

	configRoot := resolvedWorkDir
	if strings.TrimSpace(params.ShadowDir) != "" {
		resolvedShadowDir, shadowErr := ResolveWorkDir(params.ShadowDir)
		if shadowErr != nil {
			return "", fmt.Errorf("ResolveWorktreeBranchName() [bootstrap.go]: failed to resolve shadow directory: %w", shadowErr)
		}
		configRoot = resolvedShadowDir
	}

	configStore, err := newCompositeConfigStoreFunc(configRoot, configPathStr)
	if err != nil {
		return "", fmt.Errorf("ResolveWorktreeBranchName() [bootstrap.go]: failed to create config store: %w", err)
	}

	providerRegistry := models.NewProviderRegistry(configStore)
	if len(providerRegistry.List()) == 0 {
		return "", fmt.Errorf("ResolveWorktreeBranchName() [bootstrap.go]: no model providers found in config")
	}

	resolvedModelName, err := resolveModelNameFunc(params.ModelName, configStore, providerRegistry)
	if err != nil {
		return "", fmt.Errorf("ResolveWorktreeBranchName() [bootstrap.go]: failed to resolve model name: %w", err)
	}

	modelProviders, err := createProviderMapFunc(providerRegistry)
	if err != nil {
		return "", fmt.Errorf("ResolveWorktreeBranchName() [bootstrap.go]: failed to create provider map: %w", err)
	}

	branchSuffix, err := generateWorktreeBranchNameFunc(ctx, modelProviders, configStore, resolvedModelName, params.Prompt)
	if err != nil {
		return "", fmt.Errorf("ResolveWorktreeBranchName() [bootstrap.go]: failed to generate branch name: %w", err)
	}

	return prefix + branchSuffix, nil
}

// PrepareSessionVFS creates session VCS/VFS with optional worktree handling.
func PrepareSessionVFS(workDir string, worktreesBaseDir string, worktreeBranch string, hidePatterns []string, gitUserName string, gitUserEmail string, allowedPaths []string) (apis.VCS, apis.VFS, error) {
	localVFS, err := vfs.NewLocalVFS(workDir, hidePatterns, allowedPaths)
	if err != nil {
		return nil, nil, fmt.Errorf("prepareSessionVFS() [bootstrap.go]: failed to create local VFS: %w", err)
	}

	nullVCS, err := vcs.NewNullVFS(localVFS)
	if err != nil {
		return nil, nil, fmt.Errorf("prepareSessionVFS() [bootstrap.go]: failed to create NullVCS: %w", err)
	}

	var selectedVCS apis.VCS = nullVCS

	if worktreeBranch != "" {
		worktreesRoot := filepath.Join(worktreesBaseDir, ".cswdata", "work")
		gitRepo, err := vcs.NewGitRepo(workDir, worktreesRoot, hidePatterns, allowedPaths, gitUserName, gitUserEmail)
		if err != nil {
			return nil, nil, fmt.Errorf("prepareSessionVFS() [bootstrap.go]: failed to create GitVCS: %w", err)
		}

		if err := os.RemoveAll(filepath.Join(worktreesRoot, worktreeBranch)); err != nil {
			return nil, nil, fmt.Errorf("prepareSessionVFS() [bootstrap.go]: failed to remove existing worktree path: %w", err)
		}

		if err := gitRepo.DropWorktree(worktreeBranch); err != nil && !errors.Is(err, apis.ErrFileNotFound) {
			return nil, nil, fmt.Errorf("prepareSessionVFS() [bootstrap.go]: failed to drop existing worktree: %w", err)
		}

		if err := gitRepo.DeleteBranch(worktreeBranch); err != nil && !errors.Is(err, apis.ErrFileNotFound) {
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
func BuildSystem(params BuildSystemParams) (*SweSystem, BuildSystemResult, error) {
	var result BuildSystemResult

	workDir, err := ResolveWorkDir(params.WorkDir)
	if err != nil {
		return nil, result, fmt.Errorf("BuildSystem() [bootstrap.go]: %w", err)
	}

	shadowDir := ""
	if strings.TrimSpace(params.ShadowDir) != "" {
		shadowDir, err = ResolveWorkDir(params.ShadowDir)
		if err != nil {
			return nil, result, fmt.Errorf("BuildSystem() [bootstrap.go]: failed to resolve shadow directory: %w", err)
		}
	}

	configRoot := workDir
	if shadowDir != "" {
		configRoot = shadowDir
	}

	logsDir := filepath.Join(configRoot, ".cswdata", "logs")
	if err := logging.SetLogsDirectory(logsDir, true); err != nil {
		return nil, result, fmt.Errorf("BuildSystem() [bootstrap.go]: failed to initialize logging: %w", err)
	}

	configPathStr, err := BuildConfigPath(params.ProjectConfig, params.ConfigPath)
	if err != nil {
		logging.FlushLogs()
		return nil, result, fmt.Errorf("BuildSystem() [bootstrap.go]: %w", err)
	}

	configStore, err := impl.NewCompositeConfigStore(configRoot, configPathStr)
	if err != nil {
		logging.FlushLogs()
		return nil, result, fmt.Errorf("BuildSystem() [bootstrap.go]: failed to create config store: %w", err)
	}

	globalConfig, err := configStore.GetGlobalConfig()
	if err != nil {
		logging.FlushLogs()
		return nil, result, fmt.Errorf("BuildSystem() [bootstrap.go]: failed to load global config: %w", err)
	}

	shadowPatterns := append([]string(nil), globalConfig.ShadowPaths...)
	if len(shadowPatterns) == 0 {
		shadowPatterns = vfs.DefaultShadowPatterns()
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
	if params.NoRefresh {
		applyDisableRefreshToProviders(modelProviders)
	}

	modelAliasValues, err := configStore.GetModelAliases()
	if err != nil {
		logging.FlushLogs()
		return nil, result, fmt.Errorf("BuildSystem() [bootstrap.go]: failed to load model aliases: %w", err)
	}
	modelAliases, err := models.NormalizeModelAliasMap(modelAliasValues)
	if err != nil {
		logging.FlushLogs()
		return nil, result, fmt.Errorf("BuildSystem() [bootstrap.go]: failed to normalize model aliases: %w", err)
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

	if strings.TrimSpace(params.ModelName) == "" {
		if strings.TrimSpace(roleConfig.Model) != "" {
			modelName, err = ResolveModelSpec(roleConfig.Model, configStore)
			if err != nil {
				logging.FlushLogs()
				return nil, result, fmt.Errorf("BuildSystem() [bootstrap.go]: failed to resolve role model: %w", err)
			}
		}
	}

	hidePatterns, err := vfs.BuildHidePatterns(workDir, roleConfig.HiddenPatterns)
	if err != nil {
		logging.FlushLogs()
		return nil, result, fmt.Errorf("BuildSystem() [bootstrap.go]: failed to build hide patterns: %w", err)
	}

	allowedPaths := append([]string(nil), params.AllowedPaths...)
	if shadowDir != "" {
		allowedPaths = append(allowedPaths, shadowDir)
	}

	selectedVCS, selectedVFS, err := PrepareSessionVFS(workDir, configRoot, params.WorktreeBranch, hidePatterns, params.GitUserName, params.GitUserEmail, allowedPaths)
	if err != nil {
		logging.FlushLogs()
		return nil, result, fmt.Errorf("BuildSystem() [bootstrap.go]: %w", err)
	}

	effectiveWorkDir := selectedVFS.WorktreePath()

	toolVFS := selectedVFS
	if shadowDir != "" {
		shadowLocalVFS, shadowErr := vfs.NewLocalVFS(shadowDir, hidePatterns, allowedPaths)
		if shadowErr != nil {
			logging.FlushLogs()
			return nil, result, fmt.Errorf("BuildSystem() [bootstrap.go]: failed to create shadow local VFS: %w", shadowErr)
		}
		shadowOverlay, overlayErr := vfs.NewShadowVFS(selectedVFS, shadowLocalVFS, shadowPatterns)
		if overlayErr != nil {
			logging.FlushLogs()
			return nil, result, fmt.Errorf("BuildSystem() [bootstrap.go]: failed to create shadow VFS overlay: %w", overlayErr)
		}
		toolVFS = shadowOverlay
	}

	var lspClient lsp.LSP
	if params.LSPServer != "" {
		logger := logging.GetGlobalLogger()
		logger.Debug("lsp_initialization", "enabled", true, "server", params.LSPServer)
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
	tool.RegisterVFSTools(toolRegistry, toolVFS, lspClient, nil)

	taskManager, err := core.NewTaskManagerWithTasksDir(workDir, ".cswdata/tasks", configStore)
	if err != nil {
		logging.FlushLogs()
		return nil, result, fmt.Errorf("BuildSystem() [bootstrap.go]: failed to create task manager: %w", err)
	}
	taskBackend, err := core.NewTaskBackendAdapter(taskManager, selectedVCS, nil)
	if err != nil {
		logging.FlushLogs()
		return nil, result, fmt.Errorf("BuildSystem() [bootstrap.go]: failed to create task backend: %w", err)
	}

	bashRunner := runner.CommandRunner(runner.NewBashRunner(effectiveWorkDir, params.BashRunTimeout))
	cleanupFns := make([]func(), 0)
	cleanupOnError := true
	defer func() {
		if !cleanupOnError {
			return
		}
		for _, cleanupFn := range cleanupFns {
			cleanupFn()
		}
	}()

	containerRuntimeConfig, err := ResolveContainerRuntimeConfig(globalConfig, params, effectiveWorkDir, shadowDir)
	if err != nil {
		logging.FlushLogs()
		return nil, result, fmt.Errorf("BuildSystem() [bootstrap.go]: failed to resolve container config: %w", err)
	}

	if containerRuntimeConfig.Enabled {
		logger := logging.GetGlobalLogger()
		containerUser, err := resolveCurrentUserIdentity()
		if err != nil {
			logging.FlushLogs()
			return nil, result, fmt.Errorf("BuildSystem() [bootstrap.go]: failed to resolve current user identity: %w", err)
		}

		gitAuthorName, gitAuthorEmail := ResolveContainerGitAuthorIdentity()
		containerEnv := copyStringMap(containerRuntimeConfig.Env)
		if containerEnv == nil {
			containerEnv = make(map[string]string)
		}
		if _, exists := containerEnv["GIT_AUTHOR_NAME"]; !exists {
			containerEnv["GIT_AUTHOR_NAME"] = gitAuthorName
		}
		if _, exists := containerEnv["GIT_AUTHOR_EMAIL"]; !exists {
			containerEnv["GIT_AUTHOR_EMAIL"] = gitAuthorEmail
		}

		containerRunner, err := runner.NewLazyContainerRunner(runner.ContainerConfig{
			ImageName:      containerRuntimeConfig.Image,
			Workdir:        effectiveWorkDir,
			MountDirs:      containerRuntimeConfig.Mounts,
			UID:            containerUser.UID,
			GID:            containerUser.GID,
			UserName:       containerUser.UserName,
			GroupName:      containerUser.GroupName,
			HomeDir:        containerUser.HomeDir,
			Env:            containerEnv,
			ReadOnlyMounts: false,
		})
		if err != nil {
			logging.FlushLogs()
			return nil, result, fmt.Errorf("BuildSystem() [bootstrap.go]: failed to create lazy container runner: %w", err)
		}
		containerImageInfo := containerRunner.ImageInfo()
		containerIdentity := containerRunner.Identity()
		logger.Info(
			"container runtime initialized",
			"image", containerImageInfo.Name,
			"tag", containerImageInfo.Tag,
			"version", containerImageInfo.Version,
			"uid", containerIdentity.UID,
			"gid", containerIdentity.GID,
			"user", containerIdentity.UserName,
			"group", containerIdentity.GroupName,
		)

		bashRunner = containerRunner
		cleanupFns = append(cleanupFns, func() {
			if closeErr := containerRunner.Close(); closeErr != nil {
				logger := logging.GetGlobalLogger()
				logger.Warn("failed to close container runner", "error", closeErr)
			}
		})
	}

	tool.RegisterRunBashTool(toolRegistry, bashRunner, roleConfig.RunPrivileges, effectiveWorkDir, params.BashRunTimeout, params.AllowAllPermissions)
	tool.RegisterWebFetchTool(toolRegistry, nil)
	tool.RegisterSkillTool(toolRegistry, configRoot)

	if err := tool.RegisterCustomTools(toolRegistry, configStore, configRoot, bashRunner); err != nil {
		logging.FlushLogs()
		return nil, result, fmt.Errorf("BuildSystem() [bootstrap.go]: failed to register custom tools: %w", err)
	}

	basePromptGenerator, err := core.NewConfPromptGenerator(configStore, toolVFS)
	if err != nil {
		logging.FlushLogs()
		return nil, result, fmt.Errorf("BuildSystem() [bootstrap.go]: failed to create prompt generator: %w", err)
	}
	promptGenerator := basePromptGenerator

	modelTagRegistry, err := CreateModelTagRegistry(configStore, providerRegistry)
	if err != nil {
		logging.FlushLogs()
		return nil, result, fmt.Errorf("BuildSystem() [bootstrap.go]: %w", err)
	}

	sweSystem := &SweSystem{
		ModelProviders:      modelProviders,
		ModelAliases:        modelAliases,
		ModelTags:           modelTagRegistry,
		ToolSelection:       globalConfig.ToolSelection,
		PromptGenerator:     promptGenerator,
		Tools:               toolRegistry,
		VFS:                 toolVFS,
		VCS:                 selectedVCS,
		Roles:               roleRegistry,
		LSP:                 lspClient,
		ConfigStore:         configStore,
		TaskBackend:         taskBackend,
		LogBaseDir:          logsDir,
		WorkDir:             effectiveWorkDir,
		ShadowDir:           shadowDir,
		LogLLMRequests:      params.LogLLMRequests,
		LogLLMRequestsRaw:   params.LogLLMRequestsRaw,
		Thinking:            params.Thinking,
		AllowAllPermissions: params.AllowAllPermissions,
		MaxToolThreads: func() int {
			if params.MaxToolThreads > 0 {
				return params.MaxToolThreads
			}
			return globalConfig.Defaults.MaxThreads
		}(),
	}

	result = BuildSystemResult{
		WorkDir:          effectiveWorkDir,
		WorkDirRoot:      workDir,
		ShadowDir:        shadowDir,
		RoleConfig:       roleConfig,
		ModelName:        modelName,
		ConfigStore:      configStore,
		ProviderRegistry: providerRegistry,
		LogsDir:          logsDir,
		VCS:              selectedVCS,
		WorktreeBranch:   params.WorktreeBranch,
		LSPServer:        params.LSPServer,
		ShellRunner:      bashRunner,
		HostShellRunner:  runner.NewBashRunner(effectiveWorkDir, params.BashRunTimeout),
		ContainerImage:   containerRuntimeConfig.Image,
		LSPStarted:       lspClient != nil,
		LSPWorkDir:       effectiveWorkDir,
		Cleanup: func() {
			for _, cleanupFn := range cleanupFns {
				cleanupFn()
			}
		},
	}
	if containerRuntimeConfig.Enabled {
		containerImageInfo := parseContainerImageInfo(containerRuntimeConfig.Image)
		result.ContainerImageName = containerImageInfo.Name
		result.ContainerImageTag = containerImageInfo.Tag
		result.ContainerImageVersion = containerImageInfo.Version
		if containerRunner, ok := bashRunner.(runner.ContainerRunner); ok {
			result.ContainerIdentity = containerRunner.Identity()
		}
	}
	cleanupOnError = false

	return sweSystem, result, nil
}

// containerRuntimeConfig describes effective container runtime setup.
type containerRuntimeConfig struct {
	Enabled bool
	Image   string
	Mounts  map[string]string
	Env     map[string]string
}

// ResolveContainerRuntimeConfig resolves effective container runtime setup.
func ResolveContainerRuntimeConfig(globalConfig *conf.GlobalConfig, params BuildSystemParams, effectiveWorkDir string, shadowDir string) (containerRuntimeConfig, error) {
	var runtimeConfig containerRuntimeConfig
	containerDefaults := conf.ContainerConfig{}
	if globalConfig != nil && globalConfig.Defaults.Container != nil {
		containerDefaults = globalConfig.Defaults.Container.Clone()
	}

	runtimeConfig.Enabled = containerDefaults.Enabled
	if params.ContainerEnabled {
		runtimeConfig.Enabled = true
	}
	if params.ContainerDisabled {
		runtimeConfig.Enabled = false
	}

	if !runtimeConfig.Enabled {
		return runtimeConfig, nil
	}

	runtimeConfig.Image = strings.TrimSpace(params.ContainerImage)
	if runtimeConfig.Image == "" {
		runtimeConfig.Image = strings.TrimSpace(containerDefaults.Image)
	}
	if runtimeConfig.Image == "" {
		return runtimeConfig, fmt.Errorf("resolveContainerRuntimeConfig() [bootstrap.go]: container image is required when container mode is enabled")
	}

	mountSpecs := make([]string, 0, len(containerDefaults.Mounts)+len(params.ContainerMounts))
	mountSpecs = append(mountSpecs, containerDefaults.Mounts...)
	mountSpecs = append(mountSpecs, params.ContainerMounts...)
	runtimeConfig.Mounts = map[string]string{effectiveWorkDir: effectiveWorkDir}
	if strings.TrimSpace(shadowDir) != "" {
		runtimeConfig.Mounts[shadowDir] = shadowDir
	}
	for _, mountSpec := range mountSpecs {
		hostPath, containerPath, err := ParseContainerMountSpec(mountSpec)
		if err != nil {
			return runtimeConfig, err
		}
		if !filepath.IsAbs(hostPath) {
			hostPath, err = filepath.Abs(hostPath)
			if err != nil {
				return runtimeConfig, fmt.Errorf("resolveContainerRuntimeConfig() [bootstrap.go]: failed to resolve absolute mount host path %q: %w", hostPath, err)
			}
		}
		if _, err := os.Stat(hostPath); err != nil {
			return runtimeConfig, fmt.Errorf("resolveContainerRuntimeConfig() [bootstrap.go]: invalid mount host path %q: %w", hostPath, err)
		}
		runtimeConfig.Mounts[containerPath] = hostPath
	}

	envSpecs := make([]string, 0, len(containerDefaults.Env)+len(params.ContainerEnv))
	envSpecs = append(envSpecs, containerDefaults.Env...)
	envSpecs = append(envSpecs, params.ContainerEnv...)
	if len(envSpecs) > 0 {
		runtimeConfig.Env = make(map[string]string, len(envSpecs))
		for _, envSpec := range envSpecs {
			key, value, err := ParseContainerEnvSpec(envSpec)
			if err != nil {
				return runtimeConfig, err
			}
			runtimeConfig.Env[key] = value
		}
	}

	return runtimeConfig, nil
}

// ParseContainerMountSpec parses mount in host_path:container_path format.
func ParseContainerMountSpec(mountSpec string) (string, string, error) {
	parts := strings.SplitN(mountSpec, ":", 2)
	if len(parts) != 2 {
		return "", "", fmt.Errorf("parseContainerMountSpec() [bootstrap.go]: mount must be in host_path:container_path format: %q", mountSpec)
	}
	hostPath := strings.TrimSpace(parts[0])
	containerPath := strings.TrimSpace(parts[1])
	if hostPath == "" || containerPath == "" {
		return "", "", fmt.Errorf("parseContainerMountSpec() [bootstrap.go]: mount must be in host_path:container_path format: %q", mountSpec)
	}

	return hostPath, containerPath, nil
}

// ParseContainerEnvSpec parses env var in KEY=VALUE format.
func ParseContainerEnvSpec(envSpec string) (string, string, error) {
	parts := strings.SplitN(envSpec, "=", 2)
	if len(parts) != 2 {
		return "", "", fmt.Errorf("parseContainerEnvSpec() [bootstrap.go]: env must be in KEY=VALUE format: %q", envSpec)
	}
	key := strings.TrimSpace(parts[0])
	if key == "" {
		return "", "", fmt.Errorf("parseContainerEnvSpec() [bootstrap.go]: env key cannot be empty: %q", envSpec)
	}

	return key, parts[1], nil
}

func copyStringMap(values map[string]string) map[string]string {
	if len(values) == 0 {
		return nil
	}

	cloned := make(map[string]string, len(values))
	for key, value := range values {
		cloned[key] = value
	}

	return cloned
}

func parseContainerImageInfo(reference string) runner.ContainerImageInfo {
	trimmed := strings.TrimSpace(reference)
	info := runner.ContainerImageInfo{
		Reference: trimmed,
		Name:      trimmed,
		Tag:       "latest",
		Version:   "latest",
	}
	if trimmed == "" {
		return info
	}

	name := trimmed
	tag := "latest"
	lastColon := strings.LastIndex(trimmed, ":")
	lastSlash := strings.LastIndex(trimmed, "/")
	if lastColon > lastSlash {
		name = trimmed[:lastColon]
		tag = trimmed[lastColon+1:]
	}
	if strings.TrimSpace(tag) == "" {
		tag = "latest"
	}

	info.Name = name
	info.Tag = tag
	info.Version = tag
	return info
}

// ContainerUserIdentity stores host user identity mirrored in container mode.
type ContainerUserIdentity struct {
	UID       int
	GID       int
	UserName  string
	GroupName string
	HomeDir   string
}

func resolveCurrentUserIdentity() (ContainerUserIdentity, error) {
	var identity ContainerUserIdentity

	currentUser, err := user.Current()
	if err != nil {
		return identity, fmt.Errorf("resolveCurrentUserIdentity() [bootstrap.go]: failed to get current user: %w", err)
	}

	uid, err := strconv.Atoi(currentUser.Uid)
	if err != nil {
		return identity, fmt.Errorf("resolveCurrentUserIdentity() [bootstrap.go]: failed to parse uid: %w", err)
	}

	gid, err := strconv.Atoi(currentUser.Gid)
	if err != nil {
		return identity, fmt.Errorf("resolveCurrentUserIdentity() [bootstrap.go]: failed to parse gid: %w", err)
	}

	group, err := user.LookupGroupId(currentUser.Gid)
	if err != nil {
		return identity, fmt.Errorf("resolveCurrentUserIdentity() [bootstrap.go]: failed to lookup group by gid: %w", err)
	}

	if currentUser.Username == "" {
		return identity, fmt.Errorf("resolveCurrentUserIdentity() [bootstrap.go]: current user name is empty")
	}

	if currentUser.HomeDir == "" {
		return identity, fmt.Errorf("resolveCurrentUserIdentity() [bootstrap.go]: current user home directory is empty")
	}

	if group.Name == "" {
		return identity, fmt.Errorf("resolveCurrentUserIdentity() [bootstrap.go]: current user group name is empty")
	}

	identity.UID = uid
	identity.GID = gid
	identity.UserName = currentUser.Username
	identity.GroupName = group.Name
	identity.HomeDir = currentUser.HomeDir

	return identity, nil
}

// resolveContainerGitAuthorIdentity returns git author identity for container mode.
// It uses host git config values when git is available, otherwise default fallback values.
// ResolveContainerGitAuthorIdentity returns git author identity for container mode.
func ResolveContainerGitAuthorIdentity() (string, string) {
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

// BuildConfigPath builds a config path hierarchy string from base and optional custom paths.
func BuildConfigPath(projectConfig, customConfigPath string) (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("BuildConfigPath() [bootstrap.go]: failed to get user home directory: %w", err)
	}

	projectConfigPath := "@PROJ/.csw/config"
	if projectConfig != "" {
		info, err := os.Stat(projectConfig)
		if err != nil {
			if os.IsNotExist(err) {
				return "", fmt.Errorf("BuildConfigPath() [bootstrap.go]: project config directory does not exist: %s", projectConfig)
			}
			return "", fmt.Errorf("BuildConfigPath() [bootstrap.go]: failed to access project config directory %s: %w", projectConfig, err)
		}
		if !info.IsDir() {
			return "", fmt.Errorf("BuildConfigPath() [bootstrap.go]: project config path is not a directory: %s", projectConfig)
		}
		projectConfigPath = projectConfig
	}

	configPathStr := "@DEFAULTS:" + filepath.Join(homeDir, ".config", "csw") + ":" + projectConfigPath

	if customConfigPath != "" {
		if err := ValidateConfigPaths(customConfigPath); err != nil {
			return "", err
		}
		configPathStr = configPathStr + ":" + customConfigPath
	}

	return configPathStr, nil
}

// ValidateConfigPaths validates that all paths in a colon-separated string exist and are directories.
func ValidateConfigPaths(configPath string) error {
	pathComponents := filepath.SplitList(configPath)
	for _, pathComponent := range pathComponents {
		if pathComponent == "" {
			continue
		}
		info, err := os.Stat(pathComponent)
		if err != nil {
			if os.IsNotExist(err) {
				return fmt.Errorf("ValidateConfigPaths() [bootstrap.go]: config path does not exist: %s", pathComponent)
			}
			return fmt.Errorf("ValidateConfigPaths() [bootstrap.go]: failed to access config path %s: %w", pathComponent, err)
		}
		if !info.IsDir() {
			return fmt.Errorf("ValidateConfigPaths() [bootstrap.go]: config path is not a directory: %s", pathComponent)
		}
	}
	return nil
}

// ResolveWorkDir resolves the working directory from an optional path argument.
func ResolveWorkDir(dirPath string) (string, error) {
	if dirPath == "" {
		wd, err := os.Getwd()
		if err != nil {
			return "", fmt.Errorf("ResolveWorkDir() [bootstrap.go]: failed to get current working directory: %w", err)
		}
		projectDir := findNearestProjectDir(wd)
		return projectDir, nil
	}

	absPath, err := filepath.Abs(dirPath)
	if err != nil {
		return "", fmt.Errorf("ResolveWorkDir() [bootstrap.go]: failed to resolve directory path: %w", err)
	}
	info, err := os.Stat(absPath)
	if err != nil {
		return "", fmt.Errorf("ResolveWorkDir() [bootstrap.go]: failed to access directory: %w", err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("ResolveWorkDir() [bootstrap.go]: path is not a directory: %s", dirPath)
	}
	return absPath, nil
}

// findNearestProjectDir searches upwards from startDir for a directory containing .csw or .cswdata.
func findNearestProjectDir(startDir string) string {
	currentDir := startDir
	for {
		if hasProjectMarker(currentDir) {
			return currentDir
		}

		parentDir := filepath.Dir(currentDir)
		if parentDir == currentDir {
			return startDir
		}

		currentDir = parentDir
	}
}

// hasProjectMarker returns true when dirPath contains .csw or .cswdata directory.
func hasProjectMarker(dirPath string) bool {
	projectMarkers := []string{".csw", ".cswdata"}
	for _, marker := range projectMarkers {
		markerPath := filepath.Join(dirPath, marker)
		markerInfo, err := os.Stat(markerPath)
		if err != nil {
			continue
		}
		if markerInfo.IsDir() {
			return true
		}
	}

	return false
}

// ResolveModelName determines the model name to use.
func ResolveModelName(modelName string, configStore conf.ConfigStore, providerRegistry *models.ProviderRegistry) (string, error) {
	if modelName != "" {
		return ResolveModelSpec(modelName, configStore)
	}

	globalConfig, err := configStore.GetGlobalConfig()
	if err != nil {
		return "", fmt.Errorf("ResolveModelName() [bootstrap.go]: failed to get global config: %w", err)
	}

	if globalConfig.Defaults.DefaultProvider != "" {
		return globalConfig.Defaults.DefaultProvider + "/default", nil
	}

	providers := providerRegistry.List()
	if len(providers) > 0 {
		return providers[0] + "/default", nil
	}

	return "", fmt.Errorf("ResolveModelName() [bootstrap.go]: no default provider configured and no providers available")
}

// ResolveModelSpec resolves model alias or provider/model chain to normalized provider/model chain.
func ResolveModelSpec(modelSpec string, configStore conf.ConfigStore) (string, error) {
	trimmedModelSpec := strings.TrimSpace(modelSpec)
	if trimmedModelSpec == "" {
		return "", fmt.Errorf("ResolveModelSpec() [bootstrap.go]: model spec cannot be empty")
	}

	if configStore == nil {
		refs, err := models.ParseProviderModelChain(trimmedModelSpec)
		if err != nil {
			return "", fmt.Errorf("ResolveModelSpec() [bootstrap.go]: %w", err)
		}
		return models.ComposeProviderModelSpec(refs), nil
	}

	aliasValues, err := configStore.GetModelAliases()
	if err != nil {
		return "", fmt.Errorf("ResolveModelSpec() [bootstrap.go]: failed to load model aliases: %w", err)
	}
	aliases, err := models.NormalizeModelAliasMap(aliasValues)
	if err != nil {
		return "", fmt.Errorf("ResolveModelSpec() [bootstrap.go]: failed to normalize model aliases: %w", err)
	}

	refs, err := models.ExpandProviderModelChain(trimmedModelSpec, aliases)
	if err != nil {
		return "", fmt.Errorf("ResolveModelSpec() [bootstrap.go]: %w", err)
	}

	return models.ComposeProviderModelSpec(refs), nil
}

// CreateProviderMap creates a map of provider names to ModelProvider instances from a registry.
func CreateProviderMap(providerRegistry *models.ProviderRegistry) (map[string]models.ModelProvider, error) {
	modelProviders := make(map[string]models.ModelProvider)
	configStore := providerRegistry.ConfigStore()
	providerConfigPresence := make(map[string]struct{})
	if configStore != nil {
		var err error
		providerConfigPresence, err = resolveProviderConfigPresence(configStore)
		if err != nil {
			return nil, err
		}
	}

	for _, name := range providerRegistry.List() {
		provider, err := providerRegistry.Get(name)
		if err != nil {
			return nil, fmt.Errorf("CreateProviderMap() [bootstrap.go]: failed to get provider %s: %w", name, err)
		}

		if updaterTarget, ok := provider.(interface{ SetConfigUpdater(models.ConfigUpdater) }); ok {
			if _, exists := providerConfigPresence[name]; exists {
				updater := createConfigUpdaterFunc(name)
				updaterTarget.SetConfigUpdater(updater.Update())
			}
		}
		modelProviders[name] = provider
	}
	return modelProviders, nil
}

func applyDisableRefreshToProviders(modelProviders map[string]models.ModelProvider) {
	for _, provider := range modelProviders {
		providerConfigAccessor, ok := provider.(interface {
			GetConfig() *conf.ModelProviderConfig
		})
		if !ok {
			continue
		}
		providerConfig := providerConfigAccessor.GetConfig()
		if providerConfig == nil {
			continue
		}
		providerConfig.DisableRefresh = true
	}
}

func resolveProviderConfigPresence(configStore conf.ConfigStore) (map[string]struct{}, error) {
	resolved := make(map[string]struct{})
	configs, err := configStore.GetModelProviderConfigs()
	if err != nil {
		return nil, fmt.Errorf("resolveProviderConfigPresence() [bootstrap.go]: failed to get provider configs: %w", err)
	}

	for providerName := range configs {
		resolved[providerName] = struct{}{}
	}

	return resolved, nil
}

// CreateModelTagRegistry creates and populates a model tag registry from config store.
func CreateModelTagRegistry(configStore conf.ConfigStore, providerRegistry *models.ProviderRegistry) (*models.ModelTagRegistry, error) {
	modelTagRegistry := models.NewModelTagRegistry()

	globalConfig, err := configStore.GetGlobalConfig()
	if err == nil && globalConfig != nil && len(globalConfig.ModelTags) > 0 {
		if err := modelTagRegistry.SetGlobalMappings(globalConfig.ModelTags); err != nil {
			return nil, fmt.Errorf("CreateModelTagRegistry() [bootstrap.go]: failed to set global model tags: %w", err)
		}
	}

	for _, providerName := range providerRegistry.List() {
		provider, err := providerRegistry.Get(providerName)
		if err != nil {
			continue
		}
		if chatProvider, ok := provider.(interface{ GetConfig() interface{} }); ok {
			config := chatProvider.GetConfig()
			if providerConfig, ok := config.(*conf.ModelProviderConfig); ok && len(providerConfig.ModelTags) > 0 {
				if err := modelTagRegistry.SetProviderMappings(providerName, providerConfig.ModelTags); err != nil {
					return nil, fmt.Errorf("CreateModelTagRegistry() [bootstrap.go]: failed to set provider model tags: %w", err)
				}
			}
		}
	}

	return modelTagRegistry, nil
}
