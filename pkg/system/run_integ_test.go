package system

import (
	"testing"
	"time"

	"github.com/rlewczuk/csw/pkg/core"
	coretestfixture "github.com/rlewczuk/csw/pkg/core/testfixture"
	"github.com/stretchr/testify/require"
)

// cli_integ_test.go contains shared test fixtures and helpers
// used by the CLI integration test suite.

// newCliSystemFixture creates a new SweSystemFixture for CLI integration tests
func newCliSystemFixture(t *testing.T, prompt string, opts ...coretestfixture.SweSystemFixtureOption) *coretestfixture.SweSystemFixture {
	base := []coretestfixture.SweSystemFixtureOption{
		coretestfixture.WithPromptGenerator(coretestfixture.NewStaticPromptGenerator(prompt)),
	}
	return coretestfixture.NewSweSystemFixture(t, append(base, opts...)...)
}

func waitForThreadToFinish(t *testing.T, thread *core.SessionThread) {
	t.Helper()
	require.Eventually(t, func() bool {
		return !thread.IsRunning()
	}, 10*time.Second, 10*time.Millisecond)
}
