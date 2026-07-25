# Skills

Reusable instruction prompts the AI can load during a conversation via `read_skill`.

## Built-in skills

These ship with the app (embedded) and are available to every user by default:

| Name | Purpose |
|------|---------|
| `good-widgets` | Native full-width chat widget layout/theming |
| `match-stats-widget` | Football match widgets via TheSportsDB |

Disable them for **your account** under **Settings → General → Enable built-in skills**.

A user skill with the **same name** always replaces the built-in for that user (shadowing).

## Markdown format

Descriptions are critical — they are what the model sees in `<available_skills>` before deciding to call `read_skill`.

```markdown
---
name: my-skill
description: One or two sentences: what it does and when the model should use it.
---

# my-skill

…full instructions…
```

## Custom skills

Create skills under **Settings → Skills**, or upload a `.md` file with the frontmatter above.

| File | Name | Purpose |
|------|------|---------|
| `good-widgets.md` | `good-widgets` | Same content as the built-in (for reference / override) |
| `match-stats-widget.md` | `match-stats-widget` | Same content as the built-in (for reference / override) |
