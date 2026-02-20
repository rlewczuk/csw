// Package testfixture provides core integration test fixtures.
package testfixture

import (
	"testing"

	"github.com/rlewczuk/csw/pkg/conf"
	"github.com/rlewczuk/csw/pkg/core"
	"github.com/rlewczuk/csw/pkg/logging"
	"github.com/rlewczuk/csw/pkg/lsp"
	"github.com/rlewczuk/csw/pkg/models"
	"github.com/rlewczuk/csw/pkg/testutil"
	"github.com/rlewczuk/csw/pkg/tool"
	"github.com/rlewczuk/csw/pkg/vfs"
)

// StaticPromptGenerator provides a fixed prompt and tool info for tests.
type StaticPromptGenerator struct {
	Prompt string
}

// NewStaticPromptGenerator returns a prompt generator with a fixed prompt.
func NewStaticPromptGenerator(prompt string) *StaticPromptGenerator {
	return &StaticPromptGenerator{Prompt: prompt}
}

// GetPrompt returns the configured prompt.
func (g *StaticPromptGenerator) GetPrompt(tags []string, role *conf.AgentRoleConfig, state *core.AgentState) (string, error) {
	return g.Prompt, nil
}

// GetToolInfo returns a minimal tool info for tests.
func (g *StaticPromptGenerator) GetToolInfo(tags []string, toolName string, role *conf.AgentRoleConfig, state *core.AgentState) (tool.ToolInfo, error) {
	schema := tool.NewToolSchema()
	return tool.ToolInfo{
		Name:        toolName,
		Description: "Mock tool for testing",
		Schema:      schema,
	}, nil
}

// GetAgentFiles returns no files for tests.
func (g *StaticPromptGenerator) GetAgentFiles(dir string) (map[string]string, error) {
	return make(map[string]string), nil
}

// SweSystemFixture holds a configured SweSystem and its dependencies.
type SweSystemFixture struct {
	Server *testutil.MockHTTPServer
	Client models.ModelProvider
	VFS    vfs.VFS
	Tools  *tool.ToolRegistry
	System *core.SweSystem
}

// SweSystemFixtureOption configures a SweSystemFixture.
type SweSystemFixtureOption func(*SweSystemFixtureConfig)

// SweSystemFixtureConfig configures SweSystemFixture creation.
type SweSystemFixtureConfig struct {
	ProviderName     string
	ModelProvider    models.ModelProvider
	ModelProviders   map[string]models.ModelProvider
	PromptGenerator  core.PromptGenerator
	VFS              vfs.VFS
	Tools            *tool.ToolRegistry
	WorkDir          string
	SessionLogger    core.SessionLoggerFactory
	Roles            *core.AgentRoleRegistry
	ConfigStore      conf.ConfigStore
	LSP              lsp.LSP
	LogBaseDir       string
	LogLLMRequests   *bool
	RegisterVFSTools bool
}

// NewSweSystemFixture creates a SweSystemFixture with sensible defaults.
func NewSweSystemFixture(t *testing.T, opts ...SweSystemFixtureOption) *SweSystemFixture {
	config := SweSystemFixtureConfig{
		ProviderName:     "ollama",
		WorkDir:          ".",
		RegisterVFSTools: true,
	}
	for _, opt := range opts {
		opt(&config)
	}

	server := testutil.NewMockHTTPServer()
	t.Cleanup(func() {
		server.Close()
	})

	if config.VFS == nil {
		config.VFS = vfs.NewMockVFS()
	}

	if config.Tools == nil {
		config.Tools = tool.NewToolRegistry()
	}

	if config.RegisterVFSTools {
		tool.RegisterVFSTools(config.Tools, config.VFS, nil, nil)
	}

	if config.PromptGenerator == nil {
		config.PromptGenerator = NewStaticPromptGenerator("You are a test assistant.")
	}

	if config.SessionLogger == nil {
		config.SessionLogger = logging.NewTestLoggerFactory(t)
	}

	var provider models.ModelProvider
	if config.ModelProvider != nil {
		provider = config.ModelProvider
	} else if len(config.ModelProviders) == 0 {
		client, err := models.NewOllamaClientWithHTTPClient(server.URL(), server.Client())
		if err != nil {
			t.Fatalf("NewSweSystemFixture: models.NewOllamaClientWithHTTPClient failed: %v", err)
		}
		provider = client
	}

	providers := config.ModelProviders
	if providers == nil {
		providers = map[string]models.ModelProvider{config.ProviderName: provider}
	}

	system := &core.SweSystem{
		ModelProviders:       providers,
		ModelTags:            models.NewModelTagRegistry(),
		PromptGenerator:      config.PromptGenerator,
		Tools:                config.Tools,
		VFS:                  config.VFS,
		Roles:                config.Roles,
		ConfigStore:          config.ConfigStore,
		LSP:                  config.LSP,
		SessionLoggerFactory: config.SessionLogger,
		WorkDir:              config.WorkDir,
		LogBaseDir:           config.LogBaseDir,
	}
	if config.LogLLMRequests != nil {
		system.LogLLMRequests = *config.LogLLMRequests
	}

	return &SweSystemFixture{
		Server: server,
		Client: provider,
		VFS:    config.VFS,
		Tools:  config.Tools,
		System: system,
	}
}

// WithPromptGenerator sets a custom prompt generator.
func WithPromptGenerator(generator core.PromptGenerator) SweSystemFixtureOption {
	return func(config *SweSystemFixtureConfig) {
		config.PromptGenerator = generator
	}
}

// WithVFS sets a custom VFS implementation.
func WithVFS(vfsInstance vfs.VFS) SweSystemFixtureOption {
	return func(config *SweSystemFixtureConfig) {
		config.VFS = vfsInstance
	}
}

// WithTools sets a custom tool registry.
func WithTools(tools *tool.ToolRegistry) SweSystemFixtureOption {
	return func(config *SweSystemFixtureConfig) {
		config.Tools = tools
	}
}

// WithModelProvider sets the default model provider.
func WithModelProvider(provider models.ModelProvider) SweSystemFixtureOption {
	return func(config *SweSystemFixtureConfig) {
		config.ModelProvider = provider
	}
}

// WithModelProviders sets the model providers map.
func WithModelProviders(providers map[string]models.ModelProvider) SweSystemFixtureOption {
	return func(config *SweSystemFixtureConfig) {
		config.ModelProviders = providers
	}
}

// WithProviderName sets the provider name for the default provider.
func WithProviderName(name string) SweSystemFixtureOption {
	return func(config *SweSystemFixtureConfig) {
		config.ProviderName = name
	}
}

// WithWorkDir sets the system work directory.
func WithWorkDir(workDir string) SweSystemFixtureOption {
	return func(config *SweSystemFixtureConfig) {
		config.WorkDir = workDir
	}
}

// WithSessionLoggerFactory sets a custom session logger factory.
func WithSessionLoggerFactory(factory core.SessionLoggerFactory) SweSystemFixtureOption {
	return func(config *SweSystemFixtureConfig) {
		config.SessionLogger = factory
	}
}

// WithRoles sets the role registry.
func WithRoles(roles *core.AgentRoleRegistry) SweSystemFixtureOption {
	return func(config *SweSystemFixtureConfig) {
		config.Roles = roles
	}
}

// WithConfigStore sets the config store.
func WithConfigStore(store conf.ConfigStore) SweSystemFixtureOption {
	return func(config *SweSystemFixtureConfig) {
		config.ConfigStore = store
	}
}

// WithLSP sets the LSP instance.
func WithLSP(lspInstance lsp.LSP) SweSystemFixtureOption {
	return func(config *SweSystemFixtureConfig) {
		config.LSP = lspInstance
	}
}

// WithLogBaseDir sets the base directory for log output.
func WithLogBaseDir(logBaseDir string) SweSystemFixtureOption {
	return func(config *SweSystemFixtureConfig) {
		config.LogBaseDir = logBaseDir
	}
}

// WithLogLLMRequests sets LogLLMRequests on the system.
func WithLogLLMRequests(enabled bool) SweSystemFixtureOption {
	return func(config *SweSystemFixtureConfig) {
		config.LogLLMRequests = &enabled
	}
}

// WithoutVFSTools disables automatic VFS tool registration.
func WithoutVFSTools() SweSystemFixtureOption {
	return func(config *SweSystemFixtureConfig) {
		config.RegisterVFSTools = false
	}
}
