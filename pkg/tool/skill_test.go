package tool

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSkillTool_GetDescription(t *testing.T) {
	t.Run("lists available skills", func(t *testing.T) {
		workdir := t.TempDir()
		createSkillFixture(t, workdir, "alpha", "Alpha skill", "Do alpha")
		createSkillFixture(t, workdir, "beta", "Beta skill", "Do beta")

		tool := NewSkillTool(workdir)
		description, overwrite := tool.GetDescription()

		assert.False(t, overwrite)
		assert.Contains(t, description, "<available_skills>")
		assert.Contains(t, description, "<name>alpha</name>")
		assert.Contains(t, description, "<description>Alpha skill</description>")
		assert.Contains(t, description, "<name>beta</name>")
	})

	t.Run("picks up latest changes without cache", func(t *testing.T) {
		workdir := t.TempDir()
		createSkillFixture(t, workdir, "alpha", "Alpha skill", "Do alpha")

		tool := NewSkillTool(workdir)
		first, _ := tool.GetDescription()
		assert.Contains(t, first, "<name>alpha</name>")
		assert.NotContains(t, first, "<name>beta</name>")

		createSkillFixture(t, workdir, "beta", "Beta skill", "Do beta")
		second, _ := tool.GetDescription()
		assert.Contains(t, second, "<name>alpha</name>")
		assert.Contains(t, second, "<name>beta</name>")
	})

	t.Run("returns none marker when directory is missing", func(t *testing.T) {
		workdir := t.TempDir()
		tool := NewSkillTool(workdir)

		description, overwrite := tool.GetDescription()

		assert.False(t, overwrite)
		assert.Contains(t, description, "No skills are currently available")
	})
}

func TestSkillTool_Execute(t *testing.T) {
	t.Run("loads skill by name", func(t *testing.T) {
		workdir := t.TempDir()
		createSkillFixture(t, workdir, "alpha", "Alpha skill", "Follow alpha steps.")

		tool := NewSkillTool(workdir)
		response := tool.Execute(&ToolCall{
			ID:       "skill-1",
			Function: "skill",
			Arguments: NewToolValue(map[string]any{
				"name": "alpha",
			}),
		})

		require.NoError(t, response.Error)
		assert.True(t, response.Done)
		assert.Equal(t, "alpha", response.Result.String("name"))
		assert.Contains(t, response.Result.String("content"), "Follow alpha steps.")
		assert.Contains(t, response.Result.String("output"), "<skill_content name=\"alpha\">")
	})

	t.Run("returns helpful error for missing skill", func(t *testing.T) {
		workdir := t.TempDir()
		createSkillFixture(t, workdir, "alpha", "Alpha skill", "Follow alpha steps.")

		tool := NewSkillTool(workdir)
		response := tool.Execute(&ToolCall{
			ID:       "skill-2",
			Function: "skill",
			Arguments: NewToolValue(map[string]any{
				"name": "missing",
			}),
		})

		require.Error(t, response.Error)
		assert.Contains(t, response.Error.Error(), `skill "missing" not found`)
		assert.Contains(t, response.Error.Error(), "Available skills: alpha")
	})

	t.Run("reloads skill content on each execute", func(t *testing.T) {
		workdir := t.TempDir()
		createSkillFixture(t, workdir, "alpha", "Alpha skill", "Version one")

		tool := NewSkillTool(workdir)
		first := tool.Execute(&ToolCall{
			ID:       "skill-4",
			Function: "skill",
			Arguments: NewToolValue(map[string]any{
				"name": "alpha",
			}),
		})
		require.NoError(t, first.Error)
		assert.Contains(t, first.Result.String("content"), "Version one")

		createSkillFixture(t, workdir, "alpha", "Alpha skill", "Version two")
		second := tool.Execute(&ToolCall{
			ID:       "skill-5",
			Function: "skill",
			Arguments: NewToolValue(map[string]any{
				"name": "alpha",
			}),
		})
		require.NoError(t, second.Error)
		assert.Contains(t, second.Result.String("content"), "Version two")
	})

	t.Run("returns error for missing argument", func(t *testing.T) {
		tool := NewSkillTool(t.TempDir())
		response := tool.Execute(&ToolCall{
			ID:        "skill-3",
			Function:  "skill",
			Arguments: NewToolValue(map[string]any{}),
		})

		require.Error(t, response.Error)
		assert.Contains(t, response.Error.Error(), "missing required argument: name")
	})
}

func createSkillFixture(t *testing.T, workdir string, name string, description string, body string) {
	t.Helper()

	skillDir := filepath.Join(workdir, ".agents", "skills", name)
	require.NoError(t, os.MkdirAll(skillDir, 0o755))

	skillContent := "---\n" +
		"name: " + name + "\n" +
		"description: " + description + "\n" +
		"---\n\n" +
		body + "\n"
	require.NoError(t, os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(skillContent), 0o644))
}
