# pkg/gtv

`pkg/gtv` provides terminal-view primitives and theme infrastructure used by TUI code. It defines screen/event contracts, cell styling types, theme loading/application, and reusable test helpers for screen behavior verification.

## Major files

- `screen.go`: Core public API for terminal rendering and input events (`IScreenOutput`, `InputEvent`, geometry, color/attribute helpers).
- `theme.go`: Theme manager and screen interceptor that loads `*.theme.json` files and applies tag-based visual attributes.
- `mock.go`: Test helpers and doubles for screen verification and input-event capture.
