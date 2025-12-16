package core

import "io"

type SweSystem interface {
	io.Closer
	NewProject(root string) SweProject

	// TODO tool manager ?
	// TODO runtime manager ?
	// TODO lsp manager ?
	// TODO vfs manager ?
	// TODO ui manager ?

}

type SweProject interface {
	NewSession() SweSession
}

type SweTask interface {
}

type SweSession interface {
	SetRole(role string) error
	Prompt(prompt string) error
}
