---
name: gophertv-work
description: Create new TUI widget
license: MIT
compatibility: opencode
---

## What I do

Analyze user requirements and perform task taking into account following considerations:
- see `pkg/gophertv/screen.go` for basic types and APIs used by TUI; 
- see `pkg/gophertv/tui/widget.go` for `TWidget` and `IWidget` reference;
- ignore old bubbletea code in `pkg/ui/tui` and `pkg/gophertv/tui` - it is legacy code that will be removed soon
- if you work on TUI widget, look at `pkg/gophertv/tui/label.go` for sample widget implementation and `pkg/gophertv/tui/application_integ_test.go` for reference how to implement tests;
- always run all tests in whole project at the end and fix any issues occuring;

## When to use me

Use this when you work on code in `pkg/gophertv` package.

