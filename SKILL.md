---
name: liiists
description: Read and write the user's personal lists (books, movies, restaurants, todos, etc.) via the `liiists` CLI. Use whenever the user wants to add items to a list, view a list, create a new list, check things off, or dump messy text to be parsed into entries.
allowed-tools: [Bash, Read, Write]
---

# liiists

A dead-simple list manager backed by markdown files. Each list is one `.md` file in the user's lists directory (auto-detected, usually iCloud). Use the `liiists` CLI for all reads and writes — it handles file format, frontmatter, and sync correctly.

## When to use this skill

Trigger on any of:
- "add X to my Y list" / "put X on my list"
- "what's on my reading list" / "show me my todos"
- "make a new list for X"
- "check off X" / "I finished X"
- "I have a bunch of stuff to add" (free-text dump → use `parse`)
- Any reference to a personal list (books, movies, restaurants, gifts, packing, groceries, watchlist, todo, etc.)

Do **not** use this skill for:
- Project task tracking handled by other tools (`tasks.md`, GitHub issues, etc.)
- One-off reminders that belong in a calendar/Reminders app

## Setup check

Before the first command in a session, verify the CLI is installed and the lists directory is configured:

    liiists where

If the command is missing, tell the user: "Install liiists first — `brew install djt53/liiists/liiists`." Do not attempt the install yourself. If `where` reports no directory, run `liiists init`.

## Commands

| Goal | Command |
|------|---------|
| Show all lists | `liiists ls` |
| Show items in a list | `liiists ls <name>` |
| Create a new list | `liiists new <name>` |
| Create a checklist | `liiists new <name> --checklist` |
| Create a log | `liiists new <name> --log` |
| Add an item | `liiists add <list> "<item>"` |
| Add a log entry at a specific time | `liiists add <log> "<item>" --at "YYYY-MM-DD HH:MM"` |
| Add many items from messy text | `echo "<text>" \| liiists split <list>` |
| Remove an item | `liiists rm <list> "<item>"` |
| Toggle a checkbox (checklists) | `liiists check <list> "<item>"` |
| Show lists directory | `liiists where` |

List names are slugified filenames (`books-to-read`). Slugify informal names before passing to the CLI.

## Key behaviors

**Reading lists.** Always run `liiists ls <name>` rather than reading the markdown file directly — the CLI resolves titles and frontmatter consistently. Only `Read` the raw `.md` file when the user explicitly asks to see the source.

**Adding items.** Quote items with spaces. For a single item use `add`. For 3+ items at once, or any free-text dump, pipe to `split`.

**List types.**
- **list** — plain bullets
- **checklist** — supports `check` to toggle `[x]`
- **log** — timestamped reverse-chronological entries (media journals, food logs, etc.). `add` auto-stamps with the current minute; pass `--at "YYYY-MM-DD HH:MM"` to backdate. `check` does not apply to logs.

To convert a list's type, edit the file's frontmatter `type:` field directly.

**Don't invent list names.** If the user references "my list" ambiguously, run `liiists ls` first and ask which one. Never silently create a list to satisfy an `add`.

**Confirm destructive ops.** Confirm before `rm` unless the user explicitly said "remove" or "delete".

## Output style

- After adding items, brief confirmation: "Added 3 items to **books-to-read**."
- After `ls`, render items as a clean bullet list, not raw CLI output.
- Surface errors verbatim — they usually indicate a missing list or sync issue.

---

**Note on portability.** This file uses Claude Code's SKILL.md frontmatter format, but the body is plain instructions any coding-agent (Cursor, Codex CLI, etc.) can read. Other tools can ignore the frontmatter and treat this as an agent guide.
