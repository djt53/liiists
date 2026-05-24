package main

import (
	"bufio"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/mdp/qrterminal/v3"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(0)
	}

	cmd := os.Args[1]
	args := os.Args[2:]

	var err error
	switch cmd {
	case "init":
		err = cmdInit()
	case "new":
		err = cmdNew(args)
	case "add":
		err = cmdAdd(args)
	case "ls":
		err = cmdLs(args)
	case "rm":
		err = cmdRm(args)
	case "check":
		err = cmdCheck(args)
	case "split":
		err = cmdSplit(args)
	case "where":
		err = cmdWhere()
	case "link":
		err = cmdLink()
	case "help", "--help", "-h":
		printUsage()
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n", cmd)
		printUsage()
		os.Exit(1)
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %s\n", err)
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Print(`liiists — dead-simple lists, backed by markdown

usage:
  liiists init                    set up your lists directory
  liiists new <name>              create a new list
  liiists new <name> --checklist  create a checklist
  liiists new <name> --log        create a log (timestamped entries)
  liiists add <list> <item>       add an item (or pipe stdin)
                                  log entries auto-stamp now; pass
                                  --at "YYYY-MM-DD HH:MM" to backdate
  liiists ls                      show all lists
  liiists ls <name>               show items in a list
  liiists rm <list> <item>        remove an item
  liiists check <list> <item>     toggle checkbox (checklists)
  liiists split [list]            split stdin into items (strips bullets, splits on commas)
  liiists where                   show the lists directory in use
  liiists link                    show a QR code to link the iOS app to this directory
`)
}

// --- Commands ---

func cmdInit() error {
	dir, err := getListsDir()
	if err != nil {
		return err
	}

	// Check if already initialized
	if _, err := os.Stat(dir); err == nil {
		if dir == iCloudListsDir() {
			fmt.Printf("already initialized: %s\n  (synced with the liiists iOS app via iCloud)\n", dir)
		} else {
			fmt.Printf("already initialized: %s\n", dir)
		}
		return nil
	}

	// Ask for directory
	reader := bufio.NewReader(os.Stdin)
	if dir == iCloudListsDir() {
		fmt.Printf("found iCloud lists from the liiists iOS app — use them? [%s]: ", dir)
	} else {
		fmt.Printf("where should lists live? [%s]: ", dir)
	}
	input, _ := reader.ReadString('\n')
	input = strings.TrimSpace(input)

	if input != "" {
		home, _ := os.UserHomeDir()
		if strings.HasPrefix(input, "~/") {
			input = filepath.Join(home, input[2:])
		}
		dir = input

		// Save config
		configDir := filepath.Join(home, ".config", "liiists")
		os.MkdirAll(configDir, 0755)
		os.WriteFile(filepath.Join(configDir, "config"), []byte(fmt.Sprintf("lists_dir=%s\n", dir)), 0644)
	}

	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	fmt.Printf("initialized: %s\n", dir)

	// Offer to create first list
	fmt.Print("create your first list? [Y/n]: ")
	answer, _ := reader.ReadString('\n')
	answer = strings.TrimSpace(strings.ToLower(answer))
	if answer == "" || answer == "y" || answer == "yes" {
		fmt.Print("list name: ")
		name, _ := reader.ReadString('\n')
		name = strings.TrimSpace(name)
		if name != "" {
			return createList(dir, name, "list")
		}
	}

	return nil
}

func cmdNew(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: liiists new <name> [--checklist|--log]")
	}

	dir, err := getListsDir()
	if err != nil {
		return err
	}

	name := args[0]
	listType := "list"
	for _, a := range args[1:] {
		switch a {
		case "--checklist", "-c":
			listType = "checklist"
		case "--log", "-l":
			listType = "log"
		}
	}

	return createList(dir, name, listType)
}

func createList(dir, name, listType string) error {
	slug := slugify(name)
	path := filepath.Join(dir, slug+".md")

	if _, err := os.Stat(path); err == nil {
		return fmt.Errorf("list already exists: %s", slug+".md")
	}

	l := &List{
		Title:   name,
		Type:    listType,
		Created: todayStr(),
		Path:    path,
		Extra:   make(map[string]string),
	}

	if err := l.Write(); err != nil {
		return err
	}

	fmt.Printf("created: %s\n", slug+".md")
	return nil
}

func cmdAdd(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: liiists add <list> <item...> [--at \"YYYY-MM-DD HH:MM\"]")
	}

	// Pull out --at "YYYY-MM-DD HH:MM" if present. Only meaningful for log
	// lists; ignored otherwise. Accepts the space form on input; the on-disk
	// representation always uses the ISO `T` form.
	var atTime *time.Time
	cleaned := []string{args[0]}
	for i := 1; i < len(args); i++ {
		if args[i] == "--at" && i+1 < len(args) {
			parsed, err := parseAtFlag(args[i+1])
			if err != nil {
				return err
			}
			atTime = &parsed
			i++
			continue
		}
		cleaned = append(cleaned, args[i])
	}
	args = cleaned

	l, err := findList(args[0])
	if err != nil {
		return err
	}

	stamp := func() *time.Time {
		if l.Type != "log" {
			return nil
		}
		if atTime != nil {
			return atTime
		}
		now := time.Now().Truncate(time.Minute)
		return &now
	}

	appendOne := func(text string) {
		item := Item{Text: text, Timestamp: stamp()}
		l.Items = append(l.Items, item)
		if item.Timestamp != nil {
			fmt.Printf("+ %s  %s\n", item.Timestamp.Format(logTimestampLayout), text)
		} else {
			fmt.Printf("+ %s\n", text)
		}
	}

	if len(args) >= 2 {
		// Items from arguments
		text := strings.Join(args[1:], " ")
		appendOne(text)
	} else {
		// Read from stdin
		stat, _ := os.Stdin.Stat()
		if (stat.Mode() & os.ModeCharDevice) == 0 {
			scanner := bufio.NewScanner(os.Stdin)
			for scanner.Scan() {
				text := strings.TrimSpace(scanner.Text())
				if text != "" {
					appendOne(text)
				}
			}
		} else {
			return fmt.Errorf("provide items as arguments or pipe via stdin")
		}
	}

	return l.Write()
}

// parseAtFlag accepts the human-friendly space form ("2026-05-22 14:30") for
// the --at flag. The on-disk format always uses the ISO `T` form.
func parseAtFlag(raw string) (time.Time, error) {
	layouts := []string{
		"2006-01-02 15:04",
		"2006-01-02T15:04",
	}
	for _, layout := range layouts {
		if t, err := time.ParseInLocation(layout, raw, time.Local); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("--at value must be YYYY-MM-DD HH:MM, got %q", raw)
}

func cmdLs(args []string) error {
	if len(args) == 0 {
		// List all lists
		lists, err := loadAllLists()
		if err != nil {
			return err
		}

		if len(lists) == 0 {
			fmt.Println("no lists yet — run 'liiists new <name>' to create one")
			return nil
		}

		for _, l := range lists {
			count := len(l.Items)
			if l.Type == "checklist" {
				checked := 0
				for _, item := range l.Items {
					if item.IsChecked {
						checked++
					}
				}
				fmt.Printf("  %s  %d/%d\n", l.Title, checked, count)
			} else {
				fmt.Printf("  %s  %d\n", l.Title, count)
			}
		}
		return nil
	}

	// Show specific list
	l, err := findList(args[0])
	if err != nil {
		return err
	}

	fmt.Printf("%s\n", l.Title)
	if len(l.Items) == 0 {
		fmt.Println("  (empty)")
		return nil
	}

	for _, item := range l.Items {
		switch l.Type {
		case "checklist":
			if item.IsChecked {
				fmt.Printf("  [x] %s\n", item.Text)
			} else {
				fmt.Printf("  [ ] %s\n", item.Text)
			}
		case "log":
			if item.Timestamp != nil {
				fmt.Printf("  %s  %s\n", item.Timestamp.Format(logTimestampLayout), item.Text)
			} else {
				fmt.Printf("  %s\n", item.Text)
			}
		default:
			fmt.Printf("  %s\n", item.Text)
		}
	}
	return nil
}

func cmdRm(args []string) error {
	if len(args) < 2 {
		return fmt.Errorf("usage: liiists rm <list> <item text>")
	}

	l, err := findList(args[0])
	if err != nil {
		return err
	}

	query := strings.Join(args[1:], " ")
	idx, candidates := matchItem(l.Items, query)
	if idx < 0 {
		printSuggestions(query, l.Items, candidates)
		return fmt.Errorf("item not found")
	}

	fmt.Printf("- %s\n", l.Items[idx].Text)
	l.Items = append(l.Items[:idx], l.Items[idx+1:]...)
	return l.Write()
}

func cmdCheck(args []string) error {
	if len(args) < 2 {
		return fmt.Errorf("usage: liiists check <list> <item text>")
	}

	l, err := findList(args[0])
	if err != nil {
		return err
	}

	if l.Type != "checklist" {
		return fmt.Errorf("'%s' is not a checklist", l.Title)
	}

	query := strings.Join(args[1:], " ")
	idx, candidates := matchItem(l.Items, query)
	if idx < 0 {
		printSuggestions(query, l.Items, candidates)
		return fmt.Errorf("item not found")
	}

	l.Items[idx].IsChecked = !l.Items[idx].IsChecked
	if l.Items[idx].IsChecked {
		fmt.Printf("[x] %s\n", l.Items[idx].Text)
	} else {
		fmt.Printf("[ ] %s\n", l.Items[idx].Text)
	}
	return l.Write()
}

// matchItem finds an item by query with fuzzy fallback.
// Returns (idx, nil) when one item unambiguously matches (caller applies it).
// Returns (-1, candidates) when no unambiguous match — candidates is up to 3
// item indices ranked by closeness, for a "did you mean?" prompt.
//
// Match order: case-insensitive exact → case-insensitive substring (only if
// exactly one hit) → Levenshtein distance ranking of all items.
func matchItem(items []Item, query string) (int, []int) {
	if len(items) == 0 {
		return -1, nil
	}
	q := strings.ToLower(strings.TrimSpace(query))

	for i, item := range items {
		if strings.ToLower(item.Text) == q {
			return i, nil
		}
	}

	var substr []int
	for i, item := range items {
		if strings.Contains(strings.ToLower(item.Text), q) {
			substr = append(substr, i)
		}
	}
	if len(substr) == 1 {
		return substr[0], nil
	}
	if len(substr) > 1 {
		return -1, substr[:min(len(substr), 3)]
	}

	type scored struct {
		idx  int
		dist int
	}
	scores := make([]scored, len(items))
	for i, item := range items {
		scores[i] = scored{idx: i, dist: levenshtein(q, strings.ToLower(item.Text))}
	}
	sort.Slice(scores, func(i, j int) bool { return scores[i].dist < scores[j].dist })
	n := min(len(scores), 3)
	candidates := make([]int, n)
	for i := 0; i < n; i++ {
		candidates[i] = scores[i].idx
	}
	return -1, candidates
}

func printSuggestions(query string, items []Item, candidates []int) {
	if len(candidates) == 0 {
		return
	}
	fmt.Fprintf(os.Stderr, "no match for %q. did you mean:\n", query)
	for _, idx := range candidates {
		fmt.Fprintf(os.Stderr, "  %s\n", items[idx].Text)
	}
}

func levenshtein(a, b string) int {
	if len(a) == 0 {
		return len(b)
	}
	if len(b) == 0 {
		return len(a)
	}
	prev := make([]int, len(b)+1)
	curr := make([]int, len(b)+1)
	for j := 0; j <= len(b); j++ {
		prev[j] = j
	}
	for i := 1; i <= len(a); i++ {
		curr[0] = i
		for j := 1; j <= len(b); j++ {
			cost := 1
			if a[i-1] == b[j-1] {
				cost = 0
			}
			curr[j] = min(curr[j-1]+1, min(prev[j]+1, prev[j-1]+cost))
		}
		prev, curr = curr, prev
	}
	return prev[len(b)]
}

func cmdSplit(args []string) error {
	// Read from stdin
	stat, _ := os.Stdin.Stat()
	if (stat.Mode() & os.ModeCharDevice) != 0 {
		return fmt.Errorf("pipe text via stdin: echo 'a, b, c' | liiists split [list]")
	}

	var input strings.Builder
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		input.WriteString(scanner.Text())
		input.WriteString("\n")
	}

	items := parseMessyText(input.String())
	if len(items) == 0 {
		fmt.Println("no items found in input")
		return nil
	}

	// If a list name was provided, add items to it
	if len(args) >= 1 {
		l, err := findList(args[0])
		if err != nil {
			return err
		}
		for _, text := range items {
			l.Items = append(l.Items, Item{Text: text})
			fmt.Printf("+ %s\n", text)
		}
		return l.Write()
	}

	// Otherwise just print parsed items
	for _, text := range items {
		fmt.Printf("- %s\n", text)
	}
	return nil
}

func cmdWhere() error {
	dir, err := getListsDir()
	if err != nil {
		return err
	}
	fmt.Println(dir)
	if dir == iCloudListsDir() {
		fmt.Println("(synced with the liiists iOS app via iCloud)")
	}
	return nil
}

func cmdLink() error {
	dir, err := getListsDir()
	if err != nil {
		return err
	}
	link := "https://davidtingle.com/liiists/link?path=" + url.QueryEscape(dir)
	fmt.Printf("scan with the liiists iOS app to link this directory:\n\n  %s\n\n", dir)
	qrterminal.GenerateHalfBlock(link, qrterminal.L, os.Stdout)
	fmt.Printf("\n%s\n", link)
	return nil
}

func parseMessyText(text string) []string {
	lines := strings.Split(text, "\n")
	var items []string

	// If single non-empty line with commas, split on commas
	nonEmpty := 0
	singleLine := ""
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed != "" {
			nonEmpty++
			singleLine = trimmed
		}
	}
	if nonEmpty == 1 && strings.Contains(singleLine, ",") {
		for _, part := range strings.Split(singleLine, ",") {
			trimmed := strings.TrimSpace(part)
			if trimmed != "" {
				items = append(items, trimmed)
			}
		}
		return items
	}

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// Strip numbered prefixes: 1. 1) 1:
		for i, c := range line {
			if c >= '0' && c <= '9' {
				continue
			}
			if (c == '.' || c == ')' || c == ':') && i > 0 {
				line = strings.TrimSpace(line[i+1:])
			}
			break
		}

		// Strip bullet prefixes: - * • – —
		line = strings.TrimLeft(line, "-*\u2022\u2013\u2014 ")

		// Strip checkbox prefixes: [ ] [x]
		if strings.HasPrefix(line, "[ ] ") {
			line = line[4:]
		} else if strings.HasPrefix(line, "[x] ") {
			line = line[4:]
		}

		line = strings.TrimSpace(line)
		if line != "" {
			items = append(items, line)
		}
	}

	return items
}
