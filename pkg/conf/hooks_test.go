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
	err := json.Unmarshal([]byte(`{"name":"h1","description":"desc","hook":"merge","command":"echo hi"}`), &cfg)
	require.NoError(t, err)
	assert.Equal(t, "h1", cfg.Name)
	assert.Equal(t, "desc", cfg.Description)
	assert.Equal(t, "merge", cfg.Hook)
	assert.True(t, cfg.Enabled)
	assert.Equal(t, HookTypeShell, cfg.Type)
	assert.Equal(t, HookRunOnSandbox, cfg.RunOn)
	assert.Equal(t, time.Duration(0), cfg.Timeout)
}

func TestHookConfigUnmarshalYAMLDescription(t *testing.T) {
	var cfg HookConfig
	err := yaml.Unmarshal([]byte("name: h1\ndescription: test desc\nhook: merge\ncommand: echo hi\n"), &cfg)
	require.NoError(t, err)
	assert.Equal(t, "test desc", cfg.Description)
}

func TestHookConfigUnmarshalJSONLLMFields(t *testing.T) {
	var cfg HookConfig
	err := json.Unmarshal([]byte(`{"name":"h1","hook":"summary","type":"llm","prompt":"p","system_prompt":"s","model":"mock/test","thinking":"high","output_to":"out"}`), &cfg)
	require.NoError(t, err)

	assert.Equal(t, HookTypeLLM, cfg.Type)
	assert.Equal(t, "p", cfg.Prompt)
	assert.Equal(t, "s", cfg.SystemPrompt)
	assert.Equal(t, "mock/test", cfg.Model)
	assert.Equal(t, "high", cfg.Thinking)
	assert.Equal(t, "out", cfg.OutputTo)
}

func TestHookConfigUnmarshalJSONLLMDefaultsToField(t *testing.T) {
	var cfg HookConfig
	err := json.Unmarshal([]byte(`{"name":"h1","hook":"summary","type":"llm","prompt":"p"}`), &cfg)
	require.NoError(t, err)

	assert.Equal(t, HookTypeLLM, cfg.Type)
	assert.Equal(t, "result", cfg.OutputTo)
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

func TestHookConfigUnmarshalYAMLLLMFields(t *testing.T) {
	var cfg HookConfig
	err := yaml.Unmarshal([]byte("name: h1\nhook: summary\ntype: llm\nprompt: p\nsystem_prompt: s\nmodel: mock/test\nthinking: high\noutput_to: out\n"), &cfg)
	require.NoError(t, err)

	assert.Equal(t, HookTypeLLM, cfg.Type)
	assert.Equal(t, "p", cfg.Prompt)
	assert.Equal(t, "s", cfg.SystemPrompt)
	assert.Equal(t, "mock/test", cfg.Model)
	assert.Equal(t, "high", cfg.Thinking)
	assert.Equal(t, "out", cfg.OutputTo)
}

func TestHookConfigUnmarshalYAMLSubAgentFields(t *testing.T) {
	var cfg HookConfig
	err := yaml.Unmarshal([]byte("name: h1\nhook: summary\ntype: subagent\nprompt: p\nsystem_prompt: s\nmodel: mock/test\nthinking: high\nrole: reviewer\n"), &cfg)
	require.NoError(t, err)

	assert.Equal(t, HookTypeSubAgent, cfg.Type)
	assert.Equal(t, "p", cfg.Prompt)
	assert.Equal(t, "s", cfg.SystemPrompt)
	assert.Equal(t, "mock/test", cfg.Model)
	assert.Equal(t, "high", cfg.Thinking)
	assert.Equal(t, "reviewer", cfg.Role)
}

func TestHookConfigUnmarshalJSONSubAgentFields(t *testing.T) {
	var cfg HookConfig
	err := json.Unmarshal([]byte(`{"name":"h1","hook":"summary","type":"subagent","prompt":"p","system_prompt":"s","model":"mock/test","thinking":"high","role":"reviewer"}`), &cfg)
	require.NoError(t, err)

	assert.Equal(t, HookTypeSubAgent, cfg.Type)
	assert.Equal(t, "p", cfg.Prompt)
	assert.Equal(t, "s", cfg.SystemPrompt)
	assert.Equal(t, "mock/test", cfg.Model)
	assert.Equal(t, "high", cfg.Thinking)
	assert.Equal(t, "reviewer", cfg.Role)
}
