package models

import (
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewProviderRegistry(t *testing.T) {
	registry := NewProviderRegistry()
	assert.NotNil(t, registry)
	assert.NotNil(t, registry.providers)
	assert.Equal(t, 0, len(registry.providers))
}

func TestProviderRegistry_Register(t *testing.T) {
	tests := []struct {
		name        string
		provName    string
		provider    ModelProvider
		wantErr     bool
		expectedErr error
	}{
		{
			name:     "successful registration",
			provName: "test-provider",
			provider: NewMockProvider(nil),
			wantErr:  false,
		},
		{
			name:        "empty name",
			provName:    "",
			provider:    NewMockProvider(nil),
			wantErr:     true,
			expectedErr: nil, // Just check error exists
		},
		{
			name:        "nil provider",
			provName:    "test-provider",
			provider:    nil,
			wantErr:     true,
			expectedErr: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			registry := NewProviderRegistry()
			err := registry.Register(tt.provName, tt.provider)

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				// Verify provider was registered
				provider, err := registry.Get(tt.provName)
				assert.NoError(t, err)
				assert.Equal(t, tt.provider, provider)
			}
		})
	}
}

func TestProviderRegistry_RegisterDuplicate(t *testing.T) {
	registry := NewProviderRegistry()
	provider1 := NewMockProvider(nil)
	provider2 := NewMockProvider(nil)

	// Register first provider
	err := registry.Register("test", provider1)
	require.NoError(t, err)

	// Try to register second provider with same name
	err = registry.Register("test", provider2)
	assert.ErrorIs(t, err, ErrProviderAlreadyExists)

	// Verify first provider is still registered
	provider, err := registry.Get("test")
	assert.NoError(t, err)
	assert.Equal(t, provider1, provider)
}

func TestProviderRegistry_Get(t *testing.T) {
	registry := NewProviderRegistry()
	provider := NewMockProvider(nil)

	// Register a provider
	err := registry.Register("test", provider)
	require.NoError(t, err)

	tests := []struct {
		name        string
		provName    string
		wantErr     bool
		expectedErr error
	}{
		{
			name:     "existing provider",
			provName: "test",
			wantErr:  false,
		},
		{
			name:        "non-existent provider",
			provName:    "non-existent",
			wantErr:     true,
			expectedErr: ErrProviderNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider, err := registry.Get(tt.provName)

			if tt.wantErr {
				assert.Error(t, err)
				assert.ErrorIs(t, err, tt.expectedErr)
				assert.Nil(t, provider)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, provider)
			}
		})
	}
}

func TestProviderRegistry_List(t *testing.T) {
	registry := NewProviderRegistry()

	// Empty registry
	names := registry.List()
	assert.Equal(t, 0, len(names))

	// Register multiple providers
	provider1 := NewMockProvider(nil)
	provider2 := NewMockProvider(nil)
	provider3 := NewMockProvider(nil)

	err := registry.Register("provider-1", provider1)
	require.NoError(t, err)
	err = registry.Register("provider-2", provider2)
	require.NoError(t, err)
	err = registry.Register("provider-3", provider3)
	require.NoError(t, err)

	// Get list and sort for consistent comparison
	names = registry.List()
	sort.Strings(names)

	expected := []string{"provider-1", "provider-2", "provider-3"}
	assert.Equal(t, expected, names)
}

func TestProviderRegistry_LoadFromDirectory(t *testing.T) {
	tests := []struct {
		name          string
		setupFiles    func(t *testing.T, dir string)
		wantErr       bool
		expectedCount int
		expectedNames []string
	}{
		{
			name: "load valid config files",
			setupFiles: func(t *testing.T, dir string) {
				// Create test config files
				ollamaConfig := `{
					"type": "ollama",
					"name": "ollama",
					"url": "http://localhost:11434"
				}`
				err := os.WriteFile(filepath.Join(dir, "ollama.json"), []byte(ollamaConfig), 0644)
				require.NoError(t, err)

				anthropicConfig := `{
					"type": "anthropic",
					"name": "anthropic",
					"url": "https://api.anthropic.com",
					"api_key": "test-key"
				}`
				err = os.WriteFile(filepath.Join(dir, "anthropic.json"), []byte(anthropicConfig), 0644)
				require.NoError(t, err)
			},
			wantErr:       false,
			expectedCount: 2,
			expectedNames: []string{"anthropic", "ollama"},
		},
		{
			name: "load config with missing name field",
			setupFiles: func(t *testing.T, dir string) {
				// Config without name field - should use filename
				config := `{
					"type": "ollama",
					"url": "http://localhost:11434"
				}`
				err := os.WriteFile(filepath.Join(dir, "local-ollama.json"), []byte(config), 0644)
				require.NoError(t, err)
			},
			wantErr:       false,
			expectedCount: 1,
			expectedNames: []string{"local-ollama"},
		},
		{
			name: "load config with mismatched name field",
			setupFiles: func(t *testing.T, dir string) {
				// Config with different name - should override with filename
				config := `{
					"type": "ollama",
					"name": "wrong-name",
					"url": "http://localhost:11434"
				}`
				err := os.WriteFile(filepath.Join(dir, "correct-name.json"), []byte(config), 0644)
				require.NoError(t, err)
			},
			wantErr:       false,
			expectedCount: 1,
			expectedNames: []string{"correct-name"},
		},
		{
			name: "skip non-json files",
			setupFiles: func(t *testing.T, dir string) {
				// Create a valid JSON file
				config := `{
					"type": "ollama",
					"url": "http://localhost:11434"
				}`
				err := os.WriteFile(filepath.Join(dir, "ollama.json"), []byte(config), 0644)
				require.NoError(t, err)

				// Create non-JSON files that should be skipped
				err = os.WriteFile(filepath.Join(dir, "readme.txt"), []byte("test"), 0644)
				require.NoError(t, err)
				err = os.WriteFile(filepath.Join(dir, "config.yaml"), []byte("test: value"), 0644)
				require.NoError(t, err)
			},
			wantErr:       false,
			expectedCount: 1,
			expectedNames: []string{"ollama"},
		},
		{
			name: "invalid json content",
			setupFiles: func(t *testing.T, dir string) {
				// Create invalid JSON
				err := os.WriteFile(filepath.Join(dir, "invalid.json"), []byte("not valid json{"), 0644)
				require.NoError(t, err)
			},
			wantErr: true,
		},
		{
			name: "missing required fields",
			setupFiles: func(t *testing.T, dir string) {
				// Config without URL (required field)
				config := `{
					"type": "ollama"
				}`
				err := os.WriteFile(filepath.Join(dir, "invalid.json"), []byte(config), 0644)
				require.NoError(t, err)
			},
			wantErr: true,
		},
		{
			name: "unsupported provider type",
			setupFiles: func(t *testing.T, dir string) {
				config := `{
					"type": "unsupported-provider",
					"url": "http://localhost:8080"
				}`
				err := os.WriteFile(filepath.Join(dir, "unsupported.json"), []byte(config), 0644)
				require.NoError(t, err)
			},
			wantErr: true,
		},
		{
			name: "empty directory",
			setupFiles: func(t *testing.T, dir string) {
				// Don't create any files
			},
			wantErr:       false,
			expectedCount: 0,
			expectedNames: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temporary directory
			tempDir, err := os.MkdirTemp("", "registry-test-*")
			require.NoError(t, err)
			defer os.RemoveAll(tempDir)

			// Setup test files
			tt.setupFiles(t, tempDir)

			// Create registry and load configs
			registry := NewProviderRegistry()
			err = registry.LoadFromDirectory(tempDir)

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)

				// Verify expected number of providers
				names := registry.List()
				assert.Equal(t, tt.expectedCount, len(names))

				// Verify expected provider names
				sort.Strings(names)
				sort.Strings(tt.expectedNames)
				assert.Equal(t, tt.expectedNames, names)

				// Verify we can retrieve each provider
				for _, name := range tt.expectedNames {
					provider, err := registry.Get(name)
					assert.NoError(t, err)
					assert.NotNil(t, provider)
				}
			}
		})
	}
}

func TestProviderRegistry_LoadFromDirectory_NonExistentDirectory(t *testing.T) {
	registry := NewProviderRegistry()
	err := registry.LoadFromDirectory("/non/existent/directory")
	assert.Error(t, err)
}

func TestProviderRegistry_LoadFromDirectory_RealConfigs(t *testing.T) {
	// Test loading from the actual configs/models directory if it exists
	configDir := "../../configs/models"

	// Check if directory exists
	if _, err := os.Stat(configDir); os.IsNotExist(err) {
		t.Skip("configs/models directory does not exist")
	}

	registry := NewProviderRegistry()
	err := registry.LoadFromDirectory(configDir)
	assert.NoError(t, err)

	// Verify we loaded at least one provider
	names := registry.List()
	assert.Greater(t, len(names), 0)

	// Verify we can get each provider
	for _, name := range names {
		provider, err := registry.Get(name)
		assert.NoError(t, err)
		assert.NotNil(t, provider)
	}
}

func TestProviderRegistry_RegisterMultipleConfigsForSameProvider(t *testing.T) {
	// Test that we can register the same provider type under different names
	tempDir, err := os.MkdirTemp("", "registry-multi-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	// Create multiple Ollama configs with different names
	config1 := `{
		"type": "ollama",
		"url": "http://localhost:11434"
	}`
	err = os.WriteFile(filepath.Join(tempDir, "ollama-local.json"), []byte(config1), 0644)
	require.NoError(t, err)

	config2 := `{
		"type": "ollama",
		"url": "http://remote-server:11434"
	}`
	err = os.WriteFile(filepath.Join(tempDir, "ollama-remote.json"), []byte(config2), 0644)
	require.NoError(t, err)

	// Load configs
	registry := NewProviderRegistry()
	err = registry.LoadFromDirectory(tempDir)
	assert.NoError(t, err)

	// Verify both providers are registered
	names := registry.List()
	sort.Strings(names)
	assert.Equal(t, []string{"ollama-local", "ollama-remote"}, names)

	// Verify we can get both providers
	provider1, err := registry.Get("ollama-local")
	assert.NoError(t, err)
	assert.NotNil(t, provider1)

	provider2, err := registry.Get("ollama-remote")
	assert.NoError(t, err)
	assert.NotNil(t, provider2)

	// They should be different instances
	assert.NotEqual(t, provider1, provider2)
}

func TestProviderRegistry_Concurrency(t *testing.T) {
	// Test concurrent access to registry
	registry := NewProviderRegistry()

	// Register initial provider
	provider := NewMockProvider(nil)
	err := registry.Register("test", provider)
	require.NoError(t, err)

	// Run concurrent operations
	done := make(chan bool)

	// Concurrent reads
	for i := 0; i < 10; i++ {
		go func() {
			_, _ = registry.Get("test")
			_ = registry.List()
			done <- true
		}()
	}

	// Wait for all goroutines to complete
	for i := 0; i < 10; i++ {
		<-done
	}

	// Verify registry is still functional
	p, err := registry.Get("test")
	assert.NoError(t, err)
	assert.Equal(t, provider, p)
}
