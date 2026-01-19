---
name: new-tui-widget
description: Create new TUI widget
license: MIT
compatibility: opencode
---

## What I do

Analyze user requirements and create new TUI widget in `pkg/gophertv/tui/<widget-name>.go` taking into account following considerations:

- Widget should be implemented as a struct that embeds `tui.TWidget` and implements `tui.IWidget` interface
  - see `pkg/gophertv/tui/widget.go` for `TWidget` and `IWidget` reference;
  - see `pkg/gophertv/screen.go` for basic types and APIs used by TUI;
  - see existing widgets for reference: `pkg/gophertv/tui/label.go`, `pkg/gophertv/tui/layout.go`, for reference
- Struct name should start with `T` and widget name should be in CamelCase, for example `TLabel` or `TAbsoluteLayout`
- There should be widget interface coming with struct, for example `ILabel` or `IAbsoluteLayout` 
  - interface should contain all public methods exposed by widget 
  - interface should extend `tui.IWidget` interface
- implement set of unit tests for the widget in `pkg/gophertv/tui/<widget-name>_test.go`
  - see `pkg/gophertv/tui/application_integ_test.go` for reference how to implement tests;
  - always test widget behavior simulating full application as in `pkg/gophertv/tui/application_integ_test.go`
  - use mock terminal in put from `pkg/gophertv/tio/input_mock.go`
  - use screen verifier from `pkg/gophertv/mock.go`
  - use testify assertions instead of manual `if` statements
  - always run all tests in whole project at the end and verify if they work correctly, fix any issues occuring
- ignore old bubbletea code in `pkg/ui/tui` and `pkg/gophertv/tui` - it is legacy code that will be removed soon

## When to use me

Use this when you have to implement new TUI widget, typically in `pkg/gophertv/tui` package.
