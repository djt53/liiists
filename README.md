# liiists

A dead-simple list tool where markdown files are the source of truth. Every interface -- iOS app, CLI, AI tools -- reads and writes the same files.

## CLI

Manage lists from your terminal. Written in Go, zero dependencies.

```bash
brew install djt53/liiists/liiists
```

```bash
liiists init                       # set up your lists directory
liiists new "Books to Read"        # create a list
liiists add books "Project Hail Mary"  # add an item
liiists ls                         # show all lists
liiists ls books                   # show items in a list
liiists check books "Project Hail Mary"  # toggle a checkbox
liiists rm books "Project Hail Mary"     # remove an item
echo "messy, text, input" | liiists parse books  # parse text into items
liiists where                      # show active lists directory
```

The CLI auto-detects the iCloud Drive container used by the iOS app, so your lists sync automatically.

Or point it at any directory:

```yaml
# ~/.config/liiists/config.yaml
lists_dir: ~/my-lists
```

## MCP Server

Lets AI assistants manage your lists programmatically. 8 tools: `list_lists`, `read_list`, `create_list`, `add_items`, `remove_item`, `check_item`, `delete_list`, `parse_text`.

```json
{
  "mcpServers": {
    "liiists": {
      "command": "node",
      "args": ["/path/to/liiists/mcp/index.js"]
    }
  }
}
```

## Agent Skill

[`SKILL.md`](./SKILL.md) is a drop-in skill file for Claude Code (and any coding agent that reads agent guides). Copy it into your skills directory and your agent will know how to manage your lists via the CLI.

## The Format

Lists are plain markdown files. That's it.

```markdown
---
title: Books to Read
type: checklist
created: 2026-03-26
---

- [x] Project Hail Mary
- [ ] Tomorrow, and Tomorrow, and Tomorrow
- [ ] The Kaiju Preservation Society
```

- `type` is `list` (plain bullets) or `checklist` (checkboxes)
- Frontmatter is optional -- a bare bullet list is a valid list
- Title resolves from: frontmatter > H1 heading > filename

## iOS App

A native SwiftUI app, [live on the App Store](https://apps.apple.com/app/id6761671906), that reads and writes the same markdown files via iCloud Drive. Share Extension, Siri Shortcuts, Widgets, and a social Discover surface for publishing and browsing lists.

The app source lives at [djt53/liiists-app](https://github.com/djt53/liiists-app).

## License

MIT
