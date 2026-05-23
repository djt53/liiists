#!/usr/bin/env node

import { McpServer } from "@modelcontextprotocol/sdk/server/mcp.js";
import { StdioServerTransport } from "@modelcontextprotocol/sdk/server/stdio.js";
import { z } from "zod";
import fs from "fs";
import path from "path";
import os from "os";

// --- Markdown parser/writer (mirrors cli/list.go) ---

// Log entries use ISO 8601 (minute resolution, no timezone) joined to text by
// an em-dash with spaces on both sides. See decision 016. Matches the layout
// in cli/list.go and the iOS MarkdownParser.
const LOG_SEPARATOR = " — ";
const LOG_TIMESTAMP_REGEX = /^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}$/;

function parseLogTimestamp(s) {
  if (!LOG_TIMESTAMP_REGEX.test(s)) return null;
  const [datePart, timePart] = s.split("T");
  const [y, m, d] = datePart.split("-").map(Number);
  const [hh, mm] = timePart.split(":").map(Number);
  return new Date(y, m - 1, d, hh, mm);
}

function formatLogTimestamp(d) {
  const pad = (n) => String(n).padStart(2, "0");
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}T${pad(d.getHours())}:${pad(d.getMinutes())}`;
}

// Accepts the strict ISO `T` form ("2026-05-23T22:55") or the human-friendly
// space form ("2026-05-23 22:55"). Mirrors the CLI's --at flag.
function parseAtParam(raw) {
  const normalized = raw.replace(" ", "T");
  return parseLogTimestamp(normalized);
}

function nowToMinute() {
  const now = new Date();
  return new Date(now.getFullYear(), now.getMonth(), now.getDate(), now.getHours(), now.getMinutes());
}

function parseLogLine(line) {
  if (!line.startsWith("- ")) return null;
  const body = line.slice(2);
  const idx = body.indexOf(LOG_SEPARATOR);
  if (idx < 0) return null;
  const tsStr = body.slice(0, idx).trim();
  const text = body.slice(idx + LOG_SEPARATOR.length);
  const ts = parseLogTimestamp(tsStr);
  if (!ts) return null;
  return { text, isChecked: false, timestamp: ts };
}

function sortLogItems(items) {
  // Newest first; entries without a timestamp sink to the bottom.
  items.sort((a, b) => {
    if (!a.timestamp) return 1;
    if (!b.timestamp) return -1;
    return b.timestamp - a.timestamp;
  });
}

function getListsDir() {
  const home = os.homedir();
  const configPath = path.join(home, ".config", "liiists", "config");
  try {
    const data = fs.readFileSync(configPath, "utf8");
    for (const line of data.split("\n")) {
      const [key, ...rest] = line.split("=");
      if (key?.trim() === "lists_dir") {
        let dir = rest.join("=").trim();
        if (dir.startsWith("~/")) dir = path.join(home, dir.slice(2));
        return dir;
      }
    }
  } catch {}
  return path.join(home, "lists");
}

function slugify(s) {
  return s
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, "-")
    .replace(/^-|-$/g, "");
}

function deslugify(s) {
  return s
    .split("-")
    .map((w) => w.charAt(0).toUpperCase() + w.slice(1))
    .join(" ");
}

function parseList(content, filePath) {
  const list = {
    title: "",
    type: "list",
    created: "",
    items: [],
    extra: {},
    path: filePath,
  };

  let body = content;

  // Parse frontmatter
  const fmMatch = content.match(/^---\n([\s\S]+?)\n---\n?/);
  if (fmMatch) {
    body = content.slice(fmMatch[0].length);
    for (const line of fmMatch[1].split("\n")) {
      const idx = line.indexOf(":");
      if (idx === -1) continue;
      const key = line.slice(0, idx).trim();
      const val = line.slice(idx + 1).trim();
      if (key === "title") list.title = val;
      else if (key === "type") list.type = val;
      else if (key === "created") list.created = val;
      else list.extra[key] = val;
    }
  }

  // Parse H1 title
  if (!list.title) {
    const lines = body.split("\n");
    for (let i = 0; i < lines.length; i++) {
      if (lines[i].startsWith("# ")) {
        list.title = lines[i].slice(2);
        lines.splice(i, 1);
        body = lines.join("\n");
        break;
      }
    }
  }

  // Fallback: filename
  if (!list.title && filePath) {
    list.title = deslugify(path.basename(filePath, ".md"));
  }

  // Parse items
  for (const line of body.split("\n")) {
    if (list.type === "log") {
      const item = parseLogLine(line);
      if (item) list.items.push(item);
      continue;
    }
    if (line.startsWith("- [x] ")) {
      list.items.push({ text: line.slice(6), isChecked: true });
    } else if (line.startsWith("- [ ] ")) {
      list.items.push({ text: line.slice(6), isChecked: false });
    } else if (line.startsWith("- ")) {
      list.items.push({ text: line.slice(2), isChecked: false });
    }
  }

  if (list.type === "log") {
    sortLogItems(list.items);
  }

  return list;
}

function renderList(list) {
  let out = "---\n";
  out += `title: ${list.title}\n`;
  out += `type: ${list.type}\n`;
  if (list.created) out += `created: ${list.created}\n`;
  for (const [k, v] of Object.entries(list.extra || {})) {
    out += `${k}: ${v}\n`;
  }
  out += "---\n\n";

  if (list.type === "log") {
    // Reverse-chrono on disk so the file reads the same way the iOS app
    // displays.
    sortLogItems(list.items);
    for (const item of list.items) {
      if (!item.timestamp) continue;
      out += `- ${formatLogTimestamp(item.timestamp)}${LOG_SEPARATOR}${item.text}\n`;
    }
  } else {
    for (const item of list.items) {
      if (list.type === "checklist") {
        out += item.isChecked ? `- [x] ${item.text}\n` : `- [ ] ${item.text}\n`;
      } else {
        out += `- ${item.text}\n`;
      }
    }
  }

  return out;
}

function loadAllLists() {
  const dir = getListsDir();
  if (!fs.existsSync(dir)) return [];

  return fs
    .readdirSync(dir)
    .filter((f) => f.endsWith(".md"))
    .map((f) => {
      const content = fs.readFileSync(path.join(dir, f), "utf8");
      return parseList(content, path.join(dir, f));
    });
}

function findList(name) {
  const dir = getListsDir();
  const slug = slugify(name);
  const exactPath = path.join(dir, slug + ".md");

  if (fs.existsSync(exactPath)) {
    return parseList(fs.readFileSync(exactPath, "utf8"), exactPath);
  }

  // Fuzzy match by title
  const lists = loadAllLists();
  const nameLower = name.toLowerCase();
  return lists.find((l) => l.title.toLowerCase() === nameLower) || null;
}

function todayStr() {
  return new Date().toISOString().slice(0, 10);
}

// --- MCP Server ---

const server = new McpServer({
  name: "liiists",
  version: "1.0.0",
});

// Tool: list_lists
server.tool(
  "list_lists",
  "List all available lists with their item counts",
  {},
  async () => {
    const lists = loadAllLists();
    if (lists.length === 0) {
      return {
        content: [{ type: "text", text: "No lists found. Create one with create_list." }],
      };
    }

    const summary = lists.map((l) => {
      const count = l.items.length;
      if (l.type === "checklist") {
        const checked = l.items.filter((i) => i.isChecked).length;
        return `${l.title} (${checked}/${count} checked) [${l.type}]`;
      }
      return `${l.title} (${count} items) [${l.type}]`;
    });

    return {
      content: [{ type: "text", text: summary.join("\n") }],
    };
  }
);

// Tool: read_list
server.tool(
  "read_list",
  "Read the contents of a specific list",
  { name: z.string().describe("List name or slug") },
  async ({ name }) => {
    const list = findList(name);
    if (!list) {
      return {
        content: [{ type: "text", text: `List not found: ${name}` }],
        isError: true,
      };
    }

    let text = `${list.title} [${list.type}]\n`;
    if (list.items.length === 0) {
      text += "(empty)";
    } else {
      text += list.items
        .map((item) => {
          if (list.type === "checklist") {
            return item.isChecked ? `[x] ${item.text}` : `[ ] ${item.text}`;
          }
          if (list.type === "log" && item.timestamp) {
            return `${formatLogTimestamp(item.timestamp)}  ${item.text}`;
          }
          return `- ${item.text}`;
        })
        .join("\n");
    }

    return { content: [{ type: "text", text }] };
  }
);

// Tool: create_list
server.tool(
  "create_list",
  "Create a new list",
  {
    name: z.string().describe("Name for the new list"),
    type: z
      .enum(["list", "checklist", "log"])
      .optional()
      .default("list")
      .describe("List type: 'list' (plain), 'checklist' (with checkboxes), or 'log' (timestamped reverse-chronological entries)"),
  },
  async ({ name, type }) => {
    const dir = getListsDir();
    if (!fs.existsSync(dir)) {
      fs.mkdirSync(dir, { recursive: true });
    }

    const slug = slugify(name);
    const filePath = path.join(dir, slug + ".md");

    if (fs.existsSync(filePath)) {
      return {
        content: [{ type: "text", text: `List already exists: ${name}` }],
        isError: true,
      };
    }

    const list = {
      title: name,
      type: type || "list",
      created: todayStr(),
      items: [],
      extra: {},
      path: filePath,
    };

    fs.writeFileSync(filePath, renderList(list));
    return {
      content: [{ type: "text", text: `Created list: ${name} (${slug}.md)` }],
    };
  }
);

// Tool: add_items
server.tool(
  "add_items",
  "Add one or more items to a list. For log lists, entries auto-stamp with the current minute unless `at` is provided.",
  {
    list: z.string().describe("List name or slug"),
    items: z.array(z.string()).describe("Items to add"),
    at: z
      .string()
      .optional()
      .describe(
        "Log lists only. Timestamp to apply to all added entries, format 'YYYY-MM-DDTHH:MM' or 'YYYY-MM-DD HH:MM' (local time, no timezone). Defaults to now."
      ),
  },
  async ({ list: listName, items, at }) => {
    const list = findList(listName);
    if (!list) {
      return {
        content: [{ type: "text", text: `List not found: ${listName}` }],
        isError: true,
      };
    }

    let stamp = null;
    if (list.type === "log") {
      if (at) {
        stamp = parseAtParam(at);
        if (!stamp) {
          return {
            content: [
              {
                type: "text",
                text: `'at' must be 'YYYY-MM-DDTHH:MM' or 'YYYY-MM-DD HH:MM', got: ${at}`,
              },
            ],
            isError: true,
          };
        }
      } else {
        stamp = nowToMinute();
      }
    } else if (at) {
      return {
        content: [
          {
            type: "text",
            text: `'at' is only valid for log lists; '${list.title}' is a ${list.type}.`,
          },
        ],
        isError: true,
      };
    }

    const addedLines = [];
    for (const text of items) {
      if (list.type === "log") {
        list.items.push({ text, isChecked: false, timestamp: stamp });
        addedLines.push(`+ ${formatLogTimestamp(stamp)}  ${text}`);
      } else {
        list.items.push({ text, isChecked: false });
        addedLines.push(`+ ${text}`);
      }
    }

    fs.writeFileSync(list.path, renderList(list));
    return {
      content: [
        {
          type: "text",
          text: `Added ${items.length} item(s) to ${list.title}:\n${addedLines.join("\n")}`,
        },
      ],
    };
  }
);

// Tool: remove_item
server.tool(
  "remove_item",
  "Remove an item from a list by its text (case-insensitive match)",
  {
    list: z.string().describe("List name or slug"),
    item: z.string().describe("Item text to remove"),
  },
  async ({ list: listName, item }) => {
    const list = findList(listName);
    if (!list) {
      return {
        content: [{ type: "text", text: `List not found: ${listName}` }],
        isError: true,
      };
    }

    const itemLower = item.toLowerCase();
    const idx = list.items.findIndex((i) => i.text.toLowerCase() === itemLower);
    if (idx === -1) {
      return {
        content: [{ type: "text", text: `Item not found: ${item}` }],
        isError: true,
      };
    }

    const removed = list.items.splice(idx, 1)[0];
    fs.writeFileSync(list.path, renderList(list));
    return {
      content: [{ type: "text", text: `Removed: ${removed.text}` }],
    };
  }
);

// Tool: check_item
server.tool(
  "check_item",
  "Toggle an item's checkbox in a checklist (check/uncheck)",
  {
    list: z.string().describe("List name or slug"),
    item: z.string().describe("Item text to toggle"),
  },
  async ({ list: listName, item }) => {
    const list = findList(listName);
    if (!list) {
      return {
        content: [{ type: "text", text: `List not found: ${listName}` }],
        isError: true,
      };
    }

    if (list.type !== "checklist") {
      return {
        content: [{ type: "text", text: `'${list.title}' is not a checklist` }],
        isError: true,
      };
    }

    const itemLower = item.toLowerCase();
    const found = list.items.find((i) => i.text.toLowerCase() === itemLower);
    if (!found) {
      return {
        content: [{ type: "text", text: `Item not found: ${item}` }],
        isError: true,
      };
    }

    found.isChecked = !found.isChecked;
    fs.writeFileSync(list.path, renderList(list));
    return {
      content: [
        {
          type: "text",
          text: found.isChecked ? `[x] ${found.text}` : `[ ] ${found.text}`,
        },
      ],
    };
  }
);

// Tool: delete_list
server.tool(
  "delete_list",
  "Delete an entire list (permanently removes the markdown file)",
  {
    name: z.string().describe("List name or slug to delete"),
  },
  async ({ name }) => {
    const list = findList(name);
    if (!list) {
      return {
        content: [{ type: "text", text: `List not found: ${name}` }],
        isError: true,
      };
    }

    fs.unlinkSync(list.path);
    return {
      content: [{ type: "text", text: `Deleted list: ${list.title}` }],
    };
  }
);

// Tool: parse_text
server.tool(
  "parse_text",
  "Parse messy freeform text into clean list items. Handles numbered lists, bullets, dashes, commas, etc.",
  {
    text: z.string().describe("Messy text to parse into list items"),
    list: z
      .string()
      .optional()
      .describe("If provided, add parsed items to this list"),
  },
  async ({ text, list: listName }) => {
    const items = parseMessyText(text);

    if (items.length === 0) {
      return {
        content: [{ type: "text", text: "No items could be parsed from the input." }],
      };
    }

    if (listName) {
      const list = findList(listName);
      if (!list) {
        return {
          content: [{ type: "text", text: `List not found: ${listName}` }],
          isError: true,
        };
      }

      const stamp = list.type === "log" ? nowToMinute() : null;
      for (const item of items) {
        if (list.type === "log") {
          list.items.push({ text: item, isChecked: false, timestamp: stamp });
        } else {
          list.items.push({ text: item, isChecked: false });
        }
      }
      fs.writeFileSync(list.path, renderList(list));

      return {
        content: [
          {
            type: "text",
            text: `Added ${items.length} item(s) to ${list.title}:\n${items.map((i) => `+ ${i}`).join("\n")}`,
          },
        ],
      };
    }

    return {
      content: [
        {
          type: "text",
          text: `Parsed ${items.length} item(s):\n${items.map((i) => `- ${i}`).join("\n")}`,
        },
      ],
    };
  }
);

// --- Text parsing (rule-based, no LLM) ---

function parseMessyText(text) {
  const lines = text.split("\n").map((l) => l.trim());
  const items = [];

  for (let line of lines) {
    if (!line) continue;

    // Strip common prefixes: numbered (1. 1) 1:), bullets (- * •), checkboxes
    line = line
      .replace(/^\d+[\.\)\:]\s*/, "")
      .replace(/^[-\*\u2022\u2013\u2014]\s*/, "")
      .replace(/^(?:\[[ x]\]\s*)/, "")
      .trim();

    if (!line) continue;

    // If line contains commas and no other structure, split on commas
    if (
      line.includes(",") &&
      !line.includes("\n") &&
      lines.length === 1
    ) {
      for (const part of line.split(",")) {
        const trimmed = part.trim();
        if (trimmed) items.push(trimmed);
      }
    } else {
      items.push(line);
    }
  }

  return items;
}

// --- Start ---

const transport = new StdioServerTransport();
await server.connect(transport);
