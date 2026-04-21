package core

import (
	"io"
	"log/slog"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rlewczuk/csw/pkg/conf"
	"github.com/rlewczuk/csw/pkg/models"
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
		toolStartDelay: time.Millisecond,
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



func TestExecuteToolCalls_RespectsConfiguredMaxThreadsAndQueuesRemainingCalls(t *testing.T) {
	started := make(chan string, 3)
	release := make(chan struct{})
	var active int32
	var maxActive int32

	registry := tool.NewToolRegistry()
	registry.Register("toolA", &probeTool{started: started, release: release, active: &active, maxActive: &maxActive})
	registry.Register("toolB", &probeTool{started: started, release: release, active: &active, maxActive: &maxActive})
	registry.Register("toolC", &probeTool{started: started, release: release, active: &active, maxActive: &maxActive})

	configStore := &conf.CswConfig{}

	session := &SweSession{
		Tools:          registry,
		logger:         slog.New(slog.NewTextHandler(io.Discard, nil)),
		configStore:    configStore,
		maxToolThreads: 2,
		toolStartDelay: time.Millisecond,
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
	case <-time.After(5 * time.Millisecond):
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
		toolStartDelay: time.Millisecond,
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
		assert.GreaterOrEqual(t, delta, 500*time.Microsecond)
	}
}

func TestMaxToolThreadsLimit_UsesOverrideThenConfigThenDefault(t *testing.T) {
	configStore := &conf.CswConfig{GlobalConfig: &conf.GlobalConfig{Defaults: conf.RunDefaultsConfig{MaxThreads: 11}}}

	session := &SweSession{configStore: configStore}
	assert.Equal(t, 11, session.maxToolThreadsLimit())

	session.maxToolThreads = 3
	assert.Equal(t, 3, session.maxToolThreadsLimit())

	configStore.GlobalConfig = nil
	session.maxToolThreads = 0
	assert.Equal(t, defaultMaxToolThreads, session.maxToolThreadsLimit())
}
