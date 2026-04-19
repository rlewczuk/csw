package impl

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/rlewczuk/csw/pkg/conf"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLocalConfigStore_YAMLGlobalConfig(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "test-config-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Create global.yml
	globalYAML := `model-tags:
  - model: "^claude-.*"
    tag: "anthropic"
defaults:
  default-provider: "test-provider"
  default-role: "test-role"
`
	err = os.WriteFile(filepath.Join(tmpDir, "global.yml"), []byte(globalYAML), 0644)
	require.NoError(t, err)

	store, err := NewLocalConfigStore(tmpDir)
	require.NoError(t, err)
	defer store.Close()

	// Test GetGlobalConfig
	config, err := store.GetGlobalConfig()
	require.NoError(t, err)
	assert.Len(t, config.ModelTags, 1)
	assert.Equal(t, "^claude-.*", config.ModelTags[0].Model)
	assert.Equal(t, "anthropic", config.ModelTags[0].Tag)
	assert.Equal(t, "test-provider", config.Defaults.DefaultProvider)
	assert.Equal(t, "test-role", config.Defaults.DefaultRole)
}

func TestLocalConfigStore_YAMLPrecedence_Global(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "test-config-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Create both global.yml and global.json
	globalYAML := `model-tags:
  - model: "^gpt-.*"
    tag: "openai"
defaults:
  default-provider: "yaml-provider"
`
	globalJSON := `{
  "model-tags": [
    {"model": "^claude-.*", "tag": "anthropic"}
  ],
  "defaults": {
    "default-provider": "json-provider"
  }
}`

	err = os.WriteFile(filepath.Join(tmpDir, "global.yml"), []byte(globalYAML), 0644)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(tmpDir, "global.json"), []byte(globalJSON), 0644)
	require.NoError(t, err)

	store, err := NewLocalConfigStore(tmpDir)
	require.NoError(t, err)
	defer store.Close()

	// YAML should take precedence
	config, err := store.GetGlobalConfig()
	require.NoError(t, err)
	assert.Len(t, config.ModelTags, 1)
	assert.Equal(t, "^gpt-.*", config.ModelTags[0].Model)
	assert.Equal(t, "openai", config.ModelTags[0].Tag)
	assert.Equal(t, "yaml-provider", config.Defaults.DefaultProvider)
}

func TestLocalConfigStore_YAMLModelProvider(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "test-config-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	modelsDir := filepath.Join(tmpDir, "models")
	require.NoError(t, os.Mkdir(modelsDir, 0755))

	// Create YAML model provider config
	providerYAML := `type: openai
name: openai-yaml
description: OpenAI via YAML
url: https://api.openai.com/v1
api-key: yaml-key
`
	err = os.WriteFile(filepath.Join(modelsDir, "openai.yml"), []byte(providerYAML), 0644)
	require.NoError(t, err)

	store, err := NewLocalConfigStore(tmpDir)
	require.NoError(t, err)
	defer store.Close()

	// Test GetModelProviderConfigs
	configs, err := store.GetModelProviderConfigs()
	require.NoError(t, err)
	assert.Len(t, configs, 1)

	openai, ok := configs["openai"]
	require.True(t, ok)
	assert.Equal(t, "openai", openai.Type)
	assert.Equal(t, "openai", openai.Name)
	assert.Equal(t, "OpenAI via YAML", openai.Description)
	assert.Equal(t, "https://api.openai.com/v1", openai.URL)
	assert.Equal(t, "yaml-key", openai.APIKey)
}

func TestLocalConfigStore_ModelProviderConfigs_AllowedExtensionsAndDeterministicSelection(t *testing.T) {
	tmpDir := t.TempDir()
	modelsDir := filepath.Join(tmpDir, "models")
	require.NoError(t, os.Mkdir(modelsDir, 0o755))

	// All allowed extensions should be loaded.
	require.NoError(t, os.WriteFile(filepath.Join(modelsDir, "json-provider.json"), []byte(`{"type":"openai","url":"https://json.example"}`), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(modelsDir, "yaml-provider.yaml"), []byte("type: openai\nurl: https://yaml.example\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(modelsDir, "yml-provider.yml"), []byte("type: openai\nurl: https://yml.example\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(modelsDir, "conf-provider.conf"), []byte("type: openai\nurl: https://conf.example\n"), 0o644))

	// Files that are not exact allowed extensions should be ignored.
	require.NoError(t, os.WriteFile(filepath.Join(modelsDir, "ignored.json.bkp"), []byte(`{"type":"openai","url":"https://ignored.example"}`), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(modelsDir, "ignored.txt"), []byte("ignored"), 0o644))

	// Same provider basename in multiple extensions should resolve deterministically.
	// Expected priority: .yml > .yaml > .json > .conf
	require.NoError(t, os.WriteFile(filepath.Join(modelsDir, "dupe.conf"), []byte("type: openai\nurl: https://dupe-conf.example\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(modelsDir, "dupe.json"), []byte(`{"type":"openai","url":"https://dupe-json.example"}`), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(modelsDir, "dupe.yaml"), []byte("type: openai\nurl: https://dupe-yaml.example\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(modelsDir, "dupe.yml"), []byte("type: openai\nurl: https://dupe-yml.example\n"), 0o644))

	store, err := NewLocalConfigStore(tmpDir)
	require.NoError(t, err)
	defer store.Close()

	configs, err := store.GetModelProviderConfigs()
	require.NoError(t, err)
	require.Len(t, configs, 5)

	require.Contains(t, configs, "json-provider")
	require.Contains(t, configs, "yaml-provider")
	require.Contains(t, configs, "yml-provider")
	require.Contains(t, configs, "conf-provider")
	require.Contains(t, configs, "dupe")

	assert.Equal(t, "https://dupe-yml.example", configs["dupe"].URL)
	assert.Equal(t, "dupe", configs["dupe"].Name)
	assert.NotContains(t, configs, "ignored")
}

func TestLocalConfigStore_ModelProviderConfigs_NameAlwaysFromFilename(t *testing.T) {
	tmpDir := t.TempDir()
	modelsDir := filepath.Join(tmpDir, "models")
	require.NoError(t, os.Mkdir(modelsDir, 0o755))

	require.NoError(t, os.WriteFile(filepath.Join(modelsDir, "from-file.json"), []byte(`{"name":"from-config","type":"openai","url":"https://example.com"}`), 0o644))

	store, err := NewLocalConfigStore(tmpDir)
	require.NoError(t, err)
	defer store.Close()

	configs, err := store.GetModelProviderConfigs()
	require.NoError(t, err)
	require.Contains(t, configs, "from-file")
	require.NotContains(t, configs, "from-config")
	assert.Equal(t, "from-file", configs["from-file"].Name)
}

func TestLocalConfigStore_YAMLPrecedence_ModelProvider(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "test-config-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	modelsDir := filepath.Join(tmpDir, "models")
	require.NoError(t, os.Mkdir(modelsDir, 0755))

	// Create both YAML and JSON for the same provider
	providerYAML := `type: openai
name: test-provider
description: YAML version
url: https://yaml.example.com
`
	providerJSON := `{
  "type": "anthropic",
  "name": "test-provider",
  "description": "JSON version",
  "url": "https://json.example.com"
}`

	err = os.WriteFile(filepath.Join(modelsDir, "test-provider.yml"), []byte(providerYAML), 0644)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(modelsDir, "test-provider.json"), []byte(providerJSON), 0644)
	require.NoError(t, err)

	store, err := NewLocalConfigStore(tmpDir)
	require.NoError(t, err)
	defer store.Close()

	// YAML should take precedence
	configs, err := store.GetModelProviderConfigs()
	require.NoError(t, err)
	assert.Len(t, configs, 1)

	provider, ok := configs["test-provider"]
	require.True(t, ok)
	assert.Equal(t, "openai", provider.Type)
	assert.Equal(t, "YAML version", provider.Description)
	assert.Equal(t, "https://yaml.example.com", provider.URL)
}

func TestLocalConfigStore_YAMLRoleConfig(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "test-config-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	rolesDir := filepath.Join(tmpDir, "roles")
	require.NoError(t, os.Mkdir(rolesDir, 0755))

	// Create YAML role config
	roleDir := filepath.Join(rolesDir, "yaml-role")
	require.NoError(t, os.Mkdir(roleDir, 0755))

	roleYAML := `name: yaml-role
description: A role defined in YAML
vfs-privileges:
  "**":
    read: allow
    write: ask
tools-access:
  read: allow
  write: deny
`
	err = os.WriteFile(filepath.Join(roleDir, "config.yml"), []byte(roleYAML), 0644)
	require.NoError(t, err)

	store, err := NewLocalConfigStore(tmpDir)
	require.NoError(t, err)
	defer store.Close()

	// Test GetAgentRoleConfigs
	configs, err := store.GetAgentRoleConfigs()
	require.NoError(t, err)
	assert.Len(t, configs, 1)

	role, ok := configs["yaml-role"]
	require.True(t, ok)
	assert.Equal(t, "yaml-role", role.Name)
	assert.Equal(t, "A role defined in YAML", role.Description)
	assert.Equal(t, conf.AccessAllow, role.VFSPrivileges["**"].Read)
	assert.Equal(t, conf.AccessAsk, role.VFSPrivileges["**"].Write)
	assert.Equal(t, conf.AccessAllow, role.ToolsAccess["read"])
	assert.Equal(t, conf.AccessDeny, role.ToolsAccess["write"])
}

func TestLocalConfigStore_YAMLPrecedence_RoleConfig(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "test-config-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	rolesDir := filepath.Join(tmpDir, "roles")
	require.NoError(t, os.Mkdir(rolesDir, 0755))

	// Create both YAML and JSON config for the same role
	roleDir := filepath.Join(rolesDir, "test-role")
	require.NoError(t, os.Mkdir(roleDir, 0755))

	roleYAML := `name: test-role
description: YAML version
vfs-privileges: {}
tools-access:
  read: allow
`
	roleJSON := `{
  "name": "test-role",
  "description": "JSON version",
  "vfs-privileges": {},
  "tools-access": {
    "read": "deny"
  }
}`

	err = os.WriteFile(filepath.Join(roleDir, "config.yml"), []byte(roleYAML), 0644)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(roleDir, "config.json"), []byte(roleJSON), 0644)
	require.NoError(t, err)

	store, err := NewLocalConfigStore(tmpDir)
	require.NoError(t, err)
	defer store.Close()

	// YAML should take precedence
	configs, err := store.GetAgentRoleConfigs()
	require.NoError(t, err)
	assert.Len(t, configs, 1)

	role, ok := configs["test-role"]
	require.True(t, ok)
	assert.Equal(t, "YAML version", role.Description)
	assert.Equal(t, conf.AccessAllow, role.ToolsAccess["read"])
}

func TestLocalConfigStore_YAMLConfigFileWatching(t *testing.T) {
	t.Skip("hot reload removed")
}

func TestLocalConfigStore_GlobalConfigSupportsYAMLExtension(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "test-config-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	globalYAML := `defaults:
  container:
    enabled: true
    image: "golang:1.25-trixie"
`
	err = os.WriteFile(filepath.Join(tmpDir, "global.yaml"), []byte(globalYAML), 0644)
	require.NoError(t, err)

	store, err := NewLocalConfigStore(tmpDir)
	require.NoError(t, err)
	defer store.Close()

	config, err := store.GetGlobalConfig()
	require.NoError(t, err)
	require.NotNil(t, config.Defaults.Container)
	assert.True(t, config.Defaults.Container.Enabled)
	assert.Equal(t, "golang:1.25-trixie", config.Defaults.Container.Image)
}

func TestLocalConfigStore_YAMLLongExtensionConfigFileWatching(t *testing.T) {
	t.Skip("hot reload removed")
}
