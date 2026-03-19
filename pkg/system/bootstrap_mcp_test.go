package system

import (
	"testing"

	confimpl "github.com/rlewczuk/csw/pkg/conf/impl"
	"github.com/rlewczuk/csw/pkg/conf"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildRuntimeMCPConfigStore(t *testing.T) {
	tests := []struct {
		name          string
		roleMCP       []string
		mcpEnable     []string
		mcpDisable    []string
		expectedAlpha bool
		expectedBeta  bool
	}{
		{
			name:          "role mcp-servers enables listed and disables others",
			roleMCP:       []string{"alpha"},
			expectedAlpha: true,
			expectedBeta:  false,
		},
		{
			name:          "flag overrides role and config defaults",
			roleMCP:       []string{"alpha"},
			mcpEnable:     []string{"beta"},
			mcpDisable:    []string{"alpha"},
			expectedAlpha: false,
			expectedBeta:  true,
		},
		{
			name:          "flag disable only still overrides role defaults",
			roleMCP:       []string{"alpha", "beta"},
			mcpDisable:    []string{"alpha"},
			expectedAlpha: false,
			expectedBeta:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := confimpl.NewMockConfigStore()
			store.SetMCPServerConfigs(map[string]*conf.MCPServerConfig{
				"alpha": {Enabled: false, Cmd: "alpha"},
				"beta":  {Enabled: true, Cmd: "beta"},
			})

			runtimeStore, err := buildRuntimeMCPConfigStore(store, tt.roleMCP, tt.mcpEnable, tt.mcpDisable)
			require.NoError(t, err)

			configs, err := runtimeStore.GetMCPServerConfigs()
			require.NoError(t, err)
			require.Contains(t, configs, "alpha")
			require.Contains(t, configs, "beta")
			assert.Equal(t, tt.expectedAlpha, configs["alpha"].Enabled)
			assert.Equal(t, tt.expectedBeta, configs["beta"].Enabled)
		})
	}
}

func TestBuildRuntimeMCPConfigStore_ReturnsErrorForUnknownServer(t *testing.T) {
	store := confimpl.NewMockConfigStore()
	store.SetMCPServerConfigs(map[string]*conf.MCPServerConfig{
		"alpha": {Enabled: true},
	})

	_, err := buildRuntimeMCPConfigStore(store, nil, []string{"missing"}, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), `mcp server "missing" is not configured`)
}

func TestNormalizeMCPServerNames(t *testing.T) {
	result := normalizeMCPServerNames([]string{" alpha,beta ", "beta", "", "gamma, ,alpha"})
	assert.Equal(t, []string{"alpha", "beta", "gamma"}, result)
}
