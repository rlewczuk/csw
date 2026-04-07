package core

import (
	"io"
	"log/slog"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rlewczuk/csw/pkg/conf"
	"github.com/rlewczuk/csw/pkg/conf/impl"
	"github.com/rlewczuk/csw/pkg/models"
	"github.com/rlewczuk/csw/pkg/testutil"
	"github.com/rlewczuk/csw/pkg/tool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// probeTool is a test double that can be synchronized to verify parallel execution.
type probeTool struct {
	started      chan string
	release      <-chan struct{}
	active       *int32
	maxActive    *int32
	overlap      *atomic.Bool
	startTimes   *[]time.Time
	startTimesMu *sync.Mutex
	response     func(call *tool.ToolCall) *tool.ToolResponse
}

func (t *probeTool) Execute(args *tool.ToolCall) *tool.ToolResponse {
	if t.active != nil {
		current := atomic.AddInt32(t.active, 1)
		if current > 1 && t.overlap != nil {
			t.overlap.Store(true)
		}
		if t.maxActive != nil {
			for {
				maxCurrent := atomic.LoadInt32(t.maxActive)
				if current <= maxCurrent {
					break
				}
				if atomic.CompareAndSwapInt32(t.maxActive, maxCurrent, current) {
					break
				}
			}
		}
		defer atomic.AddInt32(t.active, -1)
	}

	if t.startTimes != nil && t.startTimesMu != nil {
		t.startTimesMu.Lock()
		*t.startTimes = append(*t.startTimes, time.Now())
		t.startTimesMu.Unlock()
	}

	if t.started != nil {
		t.started <- args.ID
	}

	if t.release != nil {
		<-t.release
	}

	if t.response != nil {
		return t.response(args)
	}

	return &tool.ToolResponse{Call: args, Done: true}
}

func (t *probeTool) Render(call *tool.ToolCall) (string, string, string, map[string]string) {
	return "probe", "probe", "{}", map[string]string{}
}

func (t *probeTool) GetDescription() (string, bool) {
	return "", false
}

func TestExecuteToolCalls_ExecutesCallsInParallelAndAggregatesResponses(t *testing.T) {
	started := make(chan string, 2)
	release := make(chan struct{})
	var active int32
	overlap := &atomic.Bool{}

	registry := tool.NewToolRegistry()
	registry.Register("toolA", &probeTool{
		started: started,
		release: release,
		active:  &active,
		overlap: overlap,
		response: func(call *tool.ToolCall) *tool.ToolResponse {
			return &tool.ToolResponse{Call: call, Done: true, Result: tool.NewToolValue(map[string]any{"content": "A"})}
		},
	})
	registry.Register("toolB", &probeTool{
		started: started,
		release: release,
		active:  &active,
		overlap: overlap,
		response: func(call *tool.ToolCall) *tool.ToolResponse {
			return &tool.ToolResponse{Call: call, Done: true, Result: tool.NewToolValue(map[string]any{"content": "B"})}
		},
	})

	session := &SweSession{
		Tools:  registry,
		logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	}

	calls := []*tool.ToolCall{
		{ID: "call-1", Function: "toolA", Arguments: tool.NewToolValue(map[string]any{})},
		{ID: "call-2", Function: "toolB", Arguments: tool.NewToolValue(map[string]any{})},
	}

	errCh := make(chan error, 1)
	go func() {
		errCh <- session.executeToolCalls(calls)
	}()

	for i := 0; i < 2; i++ {
		select {
		case <-started:
		case <-time.After(2 * time.Second):
			t.Fatalf("timeout waiting for tool %d to start", i+1)
		}
	}

	close(release)
	require.NoError(t, <-errCh)

	assert.True(t, overlap.Load(), "tool calls should overlap in execution")
	require.Len(t, session.messages, 1)

	toolMessage := session.messages[0]
	assert.Equal(t, models.ChatRoleUser, toolMessage.Role)
	require.Len(t, toolMessage.GetToolResponses(), 2)
	assert.Equal(t, "call-1", toolMessage.GetToolResponses()[0].Call.ID)
	assert.Equal(t, "call-2", toolMessage.GetToolResponses()[1].Call.ID)
}

func TestExecuteToolCalls_PermissionQueryCollectsSuccessfulResponsesAsPending(t *testing.T) {
	started := make(chan string, 2)
	release := make(chan struct{})

	registry := tool.NewToolRegistry()
	registry.Register("requiresPerm", &probeTool{
		started: started,
		release: release,
		response: func(call *tool.ToolCall) *tool.ToolResponse {
			return &tool.ToolResponse{
				Call: call,
				Done: true,
				Error: &tool.ToolPermissionsQuery{
					Id:      "query-1",
					Tool:    call,
					Title:   "permission required",
					Details: "need approval",
				},
			}
		},
	})
	registry.Register("okTool", &probeTool{
		started: started,
		release: release,
		response: func(call *tool.ToolCall) *tool.ToolResponse {
			return &tool.ToolResponse{Call: call, Done: true, Result: tool.NewToolValue(map[string]any{"content": "ok"})}
		},
	})

	output := testutil.NewMockSessionOutputHandler()
	session := &SweSession{
		Tools:         registry,
		outputHandler: output,
		logger:        slog.New(slog.NewTextHandler(io.Discard, nil)),
	}

	calls := []*tool.ToolCall{
		{ID: "perm-call", Function: "requiresPerm", Arguments: tool.NewToolValue(map[string]any{})},
		{ID: "ok-call", Function: "okTool", Arguments: tool.NewToolValue(map[string]any{})},
	}

	errCh := make(chan error, 1)
	go func() {
		errCh <- session.executeToolCalls(calls)
	}()

	for i := 0; i < 2; i++ {
		select {
		case <-started:
		case <-time.After(2 * time.Second):
			t.Fatalf("timeout waiting for tool %d to start", i+1)
		}
	}

	close(release)
	err := <-errCh
	require.Error(t, err)
	require.IsType(t, &tool.ToolPermissionsQuery{}, err)

	assert.Empty(t, session.messages, "tool response message should be postponed until permission is granted")
	require.Len(t, session.pendingToolResponses, 1)
	assert.Equal(t, "ok-call", session.pendingToolResponses[0].Call.ID)
	require.Len(t, session.pendingPermissionToolCalls, 1)
	assert.Equal(t, "perm-call", session.pendingPermissionToolCalls[0].ID)
	require.Len(t, output.ToolCallResults, 1)
	assert.Equal(t, "ok-call", output.ToolCallResults[0].Call.ID)
}

func TestExecuteToolCalls_MultiplePermissionQueriesAreAllRetained(t *testing.T) {
	registry := tool.NewToolRegistry()
	permissionResponse := func(call *tool.ToolCall) *tool.ToolResponse {
		return &tool.ToolResponse{
			Call: call,
			Done: true,
			Error: &tool.ToolPermissionsQuery{
				Id:      "query-" + call.ID,
				Tool:    call,
				Title:   "permission required",
				Details: "need approval",
			},
		}
	}
	registry.Register("permA", &probeTool{response: permissionResponse})
	registry.Register("permB", &probeTool{response: permissionResponse})

	session := &SweSession{
		Tools:  registry,
		logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	}

	calls := []*tool.ToolCall{
		{ID: "a", Function: "permA", Arguments: tool.NewToolValue(map[string]any{})},
		{ID: "b", Function: "permB", Arguments: tool.NewToolValue(map[string]any{})},
	}

	err := session.executeToolCalls(calls)
	require.Error(t, err)
	query, ok := err.(*tool.ToolPermissionsQuery)
	require.True(t, ok)
	assert.Equal(t, "a", query.Tool.ID)
	assert.Empty(t, session.messages)
	require.Len(t, session.pendingPermissionToolCalls, 2)
	assert.Equal(t, "a", session.pendingPermissionToolCalls[0].ID)
	assert.Equal(t, "b", session.pendingPermissionToolCalls[1].ID)
	assert.Empty(t, session.pendingToolResponses)
}

func TestExecuteToolCalls_RespectsConfiguredMaxThreadsAndQueuesRemainingCalls(t *testing.T) {
	started := make(chan string, 3)
	release := make(chan struct{})
	var active int32
	var maxActive int32

	registry := tool.NewToolRegistry()
	registry.Register("toolA", &probeTool{started: started, release: release, active: &active, maxActive: &maxActive})
	registry.Register("toolB", &probeTool{started: started, release: release, active: &active, maxActive: &maxActive})
	registry.Register("toolC", &probeTool{started: started, release: release, active: &active, maxActive: &maxActive})

	configStore := impl.NewMockConfigStore()
	configStore.SetGlobalConfig(nil)

	session := &SweSession{
		Tools:          registry,
		logger:         slog.New(slog.NewTextHandler(io.Discard, nil)),
		configStore:    configStore,
		maxToolThreads: 2,
	}

	calls := []*tool.ToolCall{
		{ID: "call-1", Function: "toolA", Arguments: tool.NewToolValue(map[string]any{})},
		{ID: "call-2", Function: "toolB", Arguments: tool.NewToolValue(map[string]any{})},
		{ID: "call-3", Function: "toolC", Arguments: tool.NewToolValue(map[string]any{})},
	}

	errCh := make(chan error, 1)
	go func() {
		errCh <- session.executeToolCalls(calls)
	}()

	for i := 0; i < 2; i++ {
		select {
		case <-started:
		case <-time.After(2 * time.Second):
			t.Fatalf("timeout waiting for tool %d to start", i+1)
		}
	}

	select {
	case id := <-started:
		t.Fatalf("unexpected third tool started before queue release: %s", id)
	case <-time.After(400 * time.Millisecond):
	}

	close(release)
	require.NoError(t, <-errCh)

	require.Len(t, session.messages, 1)
	assert.LessOrEqual(t, atomic.LoadInt32(&maxActive), int32(2))
}

func TestExecuteToolCalls_SpacesToolStartsByMinimumDelay(t *testing.T) {
	startTimes := make([]time.Time, 0, 3)
	startTimesMu := sync.Mutex{}

	registry := tool.NewToolRegistry()
	registry.Register("toolA", &probeTool{startTimes: &startTimes, startTimesMu: &startTimesMu})
	registry.Register("toolB", &probeTool{startTimes: &startTimes, startTimesMu: &startTimesMu})
	registry.Register("toolC", &probeTool{startTimes: &startTimes, startTimesMu: &startTimesMu})

	session := &SweSession{
		Tools:          registry,
		logger:         slog.New(slog.NewTextHandler(io.Discard, nil)),
		maxToolThreads: 8,
	}

	calls := []*tool.ToolCall{
		{ID: "call-1", Function: "toolA", Arguments: tool.NewToolValue(map[string]any{})},
		{ID: "call-2", Function: "toolB", Arguments: tool.NewToolValue(map[string]any{})},
		{ID: "call-3", Function: "toolC", Arguments: tool.NewToolValue(map[string]any{})},
	}

	require.NoError(t, session.executeToolCalls(calls))
	startTimesMu.Lock()
	defer startTimesMu.Unlock()
	require.Len(t, startTimes, 3)

	for i := 1; i < len(startTimes); i++ {
		delta := startTimes[i].Sub(startTimes[i-1])
		assert.GreaterOrEqual(t, delta, 240*time.Millisecond)
	}
}

func TestMaxToolThreadsLimit_UsesOverrideThenConfigThenDefault(t *testing.T) {
	configStore := impl.NewMockConfigStore()
	configStore.SetGlobalConfig(&conf.GlobalConfig{Defaults: conf.CLIDefaultsConfig{MaxToolThreads: 11}})

	session := &SweSession{configStore: configStore}
	assert.Equal(t, 11, session.maxToolThreadsLimit())

	session.maxToolThreads = 3
	assert.Equal(t, 3, session.maxToolThreadsLimit())

	configStore.SetGlobalConfig(nil)
	session.maxToolThreads = 0
	assert.Equal(t, defaultMaxToolThreads, session.maxToolThreadsLimit())
}
