package core

import "io"

type SweSystem interface {
	io.Closer
	NewProject(root string) SweProject
}

type SweProject interface {
	NewSession() SweSession
}

type SweTask interface {
	Subtasks() []SweTask
}

type SweSession interface {
	SetRole(role string) error
	Prompt(prompt string) error
}
