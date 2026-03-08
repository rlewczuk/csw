package vfs

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestShadowVFS_ReadWriteFile(t *testing.T) {
	baseDir := t.TempDir()
	shadowDir := t.TempDir()

	base, err := NewLocalVFS(baseDir, nil, nil)
	require.NoError(t, err)
	shadow, err := NewLocalVFS(shadowDir, nil, nil)
	require.NoError(t, err)

	require.NoError(t, base.WriteFile("AGENTS.md", []byte("base-agent")))
	require.NoError(t, base.WriteFile("README.md", []byte("base-readme")))
	require.NoError(t, shadow.WriteFile("AGENTS.md", []byte("shadow-agent")))

	overlay, err := NewShadowVFS(base, shadow, []string{"AGENTS.md", "**/AGENTS.md"})
	require.NoError(t, err)

	agents, err := overlay.ReadFile("AGENTS.md")
	require.NoError(t, err)
	assert.Equal(t, "shadow-agent", string(agents))

	readme, err := overlay.ReadFile("README.md")
	require.NoError(t, err)
	assert.Equal(t, "base-readme", string(readme))

	require.NoError(t, overlay.WriteFile("AGENTS.md", []byte("shadow-updated")))

	shadowAgents, err := shadow.ReadFile("AGENTS.md")
	require.NoError(t, err)
	assert.Equal(t, "shadow-updated", string(shadowAgents))

	baseAgents, err := base.ReadFile("AGENTS.md")
	require.NoError(t, err)
	assert.Equal(t, "base-agent", string(baseAgents))
}

func TestShadowVFS_ShadowedPathMissingInShadowIsInvisible(t *testing.T) {
	baseDir := t.TempDir()
	shadowDir := t.TempDir()

	base, err := NewLocalVFS(baseDir, nil, nil)
	require.NoError(t, err)
	shadow, err := NewLocalVFS(shadowDir, nil, nil)
	require.NoError(t, err)

	require.NoError(t, base.WriteFile("AGENTS.md", []byte("only-base")))

	overlay, err := NewShadowVFS(base, shadow, []string{"AGENTS.md", "**/AGENTS.md"})
	require.NoError(t, err)

	_, err = overlay.ReadFile("AGENTS.md")
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrFileNotFound)
}

func TestShadowVFS_ListAndFindMerge(t *testing.T) {
	baseDir := t.TempDir()
	shadowDir := t.TempDir()

	base, err := NewLocalVFS(baseDir, nil, nil)
	require.NoError(t, err)
	shadow, err := NewLocalVFS(shadowDir, nil, nil)
	require.NoError(t, err)

	require.NoError(t, base.WriteFile("AGENTS.md", []byte("base-agent")))
	require.NoError(t, base.WriteFile("pkg/AGENTS.md", []byte("base-pkg-agent")))
	require.NoError(t, base.WriteFile("pkg/code.go", []byte("package pkg")))
	require.NoError(t, shadow.WriteFile("AGENTS.md", []byte("shadow-agent")))

	overlay, err := NewShadowVFS(base, shadow, []string{"AGENTS.md", "**/AGENTS.md"})
	require.NoError(t, err)

	files, err := overlay.ListFiles(".", true)
	require.NoError(t, err)
	assert.Contains(t, files, "AGENTS.md")
	assert.Contains(t, files, "pkg/code.go")
	assert.NotContains(t, files, "pkg/AGENTS.md")

	found, err := overlay.FindFiles("**/AGENTS.md", true)
	require.NoError(t, err)
	assert.Equal(t, []string{"AGENTS.md"}, found)
}

func TestShadowVFS_AbsolutePathRouting(t *testing.T) {
	baseDir := t.TempDir()
	shadowDir := t.TempDir()

	base, err := NewLocalVFS(baseDir, nil, nil)
	require.NoError(t, err)
	shadow, err := NewLocalVFS(shadowDir, nil, nil)
	require.NoError(t, err)

	overlay, err := NewShadowVFS(base, shadow, []string{"AGENTS.md"})
	require.NoError(t, err)

	agentsPath := filepath.Join(baseDir, "AGENTS.md")
	require.NoError(t, overlay.WriteFile(agentsPath, []byte("from-absolute")))

	shadowAgents, err := shadow.ReadFile("AGENTS.md")
	require.NoError(t, err)
	assert.Equal(t, "from-absolute", string(shadowAgents))
}
