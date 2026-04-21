package conf

import (
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"
)

const (
	defaultsConfigPathMarker = "@DEFAULTS"
	embeddedDefaultsRoot    = "files"
)

//go:embed files/**
var embeddedDefaultsFS embed.FS

type configLoadSource struct {
	Path          string
	FS            fs.FS
	ToolDirPrefix string
}

// CswConfigLoad loads consolidated config from a colon-separated source path.
func CswConfigLoad(path string) (*CswConfig, error) {
	sources, err := resolveConfigLoadSources(path)
	if err != nil {
		return nil, err
	}

	mergedGlobalConfig := &GlobalConfig{}
	mergedModelProviderConfigs := make(map[string]*ModelProviderConfig)
	mergedModelAliases := make(map[string]ModelAliasValue)
	mergedAgentRoleConfigs := make(map[string]*AgentRoleConfig)
	mergedAgentConfigFiles := make(map[string]map[string]string)
	for sourceIndex := range sources {
		sourceGlobalConfig, loadErr := loadGlobalConfigFromSource(sources[sourceIndex])
		if loadErr != nil {
			return nil, fmt.Errorf("CswConfigLoad() [load.go]: failed to load global config from source %q: %w", sources[sourceIndex].Path, loadErr)
		}
		mergedGlobalConfig.Merge(sourceGlobalConfig)

		sourceModelProviderConfigs, loadErr := loadModelProviderConfigsFromSource(sources[sourceIndex])
		if loadErr != nil {
			return nil, fmt.Errorf("CswConfigLoad() [load.go]: failed to load model providers from source %q: %w", sources[sourceIndex].Path, loadErr)
		}
		for providerName, providerConfig := range sourceModelProviderConfigs {
			mergedModelProviderConfigs[providerName] = providerConfig.Clone()
		}

		sourceModelAliases, loadErr := loadModelAliasesFromSource(sources[sourceIndex])
		if loadErr != nil {
			return nil, fmt.Errorf("CswConfigLoad() [load.go]: failed to load model aliases from source %q: %w", sources[sourceIndex].Path, loadErr)
		}
		for aliasName, aliasValue := range sourceModelAliases {
			mergedModelAliases[aliasName] = ModelAliasValue{Values: append([]string(nil), aliasValue.Values...)}
		}

		sourceAgentRoleConfigs, loadErr := loadAgentRoleConfigsFromSource(sources[sourceIndex])
		if loadErr != nil {
			return nil, fmt.Errorf("CswConfigLoad() [load.go]: failed to load agent roles from source %q: %w", sources[sourceIndex].Path, loadErr)
		}
		for roleName, roleConfig := range sourceAgentRoleConfigs {
			existingRoleConfig, exists := mergedAgentRoleConfigs[roleName]
			if !exists {
				mergedAgentRoleConfigs[roleName] = roleConfig.Clone()
				continue
			}

			existingRoleConfig.Merge(roleConfig)
		}

		sourceAgentConfigFiles, loadErr := loadAgentConfigFilesFromSource(sources[sourceIndex])
		if loadErr != nil {
			return nil, fmt.Errorf("CswConfigLoad() [load.go]: failed to load agent config files from source %q: %w", sources[sourceIndex].Path, loadErr)
		}
		for subdir, files := range sourceAgentConfigFiles {
			existingFiles, exists := mergedAgentConfigFiles[subdir]
			if !exists {
				existingFiles = make(map[string]string)
				mergedAgentConfigFiles[subdir] = existingFiles
			}

			for filename, content := range files {
				existingFiles[filename] = content
			}
		}
	}

	return &CswConfig{
		GlobalConfig:          mergedGlobalConfig,
		AgentRoleConfigs:      mergedAgentRoleConfigs,
		ModelProviderConfigs: mergedModelProviderConfigs,
		AgentConfigFiles:      mergedAgentConfigFiles,
		ModelAliases:         mergedModelAliases,
	}, nil
}

func parseConfigLoadPath(path string) []string {
	if strings.TrimSpace(path) == "" {
		return []string{defaultsConfigPathMarker, "~/.config/csw", ".csw"}
	}

	parts := strings.Split(path, ":")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed == "" {
			continue
		}
		result = append(result, trimmed)
	}

	return result
}

func resolveConfigLoadSources(path string) ([]configLoadSource, error) {
	configPathParts := parseConfigLoadPath(path)
	sources := make([]configLoadSource, 0, len(configPathParts))

	for _, rawPathPart := range configPathParts {
		if rawPathPart == defaultsConfigPathMarker {
			embeddedSourceFS, err := fs.Sub(embeddedDefaultsFS, embeddedDefaultsRoot)
			if err != nil {
				return nil, fmt.Errorf("resolveConfigLoadSources() [load.go]: failed to initialize embedded defaults filesystem: %w", err)
			}

			sources = append(sources, configLoadSource{
				Path:          defaultsConfigPathMarker,
				FS:            embeddedSourceFS,
				ToolDirPrefix: defaultsConfigPathMarker,
			})
			continue
		}

		resolvedPathPart, err := resolveConfigPathPart(rawPathPart)
		if err != nil {
			return nil, err
		}

		pathInfo, statErr := os.Stat(resolvedPathPart)
		if statErr != nil {
			if os.IsNotExist(statErr) {
				continue
			}
			return nil, fmt.Errorf("resolveConfigLoadSources() [load.go]: failed to read path %q: %w", resolvedPathPart, statErr)
		}

		if !pathInfo.IsDir() {
			return nil, fmt.Errorf("resolveConfigLoadSources() [load.go]: path is not a directory: %q", resolvedPathPart)
		}

		sources = append(sources, configLoadSource{
			Path:          resolvedPathPart,
			FS:            os.DirFS(resolvedPathPart),
			ToolDirPrefix: resolvedPathPart,
		})
	}

	return sources, nil
}

func resolveConfigPathPart(pathPart string) (string, error) {
	if strings.HasPrefix(pathPart, "@PROJ/") {
		workingDir, err := os.Getwd()
		if err != nil {
			return "", fmt.Errorf("resolveConfigPathPart() [load.go]: failed to get current working directory: %w", err)
		}
		return filepath.Join(workingDir, strings.TrimPrefix(pathPart, "@PROJ/")), nil
	}

	if strings.HasPrefix(pathPart, "~/") {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("resolveConfigPathPart() [load.go]: failed to get user home directory: %w", err)
		}
		return filepath.Join(homeDir, strings.TrimPrefix(pathPart, "~/")), nil
	}

	if strings.HasPrefix(pathPart, "./") {
		workingDir, err := os.Getwd()
		if err != nil {
			return "", fmt.Errorf("resolveConfigPathPart() [load.go]: failed to get current working directory: %w", err)
		}
		return filepath.Join(workingDir, strings.TrimPrefix(pathPart, "./")), nil
	}

	return pathPart, nil
}

func loadGlobalConfigFromSource(source configLoadSource) (*GlobalConfig, error) {
	globalConfigPath := "global.json"
	data, err := fs.ReadFile(source.FS, globalConfigPath)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return &GlobalConfig{}, nil
		}
		return nil, fmt.Errorf("loadGlobalConfigFromSource() [load.go]: failed to read %q: %w", filepath.Join(source.Path, globalConfigPath), err)
	}

	var globalConfig GlobalConfig
	if err := json.Unmarshal(data, &globalConfig); err != nil {
		return nil, fmt.Errorf("loadGlobalConfigFromSource() [load.go]: failed to parse %q: %w", filepath.Join(source.Path, globalConfigPath), err)
	}

	return &globalConfig, nil
}

func loadModelProviderConfigsFromSource(source configLoadSource) (map[string]*ModelProviderConfig, error) {
	modelConfigsPath := "models"
	entries, err := fs.ReadDir(source.FS, modelConfigsPath)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return map[string]*ModelProviderConfig{}, nil
		}
		return nil, fmt.Errorf("loadModelProviderConfigsFromSource() [load.go]: failed to read %q: %w", filepath.Join(source.Path, modelConfigsPath), err)
	}

	modelProviderConfigs := make(map[string]*ModelProviderConfig)
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		extension := strings.ToLower(filepath.Ext(entry.Name()))
		if extension != ".json" {
			continue
		}

		providerName := strings.TrimSuffix(entry.Name(), filepath.Ext(entry.Name()))
		providerPath := path.Join(modelConfigsPath, entry.Name())
		data, readErr := fs.ReadFile(source.FS, providerPath)
		if readErr != nil {
			return nil, fmt.Errorf("loadModelProviderConfigsFromSource() [load.go]: failed to read %q: %w", filepath.Join(source.Path, filepath.FromSlash(providerPath)), readErr)
		}

		var providerConfig ModelProviderConfig
		if unmarshalErr := json.Unmarshal(data, &providerConfig); unmarshalErr != nil {
			return nil, fmt.Errorf("loadModelProviderConfigsFromSource() [load.go]: failed to parse %q: %w", filepath.Join(source.Path, filepath.FromSlash(providerPath)), unmarshalErr)
		}

		providerConfig.Name = providerName
		modelProviderConfigs[providerName] = &providerConfig
	}

	return modelProviderConfigs, nil
}

func loadModelAliasesFromSource(source configLoadSource) (map[string]ModelAliasValue, error) {
	aliasesPath := "model_aliases.jsonl"
	data, err := fs.ReadFile(source.FS, aliasesPath)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return map[string]ModelAliasValue{}, nil
		}
		return nil, fmt.Errorf("loadModelAliasesFromSource() [load.go]: failed to read %q: %w", filepath.Join(source.Path, aliasesPath), err)
	}

	trimmed := strings.TrimSpace(string(data))
	if trimmed == "" {
		return map[string]ModelAliasValue{}, nil
	}

	aliases := make(map[string]ModelAliasValue)
	if strings.HasPrefix(trimmed, "{") {
		if err := json.Unmarshal([]byte(trimmed), &aliases); err != nil {
			return nil, fmt.Errorf("loadModelAliasesFromSource() [load.go]: failed to parse %q: %w", filepath.Join(source.Path, aliasesPath), err)
		}
		return aliases, nil
	}

	lines := strings.Split(trimmed, "\n")
	for lineIndex, line := range lines {
		trimmedLine := strings.TrimSpace(line)
		if trimmedLine == "" {
			continue
		}

		entry := make(map[string]ModelAliasValue)
		if err := json.Unmarshal([]byte(trimmedLine), &entry); err != nil {
			return nil, fmt.Errorf("loadModelAliasesFromSource() [load.go]: failed to parse %q line %d: %w", filepath.Join(source.Path, aliasesPath), lineIndex+1, err)
		}

		for aliasName, aliasValue := range entry {
			aliases[aliasName] = aliasValue
		}
	}

	return aliases, nil
}

func loadAgentRoleConfigsFromSource(source configLoadSource) (map[string]*AgentRoleConfig, error) {
	rolesDir := "roles"
	entries, err := fs.ReadDir(source.FS, rolesDir)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return map[string]*AgentRoleConfig{}, nil
		}
		return nil, fmt.Errorf("loadAgentRoleConfigsFromSource() [load.go]: failed to read %q: %w", filepath.Join(source.Path, rolesDir), err)
	}

	agentRoleConfigs := make(map[string]*AgentRoleConfig)
	toolFragments, err := loadToolFragmentsFromSource(source)
	if err != nil {
		return nil, err
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		roleName := entry.Name()
		roleDir := path.Join(rolesDir, roleName)
		roleConfigPath := path.Join(roleDir, "config.json")
		data, readErr := fs.ReadFile(source.FS, roleConfigPath)
		if readErr != nil {
			if errors.Is(readErr, fs.ErrNotExist) {
				if roleName == "all" {
					promptFragments, loadErr := loadPromptFragmentsFromSource(source, roleDir)
					if loadErr != nil {
						return nil, loadErr
					}
					agentRoleConfigs["all"] = &AgentRoleConfig{
						Name:            "all",
						PromptFragments: promptFragments,
						ToolFragments:   cloneStringMapLoad(toolFragments),
					}
					continue
				}
				return nil, fmt.Errorf("loadAgentRoleConfigsFromSource() [load.go]: role %q missing config.json", roleName)
			}
			return nil, fmt.Errorf("loadAgentRoleConfigsFromSource() [load.go]: failed to read %q: %w", filepath.Join(source.Path, filepath.FromSlash(roleConfigPath)), readErr)
		}

		var roleConfig AgentRoleConfig
		if unmarshalErr := json.Unmarshal(data, &roleConfig); unmarshalErr != nil {
			return nil, fmt.Errorf("loadAgentRoleConfigsFromSource() [load.go]: failed to parse %q: %w", filepath.Join(source.Path, filepath.FromSlash(roleConfigPath)), unmarshalErr)
		}

		if roleConfig.Name == "" {
			roleConfig.Name = roleName
		}

		promptFragments, loadErr := loadPromptFragmentsFromSource(source, roleDir)
		if loadErr != nil {
			return nil, loadErr
		}
		roleConfig.PromptFragments = promptFragments
		roleConfig.ToolFragments = cloneStringMapLoad(toolFragments)

		agentRoleConfigs[roleConfig.Name] = &roleConfig
	}

	return agentRoleConfigs, nil
}

func loadPromptFragmentsFromSource(source configLoadSource, roleDir string) (map[string]string, error) {
	entries, err := fs.ReadDir(source.FS, roleDir)
	if err != nil {
		return nil, fmt.Errorf("loadPromptFragmentsFromSource() [load.go]: failed to read %q: %w", filepath.Join(source.Path, filepath.FromSlash(roleDir)), err)
	}

	fragments := make(map[string]string)
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		if strings.ToLower(filepath.Ext(entry.Name())) != ".md" {
			continue
		}

		fragmentPath := path.Join(roleDir, entry.Name())
		data, readErr := fs.ReadFile(source.FS, fragmentPath)
		if readErr != nil {
			return nil, fmt.Errorf("loadPromptFragmentsFromSource() [load.go]: failed to read %q: %w", filepath.Join(source.Path, filepath.FromSlash(fragmentPath)), readErr)
		}

		fragmentName := strings.TrimSuffix(entry.Name(), filepath.Ext(entry.Name()))
		fragments[fragmentName] = string(data)
	}

	return fragments, nil
}

func loadToolFragmentsFromSource(source configLoadSource) (map[string]string, error) {
	toolsDir := "tools"
	entries, err := fs.ReadDir(source.FS, toolsDir)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return map[string]string{}, nil
		}
		return nil, fmt.Errorf("loadToolFragmentsFromSource() [load.go]: failed to read %q: %w", filepath.Join(source.Path, toolsDir), err)
	}

	fragments := make(map[string]string)
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		toolName := entry.Name()
		toolDir := path.Join(toolsDir, toolName)
		fragments[fmt.Sprintf("%s/.tooldir", toolName)] = filepath.Join(source.ToolDirPrefix, filepath.FromSlash(toolDir))

		toolEntries, readDirErr := fs.ReadDir(source.FS, toolDir)
		if readDirErr != nil {
			return nil, fmt.Errorf("loadToolFragmentsFromSource() [load.go]: failed to read %q: %w", filepath.Join(source.Path, filepath.FromSlash(toolDir)), readDirErr)
		}

		for _, toolEntry := range toolEntries {
			if toolEntry.IsDir() {
				continue
			}

			toolFilePath := path.Join(toolDir, toolEntry.Name())
			data, readErr := fs.ReadFile(source.FS, toolFilePath)
			if readErr != nil {
				return nil, fmt.Errorf("loadToolFragmentsFromSource() [load.go]: failed to read %q: %w", filepath.Join(source.Path, filepath.FromSlash(toolFilePath)), readErr)
			}

			fragments[fmt.Sprintf("%s/%s", toolName, toolEntry.Name())] = string(data)
		}
	}

	return fragments, nil
}

func cloneStringMapLoad(input map[string]string) map[string]string {
	cloned := make(map[string]string, len(input))
	for key, value := range input {
		cloned[key] = value
	}

	return cloned
}

func loadAgentConfigFilesFromSource(source configLoadSource) (map[string]map[string]string, error) {
	agentDir := "agent"
	if _, err := fs.Stat(source.FS, agentDir); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return map[string]map[string]string{}, nil
		}
		return nil, fmt.Errorf("loadAgentConfigFilesFromSource() [load.go]: failed to stat %q: %w", filepath.Join(source.Path, agentDir), err)
	}

	result := make(map[string]map[string]string)
	walkErr := fs.WalkDir(source.FS, agentDir, func(filePath string, dirEntry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return fmt.Errorf("loadAgentConfigFilesFromSource() [load.go]: failed to walk %q: %w", filepath.Join(source.Path, filepath.FromSlash(filePath)), walkErr)
		}

		if dirEntry.IsDir() {
			return nil
		}

		data, readErr := fs.ReadFile(source.FS, filePath)
		if readErr != nil {
			return fmt.Errorf("loadAgentConfigFilesFromSource() [load.go]: failed to read %q: %w", filepath.Join(source.Path, filepath.FromSlash(filePath)), readErr)
		}

		if !strings.HasPrefix(filePath, agentDir+"/") {
			return fmt.Errorf("loadAgentConfigFilesFromSource() [load.go]: failed to resolve relative path for %q", filepath.Join(source.Path, filepath.FromSlash(filePath)))
		}
		relPath := strings.TrimPrefix(filePath, agentDir+"/")

		subdir := path.Dir(relPath)
		if subdir == "." {
			subdir = ""
		}

		if result[subdir] == nil {
			result[subdir] = make(map[string]string)
		}

		filename := path.Base(relPath)
		result[subdir][filename] = string(data)
		return nil
	})
	if walkErr != nil {
		return nil, walkErr
	}

	return result, nil
}
