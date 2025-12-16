package test

import (
	"github.com/codesnort/codesnort-swe/internal/core"
)

type TestModelRequest struct {
}

type TestModelResponse struct {
	Content string
	Error   error
	Done    bool
}

type TestModel struct {
	Requests  []*TestModelRequest
	Responses []*TestModelResponse
}

type TestFixtureSystem struct {
	Model *TestModel
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
