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
	"time"

	"github.com/rlewczuk/csw/pkg/conf"
	"github.com/rlewczuk/csw/pkg/conf/impl"
	"github.com/rlewczuk/csw/pkg/core"
	"github.com/rlewczuk/csw/pkg/logging"
	"github.com/rlewczuk/csw/pkg/lsp"
	"github.com/rlewczuk/csw/pkg/models"
	"github.com/rlewczuk/csw/pkg/runner"
	"github.com/rlewczuk/csw/pkg/system"
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
	WorkDir           string
	ShadowDir         string
	ConfigPath        string
	ProjectConfig     string
	ModelName         string
	RoleName          string
	WorktreeBranch    string
	ContinueWorktree  bool
	GitUserName       string
	GitUserEmail      string
	ContainerEnabled  bool
	ContainerDisabled bool
	ContainerImage    string
	ContainerMounts   []string
	ContainerEnv      []string
	LSPServer         string
	LogLLMRequests    bool
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
}

// BuildSystemResult contains outputs from building a SweSystem.
type BuildSystemResult struct {
	WorkDir          string
	WorkDirRoot      string
	ShadowDir        string
	RoleConfig       conf.AgentRoleConfig
	ModelName        string
	ConfigStore      conf.ConfigStore
	ProviderRegistry *models.ProviderRegistry
	LogsDir          string
	VCS              vfs.VCS
	WorktreeBranch   string
	LSPServer        string
	ContainerImage   string
	LSPStarted       bool
	LSPWorkDir       string
	Cleanup          func()
}

func prepareSessionVFS(workDir string, worktreesBaseDir string, worktreeBranch string, continueWorktree bool, hidePatterns []string, gitUserName string, gitUserEmail string, allowedPaths []string) (vfs.VCS, vfs.VFS, error) {
	localVFS, err := vfs.NewLocalVFS(workDir, hidePatterns, allowedPaths)
	if err != nil {
		return nil, nil, fmt.Errorf("prepareSessionVFS() [bootstrap.go]: failed to create local VFS: %w", err)
	}

	nullVCS, err := vfs.NewNullVFS(localVFS)
	if err != nil {
		return nil, nil, fmt.Errorf("prepareSessionVFS() [bootstrap.go]: failed to create NullVCS: %w", err)
	}

	var selectedVCS vfs.VCS = nullVCS

	if worktreeBranch != "" {
		worktreesRoot := filepath.Join(worktreesBaseDir, ".cswdata", "work")
		gitRepo, err := vfs.NewGitRepo(workDir, worktreesRoot, hidePatterns, allowedPaths, gitUserName, gitUserEmail)
		if err != nil {
			return nil, nil, fmt.Errorf("prepareSessionVFS() [bootstrap.go]: failed to create GitVCS: %w", err)
		}

		if continueWorktree {
			branches, listErr := gitRepo.ListBranches("")
			if listErr != nil {
				return nil, nil, fmt.Errorf("prepareSessionVFS() [bootstrap.go]: failed to list branches for continue mode: %w", listErr)
			}

			branchExists := false
			for _, branch := range branches {
				if branch == worktreeBranch {
					branchExists = true
					break
				}
			}

			if !branchExists {
				return nil, nil, fmt.Errorf("prepareSessionVFS() [bootstrap.go]: worktree branch %q not found: %w", worktreeBranch, vfs.ErrFileNotFound)
			}
		} else {
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
func BuildSystem(params BuildSystemParams) (*system.SweSystem, BuildSystemResult, error) {
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

	allowedPaths := append([]string(nil), params.AllowedPaths...)
	if shadowDir != "" {
		allowedPaths = append(allowedPaths, shadowDir)
	}

	selectedVCS, selectedVFS, err := prepareSessionVFS(workDir, configRoot, params.WorktreeBranch, params.ContinueWorktree, hidePatterns, params.GitUserName, params.GitUserEmail, allowedPaths)
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
	tool.RegisterVFSTools(toolRegistry, toolVFS, lspClient, nil)

	bashRunner := runner.CommandRunner(runner.NewBashRunner(effectiveWorkDir, params.BashRunTimeout))
	cleanupFn := func() {}

	containerRuntimeConfig, err := resolveContainerRuntimeConfig(globalConfig, params, effectiveWorkDir, shadowDir)
	if err != nil {
		logging.FlushLogs()
		return nil, result, fmt.Errorf("BuildSystem() [bootstrap.go]: failed to resolve container config: %w", err)
	}

	if containerRuntimeConfig.Enabled {
		containerUser, err := resolveCurrentUserIdentity()
		if err != nil {
			logging.FlushLogs()
			return nil, result, fmt.Errorf("BuildSystem() [bootstrap.go]: failed to resolve current user identity: %w", err)
		}

		gitAuthorName, gitAuthorEmail := resolveContainerGitAuthorIdentity()
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

		containerRunner, err := runner.NewContainerRunner(runner.ContainerConfig{
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

	tool.RegisterRunBashTool(toolRegistry, bashRunner, roleConfig.RunPrivileges, effectiveWorkDir, params.BashRunTimeout)
	tool.RegisterWebFetchTool(toolRegistry, nil)
	tool.RegisterSkillTool(toolRegistry, configRoot)
	if err := tool.RegisterCustomTools(toolRegistry, configStore, configRoot, bashRunner); err != nil {
		logging.FlushLogs()
		return nil, result, fmt.Errorf("BuildSystem() [bootstrap.go]: failed to register custom tools: %w", err)
	}

	promptGenerator, err := core.NewConfPromptGenerator(configStore, toolVFS)
	if err != nil {
		logging.FlushLogs()
		return nil, result, fmt.Errorf("BuildSystem() [bootstrap.go]: failed to create prompt generator: %w", err)
	}

	modelTagRegistry, err := CreateModelTagRegistry(configStore, providerRegistry)
	if err != nil {
		logging.FlushLogs()
		return nil, result, fmt.Errorf("BuildSystem() [bootstrap.go]: %w", err)
	}

	sweSystem := &system.SweSystem{
		ModelProviders:  modelProviders,
		ModelTags:       modelTagRegistry,
		ToolSelection:   globalConfig.ToolSelection,
		PromptGenerator: promptGenerator,
		Tools:           toolRegistry,
		VFS:             toolVFS,
		Roles:           roleRegistry,
		LSP:             lspClient,
		ConfigStore:     configStore,
		LogBaseDir:      logsDir,
		WorkDir:         effectiveWorkDir,
		ShadowDir:       shadowDir,
		LogLLMRequests:  params.LogLLMRequests,
		Thinking:        params.Thinking,
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
		ContainerImage:   containerRuntimeConfig.Image,
		LSPStarted:       lspClient != nil,
		LSPWorkDir:       effectiveWorkDir,
		Cleanup:          cleanupFn,
	}

	return sweSystem, result, nil
}

// containerRuntimeConfig describes effective container runtime setup.
type containerRuntimeConfig struct {
	Enabled bool
	Image   string
	Mounts  map[string]string
	Env     map[string]string
}

func resolveContainerRuntimeConfig(globalConfig *conf.GlobalConfig, params BuildSystemParams, effectiveWorkDir string, shadowDir string) (containerRuntimeConfig, error) {
	var runtimeConfig containerRuntimeConfig

	runtimeConfig.Enabled = globalConfig.Container.Enabled
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
		runtimeConfig.Image = strings.TrimSpace(globalConfig.Container.Image)
	}
	if runtimeConfig.Image == "" {
		return runtimeConfig, fmt.Errorf("resolveContainerRuntimeConfig() [bootstrap.go]: container image is required when container mode is enabled")
	}

	mountSpecs := make([]string, 0, len(globalConfig.Container.Mounts)+len(params.ContainerMounts))
	mountSpecs = append(mountSpecs, globalConfig.Container.Mounts...)
	mountSpecs = append(mountSpecs, params.ContainerMounts...)
	runtimeConfig.Mounts = map[string]string{effectiveWorkDir: effectiveWorkDir}
	if strings.TrimSpace(shadowDir) != "" {
		runtimeConfig.Mounts[shadowDir] = shadowDir
	}
	for _, mountSpec := range mountSpecs {
		hostPath, containerPath, err := parseContainerMountSpec(mountSpec)
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

	envSpecs := make([]string, 0, len(globalConfig.Container.Env)+len(params.ContainerEnv))
	envSpecs = append(envSpecs, globalConfig.Container.Env...)
	envSpecs = append(envSpecs, params.ContainerEnv...)
	if len(envSpecs) > 0 {
		runtimeConfig.Env = make(map[string]string, len(envSpecs))
		for _, envSpec := range envSpecs {
			key, value, err := parseContainerEnvSpec(envSpec)
			if err != nil {
				return runtimeConfig, err
			}
			runtimeConfig.Env[key] = value
		}
	}

	return runtimeConfig, nil
}

func parseContainerMountSpec(mountSpec string) (string, string, error) {
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

func parseContainerEnvSpec(envSpec string) (string, string, error) {
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
