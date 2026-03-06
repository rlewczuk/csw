package tool

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

const (
	skillDirectoryPath = ".agents/skills"
	skillManifestFile  = "SKILL.md"
)

// skillMetadata contains parsed skill manifest front matter.
type skillMetadata struct {
	Name        string `yaml:"name"`
	Description string `yaml:"description"`
}

// skillEntry describes a single available skill.
type skillEntry struct {
	Name        string
	Description string
	Location    string
	Content     string
}

// SkillTool loads skill instructions from .agents/skills directory.
type SkillTool struct {
	workdir string
}

// NewSkillTool creates a new SkillTool instance.
func NewSkillTool(workdir string) *SkillTool {
	return &SkillTool{workdir: workdir}
}

// GetDescription returns dynamic skill listing appended to static tool description.
func (t *SkillTool) GetDescription() (string, bool) {
	skills, err := t.listSkills()
	if err != nil {
		return fmt.Sprintf("\n\n<available_skills>\n  <error>%s</error>\n</available_skills>", escapeXMLText(err.Error())), false
	}

	if len(skills) == 0 {
		return "\n\n<available_skills>\n  <none>No skills are currently available.</none>\n</available_skills>", false
	}

	lines := make([]string, 0, len(skills)*5+2)
	lines = append(lines, "", "", "<available_skills>")
	for _, skill := range skills {
		lines = append(lines,
			"  <skill>",
			fmt.Sprintf("    <name>%s</name>", escapeXMLText(skill.Name)),
			fmt.Sprintf("    <description>%s</description>", escapeXMLText(skill.Description)),
			fmt.Sprintf("    <location>%s</location>", escapeXMLText(skill.Location)),
			"  </skill>",
		)
	}
	lines = append(lines, "</available_skills>")

	return strings.Join(lines, "\n"), false
}

// Execute loads and returns requested skill content.
func (t *SkillTool) Execute(args *ToolCall) *ToolResponse {
	skillName, ok := args.Arguments.StringOK("name")
	if !ok || strings.TrimSpace(skillName) == "" {
		return &ToolResponse{
			Call:  args,
			Error: fmt.Errorf("SkillTool.Execute() [skill.go]: missing required argument: name"),
			Done:  true,
		}
	}

	skill, err := t.getSkill(skillName)
	if err != nil {
		return &ToolResponse{Call: args, Error: err, Done: true}
	}

	output := strings.Join([]string{
		fmt.Sprintf("<skill_content name=\"%s\">", escapeXMLText(skill.Name)),
		fmt.Sprintf("# Skill: %s", skill.Name),
		"",
		strings.TrimSpace(skill.Content),
		"",
		fmt.Sprintf("Base directory for this skill: %s", skill.Location),
		"Relative paths in this skill are relative to this base directory.",
		"</skill_content>",
	}, "\n")

	var result ToolValue
	result.Set("name", skill.Name)
	result.Set("location", skill.Location)
	result.Set("content", skill.Content)
	result.Set("output", output)

	return &ToolResponse{Call: args, Result: result, Done: true}
}

// Render returns a string representation of the tool call.
func (t *SkillTool) Render(call *ToolCall) (string, string, map[string]string) {
	name := call.Arguments.String("name")
	if name == "" {
		name = call.Arguments.String("tool")
	}

	oneLiner := truncateString(fmt.Sprintf("load skill %s", name), 128)
	full := oneLiner
	if output, ok := call.Arguments.StringOK("output"); ok && output != "" {
		full += "\n\n" + output
	}
	if errMsg, ok := call.Arguments.StringOK("error"); ok && errMsg != "" {
		errOneLiner, errFull := formatRenderError(errMsg)
		oneLiner += "\n" + errOneLiner
		full += "\n\n" + errFull
	}

	return oneLiner, full, make(map[string]string)
}

func (t *SkillTool) getSkill(skillName string) (*skillEntry, error) {
	skills, err := t.listSkills()
	if err != nil {
		return nil, fmt.Errorf("SkillTool.getSkill() [skill.go]: failed to load skills: %w", err)
	}

	for _, skill := range skills {
		if skill.Name == skillName {
			return &skill, nil
		}
	}

	availableNames := make([]string, 0, len(skills))
	for _, skill := range skills {
		availableNames = append(availableNames, skill.Name)
	}
	availableText := "none"
	if len(availableNames) > 0 {
		availableText = strings.Join(availableNames, ", ")
	}

	return nil, fmt.Errorf("SkillTool.getSkill() [skill.go]: skill %q not found. Available skills: %s", skillName, availableText)
}

func (t *SkillTool) listSkills() ([]skillEntry, error) {
	skillsDir := filepath.Join(t.workdir, skillDirectoryPath)

	entries, err := os.ReadDir(skillsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return []skillEntry{}, nil
		}
		return nil, fmt.Errorf("SkillTool.listSkills() [skill.go]: failed to read skills directory %s: %w", skillsDir, err)
	}

	skills := make([]skillEntry, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		skillDir := filepath.Join(skillsDir, entry.Name())
		skillFilePath := filepath.Join(skillDir, skillManifestFile)
		contentBytes, readErr := os.ReadFile(skillFilePath)
		if readErr != nil {
			if os.IsNotExist(readErr) {
				continue
			}
			return nil, fmt.Errorf("SkillTool.listSkills() [skill.go]: failed to read skill manifest %s: %w", skillFilePath, readErr)
		}

		skill, parseErr := parseSkillManifest(entry.Name(), skillDir, string(contentBytes))
		if parseErr != nil {
			return nil, parseErr
		}

		skills = append(skills, skill)
	}

	sort.Slice(skills, func(i, j int) bool {
		return skills[i].Name < skills[j].Name
	})

	return skills, nil
}

func parseSkillManifest(dirName string, skillDir string, content string) (skillEntry, error) {
	entry := skillEntry{
		Name:     strings.TrimSpace(dirName),
		Location: skillDir,
	}

	metadata, body, err := splitSkillFrontMatter(content)
	if err != nil {
		return skillEntry{}, fmt.Errorf("parseSkillManifest() [skill.go]: failed to parse skill %s: %w", dirName, err)
	}
	entry.Content = strings.TrimSpace(body)
	if entry.Content == "" {
		entry.Content = strings.TrimSpace(content)
	}

	if metadata.Name != "" {
		entry.Name = metadata.Name
	}
	entry.Description = metadata.Description
	if strings.TrimSpace(entry.Description) == "" {
		entry.Description = "No description available."
	}

	if strings.TrimSpace(entry.Name) == "" {
		return skillEntry{}, fmt.Errorf("parseSkillManifest() [skill.go]: missing skill name for directory %s", dirName)
	}

	return entry, nil
}

func splitSkillFrontMatter(content string) (skillMetadata, string, error) {
	trimmed := strings.TrimSpace(content)
	if !strings.HasPrefix(trimmed, "---") {
		return skillMetadata{}, content, nil
	}

	parts := strings.SplitN(trimmed, "\n---", 2)
	if len(parts) != 2 {
		return skillMetadata{}, "", fmt.Errorf("splitSkillFrontMatter() [skill.go]: invalid front matter block")
	}

	header := strings.TrimPrefix(parts[0], "---")
	body := strings.TrimPrefix(parts[1], "\n")

	var metadata skillMetadata
	if err := yaml.Unmarshal([]byte(header), &metadata); err != nil {
		return skillMetadata{}, "", fmt.Errorf("splitSkillFrontMatter() [skill.go]: failed to parse front matter: %w", err)
	}

	return metadata, body, nil
}

func escapeXMLText(value string) string {
	replacer := strings.NewReplacer(
		"&", "&amp;",
		"<", "&lt;",
		">", "&gt;",
		"\"", "&quot;",
		"'", "&apos;",
	)
	return replacer.Replace(value)
}
