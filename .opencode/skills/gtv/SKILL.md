---
name: gtv
description: Create new TUI widget
license: MIT
compatibility: opencode
---

## What I do

Analyze user requirements and perform task taking into account following considerations:
- see `pkg/gtv/screen.go` for basic types and APIs used by TUI; 
- see `pkg/gtv/tui/widget.go` for `TWidget` and `IWidget` reference;
- ignore old bubbletea code in `pkg/ui/tui` and `pkg/gtv/tui` - it is legacy code that will be removed soon
- if you work on TUI widget, look at `pkg/gtv/tui/label.go` for sample widget implementation and `pkg/gtv/tui/application_integ_test.go` for reference how to implement tests;
- if you look on focusable widget, look at `pkg/gtv/tui/focusable.go` and `pkg/gtv/tui/focusable_test.go` and `pkg/gtv/tui/input_box.go` for reference;
- when implementing or changing behavior, implement unit tests or adapt existing tests that are outdated because of change;
  - ensure that all common and corner cases are covered by tests;
- always run all tests in whole project at the end and fix any issues occuring;

## When to use me

Use this when you work on code in `pkg/gtv` package.

