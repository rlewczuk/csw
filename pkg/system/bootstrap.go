package system

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/rlewczuk/csw/pkg/apis"
	"github.com/rlewczuk/csw/pkg/conf"
	"github.com/rlewczuk/csw/pkg/core"
	"github.com/rlewczuk/csw/pkg/logging"
	"github.com/rlewczuk/csw/pkg/lsp"
	"github.com/rlewczuk/csw/pkg/models"
	"github.com/rlewczuk/csw/pkg/runner"
	"github.com/rlewczuk/csw/pkg/tool"
	"github.com/rlewczuk/csw/pkg/vcs"
	"github.com/rlewczuk/csw/pkg/vfs"
)

var newCompositeConfigStoreFunc = func(projectRoot string, configPath string) (*conf.CswConfig, error) {
	resolvedConfigPath, err := ResolveConfigPathForProjectRoot(configPath, projectRoot)
	if err != nil {
		return nil, fmt.Errorf("newCompositeConfigStoreFunc() [bootstrap.go]: failed to resolve config path for project root: %w", err)
	}

	return conf.CswConfigLoad(resolvedConfigPath)
}
var resolveModelNameFunc = ResolveModelName
var createProviderMapFunc = CreateProviderMap
var generateWorktreeBranchNameFunc = core.GenerateWorktreeBranchName
var createConfigUpdaterFunc = models.NewConfigUpdater

// SetNewCompositeConfigStoreFuncForTest overrides composite store constructor in tests.
func SetNewCompositeConfigStoreFuncForTest(fn func(projectRoot string, configPath string) (*conf.CswConfig, error)) {
	newCompositeConfigStoreFunc = fn
}

// NewCompositeConfigStoreFuncForTest returns current composite store constructor.
func NewCompositeConfigStoreFuncForTest() func(projectRoot string, configPath string) (*conf.CswConfig, error) {
	return newCompositeConfigStoreFunc
}

// SetResolveModelNameFuncForTest overrides model name resolver in tests.
func SetResolveModelNameFuncForTest(fn func(modelName string, configStore *conf.CswConfig, providerRegistry *models.ProviderRegistry) (string, error)) {
	resolveModelNameFunc = fn
}

// ResolveModelNameFuncForTest returns current model name resolver.
func ResolveModelNameFuncForTest() func(modelName string, configStore *conf.CswConfig, providerRegistry *models.ProviderRegistry) (string, error) {
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
func SetGenerateWorktreeBranchNameFuncForTest(fn func(ctx context.Context, modelProviders map[string]models.ModelProvider, configStore *conf.CswConfig, model string, inputPrompt string) (string, error)) {
	generateWorktreeBranchNameFunc = fn
}

// GenerateWorktreeBranchNameFuncForTest returns current branch name generator.
func GenerateWorktreeBranchNameFuncForTest() func(ctx context.Context, modelProviders map[string]models.ModelProvider, configStore *conf.CswConfig, model string, inputPrompt string) (string, error) {
	return generateWorktreeBranchNameFunc
}

// BuildSystemResult contains outputs from building a SweSystem.
type BuildSystemResult struct {
	WorkDir               string
	WorkDirRoot           string
	ShadowDir             string
	RoleConfig            conf.AgentRoleConfig
	ModelName             string
	ConfigStore           *conf.CswConfig
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

// ResolveRunDefaultsParams contains inputs for resolving run command parameters.
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

// ResolveRunDefaults resolves run command parameters from effective global config.
func ResolveRunDefaults(params ResolveRunDefaultsParams) (conf.RunParameters, error) {
	var parameters conf.RunParameters

	resolvedWorkDir, err := ResolveWorkDir(params.WorkDir)
	if err != nil {
		return parameters, fmt.Errorf("ResolveRunDefaults() [bootstrap.go]: failed to resolve work directory: %w", err)
	}

	configPathStr, err := BuildConfigPath(params.ProjectConfig, params.ConfigPath)
	if err != nil {
		return parameters, fmt.Errorf("ResolveRunDefaults() [bootstrap.go]: failed to build config path: %w", err)
	}

	configRoot := resolvedWorkDir
	if strings.TrimSpace(params.ShadowDir) != "" {
		resolvedShadowDir, shadowErr := ResolveWorkDir(params.ShadowDir)
		if shadowErr != nil {
			return parameters, fmt.Errorf("ResolveRunDefaults() [bootstrap.go]: failed to resolve shadow directory: %w", shadowErr)
		}
		configRoot = resolvedShadowDir
	}

	configStore, err := newCompositeConfigStoreFunc(configRoot, configPathStr)
	if err != nil {
		return parameters, fmt.Errorf("ResolveRunDefaults() [bootstrap.go]: failed to create config store: %w", err)
	}

	globalConfig := configStore.GlobalConfig
	if globalConfig == nil {
		return parameters, nil
	}

	return globalConfig.Parameters, nil
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
func BuildSystem(configStore *conf.CswConfig) (*SweSystem, BuildSystemResult, error) {
	var result BuildSystemResult
	if configStore == nil {
		configStore = &conf.CswConfig{}
	}
	globalConfig := configStore.GlobalConfig
	if globalConfig == nil {
		globalConfig = &conf.GlobalConfig{}
		configStore.GlobalConfig = globalConfig
	}
	parameters := globalConfig.Parameters
	bashRunTimeout, err := parseBashRunTimeout(parameters.BashRunTimeout)
	if err != nil {
		return nil, result, err
	}

	workDir, err := ResolveWorkDir(parameters.Workdir)
	if err != nil {
		return nil, result, fmt.Errorf("BuildSystem() [bootstrap.go]: %w", err)
	}

	shadowDir := ""
	if strings.TrimSpace(parameters.ShadowDir) != "" {
		shadowDir, err = ResolveWorkDir(parameters.ShadowDir)
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

	shadowPatterns := append([]string(nil), globalConfig.ShadowPaths...)
	if len(shadowPatterns) == 0 {
		shadowPatterns = vfs.DefaultShadowPatterns()
	}

	providerRegistry := models.NewProviderRegistry(configStore)
	if len(providerRegistry.List()) == 0 {
		logging.FlushLogs()
		return nil, result, fmt.Errorf("BuildSystem() [bootstrap.go]: no model providers found in config")
	}

	modelName, err := ResolveModelName(parameters.Model, configStore, providerRegistry)
	if err != nil {
		logging.FlushLogs()
		return nil, result, fmt.Errorf("BuildSystem() [bootstrap.go]: %w", err)
	}

	modelProviders, err := CreateProviderMap(providerRegistry)
	if err != nil {
		logging.FlushLogs()
		return nil, result, fmt.Errorf("BuildSystem() [bootstrap.go]: %w", err)
	}
	if parameters.NoRefresh {
		applyDisableRefreshToProviders(modelProviders)
	}

	modelAliases, err := models.NormalizeModelAliasMap(configStore.ModelAliases)
	if err != nil {
		logging.FlushLogs()
		return nil, result, fmt.Errorf("BuildSystem() [bootstrap.go]: failed to normalize model aliases: %w", err)
	}

	roleRegistry := core.NewAgentRoleRegistry(configStore)
	if len(roleRegistry.List()) == 0 {
		logging.FlushLogs()
		return nil, result, fmt.Errorf("BuildSystem() [bootstrap.go]: no roles found in config")
	}

	roleConfig, ok := roleRegistry.Get(parameters.Role)
	if !ok {
		logging.FlushLogs()
		return nil, result, fmt.Errorf("BuildSystem() [bootstrap.go]: role not found: %s (available: %v)", parameters.Role, roleRegistry.List())
	}

	if strings.TrimSpace(parameters.Model) == "" {
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

	allowedPaths := append([]string(nil), parameters.VFSAllow...)
	if shadowDir != "" {
		allowedPaths = append(allowedPaths, shadowDir)
	}

	selectedVCS, selectedVFS, err := PrepareSessionVFS(workDir, configRoot, parameters.Worktree, hidePatterns, parameters.GitUserName, parameters.GitUserEmail, allowedPaths)
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
	if parameters.LSPServer != "" {
		logger := logging.GetGlobalLogger()
		logger.Debug("lsp_initialization", "enabled", true, "server", parameters.LSPServer)
		if _, err := os.Stat(parameters.LSPServer); err != nil {
			logger.Warn("LSP server binary not found, continuing without LSP", "server", parameters.LSPServer, "error", err)
		} else {
			client, err := lsp.NewClient(parameters.LSPServer, effectiveWorkDir)
			if err != nil {
				logger.Warn("failed to create LSP client, continuing without LSP", "error", err)
			} else if err := client.Init(false); err != nil {
				logger.Warn("failed to initialize LSP client, continuing without LSP", "error", err)
			} else {
				lspClient = client
				logger.Debug("lsp_initialized", "server", parameters.LSPServer)
			}
		}
	}

	vfsReadLimit := int(tool.DefaultVFSReadLimitLines)
	if globalConfig.Parameters.VfsReadLimit != nil {
		vfsReadLimit = *globalConfig.Parameters.VfsReadLimit
	}
	if parameters.VfsReadLimit != nil {
		vfsReadLimit = *parameters.VfsReadLimit
	}
	if vfsReadLimit < 0 {
		logging.FlushLogs()
		return nil, result, fmt.Errorf("BuildSystem() [bootstrap.go]: --vfs-read-limit must be >= 0")
	}

	toolRegistry := tool.NewToolRegistry()
	tool.RegisterVFSTools(toolRegistry, toolVFS, lspClient, nil, vfsReadLimit)

	taskManager, err := core.NewTaskManagerWithTasksDir(workDir, ".cswdata/tasks", configStore)
	if err != nil {
		logging.FlushLogs()
		return nil, result, fmt.Errorf("BuildSystem() [bootstrap.go]: failed to create task manager: %w", err)
	}

	bashRunner := runner.CommandRunner(runner.NewBashRunner(effectiveWorkDir, bashRunTimeout))
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

	containerRuntimeConfig, err := ResolveContainerRuntimeConfig(globalConfig, parameters, effectiveWorkDir, shadowDir)
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

	runBashMaxOutput := tool.DefaultRunBashMaxOutputBytes
	if globalConfig.Parameters.RunBashMax != nil {
		runBashMaxOutput = *globalConfig.Parameters.RunBashMax
	}
	if parameters.RunBashMax != nil {
		runBashMaxOutput = *parameters.RunBashMax
	}
	if runBashMaxOutput < 0 {
		logging.FlushLogs()
		return nil, result, fmt.Errorf("BuildSystem() [bootstrap.go]: --run-bash-max must be >= 0")
	}

	runBashOutputWorkdir := ""
	if shadowDir != "" {
		runBashOutputWorkdir = shadowDir
	}
	tool.RegisterRunBashTool(toolRegistry, bashRunner, roleConfig.RunPrivileges, effectiveWorkDir, bashRunTimeout, parameters.AllowAllPermissions, runBashMaxOutput, runBashOutputWorkdir)
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
		Config:              configStore,
		TaskManager:         taskManager,
		TaskVCS:             selectedVCS,
		LogBaseDir:          logsDir,
		WorkDir:             effectiveWorkDir,
		ShadowDir:           shadowDir,
		LogLLMRequests:      parameters.LogLLMRequests,
		LogLLMRequestsRaw:   parameters.LogLLMRequestsRaw,
		Thinking:            parameters.Thinking,
		AllowAllPermissions: parameters.AllowAllPermissions,
		MaxToolThreads: func() int {
			if parameters.MaxThreads > 0 {
				return parameters.MaxThreads
			}
			return globalConfig.Parameters.MaxThreads
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
		WorktreeBranch:   parameters.Worktree,
		LSPServer:        parameters.LSPServer,
		ShellRunner:      bashRunner,
		HostShellRunner:  runner.NewBashRunner(effectiveWorkDir, bashRunTimeout),
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
