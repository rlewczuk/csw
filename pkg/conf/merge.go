package conf

import "strings"

// Clone returns a deep copy of ToolSelectionConfig.
func (c ToolSelectionConfig) Clone() ToolSelectionConfig {
	cloned := ToolSelectionConfig{
		Default: make(map[string]bool, len(c.Default)),
		Tags:    make(map[string]map[string]bool, len(c.Tags)),
	}

	for toolName, enabled := range c.Default {
		cloned.Default[toolName] = enabled
	}

	for tag, tools := range c.Tags {
		clonedTools := make(map[string]bool, len(tools))
		for toolName, enabled := range tools {
			clonedTools[toolName] = enabled
		}
		cloned.Tags[tag] = clonedTools
	}

	return cloned
}

// Merge merges overrides into ToolSelectionConfig.
//
// Default and tag-specific tool values are merged per key, and keys from override
// replace values from the receiver.
func (c *ToolSelectionConfig) Merge(override ToolSelectionConfig) {
	if c.Default == nil {
		c.Default = make(map[string]bool)
	}
	for toolName, enabled := range override.Default {
		c.Default[toolName] = enabled
	}

	if c.Tags == nil {
		c.Tags = make(map[string]map[string]bool)
	}
	for tag, tools := range override.Tags {
		if c.Tags[tag] == nil {
			c.Tags[tag] = make(map[string]bool)
		}
		for toolName, enabled := range tools {
			c.Tags[tag][toolName] = enabled
		}
	}
}

// Clone returns a deep copy of ContainerConfig.
func (c ContainerConfig) Clone() ContainerConfig {
	cloned := ContainerConfig{
		Mounts:  make([]string, len(c.Mounts)),
		Env:     make([]string, len(c.Env)),
		Image:   c.Image,
		Enabled: c.Enabled,
	}
	copy(cloned.Mounts, c.Mounts)
	copy(cloned.Env, c.Env)

	return cloned
}

// Merge merges overrides into ContainerConfig.
func (c *ContainerConfig) Merge(override ContainerConfig) {
	if len(override.Mounts) > 0 {
		c.Mounts = append([]string(nil), override.Mounts...)
	}
	if len(override.Env) > 0 {
		c.Env = append([]string(nil), override.Env...)
	}
	if override.Image != "" {
		c.Image = override.Image
	}
	if override.Enabled {
		c.Enabled = true
	}
}

// MergeFrom merges overrides into RunDefaultsConfig.
func (c *RunDefaultsConfig) MergeFrom(override RunDefaultsConfig) {
	if override.DefaultProvider != "" {
		c.DefaultProvider = override.DefaultProvider
	}
	if override.DefaultRole != "" {
		c.DefaultRole = override.DefaultRole
	}
	if override.Container != nil {
		overrideHasValue := override.Container.Enabled || override.Container.Image != "" || len(override.Container.Mounts) > 0 || len(override.Container.Env) > 0
		if c.Container == nil {
			if !overrideHasValue {
				goto mergeOtherFields
			}
			containerCopy := override.Container.Clone()
			c.Container = &containerCopy
		} else {
			c.Container.Merge(*override.Container)
		}
	}

mergeOtherFields:
	if override.Model != "" {
		c.Model = override.Model
	}
	if override.Worktree != "" {
		c.Worktree = override.Worktree
	}
	if override.Merge {
		c.Merge = true
	}
	if override.LogLLMRequests {
		c.LogLLMRequests = true
	}
	if override.LogLLMRequestsRaw {
		c.LogLLMRequestsRaw = true
	}
	if override.Thinking != "" {
		c.Thinking = override.Thinking
	}
	if override.LSPServer != "" {
		c.LSPServer = override.LSPServer
	}
	if override.GitUserName != "" {
		c.GitUserName = override.GitUserName
	}
	if override.GitUserEmail != "" {
		c.GitUserEmail = override.GitUserEmail
	}
	if override.MaxThreads > 0 {
		c.MaxThreads = override.MaxThreads
	}
	if override.TaskDir != "" {
		c.TaskDir = override.TaskDir
	}
	if override.ShadowDir != "" {
		c.ShadowDir = override.ShadowDir
	}
	if override.AllowAllPermissions {
		c.AllowAllPermissions = true
	}
	if len(override.VFSAllow) > 0 {
		c.VFSAllow = append([]string(nil), override.VFSAllow...)
	}
}

// Clone returns a deep copy of GlobalConfig.
func (c *GlobalConfig) Clone() *GlobalConfig {
	if c == nil {
		return nil
	}

	cloned := &GlobalConfig{
		ModelTags:                  make([]ModelTagMapping, len(c.ModelTags)),
		ToolSelection:              c.ToolSelection.Clone(),
		ContextCompactionThreshold: c.ContextCompactionThreshold,
		LLMRetryMaxAttempts:        c.LLMRetryMaxAttempts,
		LLMRetryMaxBackoffSeconds:  c.LLMRetryMaxBackoffSeconds,
		Defaults:                   c.Defaults,
		ShadowPaths:                append([]string(nil), c.ShadowPaths...),
	}
	if c.Defaults.Container != nil {
		containerCopy := c.Defaults.Container.Clone()
		cloned.Defaults.Container = &containerCopy
	}
	if len(c.Defaults.VFSAllow) > 0 {
		cloned.Defaults.VFSAllow = append([]string(nil), c.Defaults.VFSAllow...)
	}
	copy(cloned.ModelTags, c.ModelTags)

	return cloned
}

// Merge merges overrides into GlobalConfig.
func (c *GlobalConfig) Merge(override *GlobalConfig) {
	if override == nil {
		return
	}

	c.ModelTags = append(c.ModelTags, override.ModelTags...)
	c.ToolSelection.Merge(override.ToolSelection)

	if override.ContextCompactionThreshold > 0 {
		c.ContextCompactionThreshold = override.ContextCompactionThreshold
	}
	if override.LLMRetryMaxAttempts > 0 {
		c.LLMRetryMaxAttempts = override.LLMRetryMaxAttempts
	}
	if override.LLMRetryMaxBackoffSeconds > 0 {
		c.LLMRetryMaxBackoffSeconds = override.LLMRetryMaxBackoffSeconds
	}

	c.Defaults.MergeFrom(override.Defaults)
	if len(override.ShadowPaths) > 0 {
		c.ShadowPaths = append([]string(nil), override.ShadowPaths...)
	}

}

// Clone returns a deep copy of AgentRoleConfig.
func (c *AgentRoleConfig) Clone() *AgentRoleConfig {
	if c == nil {
		return nil
	}

	cloned := *c

	if c.VFSPrivileges != nil {
		cloned.VFSPrivileges = make(map[string]FileAccess, len(c.VFSPrivileges))
		for key, value := range c.VFSPrivileges {
			cloned.VFSPrivileges[key] = value
		}
	}

	if c.ToolsAccess != nil {
		cloned.ToolsAccess = make(map[string]AccessFlag, len(c.ToolsAccess))
		for key, value := range c.ToolsAccess {
			cloned.ToolsAccess[key] = value
		}
	}

	if c.RunPrivileges != nil {
		cloned.RunPrivileges = make(map[string]AccessFlag, len(c.RunPrivileges))
		for key, value := range c.RunPrivileges {
			cloned.RunPrivileges[key] = value
		}
	}

	if c.PromptFragments != nil {
		cloned.PromptFragments = make(map[string]string, len(c.PromptFragments))
		for key, value := range c.PromptFragments {
			cloned.PromptFragments[key] = value
		}
	}

	if c.ToolFragments != nil {
		cloned.ToolFragments = make(map[string]string, len(c.ToolFragments))
		for key, value := range c.ToolFragments {
			cloned.ToolFragments[key] = value
		}
	}

	if c.HiddenPatterns != nil {
		cloned.HiddenPatterns = make([]string, len(c.HiddenPatterns))
		copy(cloned.HiddenPatterns, c.HiddenPatterns)
	}

	if c.Aliases != nil {
		cloned.Aliases = make([]string, len(c.Aliases))
		copy(cloned.Aliases, c.Aliases)
	}

	return &cloned
}

// Merge merges overrides into AgentRoleConfig.
//
// Scalar fields and privilege maps are replaced by override values. PromptFragments
// and ToolFragments are merged by key; empty or whitespace-only content in override
// removes an existing fragment. HiddenPatterns are appended.
func (c *AgentRoleConfig) Merge(override *AgentRoleConfig) {
	if override == nil {
		return
	}

	existingPromptFragments := cloneStringMap(c.PromptFragments)
	existingToolFragments := cloneStringMap(c.ToolFragments)
	existingHiddenPatterns := append([]string(nil), c.HiddenPatterns...)

	clonedOverride := override.Clone()
	*c = *clonedOverride

	c.PromptFragments = existingPromptFragments
	mergeFragments(c.PromptFragments, override.PromptFragments)

	c.ToolFragments = existingToolFragments
	mergeFragments(c.ToolFragments, override.ToolFragments)

	c.HiddenPatterns = append(existingHiddenPatterns, override.HiddenPatterns...)
	if override.Aliases != nil {
		c.Aliases = append([]string(nil), override.Aliases...)
	}
}

func cloneStringMap(value map[string]string) map[string]string {
	if value == nil {
		return make(map[string]string)
	}

	cloned := make(map[string]string, len(value))
	for key, item := range value {
		cloned[key] = item
	}

	return cloned
}

func mergeFragments(target map[string]string, overrides map[string]string) {
	for filename, content := range overrides {
		if strings.TrimSpace(content) == "" {
			delete(target, filename)
			continue
		}

		target[filename] = content
	}
}
