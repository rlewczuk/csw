---
name: gtv-widget
description: Create new TUI widget for gtv
license: MIT
compatibility: opencode
---

## What I do

Analyze user requirements and create new TUI widget in `pkg/gtv/tui/<widget-name>.go` taking into account following considerations:

- Widget should be implemented as a struct that embeds `tui.TWidget` and implements `tui.IWidget` interface
  - see `pkg/gtv/tui/widget.go` for `TWidget` and `IWidget` reference;
  - see `pkg/gtv/screen.go` for basic types and APIs used by TUI;
  - see existing widgets for reference: `pkg/gtv/tui/label.go`, `pkg/gtv/tui/layout.go`, for reference
- Struct name should start with `T` and widget name should be in CamelCase, for example `TLabel` or `TAbsoluteLayout`
- There should be widget interface coming with struct, for example `ILabel` or `IAbsoluteLayout` 
  - interface should contain all public methods exposed by widget 
  - interface should extend `tui.IWidget` interface
- implement set of unit tests for the widget in `pkg/gtv/tui/<widget-name>_test.go`
  - see `pkg/gtv/tui/application_integ_test.go` for reference how to implement tests;
  - always test widget behavior simulating full application as in `pkg/gtv/tui/application_integ_test.go`
  - use mock terminal in put from `pkg/gtv/tio/input_mock.go`
  - use screen verifier from `pkg/gtv/mock.go`
  - use testify assertions instead of manual `if` statements
  - always run all tests in whole project at the end and verify if they work correctly, fix any issues occuring
- ignore old bubbletea code in `pkg/ui/tui` and `pkg/gtv/tui` - it is legacy code that will be removed soon
- make sure theme cell attributes are used, instead of hardcoded:
  - widget shoud use `CellTag()` to set theme tag on widget creation
  - theme tag names should start with `<widget-name>-` prefix, for example `label-`, `button-`, `layout-`
  - update default theme in `pkg/gtv/tui/themes/default.theme.json` to include new theme tag names and colors
  

## When to use me

Use this when you have to implement new TUI widget, typically in `pkg/gtv/tui` package.
