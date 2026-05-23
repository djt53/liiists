# liiists

One list app that works from the terminal, on iOS, and through any AI agent — all reading the same markdown files.

- **Terminal:** `brew install djt53/liiists/liiists`, then `liiists add books "Project Hail Mary"`
- **iOS:** [native app on the App Store](https://apps.apple.com/app/id6761671906) with iCloud sync, Share Extension, Siri, Widgets
- **AI agents:** [MCP server](#mcp-server) for Claude, Cursor, Codex CLI, etc., plus a drop-in [agent skill](#agent-skill)

Every surface reads and writes the same plain-text `.md` files. No proprietary format, no lock-in, no server you don't control.

**Frequently asked:**
- **Can I keep my lists in my Obsidian vault?** Yes — point `lists_dir` at any folder, including a vault. The iOS app supports this too via the document picker.
- **Can I version-control my lists with git?** Yes — they're just `.md` files in a directory. `git init` away.
- **Do I need an account?** No. Sign-in is only required to publish a list publicly via the iOS app's Discover surface; everything else works offline and locally.

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
echo "milk, bread, eggs" | liiists split groceries  # split stdin on commas/bullets and add
liiists where                      # show active lists directory
```

The CLI auto-detects the iCloud Drive container used by the iOS app, so your lists sync automatically.

Or point it at any directory — your Obsidian vault, a git-tracked dotfiles repo, anywhere:

```
# ~/.config/liiists/config
lists_dir=/Users/you/Obsidian/Vault/lists
```

(`liiists init` writes this for you; the snippet above is just what the file looks like.)

## MCP Server

Most list apps don't let your AI agent touch them. liiists ships an [MCP](https://modelcontextprotocol.io) server so Claude, Cursor, Codex CLI, and anything else that speaks MCP can read and write your lists directly — same files the iOS app and CLI use.

Eight tools cover the full surface: `list_lists`, `read_list`, `create_list`, `add_items`, `remove_item`, `check_item`, `delete_list`, `parse_text`.

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

Example prompts that just work:
- *"Add everything from this article to my reading list"* — paste a URL, the agent extracts titles and appends them
- *"What's on my groceries list?"* — agent reads the file directly
- *"Move the watched movies from my queue to my watched list"* — read, diff, write

## Agent Skill

[`SKILL.md`](./SKILL.md) is a drop-in skill file for Claude Code (and any coding agent that reads agent guides). Copy it into your skills directory and your agent will know how to drive the CLI — no MCP setup required. Useful if you're already in a terminal session and just want to say *"add this to my groceries list"* to your coding agent.

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

- `type` is `list` (plain bullets), `checklist` (checkboxes), or `log` (timestamped reverse-chronological entries)
- Frontmatter is optional -- a bare bullet list is a valid list
- Title resolves from: frontmatter > H1 heading > filename

Log entries use a timestamped bullet format — naive local datetime joined to text with an em-dash:

```markdown
- 2026-05-23T22:55 — Sirat #film
- 2026-05-22T19:30 — Severance S2E10
```

`liiists add <log> "<item>"` auto-stamps with the current minute; `--at "YYYY-MM-DD HH:MM"` backdates. MCP's `add_items` accepts an `at` field with the same format.

## iOS App

A native SwiftUI app, [live on the App Store](https://apps.apple.com/app/id6761671906), that reads and writes the same markdown files via iCloud Drive (or any folder you point it at). Includes:

- **Share Extension** — share a URL or text from any app into a chosen list
- **Siri Shortcuts** — *"Hey Siri, add milk to my groceries list"*
- **Widgets** — single list, all lists, or quick-add from the home screen
- **On-device AI "Suggest more"** — Apple Foundation Models suggests new items based on what's already on the list. Runs locally; no server call.
- **Discover** — optionally publish a list publicly. Other users can browse, upvote, and save published lists. Anonymous-first; no account needed unless you want to publish.
- **Log lists** — timestamped reverse-chronological entries for media journals, food diaries, etc. Searchable by text or date. (Also editable from the CLI and MCP — see the [Format](#the-format) section.)

Source: [djt53/liiists-app](https://github.com/djt53/liiists-app).

## License

MIT
