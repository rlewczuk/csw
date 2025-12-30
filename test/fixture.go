package test

import (
	"github.com/codesnort/codesnort-swe/internal/core"
	"github.com/codesnort/codesnort-swe/pkg/models/mock"
)

type TestModelRequest struct {
}

type TestModelResponse struct {
	Content string
	Error   error
	Done    bool
}

type TestFixtureSystem struct {
	ModelProvider *mock.MockProvider
}

func (t *TestFixtureSystem) Close() error {
	//TODO implement me
	panic("implement me")
}

func (t *TestFixtureSystem) NewProject(root string) core.SweProject {
	//TODO implement me
	panic("implement me")
}

func NewTestSystem() *TestFixtureSystem {
	return &TestFixtureSystem{}
}
