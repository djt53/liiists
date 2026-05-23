package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

// List represents a parsed markdown list file.
type List struct {
	Title   string
	Type    string // "list", "checklist", or "log"
	Created string
	Items   []Item
	Extra   map[string]string // unknown frontmatter fields, preserved on write
	Path    string            // absolute file path
}

// Item represents a single list entry.
type Item struct {
	Text      string
	IsChecked bool
	// Timestamp is populated only for entries in a "log" list. Naive local
	// datetime, minute resolution. See decision 016.
	Timestamp *time.Time
}

// logTimestampLayout — ISO 8601, minute resolution, no timezone. Matches the
// iOS app's MarkdownParser.logTimestampFormatter.
const logTimestampLayout = "2006-01-02T15:04"

// logSeparator — em-dash with spaces on both sides. First occurrence after
// the timestamp delimits the entry text; subsequent em-dashes belong to the
// text.
const logSeparator = " — "

// --- Parsing ---

var frontmatterRe = regexp.MustCompile(`(?s)\A---\n(.+?)\n---\n?`)

func ParseFile(path string) (*List, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return Parse(string(data), path)
}

func Parse(content, path string) (*List, error) {
	l := &List{
		Type:  "list",
		Path:  path,
		Extra: make(map[string]string),
	}

	body := content

	// Parse frontmatter
	if m := frontmatterRe.FindStringSubmatch(content); m != nil {
		body = content[len(m[0]):]
		for _, line := range strings.Split(m[1], "\n") {
			parts := strings.SplitN(line, ":", 2)
			if len(parts) != 2 {
				continue
			}
			key := strings.TrimSpace(parts[0])
			val := strings.TrimSpace(parts[1])
			switch key {
			case "title":
				l.Title = val
			case "type":
				l.Type = val
			case "created":
				l.Created = val
			default:
				l.Extra[key] = val
			}
		}
	}

	// Parse H1 title if no frontmatter title
	if l.Title == "" {
		lines := strings.Split(body, "\n")
		for i, line := range lines {
			if strings.HasPrefix(line, "# ") {
				l.Title = strings.TrimPrefix(line, "# ")
				lines = append(lines[:i], lines[i+1:]...)
				body = strings.Join(lines, "\n")
				break
			}
		}
	}

	// Fallback: derive title from filename
	if l.Title == "" && path != "" {
		base := strings.TrimSuffix(filepath.Base(path), ".md")
		l.Title = deslugify(base)
	}

	// Parse items
	scanner := bufio.NewScanner(strings.NewReader(body))
	for scanner.Scan() {
		line := scanner.Text()
		if l.Type == "log" {
			if item, ok := parseLogEntry(line); ok {
				l.Items = append(l.Items, item)
			}
			continue
		}
		if strings.HasPrefix(line, "- [x] ") {
			l.Items = append(l.Items, Item{Text: strings.TrimPrefix(line, "- [x] "), IsChecked: true})
		} else if strings.HasPrefix(line, "- [ ] ") {
			l.Items = append(l.Items, Item{Text: strings.TrimPrefix(line, "- [ ] "), IsChecked: false})
		} else if strings.HasPrefix(line, "- ") {
			l.Items = append(l.Items, Item{Text: strings.TrimPrefix(line, "- ")})
		}
	}

	// Log entries sort newest first regardless of on-disk order.
	if l.Type == "log" {
		sortLogItems(l.Items)
	}

	return l, nil
}

func parseLogEntry(line string) (Item, bool) {
	if !strings.HasPrefix(line, "- ") {
		return Item{}, false
	}
	body := strings.TrimPrefix(line, "- ")
	idx := strings.Index(body, logSeparator)
	if idx < 0 {
		return Item{}, false
	}
	tsStr := strings.TrimSpace(body[:idx])
	text := body[idx+len(logSeparator):]
	ts, err := time.ParseInLocation(logTimestampLayout, tsStr, time.Local)
	if err != nil {
		return Item{}, false
	}
	return Item{Text: text, Timestamp: &ts}, true
}

func sortLogItems(items []Item) {
	// Newest first; items without a timestamp sink to the bottom.
	sort.SliceStable(items, func(i, j int) bool {
		ti, tj := items[i].Timestamp, items[j].Timestamp
		if ti == nil {
			return false
		}
		if tj == nil {
			return true
		}
		return ti.After(*tj)
	})
}

// --- Writing ---

func (l *List) Write() error {
	return os.WriteFile(l.Path, []byte(l.Render()), 0644)
}

func (l *List) Render() string {
	var b strings.Builder

	// Frontmatter
	b.WriteString("---\n")
	b.WriteString(fmt.Sprintf("title: %s\n", l.Title))
	b.WriteString(fmt.Sprintf("type: %s\n", l.Type))
	if l.Created != "" {
		b.WriteString(fmt.Sprintf("created: %s\n", l.Created))
	}
	for k, v := range l.Extra {
		b.WriteString(fmt.Sprintf("%s: %s\n", k, v))
	}
	b.WriteString("---\n\n")

	// Items
	if l.Type == "log" {
		// Reverse-chrono on disk so the file reads the same way the iOS app
		// displays.
		sortLogItems(l.Items)
		for _, item := range l.Items {
			if item.Timestamp == nil {
				continue
			}
			b.WriteString(fmt.Sprintf("- %s%s%s\n", item.Timestamp.Format(logTimestampLayout), logSeparator, item.Text))
		}
	} else {
		for _, item := range l.Items {
			if l.Type == "checklist" {
				if item.IsChecked {
					b.WriteString(fmt.Sprintf("- [x] %s\n", item.Text))
				} else {
					b.WriteString(fmt.Sprintf("- [ ] %s\n", item.Text))
				}
			} else {
				b.WriteString(fmt.Sprintf("- %s\n", item.Text))
			}
		}
	}

	return b.String()
}

// --- Helpers ---

func slugify(s string) string {
	s = strings.ToLower(s)
	s = regexp.MustCompile(`[^a-z0-9]+`).ReplaceAllString(s, "-")
	s = strings.Trim(s, "-")
	return s
}

func deslugify(s string) string {
	words := strings.Split(s, "-")
	for i, w := range words {
		if len(w) > 0 {
			words[i] = strings.ToUpper(w[:1]) + w[1:]
		}
	}
	return strings.Join(words, " ")
}

func getListsDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}

	// Check config file
	configPath := filepath.Join(home, ".config", "liiists", "config")
	if data, err := os.ReadFile(configPath); err == nil {
		for _, line := range strings.Split(string(data), "\n") {
			parts := strings.SplitN(line, "=", 2)
			if len(parts) == 2 && strings.TrimSpace(parts[0]) == "lists_dir" {
				dir := strings.TrimSpace(parts[1])
				if strings.HasPrefix(dir, "~/") {
					dir = filepath.Join(home, dir[2:])
				}
				return dir, nil
			}
		}
	}

	// Auto-detect iCloud container (shared with iOS app)
	if dir := iCloudListsDir(); dir != "" {
		return dir, nil
	}

	// Default
	return filepath.Join(home, "lists"), nil
}

// iCloudListsDir returns the local path to the liiists iOS app's iCloud
// container if it exists on this Mac, or "" otherwise.
func iCloudListsDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	dir := filepath.Join(home, "Library", "Mobile Documents", "iCloud~com~davidtingle~liiists", "Documents")
	if info, err := os.Stat(dir); err == nil && info.IsDir() {
		return dir
	}
	return ""
}

func loadAllLists() ([]*List, error) {
	dir, err := getListsDir()
	if err != nil {
		return nil, err
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("lists directory not found: %s\nRun 'liiists init' first", dir)
		}
		return nil, err
	}

	var lists []*List
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
			continue
		}
		l, err := ParseFile(filepath.Join(dir, e.Name()))
		if err != nil {
			continue
		}
		lists = append(lists, l)
	}
	return lists, nil
}

func findList(name string) (*List, error) {
	dir, err := getListsDir()
	if err != nil {
		return nil, err
	}

	// Try exact slug match first
	slug := slugify(name)
	path := filepath.Join(dir, slug+".md")
	if _, err := os.Stat(path); err == nil {
		return ParseFile(path)
	}

	// Fuzzy: search by title
	lists, err := loadAllLists()
	if err != nil {
		return nil, err
	}

	nameLower := strings.ToLower(name)
	for _, l := range lists {
		if strings.ToLower(l.Title) == nameLower {
			return l, nil
		}
	}

	return nil, fmt.Errorf("list not found: %s", name)
}

func todayStr() string {
	return time.Now().Format("2006-01-02")
}
