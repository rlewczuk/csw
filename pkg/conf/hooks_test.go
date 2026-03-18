package conf

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestHookConfigUnmarshalJSONDefaults(t *testing.T) {
	var cfg HookConfig
	err := json.Unmarshal([]byte(`{"name":"h1","hook":"merge","command":"echo hi"}`), &cfg)
	require.NoError(t, err)
	assert.Equal(t, "h1", cfg.Name)
	assert.Equal(t, "merge", cfg.Hook)
	assert.True(t, cfg.Enabled)
	assert.Equal(t, HookTypeShell, cfg.Type)
	assert.Equal(t, HookRunOnSandbox, cfg.RunOn)
	assert.Equal(t, time.Duration(0), cfg.Timeout)
}

func TestHookConfigUnmarshalYAMLParsesTimeout(t *testing.T) {
	var cfg HookConfig
	err := yaml.Unmarshal([]byte("name: h1\nhook: merge\ncommand: echo hi\ntimeout: 10s\nrun-on: host\n"), &cfg)
	require.NoError(t, err)
	assert.Equal(t, 10*time.Second, cfg.Timeout)
	assert.Equal(t, HookRunOnHost, cfg.RunOn)
}

func TestHookConfigUnmarshalYAMLUsesFilenameProvidedName(t *testing.T) {
	var cfg HookConfig
	err := yaml.Unmarshal([]byte("hook: merge\ncommand: echo hi\n"), &cfg)
	require.NoError(t, err)
	assert.Equal(t, "merge", cfg.Hook)
	assert.Equal(t, "", cfg.Name)
}
