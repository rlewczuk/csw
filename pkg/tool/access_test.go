package tool

import (
	"errors"
	"testing"

	"github.com/rlewczuk/csw/pkg/conf"
	"github.com/stretchr/testify/assert"
)

// mockTool is a simple test double for the Tool interface
type mockTool struct {
	name        string
	description string
	executed    bool
	result      ToolValue
	err         error
}

func (m *mockTool) GetDescription() (string, bool) {
	return "", false
}

func (m *mockTool) Execute(call *ToolCall) *ToolResponse {
	m.executed = true
	return &ToolResponse{
		Call:   call,
		Error:  m.err,
		Result: m.result,
		Done:   true,
	}
}

func TestMatchPattern(t *testing.T) {
	tests := []struct {
		name        string
		pattern     string
		toolName    string
		shouldMatch bool
		description string
	}{
		{
			name:        "exact match",
			pattern:     "vfsRead",
			toolName:    "vfsRead",
			shouldMatch: true,
			description: "exact pattern should match exactly",
		},
		{
			name:        "exact no match",
			pattern:     "vfsRead",
			toolName:    "vfsWrite",
			shouldMatch: false,
			description: "exact pattern should not match different name",
		},
		{
			name:        "single wildcard match",
			pattern:     "vfs*",
			toolName:    "vfsRead",
			shouldMatch: true,
			description: "single wildcard should match one segment",
		},
		{
			name:        "single wildcard no match nested",
			pattern:     "vfs.*",
			toolName:    "vfs.local.read",
			shouldMatch: false,
			description: "single wildcard should not match multiple segments",
		},
		{
			name:        "multi wildcard match one level",
			pattern:     "vfs**",
			toolName:    "vfsRead",
			shouldMatch: true,
			description: "** should match one segment",
		},
		{
			name:        "multi wildcard match nested",
			pattern:     "vfs.**",
			toolName:    "vfs.local.read",
			shouldMatch: true,
			description: "** should match multiple segments",
		},
		{
			name:        "multi wildcard match deeply nested",
			pattern:     "vfs.**",
			toolName:    "vfs.local.impl.read",
			shouldMatch: true,
			description: "** should match deeply nested segments",
		},
		{
			name:        "partial wildcard prefix",
			pattern:     "vfs*",
			toolName:    "vfsRead",
			shouldMatch: true,
			description: "partial wildcard should match prefix",
		},
		{
			name:        "partial wildcard match different suffix",
			pattern:     "vfs*",
			toolName:    "vfsWrite",
			shouldMatch: true,
			description: "partial wildcard should match vfsWrite with vfs* pattern",
		},
		{
			name:        "partial wildcard multiple matches",
			pattern:     "foo.ba*",
			toolName:    "foo.bar",
			shouldMatch: true,
			description: "ba* should match bar",
		},
		{
			name:        "partial wildcard multiple matches 2",
			pattern:     "foo.ba*",
			toolName:    "foo.baz",
			shouldMatch: true,
			description: "ba* should match baz",
		},
		{
			name:        "partial wildcard no match nested",
			pattern:     "foo.ba*",
			toolName:    "foo.bar.baz",
			shouldMatch: false,
			description: "ba* should not match bar.baz (extra segment)",
		},
		{
			name:        "global wildcard match",
			pattern:     "**",
			toolName:    "anything",
			shouldMatch: true,
			description: "** should match any single segment",
		},
		{
			name:        "global wildcard match nested",
			pattern:     "**",
			toolName:    "any.nested.tool",
			shouldMatch: true,
			description: "** should match any nested segments",
		},
		{
			name:        "multiple segments exact",
			pattern:     "api.v1.read",
			toolName:    "api.v1.read",
			shouldMatch: true,
			description: "multiple exact segments should match",
		},
		{
			name:        "multiple segments no match",
			pattern:     "api.v1.read",
			toolName:    "api.v2.read",
			shouldMatch: false,
			description: "multiple segments should not match if any differs",
		},
		{
			name:        "wildcard middle segment",
			pattern:     "api.*.read",
			toolName:    "api.v1.read",
			shouldMatch: true,
			description: "wildcard in middle should match",
		},
		{
			name:        "wildcard middle no match nested",
			pattern:     "api.*.read",
			toolName:    "api.v1.v2.read",
			shouldMatch: false,
			description: "single wildcard should not match multiple segments",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			matches, _ := matchPattern(tt.pattern, tt.toolName)
			if tt.shouldMatch {
				assert.True(t, matches, tt.description)
			} else {
				assert.False(t, matches, tt.description)
			}
		})
	}
}

func TestMatchPatternSpecificity(t *testing.T) {
	tests := []struct {
		name         string
		pattern1     string
		pattern2     string
		toolName     string
		moreSpecific string
		description  string
	}{
		{
			name:         "exact vs wildcard",
			pattern1:     "vfs.read",
			pattern2:     "vfs.*",
			toolName:     "vfs.read",
			moreSpecific: "vfs.read",
			description:  "exact match should be more specific than wildcard",
		},
		{
			name:         "single vs multi wildcard",
			pattern1:     "vfs.*",
			pattern2:     "vfs.**",
			toolName:     "vfs.read",
			moreSpecific: "vfs.*",
			description:  "single wildcard should be more specific than **",
		},
		{
			name:         "partial vs full wildcard",
			pattern1:     "vfs.r*",
			pattern2:     "vfs.*",
			toolName:     "vfs.read",
			moreSpecific: "vfs.r*",
			description:  "partial wildcard should be more specific than full wildcard",
		},
		{
			name:         "prefix vs global wildcard",
			pattern1:     "vfs.**",
			pattern2:     "**",
			toolName:     "vfs.read",
			moreSpecific: "vfs.**",
			description:  "prefix wildcard should be more specific than global",
		},
		{
			name:         "two segment exact vs one segment wildcard",
			pattern1:     "api.v1.*",
			pattern2:     "api.**",
			toolName:     "api.v1.read",
			moreSpecific: "api.v1.*",
			description:  "more exact segments should be more specific",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			matches1, spec1 := matchPattern(tt.pattern1, tt.toolName)
			matches2, spec2 := matchPattern(tt.pattern2, tt.toolName)

			assert.True(t, matches1, "pattern1 should match")
			assert.True(t, matches2, "pattern2 should match")

			if tt.moreSpecific == tt.pattern1 {
				assert.Greater(t, spec1, spec2, tt.description)
			} else {
				assert.Greater(t, spec2, spec1, tt.description)
			}
		})
	}
}

func TestMatchSegmentPattern(t *testing.T) {
	tests := []struct {
		name        string
		pattern     string
		segment     string
		shouldMatch bool
	}{
		{"prefix wildcard", "r*", "read", true},
		{"prefix wildcard no match", "r*", "write", false},
		{"suffix wildcard", "*ad", "read", true},
		{"suffix wildcard no match", "*ad", "write", false},
		{"middle wildcard", "r*d", "read", true},
		{"middle wildcard no match", "r*d", "write", false},
		{"multiple wildcards", "r*a*", "read", true},
		{"multiple wildcards no match", "r*a*", "write", false},
		{"just wildcard", "*", "anything", true},
		{"empty pattern parts", "**", "anything", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := matchSegmentPattern(tt.pattern, tt.segment)
			assert.Equal(t, tt.shouldMatch, result)
		})
	}
}

func TestAccessControlTool_ResolveAccessFlag(t *testing.T) {
	tests := []struct {
		name       string
		privileges map[string]conf.AccessFlag
		toolName   string
		expected   conf.AccessFlag
	}{
		{
			name: "exact match allow",
			privileges: map[string]conf.AccessFlag{
				"vfsRead": conf.AccessAllow,
			},
			toolName: "vfsRead",
			expected: conf.AccessAllow,
		},
		{
			name: "exact match deny",
			privileges: map[string]conf.AccessFlag{
				"vfsWrite": conf.AccessDeny,
			},
			toolName: "vfsWrite",
			expected: conf.AccessDeny,
		},
		{
			name: "wildcard match",
			privileges: map[string]conf.AccessFlag{
				"vfs*": conf.AccessAllow,
			},
			toolName: "vfsRead",
			expected: conf.AccessAllow,
		},
		{
			name: "multi wildcard match",
			privileges: map[string]conf.AccessFlag{
				"vfs.**": conf.AccessAllow,
			},
			toolName: "vfs.local.read",
			expected: conf.AccessAllow,
		},
		{
			name: "global default allow",
			privileges: map[string]conf.AccessFlag{
				"**": conf.AccessAllow,
			},
			toolName: "any.tool.name",
			expected: conf.AccessAllow,
		},
		{
			name: "global default deny",
			privileges: map[string]conf.AccessFlag{
				"**": conf.AccessDeny,
			},
			toolName: "any.tool.name",
			expected: conf.AccessDeny,
		},
		{
			name:       "no match defaults to deny",
			privileges: map[string]conf.AccessFlag{},
			toolName:   "vfsRead",
			expected:   conf.AccessDeny,
		},
		{
			name: "most specific wins - exact over wildcard",
			privileges: map[string]conf.AccessFlag{
				"vfs.*":   conf.AccessDeny,
				"vfsRead": conf.AccessAllow,
			},
			toolName: "vfsRead",
			expected: conf.AccessAllow,
		},
		{
			name: "most specific wins - wildcard over global",
			privileges: map[string]conf.AccessFlag{
				"**":   conf.AccessDeny,
				"vfs*": conf.AccessAllow,
			},
			toolName: "vfsRead",
			expected: conf.AccessAllow,
		},
		{
			name: "most specific wins - partial over full wildcard",
			privileges: map[string]conf.AccessFlag{
				"vfs*":  conf.AccessDeny,
				"vfsR*": conf.AccessAllow,
			},
			toolName: "vfsRead",
			expected: conf.AccessAllow,
		},
		{
			name: "complex hierarchy",
			privileges: map[string]conf.AccessFlag{
				"**":           conf.AccessDeny,
				"vfs.**":       conf.AccessAllow,
				"vfs.local.*":  conf.AccessDeny,
				"vfs.local.ro": conf.AccessAllow,
			},
			toolName: "vfs.local.ro",
			expected: conf.AccessAllow,
		},
		{
			name: "ask flag",
			privileges: map[string]conf.AccessFlag{
				"sensitive.*": conf.AccessAsk,
			},
			toolName: "sensitive.operation",
			expected: conf.AccessAsk,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &mockTool{name: tt.toolName}
			ac := NewAccessControlTool(mock, tt.privileges)
			result := ac.resolveAccessFlag(tt.toolName)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestAccessControlTool_Execute_Allow(t *testing.T) {
	mock := &mockTool{
		name:   "vfsRead",
		result: NewToolValue("test result"),
		err:    nil,
	}

	privileges := map[string]conf.AccessFlag{
		"vfs*": conf.AccessAllow,
	}

	ac := NewAccessControlTool(mock, privileges)
	call := &ToolCall{
		ID:        "test-id",
		Function:  "vfsRead",
		Arguments: NewToolValue(map[string]any{"path": "/test"}),
	}

	response := ac.Execute(call)

	assert.True(t, mock.executed, "underlying tool should be executed")
	assert.NoError(t, response.Error, "should not return error for allowed tool")
	assert.Equal(t, "test result", response.Result.AsString())
}

func TestAccessControlTool_Execute_Deny(t *testing.T) {
	mock := &mockTool{
		name:   "vfsWrite",
		result: NewToolValue("should not execute"),
		err:    nil,
	}

	privileges := map[string]conf.AccessFlag{
		"vfsWrite": conf.AccessDeny,
	}

	ac := NewAccessControlTool(mock, privileges)
	call := &ToolCall{
		ID:        "test-id",
		Function:  "vfsWrite",
		Arguments: NewToolValue(map[string]any{"path": "/test"}),
	}

	response := ac.Execute(call)

	assert.False(t, mock.executed, "underlying tool should not be executed")
	assert.Error(t, response.Error, "should return error for denied tool")
	assert.Contains(t, response.Error.Error(), "access denied")
	assert.True(t, response.Done)
}

func TestAccessControlTool_Execute_Ask(t *testing.T) {
	mock := &mockTool{
		name:   "sensitive.operation",
		result: NewToolValue("should not execute"),
		err:    nil,
	}

	privileges := map[string]conf.AccessFlag{
		"sensitive.*": conf.AccessAsk,
	}

	ac := NewAccessControlTool(mock, privileges)
	call := &ToolCall{
		ID:        "test-id",
		Function:  "sensitive.operation",
		Arguments: NewToolValue(map[string]any{}),
	}

	response := ac.Execute(call)

	assert.False(t, mock.executed, "underlying tool should not be executed for ask")
	assert.Error(t, response.Error, "should return error for ask (not yet implemented)")
	assert.Contains(t, response.Error.Error(), "permission query")
	assert.True(t, response.Done)
}

func TestAccessControlTool_Execute_DefaultDeny(t *testing.T) {
	mock := &mockTool{
		name:   "unknown.tool",
		result: NewToolValue("should not execute"),
		err:    nil,
	}

	// Empty privileges - should default to deny
	privileges := map[string]conf.AccessFlag{}

	ac := NewAccessControlTool(mock, privileges)
	call := &ToolCall{
		ID:        "test-id",
		Function:  "unknown.tool",
		Arguments: NewToolValue(map[string]any{}),
	}

	response := ac.Execute(call)

	assert.False(t, mock.executed, "underlying tool should not be executed by default")
	assert.Error(t, response.Error, "should return error when no privileges match")
	assert.Contains(t, response.Error.Error(), "access denied")
}

func TestAccessControlTool_Execute_ToolError(t *testing.T) {
	mock := &mockTool{
		name:   "vfsRead",
		result: NewToolValue(nil),
		err:    errors.New("file not found"),
	}

	privileges := map[string]conf.AccessFlag{
		"vfs*": conf.AccessAllow,
	}

	ac := NewAccessControlTool(mock, privileges)
	call := &ToolCall{
		ID:        "test-id",
		Function:  "vfsRead",
		Arguments: NewToolValue(map[string]any{"path": "/nonexistent"}),
	}

	response := ac.Execute(call)

	assert.True(t, mock.executed, "tool should be executed")
	assert.Error(t, response.Error, "should propagate tool error")
	assert.Equal(t, "file not found", response.Error.Error())
}

func TestAccessControlTool_MultipleMatchingPatterns(t *testing.T) {
	mock := &mockTool{name: "api.v1.users.read"}

	privileges := map[string]conf.AccessFlag{
		"**":              conf.AccessDeny,
		"api.**":          conf.AccessAllow,
		"api.v1.**":       conf.AccessDeny,
		"api.v1.users.*":  conf.AccessAllow,
		"api.v1.users.r*": conf.AccessDeny,
	}

	ac := NewAccessControlTool(mock, privileges)

	// The most specific pattern "api.v1.users.r*" should win
	flag := ac.resolveAccessFlag("api.v1.users.read")
	assert.Equal(t, conf.AccessDeny, flag, "most specific pattern should win")

	call := &ToolCall{
		ID:        "test-id",
		Function:  "api.v1.users.read",
		Arguments: NewToolValue(map[string]any{}),
	}

	response := ac.Execute(call)
	assert.False(t, mock.executed)
	assert.Error(t, response.Error)
}

func TestAccessControlTool_GlobalDefault(t *testing.T) {
	tests := []struct {
		name       string
		privileges map[string]conf.AccessFlag
		toolNames  []string
		expected   conf.AccessFlag
	}{
		{
			name: "global allow as default",
			privileges: map[string]conf.AccessFlag{
				"**": conf.AccessAllow,
			},
			toolNames: []string{"any.tool", "another.tool.nested", "simple"},
			expected:  conf.AccessAllow,
		},
		{
			name: "global deny with specific allows",
			privileges: map[string]conf.AccessFlag{
				"**":      conf.AccessDeny,
				"vfsRead": conf.AccessAllow,
			},
			toolNames: []string{"vfsWrite", "api.call", "other"},
			expected:  conf.AccessDeny,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for _, toolName := range tt.toolNames {
				mock := &mockTool{name: toolName}
				ac := NewAccessControlTool(mock, tt.privileges)
				flag := ac.resolveAccessFlag(toolName)
				assert.Equal(t, tt.expected, flag, "tool: %s", toolName)
			}
		})
	}
}

func (m *mockTool) Render(call *ToolCall) (string, string, map[string]string) {
	return m.name, m.name, make(map[string]string)
}
