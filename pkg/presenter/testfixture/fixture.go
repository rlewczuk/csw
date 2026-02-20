// Package testfixture provides presenter integration test fixtures.
package testfixture

import (
	"testing"

	"github.com/rlewczuk/csw/pkg/core"
	coretestfixture "github.com/rlewczuk/csw/pkg/core/testfixture"
	"github.com/rlewczuk/csw/pkg/presenter"
)

// PresenterFixture provides shared setup for presenter integration tests.
type PresenterFixture struct {
	*coretestfixture.SweSystemFixture
}

// NewPresenterFixture creates a presenter fixture with a SweSystem.
func NewPresenterFixture(t *testing.T, opts ...coretestfixture.SweSystemFixtureOption) *PresenterFixture {
	return &PresenterFixture{
		SweSystemFixture: coretestfixture.NewSweSystemFixture(t, opts...),
	}
}

// NewSessionThread creates a new session thread for the fixture system.
func (f *PresenterFixture) NewSessionThread(handler core.SessionThreadOutput) *core.SessionThread {
	return core.NewSessionThread(f.System, handler)
}

// NewSessionThreadWithSession creates a new session thread with an existing session.
func (f *PresenterFixture) NewSessionThreadWithSession(session *core.SweSession, handler core.SessionThreadOutput) *core.SessionThread {
	return core.NewSessionThreadWithSession(f.System, session, handler)
}

// NewAppPresenter creates an AppPresenter for the fixture system.
func (f *PresenterFixture) NewAppPresenter(model string, role string) *presenter.AppPresenter {
	return presenter.NewAppPresenter(f.System, model, role)
}

// NewChatPresenter creates a ChatPresenter for the fixture system.
func (f *PresenterFixture) NewChatPresenter(thread *core.SessionThread) *presenter.ChatPresenter {
	return presenter.NewChatPresenter(f.System, thread)
}
