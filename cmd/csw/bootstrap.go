package main

import (
	"fmt"
	"path/filepath"

	"github.com/codesnort/codesnort-swe/pkg/conf"
	"github.com/codesnort/codesnort-swe/pkg/conf/impl"
	"github.com/codesnort/codesnort-swe/pkg/core"
	"github.com/codesnort/codesnort-swe/pkg/logging"
	"github.com/codesnort/codesnort-swe/pkg/lsp"
	"github.com/codesnort/codesnort-swe/pkg/models"
	"github.com/codesnort/codesnort-swe/pkg/runner"
	"github.com/codesnort/codesnort-swe/pkg/tool"
	"github.com/codesnort/codesnort-swe/pkg/vfs"
)

// BuildSystemParams contains inputs for constructing a SweSystem.
type BuildSystemParams struct {
	WorkDir        string
	ConfigPath     string
	ModelName      string
	RoleName       string
	LSPServer      string
	LogLLMRequests bool
}

// BuildSystemResult contains outputs from building a SweSystem.
type BuildSystemResult struct {
	WorkDir          string
	RoleConfig       conf.AgentRoleConfig
	ModelName        string
	ConfigStore      conf.ConfigStore
	ProviderRegistry *models.ProviderRegistry
	LogsDir          string
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

	localVFS, err := vfs.NewLocalVFS(workDir, hidePatterns)
	if err != nil {
		logging.FlushLogs()
		return nil, result, fmt.Errorf("BuildSystem() [bootstrap.go]: failed to create VFS: %w", err)
	}

	var lspClient lsp.LSP
	if params.LSPServer != "" {
		logger := logging.GetGlobalLogger()
		logger.Debug("lsp_initialization", "enabled", true, "server", params.LSPServer)
		client, err := lsp.NewClient(params.LSPServer, workDir)
		if err != nil {
			logger.Warn("failed to create LSP client, continuing without LSP", "error", err)
		} else if err := client.Init(false); err != nil {
			logger.Warn("failed to initialize LSP client, continuing without LSP", "error", err)
		} else {
			lspClient = client
			logger.Debug("lsp_initialized", "server", params.LSPServer)
		}
	}

	toolRegistry := tool.NewToolRegistry()
	tool.RegisterVFSTools(toolRegistry, localVFS, lspClient, nil)

	bashRunner := runner.NewBashRunner(workDir, 0)
	tool.RegisterRunBashTool(toolRegistry, bashRunner, roleConfig.RunPrivileges)

	promptGenerator, err := core.NewConfPromptGenerator(configStore, localVFS)
	if err != nil {
		logging.FlushLogs()
		return nil, result, fmt.Errorf("BuildSystem() [bootstrap.go]: failed to create prompt generator: %w", err)
	}

	modelTagRegistry, err := CreateModelTagRegistry(configStore, providerRegistry)
	if err != nil {
		logging.FlushLogs()
		return nil, result, fmt.Errorf("BuildSystem() [bootstrap.go]: %w", err)
	}

	sweSystem := &core.SweSystem{
		ModelProviders:  modelProviders,
		ModelTags:       modelTagRegistry,
		PromptGenerator: promptGenerator,
		Tools:           toolRegistry,
		VFS:             localVFS,
		Roles:           roleRegistry,
		LSP:             lspClient,
		ConfigStore:     configStore,
		LogBaseDir:      logsDir,
		WorkDir:         workDir,
		LogLLMRequests:  params.LogLLMRequests,
	}

	result = BuildSystemResult{
		WorkDir:          workDir,
		RoleConfig:       roleConfig,
		ModelName:        modelName,
		ConfigStore:      configStore,
		ProviderRegistry: providerRegistry,
		LogsDir:          logsDir,
	}

	return sweSystem, result, nil
}
