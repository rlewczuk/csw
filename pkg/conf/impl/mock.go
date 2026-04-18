// Package impl provides test doubles for conf.ConfigStore interface.
package impl

import (
	"fmt"
	"io/fs"
	"sync"

	"github.com/rlewczuk/csw/pkg/conf"
)

// MockConfigStore is a test double implementation of conf.ConfigStore interface.
// It allows tests to control configuration values and update timestamps.
type MockConfigStore struct {
	mu sync.RWMutex

	globalConfig         *conf.GlobalConfig
	modelProviderConfigs map[string]*conf.ModelProviderConfig
	modelAliases         map[string]conf.ModelAliasValue
	agentRoleConfigs     map[string]*conf.AgentRoleConfig
	agentConfigFiles     map[string][]byte

	// Error injection for testing
	GetGlobalConfigErr         error
	GetModelProviderConfigsErr error
	GetAgentRoleConfigsErr     error
	GetModelAliasesErr         error
	GetAgentConfigFileErr      error
}

// NewMockConfigStore creates a new MockConfigStore with empty configuration.
func NewMockConfigStore() *MockConfigStore {
	return &MockConfigStore{
		globalConfig:         &conf.GlobalConfig{},
		modelProviderConfigs: make(map[string]*conf.ModelProviderConfig),
		modelAliases:         make(map[string]conf.ModelAliasValue),
		agentRoleConfigs:     make(map[string]*conf.AgentRoleConfig),
		agentConfigFiles:     make(map[string][]byte),
	}
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
}

// SetModelProviderConfigs sets the model provider configurations and updates the timestamp.
func (m *MockConfigStore) SetModelProviderConfigs(configs map[string]*conf.ModelProviderConfig) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.modelProviderConfigs = configs
}

// SetModelAliases sets model aliases and updates timestamp.
func (m *MockConfigStore) SetModelAliases(aliases map[string]conf.ModelAliasValue) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.modelAliases = aliases
}

// SetAgentRoleConfigs sets the agent role configurations and updates the timestamp.
func (m *MockConfigStore) SetAgentRoleConfigs(configs map[string]*conf.AgentRoleConfig) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.agentRoleConfigs = configs
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

// GetModelAliases returns configured model aliases.
func (m *MockConfigStore) GetModelAliases() (map[string]conf.ModelAliasValue, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.GetModelAliasesErr != nil {
		return nil, m.GetModelAliasesErr
	}

	aliases := make(map[string]conf.ModelAliasValue, len(m.modelAliases))
	for key, value := range m.modelAliases {
		aliases[key] = conf.ModelAliasValue{Values: append([]string(nil), value.Values...)}
	}

	return aliases, nil
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
