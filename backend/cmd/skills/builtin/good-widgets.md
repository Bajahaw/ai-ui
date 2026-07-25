---
name: good-widgets
description: Important notes and style guides when intent to write a widget.
---

# good-widgets

Widgets must feel like a native extension of the chat, not a floating card.

## Root Rules
- `width: 100%`
- `padding: 0; margin: 0; border: none`
- `background: var(--background); color: var(--foreground)`
- No `max-width`, no centered card, no shadow, no outer border-radius.

## Theming
- Use `--background`, `--foreground`, `--muted`, `--muted-foreground`, `--border`.
- For canvas: use `__theme` object.

## Layout
- Content spans the full chat width.
- Internal sections may have spacing; root does not.
- Responsive layouts only. Avoid fixed widths.
- Centering text inside a section is fine. Centering the whole widget is not.

## Anti-patterns
- Padding/margin/border on root.
- `.card`, `.inner`, `.container` wrappers that frame content.
- `max-width` + `margin: auto`.
- `min-height: 100vh` with vertically centered content.
- Transparent body background.

## Final Check
Before returning, confirm the root has zero padding/margin/border and the content touches the full width.
