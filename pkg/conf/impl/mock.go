// Package impl provides test doubles for conf.ConfigStore interface.
package impl

import (
	"fmt"
	"io/fs"
	"sync"
	"time"

	"github.com/rlewczuk/csw/pkg/conf"
)

// MockConfigStore is a test double implementation of conf.ConfigStore interface.
// It allows tests to control configuration values and update timestamps.
type MockConfigStore struct {
	mu sync.RWMutex

	globalConfig               *conf.GlobalConfig
	globalConfigUpdate         time.Time
	modelProviderConfigs       map[string]*conf.ModelProviderConfig
	modelProviderConfigsUpdate time.Time
	mcpServerConfigs           map[string]*conf.MCPServerConfig
	mcpServerConfigsUpdate     time.Time
	agentRoleConfigs           map[string]*conf.AgentRoleConfig
	agentRoleConfigsUpdate     time.Time
	agentConfigFiles           map[string][]byte

	// Error injection for testing
	GetGlobalConfigErr                error
	GetModelProviderConfigsErr        error
	GetAgentRoleConfigsErr            error
	LastGlobalConfigUpdateErr         error
	LastModelProviderConfigsUpdateErr error
	GetMCPServerConfigsErr            error
	LastMCPServerConfigsUpdateErr     error
	LastAgentRoleConfigsUpdateErr     error
	GetAgentConfigFileErr             error
}

// NewMockConfigStore creates a new MockConfigStore with empty configuration.
func NewMockConfigStore() *MockConfigStore {
	return &MockConfigStore{
		globalConfig:               &conf.GlobalConfig{},
		globalConfigUpdate:         time.Now(),
		modelProviderConfigs:       make(map[string]*conf.ModelProviderConfig),
		modelProviderConfigsUpdate: time.Now(),
		mcpServerConfigs:           make(map[string]*conf.MCPServerConfig),
		mcpServerConfigsUpdate:     time.Now(),
		agentRoleConfigs:           make(map[string]*conf.AgentRoleConfig),
		agentRoleConfigsUpdate:     time.Now(),
		agentConfigFiles:           make(map[string][]byte),
	}
}

// SetMCPServerConfigs sets MCP server configurations and updates timestamp.
func (m *MockConfigStore) SetMCPServerConfigs(configs map[string]*conf.MCPServerConfig) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.mcpServerConfigs = configs
	m.mcpServerConfigsUpdate = time.Now()
}

// SetAgentConfigFile sets agent config file content for tests.
func (m *MockConfigStore) SetAgentConfigFile(subdir, filename string, data []byte) {
	m.mu.Lock()
	defer m.mu.Unlock()
	key := subdir + "/" + filename
	copyData := make([]byte, len(data))
	copy(copyData, data)
	m.agentConfigFiles[key] = copyData
}

// SetGlobalConfig sets the global configuration and updates the timestamp.
func (m *MockConfigStore) SetGlobalConfig(config *conf.GlobalConfig) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.globalConfig = config
	m.globalConfigUpdate = time.Now()
}

// SetModelProviderConfigs sets the model provider configurations and updates the timestamp.
func (m *MockConfigStore) SetModelProviderConfigs(configs map[string]*conf.ModelProviderConfig) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.modelProviderConfigs = configs
	m.modelProviderConfigsUpdate = time.Now()
}

// SetAgentRoleConfigs sets the agent role configurations and updates the timestamp.
func (m *MockConfigStore) SetAgentRoleConfigs(configs map[string]*conf.AgentRoleConfig) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.agentRoleConfigs = configs
	m.agentRoleConfigsUpdate = time.Now()
}

// UpdateGlobalConfigTimestamp updates the global config timestamp without changing the config.
func (m *MockConfigStore) UpdateGlobalConfigTimestamp() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.globalConfigUpdate = time.Now()
}

// UpdateModelProviderConfigsTimestamp updates the model provider configs timestamp.
func (m *MockConfigStore) UpdateModelProviderConfigsTimestamp() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.modelProviderConfigsUpdate = time.Now()
}

// UpdateMCPServerConfigsTimestamp updates MCP server configs timestamp.
func (m *MockConfigStore) UpdateMCPServerConfigsTimestamp() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.mcpServerConfigsUpdate = time.Now()
}

// UpdateAgentRoleConfigsTimestamp updates the agent role configs timestamp.
func (m *MockConfigStore) UpdateAgentRoleConfigsTimestamp() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.agentRoleConfigsUpdate = time.Now()
}

// GetGlobalConfig returns the global configuration.
func (m *MockConfigStore) GetGlobalConfig() (*conf.GlobalConfig, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.GetGlobalConfigErr != nil {
		return nil, m.GetGlobalConfigErr
	}

	return m.globalConfig.Clone(), nil
}

// LastGlobalConfigUpdate returns the timestamp of the last global config update.
func (m *MockConfigStore) LastGlobalConfigUpdate() (time.Time, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.LastGlobalConfigUpdateErr != nil {
		return time.Time{}, m.LastGlobalConfigUpdateErr
	}

	return m.globalConfigUpdate, nil
}

// GetModelProviderConfigs returns the model provider configurations.
func (m *MockConfigStore) GetModelProviderConfigs() (map[string]*conf.ModelProviderConfig, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.GetModelProviderConfigsErr != nil {
		return nil, m.GetModelProviderConfigsErr
	}

	// Return a copy
	configs := make(map[string]*conf.ModelProviderConfig, len(m.modelProviderConfigs))
	for k, v := range m.modelProviderConfigs {
		configs[k] = v.Clone()
	}

	return configs, nil
}

// LastModelProviderConfigsUpdate returns the timestamp of the last model provider configs update.
func (m *MockConfigStore) LastModelProviderConfigsUpdate() (time.Time, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.LastModelProviderConfigsUpdateErr != nil {
		return time.Time{}, m.LastModelProviderConfigsUpdateErr
	}

	return m.modelProviderConfigsUpdate, nil
}

// GetMCPServerConfigs returns MCP server configurations.
func (m *MockConfigStore) GetMCPServerConfigs() (map[string]*conf.MCPServerConfig, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.GetMCPServerConfigsErr != nil {
		return nil, m.GetMCPServerConfigsErr
	}

	configs := make(map[string]*conf.MCPServerConfig, len(m.mcpServerConfigs))
	for key, value := range m.mcpServerConfigs {
		configs[key] = value.Clone()
	}

	return configs, nil
}

// LastMCPServerConfigsUpdate returns timestamp of last MCP server configs update.
func (m *MockConfigStore) LastMCPServerConfigsUpdate() (time.Time, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.LastMCPServerConfigsUpdateErr != nil {
		return time.Time{}, m.LastMCPServerConfigsUpdateErr
	}

	return m.mcpServerConfigsUpdate, nil
}

// GetAgentRoleConfigs returns the agent role configurations.
func (m *MockConfigStore) GetAgentRoleConfigs() (map[string]*conf.AgentRoleConfig, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.GetAgentRoleConfigsErr != nil {
		return nil, m.GetAgentRoleConfigsErr
	}

	// Return a copy
	configs := make(map[string]*conf.AgentRoleConfig, len(m.agentRoleConfigs))
	for k, v := range m.agentRoleConfigs {
		configCopy := *v
		if v.VFSPrivileges != nil {
			configCopy.VFSPrivileges = make(map[string]conf.FileAccess, len(v.VFSPrivileges))
			for pk, pv := range v.VFSPrivileges {
				configCopy.VFSPrivileges[pk] = pv
			}
		}
		if v.ToolsAccess != nil {
			configCopy.ToolsAccess = make(map[string]conf.AccessFlag, len(v.ToolsAccess))
			for tk, tv := range v.ToolsAccess {
				configCopy.ToolsAccess[tk] = tv
			}
		}
		if v.RunPrivileges != nil {
			configCopy.RunPrivileges = make(map[string]conf.AccessFlag, len(v.RunPrivileges))
			for rk, rv := range v.RunPrivileges {
				configCopy.RunPrivileges[rk] = rv
			}
		}
		if v.PromptFragments != nil {
			configCopy.PromptFragments = make(map[string]string, len(v.PromptFragments))
			for fk, fv := range v.PromptFragments {
				configCopy.PromptFragments[fk] = fv
			}
		}
		if v.ToolFragments != nil {
			configCopy.ToolFragments = make(map[string]string, len(v.ToolFragments))
			for fk, fv := range v.ToolFragments {
				configCopy.ToolFragments[fk] = fv
			}
		}
		configs[k] = &configCopy
	}

	return configs, nil
}

// LastAgentRoleConfigsUpdate returns the timestamp of the last agent role configs update.
func (m *MockConfigStore) LastAgentRoleConfigsUpdate() (time.Time, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.LastAgentRoleConfigsUpdateErr != nil {
		return time.Time{}, m.LastAgentRoleConfigsUpdateErr
	}

	return m.agentRoleConfigsUpdate, nil
}

// GetAgentConfigFile returns configured agent config file content.
func (m *MockConfigStore) GetAgentConfigFile(subdir, filename string) ([]byte, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.GetAgentConfigFileErr != nil {
		return nil, m.GetAgentConfigFileErr
	}

	key := subdir + "/" + filename
	data, ok := m.agentConfigFiles[key]
	if !ok {
		return nil, fmt.Errorf("MockConfigStore.GetAgentConfigFile() [mock.go]: file not found: agent/%s/%s: %w", subdir, filename, fs.ErrNotExist)
	}

	copyData := make([]byte, len(data))
	copy(copyData, data)
	return copyData, nil
}
