package system

import (
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/rlewczuk/csw/pkg/conf"
	"github.com/rlewczuk/csw/pkg/runner"
	"github.com/rlewczuk/csw/pkg/vcs"
)

const (
	// defaultGitAuthorName is used in container mode when git identity cannot be resolved.
	defaultGitAuthorName = "CSW"
	// defaultGitAuthorEmail is used in container mode when git identity cannot be resolved.
	defaultGitAuthorEmail = "csw@example.com"
)

// gitLookPathFunc resolves executable path for git and can be overridden in tests.
var gitLookPathFunc = exec.LookPath

// gitConfigValueFunc resolves git config values and can be overridden in tests.
var gitConfigValueFunc = vcs.ReadGitConfigValue

// containerRuntimeConfig describes effective container runtime setup.
type containerRuntimeConfig struct {
	Enabled bool
	Image   string
	Mounts  map[string]string
	Env     map[string]string
}

// ResolveContainerRuntimeConfig resolves effective container runtime setup.
func ResolveContainerRuntimeConfig(globalConfig *conf.GlobalConfig, params BuildSystemParams, effectiveWorkDir string, shadowDir string) (containerRuntimeConfig, error) {
	var runtimeConfig containerRuntimeConfig
	containerDefaults := conf.ContainerConfig{}
	if globalConfig != nil && globalConfig.Defaults.Container != nil {
		containerDefaults = globalConfig.Defaults.Container.Clone()
	}

	runtimeConfig.Enabled = containerDefaults.Enabled
	if params.ContainerEnabled {
		runtimeConfig.Enabled = true
	}
	if params.ContainerDisabled {
		runtimeConfig.Enabled = false
	}

	if !runtimeConfig.Enabled {
		return runtimeConfig, nil
	}

	runtimeConfig.Image = strings.TrimSpace(params.ContainerImage)
	if runtimeConfig.Image == "" {
		runtimeConfig.Image = strings.TrimSpace(containerDefaults.Image)
	}
	if runtimeConfig.Image == "" {
		return runtimeConfig, fmt.Errorf("ResolveContainerRuntimeConfig() [bootstrap_container.go]: container image is required when container mode is enabled")
	}

	mountSpecs := make([]string, 0, len(containerDefaults.Mounts)+len(params.ContainerMounts))
	mountSpecs = append(mountSpecs, containerDefaults.Mounts...)
	mountSpecs = append(mountSpecs, params.ContainerMounts...)
	runtimeConfig.Mounts = map[string]string{effectiveWorkDir: effectiveWorkDir}
	if strings.TrimSpace(shadowDir) != "" {
		runtimeConfig.Mounts[shadowDir] = shadowDir
	}
	for _, mountSpec := range mountSpecs {
		hostPath, containerPath, err := ParseContainerMountSpec(mountSpec)
		if err != nil {
			return runtimeConfig, err
		}
		if !filepath.IsAbs(hostPath) {
			hostPath, err = filepath.Abs(hostPath)
			if err != nil {
				return runtimeConfig, fmt.Errorf("ResolveContainerRuntimeConfig() [bootstrap_container.go]: failed to resolve absolute mount host path %q: %w", hostPath, err)
			}
		}
		if _, err := os.Stat(hostPath); err != nil {
			return runtimeConfig, fmt.Errorf("ResolveContainerRuntimeConfig() [bootstrap_container.go]: invalid mount host path %q: %w", hostPath, err)
		}
		runtimeConfig.Mounts[containerPath] = hostPath
	}

	envSpecs := make([]string, 0, len(containerDefaults.Env)+len(params.ContainerEnv))
	envSpecs = append(envSpecs, containerDefaults.Env...)
	envSpecs = append(envSpecs, params.ContainerEnv...)
	if len(envSpecs) > 0 {
		runtimeConfig.Env = make(map[string]string, len(envSpecs))
		for _, envSpec := range envSpecs {
			key, value, err := ParseContainerEnvSpec(envSpec)
			if err != nil {
				return runtimeConfig, err
			}
			runtimeConfig.Env[key] = value
		}
	}

	return runtimeConfig, nil
}

// ParseContainerMountSpec parses mount in host_path:container_path format.
func ParseContainerMountSpec(mountSpec string) (string, string, error) {
	parts := strings.SplitN(mountSpec, ":", 2)
	if len(parts) != 2 {
		return "", "", fmt.Errorf("ParseContainerMountSpec() [bootstrap_container.go]: mount must be in host_path:container_path format: %q", mountSpec)
	}
	hostPath := strings.TrimSpace(parts[0])
	containerPath := strings.TrimSpace(parts[1])
	if hostPath == "" || containerPath == "" {
		return "", "", fmt.Errorf("ParseContainerMountSpec() [bootstrap_container.go]: mount must be in host_path:container_path format: %q", mountSpec)
	}

	return hostPath, containerPath, nil
}

// ParseContainerEnvSpec parses env var in KEY=VALUE format.
func ParseContainerEnvSpec(envSpec string) (string, string, error) {
	parts := strings.SplitN(envSpec, "=", 2)
	if len(parts) != 2 {
		return "", "", fmt.Errorf("ParseContainerEnvSpec() [bootstrap_container.go]: env must be in KEY=VALUE format: %q", envSpec)
	}
	key := strings.TrimSpace(parts[0])
	if key == "" {
		return "", "", fmt.Errorf("ParseContainerEnvSpec() [bootstrap_container.go]: env key cannot be empty: %q", envSpec)
	}

	return key, parts[1], nil
}

func copyStringMap(values map[string]string) map[string]string {
	if len(values) == 0 {
		return nil
	}

	cloned := make(map[string]string, len(values))
	for key, value := range values {
		cloned[key] = value
	}

	return cloned
}

func parseContainerImageInfo(reference string) runner.ContainerImageInfo {
	trimmed := strings.TrimSpace(reference)
	info := runner.ContainerImageInfo{
		Reference: trimmed,
		Name:      trimmed,
		Tag:       "latest",
		Version:   "latest",
	}
	if trimmed == "" {
		return info
	}

	name := trimmed
	tag := "latest"
	lastColon := strings.LastIndex(trimmed, ":")
	lastSlash := strings.LastIndex(trimmed, "/")
	if lastColon > lastSlash {
		name = trimmed[:lastColon]
		tag = trimmed[lastColon+1:]
	}
	if strings.TrimSpace(tag) == "" {
		tag = "latest"
	}

	info.Name = name
	info.Tag = tag
	info.Version = tag
	return info
}

// ContainerUserIdentity stores host user identity mirrored in container mode.
type ContainerUserIdentity struct {
	UID       int
	GID       int
	UserName  string
	GroupName string
	HomeDir   string
}

func resolveCurrentUserIdentity() (ContainerUserIdentity, error) {
	var identity ContainerUserIdentity

	currentUser, err := user.Current()
	if err != nil {
		return identity, fmt.Errorf("resolveCurrentUserIdentity() [bootstrap_container.go]: failed to get current user: %w", err)
	}

	uid, err := strconv.Atoi(currentUser.Uid)
	if err != nil {
		return identity, fmt.Errorf("resolveCurrentUserIdentity() [bootstrap_container.go]: failed to parse uid: %w", err)
	}

	gid, err := strconv.Atoi(currentUser.Gid)
	if err != nil {
		return identity, fmt.Errorf("resolveCurrentUserIdentity() [bootstrap_container.go]: failed to parse gid: %w", err)
	}

	group, err := user.LookupGroupId(currentUser.Gid)
	if err != nil {
		return identity, fmt.Errorf("resolveCurrentUserIdentity() [bootstrap_container.go]: failed to lookup group by gid: %w", err)
	}

	if currentUser.Username == "" {
		return identity, fmt.Errorf("resolveCurrentUserIdentity() [bootstrap_container.go]: current user name is empty")
	}

	if currentUser.HomeDir == "" {
		return identity, fmt.Errorf("resolveCurrentUserIdentity() [bootstrap_container.go]: current user home directory is empty")
	}

	if group.Name == "" {
		return identity, fmt.Errorf("resolveCurrentUserIdentity() [bootstrap_container.go]: current user group name is empty")
	}

	identity.UID = uid
	identity.GID = gid
	identity.UserName = currentUser.Username
	identity.GroupName = group.Name
	identity.HomeDir = currentUser.HomeDir

	return identity, nil
}

// ResolveContainerGitAuthorIdentity returns git author identity for container mode.
func ResolveContainerGitAuthorIdentity() (string, string) {
	name := defaultGitAuthorName
	email := defaultGitAuthorEmail

	if _, err := gitLookPathFunc("git"); err != nil {
		return name, email
	}

	resolvedName, err := gitConfigValueFunc("user.name")
	if err == nil && resolvedName != "" {
		name = resolvedName
	}

	resolvedEmail, err := gitConfigValueFunc("user.email")
	if err == nil && resolvedEmail != "" {
		email = resolvedEmail
	}

	return name, email
}
