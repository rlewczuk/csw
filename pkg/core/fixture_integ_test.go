package core

import (
	"testing"

	"github.com/codesnort/codesnort-swe/pkg/conf"
	"github.com/codesnort/codesnort-swe/pkg/logging"
	"github.com/codesnort/codesnort-swe/pkg/lsp"
	"github.com/codesnort/codesnort-swe/pkg/models"
	"github.com/codesnort/codesnort-swe/pkg/testutil"
	"github.com/codesnort/codesnort-swe/pkg/tool"
	"github.com/codesnort/codesnort-swe/pkg/vfs"
)

type sweSystemFixture struct {
	server *testutil.MockHTTPServer
	client models.ModelProvider
	vfs    vfs.VFS
	tools  *tool.ToolRegistry
	system *SweSystem
}

type sweSystemFixtureOption func(*sweSystemFixtureConfig)

type sweSystemFixtureConfig struct {
	providerName     string
	modelProvider    models.ModelProvider
	modelProviders   map[string]models.ModelProvider
	promptGenerator  PromptGenerator
	vfsInstance      vfs.VFS
	tools            *tool.ToolRegistry
	workDir          string
	roles            *AgentRoleRegistry
	configStore      conf.ConfigStore
	lspInstance      lsp.LSP
	logBaseDir       string
	logLLMRequests   *bool
	registerVFSTools bool
}

func newSweSystemFixture(t *testing.T, prompt string, opts ...sweSystemFixtureOption) *sweSystemFixture {
	config := sweSystemFixtureConfig{
		providerName:     "ollama",
		workDir:          ".",
		registerVFSTools: true,
	}
	for _, opt := range opts {
		opt(&config)
	}

	server := testutil.NewMockHTTPServer()
	t.Cleanup(func() {
		server.Close()
	})

	if config.vfsInstance == nil {
		config.vfsInstance = vfs.NewMockVFS()
	}

	if config.tools == nil {
		config.tools = tool.NewToolRegistry()
	}

	if config.registerVFSTools {
		tool.RegisterVFSTools(config.tools, config.vfsInstance, nil, nil)
	}

	if config.promptGenerator == nil {
		config.promptGenerator = &testPromptGenerator{prompt: prompt}
	}

	var provider models.ModelProvider
	if config.modelProvider != nil {
		provider = config.modelProvider
	} else if len(config.modelProviders) == 0 {
		client, err := models.NewOllamaClientWithHTTPClient(server.URL(), server.Client())
		if err != nil {
			t.Fatalf("newSweSystemFixture: models.NewOllamaClientWithHTTPClient failed: %v", err)
		}
		provider = client
	}

	providers := config.modelProviders
	if providers == nil {
		providers = map[string]models.ModelProvider{config.providerName: provider}
	}

	system := &SweSystem{
		ModelProviders:       providers,
		ModelTags:            models.NewModelTagRegistry(),
		PromptGenerator:      config.promptGenerator,
		Tools:                config.tools,
		VFS:                  config.vfsInstance,
		Roles:                config.roles,
		ConfigStore:          config.configStore,
		LSP:                  config.lspInstance,
		SessionLoggerFactory: logging.NewTestLoggerFactory(t),
		WorkDir:              config.workDir,
		LogBaseDir:           config.logBaseDir,
	}
	if config.logLLMRequests != nil {
		system.LogLLMRequests = *config.logLLMRequests
	}

	return &sweSystemFixture{
		server: server,
		client: provider,
		vfs:    config.vfsInstance,
		tools:  config.tools,
		system: system,
	}
}

type testPromptGenerator struct {
	prompt string
}

func (g *testPromptGenerator) GetPrompt(tags []string, role *conf.AgentRoleConfig, state *AgentState) (string, error) {
	return g.prompt, nil
}

func (g *testPromptGenerator) GetToolInfo(tags []string, toolName string, role *conf.AgentRoleConfig, state *AgentState) (tool.ToolInfo, error) {
	schema := tool.NewToolSchema()
	return tool.ToolInfo{
		Name:        toolName,
		Description: "Mock tool for testing",
		Schema:      schema,
	}, nil
}

func (g *testPromptGenerator) GetAgentFiles(dir string) (map[string]string, error) {
	return make(map[string]string), nil
}

func withModelProviders(providers map[string]models.ModelProvider) sweSystemFixtureOption {
	return func(config *sweSystemFixtureConfig) {
		config.modelProviders = providers
	}
}

func withModelProvider(provider models.ModelProvider) sweSystemFixtureOption {
	return func(config *sweSystemFixtureConfig) {
		config.modelProvider = provider
	}
}

func withProviderName(name string) sweSystemFixtureOption {
	return func(config *sweSystemFixtureConfig) {
		config.providerName = name
	}
}

func withVFS(vfsInstance vfs.VFS) sweSystemFixtureOption {
	return func(config *sweSystemFixtureConfig) {
		config.vfsInstance = vfsInstance
	}
}

func withTools(tools *tool.ToolRegistry) sweSystemFixtureOption {
	return func(config *sweSystemFixtureConfig) {
		config.tools = tools
	}
}

func withRoles(roles *AgentRoleRegistry) sweSystemFixtureOption {
	return func(config *sweSystemFixtureConfig) {
		config.roles = roles
	}
}

func withConfigStore(store conf.ConfigStore) sweSystemFixtureOption {
	return func(config *sweSystemFixtureConfig) {
		config.configStore = store
	}
}

func withLSP(lspInstance lsp.LSP) sweSystemFixtureOption {
	return func(config *sweSystemFixtureConfig) {
		config.lspInstance = lspInstance
	}
}

func withWorkDir(workDir string) sweSystemFixtureOption {
	return func(config *sweSystemFixtureConfig) {
		config.workDir = workDir
	}
}

func withLogBaseDir(logBaseDir string) sweSystemFixtureOption {
	return func(config *sweSystemFixtureConfig) {
		config.logBaseDir = logBaseDir
	}
}

func withLogLLMRequests(enabled bool) sweSystemFixtureOption {
	return func(config *sweSystemFixtureConfig) {
		config.logLLMRequests = &enabled
	}
}

func withoutVFSTools() sweSystemFixtureOption {
	return func(config *sweSystemFixtureConfig) {
		config.registerVFSTools = false
	}
}
