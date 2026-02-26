package core

import (
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rlewczuk/csw/pkg/models"
	"github.com/rlewczuk/csw/pkg/tool"
)

func TestCompactMessagesStep1ReplaceFileParts(t *testing.T) {
	t.Run("replaces file-related parts with full file content when summary exceeds file length", func(t *testing.T) {
		tmpDir := t.TempDir()
		filePath := filepath.Join(tmpDir, "a.txt")
		require.NoError(t, os.WriteFile(filePath, []byte("abc"), 0644))

		call := compactTestToolCall("c1", "vfsRead", map[string]any{"path": filePath})
		resp := compactTestToolResponse(call, map[string]any{"content": strings.Repeat("x", 40)})

		messages := []*models.ChatMessage{
			compactTestMessage(models.ChatRoleAssistant,
				models.ChatMessagePart{Text: strings.Repeat("analysis ", 10)},
				models.ChatMessagePart{ToolCall: call},
			),
			compactTestMessage(models.ChatRoleUser, models.ChatMessagePart{ToolResponse: resp}),
			models.NewTextMessage(models.ChatRoleAssistant, "keep me"),
		}

		got := compactMessagesStep1ReplaceFileParts(messages)

		assert.False(t, compactTestHasToolCall(got, "c1"))
		assert.False(t, compactTestHasToolResponse(got, "c1"))
		assert.True(t, compactTestHasCompactedFileMessage(got, filePath, "abc"))
		assert.Equal(t, "keep me", got[len(got)-1].GetText())
	})

	t.Run("does not replace when summary is shorter than file", func(t *testing.T) {
		tmpDir := t.TempDir()
		filePath := filepath.Join(tmpDir, "b.txt")
		require.NoError(t, os.WriteFile(filePath, []byte(strings.Repeat("z", 100)), 0644))

		call := compactTestToolCall("c2", "vfsRead", map[string]any{"path": filePath})
		resp := compactTestToolResponse(call, map[string]any{"content": "short"})

		messages := []*models.ChatMessage{
			compactTestMessage(models.ChatRoleAssistant, models.ChatMessagePart{ToolCall: call}),
			compactTestMessage(models.ChatRoleUser, models.ChatMessagePart{ToolResponse: resp}),
		}

		got := compactMessagesStep1ReplaceFileParts(messages)

		assert.True(t, compactTestHasToolCall(got, "c2"))
		assert.True(t, compactTestHasToolResponse(got, "c2"))
		assert.False(t, compactTestHasCompactedFileMessage(got, filePath, ""))
	})

	t.Run("skips missing files gracefully", func(t *testing.T) {
		missing := filepath.Join(t.TempDir(), "missing.txt")
		call := compactTestToolCall("c3", "vfsRead", map[string]any{"path": missing})

		messages := []*models.ChatMessage{
			compactTestMessage(models.ChatRoleAssistant, models.ChatMessagePart{ToolCall: call}),
		}

		got := compactMessagesStep1ReplaceFileParts(messages)
		assert.True(t, compactTestHasToolCall(got, "c3"))
	})
}

func TestCompactMessagesStep2KeepLastTodo(t *testing.T) {
	t.Run("keeps messages unchanged when there are no todo tools", func(t *testing.T) {
		messages := []*models.ChatMessage{
			models.NewTextMessage(models.ChatRoleUser, "hello"),
			models.NewTextMessage(models.ChatRoleAssistant, "world"),
		}

		got := compactMessagesStep2KeepLastTodo(messages)
		require.Len(t, got, 2)
		assert.Equal(t, "hello", got[0].GetText())
		assert.Equal(t, "world", got[1].GetText())
	})

	call1 := compactTestToolCall("todo-1", "todoRead", map[string]any{})
	resp1 := compactTestToolResponse(call1, map[string]any{"content": "first"})
	call2 := compactTestToolCall("todo-2", "todoWrite", map[string]any{"todos": []any{}})
	resp2 := compactTestToolResponse(call2, map[string]any{"content": "second"})

	messages := []*models.ChatMessage{
		compactTestMessage(models.ChatRoleAssistant,
			models.ChatMessagePart{Text: "pre"},
			models.ChatMessagePart{ToolCall: call1},
		),
		compactTestMessage(models.ChatRoleUser, models.ChatMessagePart{ToolResponse: resp1}),
		compactTestMessage(models.ChatRoleAssistant, models.ChatMessagePart{ToolCall: call2}),
		compactTestMessage(models.ChatRoleUser, models.ChatMessagePart{ToolResponse: resp2}),
	}

	got := compactMessagesStep2KeepLastTodo(messages)

	assert.False(t, compactTestHasToolCall(got, "todo-1"))
	assert.False(t, compactTestHasToolResponse(got, "todo-1"))
	assert.True(t, compactTestHasToolCall(got, "todo-2"))
	assert.True(t, compactTestHasToolResponse(got, "todo-2"))
	assert.Contains(t, got[0].GetText(), "pre")
}

func TestCompactMessagesStep3ClipRunBashResponses(t *testing.T) {
	call := compactTestToolCall("bash-1", "runBash", map[string]any{"command": "x"})
	resp := compactTestToolResponse(call, map[string]any{"output": compactTestLines(30), "exit_code": float64(0)})

	messages := []*models.ChatMessage{
		compactTestMessage(models.ChatRoleUser, models.ChatMessagePart{ToolResponse: resp}),
	}

	got := compactMessagesStep3ClipRunBashResponses(messages)
	output := compactTestToolResponseOutputByID(got, "bash-1")
	assert.Equal(t, 16, len(strings.Split(output, "\n")))

	call2 := compactTestToolCall("bash-2", "runBash", map[string]any{"command": "y"})
	resp2 := compactTestToolResponse(call2, map[string]any{"output": compactTestLines(4), "exit_code": float64(0)})
	got2 := compactMessagesStep3ClipRunBashResponses([]*models.ChatMessage{
		compactTestMessage(models.ChatRoleUser, models.ChatMessagePart{ToolResponse: resp2}),
	})
	assert.Equal(t, 4, len(strings.Split(compactTestToolResponseOutputByID(got2, "bash-2"), "\n")))

	t.Run("ignores runBash responses without output field", func(t *testing.T) {
		call := compactTestToolCall("bash-3", "runBash", map[string]any{"command": "pwd"})
		resp := compactTestToolResponse(call, map[string]any{"exit_code": float64(0)})

		got := compactMessagesStep3ClipRunBashResponses([]*models.ChatMessage{
			compactTestMessage(models.ChatRoleUser, models.ChatMessagePart{ToolResponse: resp}),
		})

		assert.Equal(t, "", compactTestToolResponseOutputByID(got, "bash-3"))
	})
}

func TestCompactMessagesStep4PruneGrepFind(t *testing.T) {
	t.Run("no-op when below threshold", func(t *testing.T) {
		call := compactTestToolCall("g-1", "vfsGrep", map[string]any{"pattern": "x"})
		resp := compactTestToolResponse(call, map[string]any{"content": "ok"})
		messages := []*models.ChatMessage{
			compactTestMessage(models.ChatRoleAssistant, models.ChatMessagePart{ToolCall: call}),
			compactTestMessage(models.ChatRoleUser, models.ChatMessagePart{ToolResponse: resp}),
		}

		got := compactMessagesStep4PruneGrepFindWithLimit(messages, 10000)
		assert.True(t, compactTestHasToolCall(got, "g-1"))
		assert.True(t, compactTestHasToolResponse(got, "g-1"))
	})

	t.Run("keeps only last three ids for grep and find with pairs", func(t *testing.T) {
		messages := make([]*models.ChatMessage, 0)
		for i := 1; i <= 5; i++ {
			id := "g-" + compactTestItoa(i)
			call := compactTestToolCall(id, "vfsGrep", map[string]any{"pattern": "p"})
			resp := compactTestToolResponse(call, map[string]any{"content": "grep"})
			messages = append(messages,
				compactTestMessage(models.ChatRoleAssistant, models.ChatMessagePart{ToolCall: call}),
				compactTestMessage(models.ChatRoleUser, models.ChatMessagePart{ToolResponse: resp}),
			)
		}
		for i := 1; i <= 4; i++ {
			id := "f-" + compactTestItoa(i)
			call := compactTestToolCall(id, "vfsFind", map[string]any{"query": "*.go"})
			resp := compactTestToolResponse(call, map[string]any{"content": "find"})
			messages = append(messages,
				compactTestMessage(models.ChatRoleAssistant, models.ChatMessagePart{ToolCall: call}),
				compactTestMessage(models.ChatRoleUser, models.ChatMessagePart{ToolResponse: resp}),
			)
		}

		got := compactMessagesStep4PruneGrepFindWithLimit(messages, 100)

		assert.False(t, compactTestHasToolCall(got, "g-1"))
		assert.False(t, compactTestHasToolResponse(got, "g-1"))
		assert.False(t, compactTestHasToolCall(got, "g-2"))
		assert.False(t, compactTestHasToolResponse(got, "g-2"))
		assert.True(t, compactTestHasToolCall(got, "g-3"))
		assert.True(t, compactTestHasToolCall(got, "g-4"))
		assert.True(t, compactTestHasToolCall(got, "g-5"))

		assert.False(t, compactTestHasToolCall(got, "f-1"))
		assert.False(t, compactTestHasToolResponse(got, "f-1"))
		assert.True(t, compactTestHasToolCall(got, "f-2"))
		assert.True(t, compactTestHasToolCall(got, "f-3"))
		assert.True(t, compactTestHasToolCall(got, "f-4"))
	})
}

func TestCompactMessagesStep5TrimAssistantThinking(t *testing.T) {
	messages := []*models.ChatMessage{
		models.NewTextMessage(models.ChatRoleSystem, "sys"),
		compactTestMessage(models.ChatRoleAssistant, models.ChatMessagePart{ReasoningContent: strings.Repeat("a", 180)}),
		compactTestMessage(models.ChatRoleAssistant, models.ChatMessagePart{ReasoningContent: strings.Repeat("b", 30)}),
	}

	got := compactMessagesStep5TrimAssistantThinkingWithLimit(messages, 200)

	assert.Equal(t, "", got[1].Parts[0].ReasoningContent)
	assert.Equal(t, strings.Repeat("b", 30), got[2].Parts[0].ReasoningContent)

	unchanged := compactMessagesStep5TrimAssistantThinkingWithLimit(messages, 10000)
	assert.Equal(t, strings.Repeat("a", 180), unchanged[1].Parts[0].ReasoningContent)
}

func TestCompactMessagesStep6DropOldMessagesPreservingUserAndCompacted(t *testing.T) {
	compacted := newCompactedFileContentMessage("/tmp/a.go", strings.Repeat("x", 40))
	messages := []*models.ChatMessage{
		models.NewTextMessage(models.ChatRoleSystem, "initial prompt"),
		models.NewTextMessage(models.ChatRoleAssistant, strings.Repeat("drop-1", 30)),
		models.NewTextMessage(models.ChatRoleUser, "real user question"),
		models.NewTextMessage(models.ChatRoleUser, "<system>internal</system>"),
		compacted,
		models.NewTextMessage(models.ChatRoleAssistant, strings.Repeat("drop-2", 30)),
	}

	got := compactMessagesStep6DropOldMessagesPreservingUserAndCompactedWithLimit(messages, 200)

	require.NotEmpty(t, got)
	assert.Equal(t, "initial prompt", got[0].GetText())
	assert.True(t, compactTestContainsText(got, "real user question"))
	assert.True(t, compactTestHasCompactedFileMessage(got, "/tmp/a.go", ""))
	assert.False(t, compactTestContainsText(got, strings.Repeat("drop-1", 30)))
	assert.False(t, compactTestContainsText(got, "<system>internal</system>"))
}

func TestCompactMessagesStep7DropOldCompactedMessages(t *testing.T) {
	messages := []*models.ChatMessage{
		models.NewTextMessage(models.ChatRoleSystem, strings.Repeat("s", 40)),
		newCompactedFileContentMessage("/tmp/old.go", strings.Repeat("o", 70)),
		newCompactedFileContentMessage("/tmp/new.go", strings.Repeat("n", 70)),
	}

	got := compactMessagesStep7DropOldCompactedMessagesWithLimit(messages, 200)

	assert.False(t, compactTestHasCompactedFileMessage(got, "/tmp/old.go", ""))
	assert.True(t, compactTestHasCompactedFileMessage(got, "/tmp/new.go", ""))
}

func TestCompactMessagesStep8EnsureToolCallResponsePairs(t *testing.T) {
	call1 := compactTestToolCall("id-1", "vfsRead", map[string]any{"path": "a"})
	call2 := compactTestToolCall("id-2", "vfsRead", map[string]any{"path": "b"})
	resp1 := compactTestToolResponse(call1, map[string]any{"content": "ok"})
	orphanResp := &tool.ToolResponse{Call: compactTestToolCall("id-3", "vfsRead", map[string]any{"path": "c"}), Error: errors.New("x"), Done: true}

	messages := []*models.ChatMessage{
		compactTestMessage(models.ChatRoleAssistant,
			models.ChatMessagePart{ToolCall: call1},
			models.ChatMessagePart{ToolCall: call2},
		),
		compactTestMessage(models.ChatRoleUser,
			models.ChatMessagePart{ToolResponse: resp1},
			models.ChatMessagePart{ToolResponse: orphanResp},
		),
	}

	got := compactMessagesStep8EnsureToolCallResponsePairs(messages)

	assert.True(t, compactTestHasToolCall(got, "id-1"))
	assert.True(t, compactTestHasToolResponse(got, "id-1"))
	assert.False(t, compactTestHasToolCall(got, "id-2"))
	assert.False(t, compactTestHasToolResponse(got, "id-3"))

	t.Run("drops interactions with empty IDs", func(t *testing.T) {
		emptyCall := compactTestToolCall("", "vfsRead", map[string]any{"path": "d"})
		emptyResp := compactTestToolResponse(emptyCall, map[string]any{"content": "x"})

		withEmpty := compactMessagesStep8EnsureToolCallResponsePairs([]*models.ChatMessage{
			compactTestMessage(models.ChatRoleAssistant, models.ChatMessagePart{ToolCall: emptyCall}),
			compactTestMessage(models.ChatRoleUser, models.ChatMessagePart{ToolResponse: emptyResp}),
		})

		assert.False(t, compactTestHasToolCall(withEmpty, ""))
		assert.False(t, compactTestHasToolResponse(withEmpty, ""))
	})
}

func TestCompactMessages_EndToEnd(t *testing.T) {
	t.Run("applies all compaction steps and keeps consistency", func(t *testing.T) {
		tmpDir := t.TempDir()
		filePath := filepath.Join(tmpDir, "ctx.txt")
		require.NoError(t, os.WriteFile(filePath, []byte("tiny"), 0644))

		fileCall := compactTestToolCall("file-1", "vfsRead", map[string]any{"path": filePath})
		fileResp := compactTestToolResponse(fileCall, map[string]any{"content": strings.Repeat("long", 50)})

		todoOld := compactTestToolCall("todo-old", "todoRead", map[string]any{})
		todoOldResp := compactTestToolResponse(todoOld, map[string]any{"content": "old"})
		todoNew := compactTestToolCall("todo-new", "todoWrite", map[string]any{"todos": []any{}})
		todoNewResp := compactTestToolResponse(todoNew, map[string]any{"content": "new"})

		bashCall := compactTestToolCall("bash-1", "runBash", map[string]any{"command": "ls"})
		bashResp := compactTestToolResponse(bashCall, map[string]any{"output": compactTestLines(40), "exit_code": float64(0)})

		grepOld := compactTestToolCall("grep-1", "vfsGrep", map[string]any{"pattern": "a"})
		grepOldResp := compactTestToolResponse(grepOld, map[string]any{"content": "old grep"})

		messages := []*models.ChatMessage{
			models.NewTextMessage(models.ChatRoleSystem, "initial system prompt"),
			models.NewTextMessage(models.ChatRoleUser, "first real user prompt"),
			compactTestMessage(models.ChatRoleAssistant,
				models.ChatMessagePart{ReasoningContent: strings.Repeat("r", 3500)},
				models.ChatMessagePart{ToolCall: fileCall},
			),
			compactTestMessage(models.ChatRoleUser, models.ChatMessagePart{ToolResponse: fileResp}),
			compactTestMessage(models.ChatRoleAssistant, models.ChatMessagePart{ToolCall: todoOld}),
			compactTestMessage(models.ChatRoleUser, models.ChatMessagePart{ToolResponse: todoOldResp}),
			compactTestMessage(models.ChatRoleAssistant, models.ChatMessagePart{ToolCall: todoNew}),
			compactTestMessage(models.ChatRoleUser, models.ChatMessagePart{ToolResponse: todoNewResp}),
			compactTestMessage(models.ChatRoleAssistant, models.ChatMessagePart{ToolCall: bashCall}),
			compactTestMessage(models.ChatRoleUser, models.ChatMessagePart{ToolResponse: bashResp}),
			compactTestMessage(models.ChatRoleAssistant, models.ChatMessagePart{ToolCall: grepOld}),
			compactTestMessage(models.ChatRoleUser, models.ChatMessagePart{ToolResponse: grepOldResp}),
			models.NewTextMessage(models.ChatRoleAssistant, strings.Repeat("noise", 120)),
		}

		got := CompactMessages(messages)

		assert.True(t, compactTestHasCompactedFileMessage(got, filePath, "tiny"))
		assert.False(t, compactTestHasToolCall(got, "todo-old"))
		assert.False(t, compactTestHasToolResponse(got, "todo-old"))
		assert.True(t, compactTestHasToolCall(got, "todo-new"))
		assert.True(t, compactTestHasToolResponse(got, "todo-new"))
		assert.Equal(t, 16, len(strings.Split(compactTestToolResponseOutputByID(got, "bash-1"), "\n")))
		assert.False(t, compactTestHasToolCall(got, "grep-1"))
		assert.False(t, compactTestHasToolResponse(got, "grep-1"))
		assert.True(t, compactTestContainsText(got, "first real user prompt"))

		for _, msg := range got {
			for _, part := range msg.Parts {
				if part.ToolCall != nil {
					assert.True(t, compactTestHasToolResponse(got, part.ToolCall.ID))
				}
				if part.ToolResponse != nil && part.ToolResponse.Call != nil {
					assert.True(t, compactTestHasToolCall(got, part.ToolResponse.Call.ID))
				}
			}
		}
	})

	t.Run("handles empty and nil messages", func(t *testing.T) {
		assert.Nil(t, CompactMessages(nil))

		messages := []*models.ChatMessage{nil, models.NewTextMessage(models.ChatRoleUser, "hi")}
		got := CompactMessages(messages)
		require.Len(t, got, 1)
		assert.Equal(t, "hi", got[0].GetText())
	})
}

func compactTestMessage(role models.ChatRole, parts ...models.ChatMessagePart) *models.ChatMessage {
	return &models.ChatMessage{Role: role, Parts: parts}
}

func compactTestToolCall(id string, function string, args map[string]any) *tool.ToolCall {
	return &tool.ToolCall{ID: id, Function: function, Arguments: tool.NewToolValue(args)}
}

func compactTestToolResponse(call *tool.ToolCall, result map[string]any) *tool.ToolResponse {
	return &tool.ToolResponse{Call: call, Result: tool.NewToolValue(result), Done: true}
}

func compactTestHasToolCall(messages []*models.ChatMessage, id string) bool {
	for _, msg := range messages {
		if msg == nil {
			continue
		}
		for _, part := range msg.Parts {
			if part.ToolCall != nil && part.ToolCall.ID == id {
				return true
			}
		}
	}
	return false
}

func compactTestHasToolResponse(messages []*models.ChatMessage, id string) bool {
	for _, msg := range messages {
		if msg == nil {
			continue
		}
		for _, part := range msg.Parts {
			if part.ToolResponse != nil && part.ToolResponse.Call != nil && part.ToolResponse.Call.ID == id {
				return true
			}
		}
	}
	return false
}

func compactTestToolResponseOutputByID(messages []*models.ChatMessage, id string) string {
	for _, msg := range messages {
		if msg == nil {
			continue
		}
		for _, part := range msg.Parts {
			if part.ToolResponse != nil && part.ToolResponse.Call != nil && part.ToolResponse.Call.ID == id {
				if output, ok := part.ToolResponse.Result.StringOK("output"); ok {
					return output
				}
			}
		}
	}
	return ""
}

func compactTestHasCompactedFileMessage(messages []*models.ChatMessage, path string, content string) bool {
	for _, msg := range messages {
		if msg == nil {
			continue
		}
		text := msg.GetText()
		if !strings.Contains(text, compactedFileMessagePrefix+path) {
			continue
		}
		if content == "" || strings.Contains(text, content) {
			return true
		}
	}
	return false
}

func compactTestContainsText(messages []*models.ChatMessage, text string) bool {
	for _, msg := range messages {
		if msg == nil {
			continue
		}
		if strings.Contains(msg.GetText(), text) {
			return true
		}
	}
	return false
}

func compactTestLines(count int) string {
	if count <= 0 {
		return ""
	}
	lines := make([]string, 0, count)
	for i := 1; i <= count; i++ {
		lines = append(lines, "line "+compactTestItoa(i))
	}
	return strings.Join(lines, "\n")
}

func compactTestItoa(v int) string {
	return strconv.Itoa(v)
}
