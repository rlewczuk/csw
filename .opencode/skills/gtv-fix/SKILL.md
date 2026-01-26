---
name: gtv-fix
description: Debug and Fix GTV TUI issues
license: MIT
compatibility: opencode
---

## What I do

Analyze user buf report and try finding and fixing the issue:
- gtv library has layered design, unless user specifies otherwise, assume that issue can be in any of the layers;
  - for example, if issue is in widget rendering, it can be in widget implementation, in layout implementation, in theme implementation, in screen buffer implementation, in terminal rendering, etc.;
  - another example, if issue is with wrong or no reaction to key press, it can be in widget implementation, in layout implementation, in event handling, in key parsing, etc.
- always start with thorough analysis, then try to reproduce problem by writing unit test or integration test, then fix the issue and validate that test pass;
  - test should be added to a file appropriate to component being suspected of having the issue (for example widget from `input_box.go` should have test added to `input_box_test.go`);
  - if test does not reproduce the issue, start again with analysis and write another test;
- after fixing the issue, run all tests in project to ensure that no other tests are broken by the fix;
- take into consideration structure of GTV library described below:
- always read `pkg/gtv/screen.go` and `pkg/gtv/tui/widget.go` files, remaining files should be read only when needed;
- if issue stems from not using proper widget, consifer refactoring UI to use proper widget, do not fix it by adding more code to work around wrong widget usage;
  - for example if view does not resize properly, it is possible that wrong layout is used, or layout is not used at all and absolute positioning is used instead -- use for example flex layout (or other if applicable);

Structure and design of GTV TUI library:
* library is located in `pkg/gtv` package;
* `pkg/gtv` package contains following subpackages:
  - file `screen.go` contains basic types and APIs used by TUI, see `pkg/gtv/screen.go` for reference and read it carefully;
    - always read `pkg/gtv/screen.go` file as it is central to understanding how TUI library communicates with terminal;
  - library operates on memory buffer (`pkg/gtv/tio/buffer.go`) that is then rendered to terminal, all widgets render to memory buffer and receive parsed input events;
  - `pkg/gtv/tio` - terminal I/O implementation -- this is lowest layer that deals with terminal input and output;
    - file `input.go` contains code parsing input events (keys, mouse, resize events) from terminal and converting them to `InputEvent` objects;
    - file `buffer.go` contains implementation of memory buffer that is used by TUI library to render widgets to;
    - file `renderer.go` contains implementation of renderer that takes memory buffer and renders it to terminal;
  - `pkg/gtv/tui` - TUI widgets and application implementation;
    - file `widget.go` contains base widget implementation and widget interface;
    - file `application.go` contains `TApplication` implementation that is responsible for running main event loop and managing screen and widgets;
    - each widget consists of `T<WidgetName>` struct and `I<WidgetName>` interface, see `widget.go` for reference;
    - remaining files contain implementation of individual widgets:
      - `button.go` - button (as in dialogs);
      - `input_box.go` - input box, `text_area.go` - multiline input box;
      - `label.go` - label widget;
      - `layout.go` - layout widget, `absolute_layout.go` - absolute layout widget, `flex_layout.go` - flex layout widget, `zaxis_layout.go` - z-axis layout widget;
      - `menu.go` - menu widget;
      - `frame.go` - displays frame with a widget inside (eg. layout with more widgets inside) with border and title;
      - `resizable.go` - abstract class for resizable widget, base for all widgets that can be resized;
      - `focusable.go` - abstract class for focusable widget, base for all widgets that can be focused; 
  - `mdv/*.go` - markdown view implementation;
  - `util/*.go` - utility functions and types used by other packages;

## When to use me

Use this skill to track and fix bugs in TUI code.

