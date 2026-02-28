package tool

import (
	"errors"
	"testing"
	"time"

	"github.com/rlewczuk/csw/pkg/conf"
	"github.com/rlewczuk/csw/pkg/conf/impl"
	"github.com/rlewczuk/csw/pkg/runner"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRegisterCustomTools(t *testing.T) {
	t.Run("registers json and yaml custom tools", func(t *testing.T) {
		store := impl.NewMockConfigStore()
		store.SetAgentRoleConfigs(map[string]*conf.AgentRoleConfig{
			"all": {
				Name: "all",
				ToolFragments: map[string]string{
					"echoTool/.tooldir":           "/tmp/echo",
					"echoTool/echoTool.json":      `{"command":"echo {{.arg.value}}","roles":["developer"]}`,
					"yamlTool/.tooldir":           "/tmp/yaml",
					"yamlTool/yamlTool.yaml":      "command: ['printf', '{{.arg.value}}']\n",
					"noConfig/noConfig.md":        "description",
					"builtin/vfsRead.schema.json": `{}`,
				},
			},
		})

		registry := NewToolRegistry()
		err := RegisterCustomTools(registry, store, "/project", runner.NewMockRunner())
		require.NoError(t, err)

		echoTool, err := registry.Get("echoTool")
		require.NoError(t, err)
		restricted, ok := echoTool.(RoleRestrictedTool)
		require.True(t, ok)
		assert.True(t, restricted.IsRoleAllowed("developer"))
		assert.False(t, restricted.IsRoleAllowed("tester"))

		_, err = registry.Get("yamlTool")
		require.NoError(t, err)
		_, err = registry.Get("noConfig")
		require.Error(t, err)
	})

	t.Run("returns error for invalid config", func(t *testing.T) {
		store := impl.NewMockConfigStore()
		store.SetAgentRoleConfigs(map[string]*conf.AgentRoleConfig{
			"all": {
				Name: "all",
				ToolFragments: map[string]string{
					"broken/broken.json": "{invalid}",
				},
			},
		})

		err := RegisterCustomTools(NewToolRegistry(), store, "/project", runner.NewMockRunner())
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to parse broken/broken.json")
	})
}

func TestCustomCommandTool_Execute(t *testing.T) {
	t.Run("renders command cwd env and result", func(t *testing.T) {
		mockRunner := runner.NewMockRunner()
		mockRunner.SetResponse("export ANSWER='42'; echo hello", "world", 0, nil)

		tool, err := newCustomCommandTool("echo", "/work", customToolDefinition{
			Command: "echo {{.arg.input}}",
			Cwd:     "{{.workdir}}",
			Env: map[string]string{
				"ANSWER": "{{.arg.number}}",
			},
			Result: map[string]any{
				"joined": "{{.arg.input}}={{.stdout}}",
			},
			Timeout: 5,
		}, mockRunner)
		require.NoError(t, err)

		call := &ToolCall{
			ID:       "1",
			Function: "echo",
			Arguments: NewToolValue(map[string]any{
				"input":  "hello",
				"number": "42",
			}),
		}

		response := tool.Execute(call)
		require.NoError(t, response.Error)
		assert.True(t, response.Done)
		assert.Equal(t, "hello=world", response.Result.Get("joined").AsString())

		exec := mockRunner.GetLastExecution()
		require.NotNil(t, exec)
		assert.Equal(t, "/work", exec.Workdir)
		assert.Equal(t, 5*time.Second, exec.Timeout)
	})

	t.Run("returns stdout as default result", func(t *testing.T) {
		mockRunner := runner.NewMockRunner()
		mockRunner.SetResponse("echo ok", "ok", 0, nil)

		tool, err := newCustomCommandTool("echo", "/work", customToolDefinition{Command: "echo ok"}, mockRunner)
		require.NoError(t, err)

		response := tool.Execute(&ToolCall{ID: "1", Function: "echo", Arguments: NewToolValue(map[string]any{})})
		require.NoError(t, response.Error)
		assert.Equal(t, "ok", response.Result.AsString())
	})

	t.Run("uses error template on command failure", func(t *testing.T) {
		mockRunner := runner.NewMockRunner()
		mockRunner.SetResponse("fail", "", 1, errors.New("boom"))

		tool, err := newCustomCommandTool("f", "/work", customToolDefinition{
			Command: "fail",
			Error:   "failed: {{.exitCode}} {{.stdout}}",
		}, mockRunner)
		require.NoError(t, err)

		response := tool.Execute(&ToolCall{ID: "1", Function: "f", Arguments: NewToolValue(map[string]any{})})
		require.Error(t, response.Error)
		assert.Contains(t, response.Error.Error(), "failed: 1")
	})

	t.Run("supports command array", func(t *testing.T) {
		mockRunner := runner.NewMockRunner()
		mockRunner.SetResponse("'printf' '%s %s' 'a' 'b'", "a b", 0, nil)

		tool, err := newCustomCommandTool("arr", "/work", customToolDefinition{
			Command: []any{"printf", "%s %s", "{{.arg.a}}", "{{.arg.b}}"},
		}, mockRunner)
		require.NoError(t, err)

		response := tool.Execute(&ToolCall{ID: "1", Function: "arr", Arguments: NewToolValue(map[string]any{"a": "a", "b": "b"})})
		require.NoError(t, response.Error)
		assert.Equal(t, "a b", response.Result.AsString())
	})

	t.Run("fails when command template is invalid", func(t *testing.T) {
		tool, err := newCustomCommandTool("x", "/work", customToolDefinition{Command: "{{"}, runner.NewMockRunner())
		require.NoError(t, err)

		response := tool.Execute(&ToolCall{ID: "1", Function: "x", Arguments: NewToolValue(map[string]any{})})
		require.Error(t, response.Error)
		assert.Contains(t, response.Error.Error(), "failed to render command")
	})
}

func TestParseCustomToolTimeout(t *testing.T) {
	tests := []struct {
		name    string
		input   any
		want    time.Duration
		wantErr bool
	}{
		{name: "default", input: nil, want: 120 * time.Second},
		{name: "int seconds", input: 10, want: 10 * time.Second},
		{name: "float seconds", input: float64(2), want: 2 * time.Second},
		{name: "duration string", input: "3m", want: 3 * time.Minute},
		{name: "string seconds", input: "7", want: 7 * time.Second},
		{name: "invalid", input: "abc", wantErr: true},
		{name: "non positive", input: 0, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseCustomToolTimeout(tt.input)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}
