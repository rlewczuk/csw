// Package testfixture provides TUI integration test fixtures.
package testfixture

import (
	"testing"

	coretestfixture "github.com/rlewczuk/csw/pkg/core/testfixture"
	presentertestfixture "github.com/rlewczuk/csw/pkg/presenter/testfixture"
)

// TuiFixture provides shared setup for TUI integration tests.
type TuiFixture struct {
	*presentertestfixture.PresenterFixture
}

// NewTuiFixture creates a TUI fixture with a SweSystem.
func NewTuiFixture(t *testing.T, opts ...coretestfixture.SweSystemFixtureOption) *TuiFixture {
	return &TuiFixture{
		PresenterFixture: presentertestfixture.NewPresenterFixture(t, opts...),
	}
}
