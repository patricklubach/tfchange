// tfchange is a Terraform plan formatter. It can display the plan in
// several output modes: a terminal UI (tui), a table, markdown, a
// text diff, or a raw JSON representation of the filtered plan.
//
// Usage examples:
//   tfchange -mode tui <plan.json
//   tfchange -mode table
//   tfchange -mode md
//   tfchange -mode text
//   tfchange -mode json
//
// For more details see the README.

package main

import (
	"encoding/json"
	"flag"
	"fmt"
    "bytes"
    "io"
    "log"
    "os"
    "sort"
    "strings"

    "github.com/charmbracelet/bubbles/viewport"
    tea "github.com/charmbracelet/bubbletea"
    "github.com/charmbracelet/lipgloss"
    "github.com/fatih/color"
    "github.com/olekukonko/tablewriter"
    "github.com/olekukonko/tablewriter/renderer"

)

type Action string

const (
	ActionNoOp    Action = "no-op"
	ActionCreate  Action = "create"
	ActionRead    Action = "read"
	ActionUpdate  Action = "update"
	ActionDelete  Action = "delete"
	ActionReplace Action = "replace"

const version = "0.0.1"
)

type ActionList []Action

func (al ActionList) IsNoOp() bool {
	if len(al) == 0 {
		return true
	}
	for _, a := range al {
		if a != ActionNoOp && a != ActionRead {
			return false
		}
	}
	return true
}

// Change tracks attribute states, unknown dynamic values, and replacement triggers.
type Change struct {
	Actions      ActionList     `json:"actions"`
	Before       map[string]any `json:"before"`
	After        map[string]any `json:"after"`
	AfterUnknown map[string]any `json:"after_unknown"`
	ReplacePaths any            `json:"replace_paths,omitempty"`
}

type ResourceChange struct {
	Address string  `json:"address"`
	Type    string  `json:"type"`
	Name    string  `json:"name"`
	Change  *Change `json:"change"`
}

type Plan struct {
	ResourceChanges []*ResourceChange `json:"resource_changes"`
}

type SummaryCounts struct {
	Create  int
	Update  int
	Delete  int
	Replace int
}

// String formats the accumulated resource change counts into a single-line summary string.
func (s SummaryCounts) Summary(useColor bool) string {
	parts := []string{}

	format := func(count int, label string, colorFunc func(a ...any) string) string {
		str := fmt.Sprintf("%d to %s", count, label)
		if useColor && colorFunc != nil {
			return colorFunc(str)
		}
		return str
	}

	if s.Create > 0 {
		parts = append(parts, format(s.Create, "be created", color.New(color.FgGreen).SprintFunc()))
	}
	if s.Update > 0 {
		parts = append(parts, format(s.Update, "be updated", color.New(color.FgYellow).SprintFunc()))
	}
	if s.Replace > 0 {
		parts = append(parts, format(s.Replace, "be replaced", color.New(color.FgMagenta).SprintFunc()))
	}
	if s.Delete > 0 {
		parts = append(parts, format(s.Delete, "be destroyed", color.New(color.FgRed).SprintFunc()))
	}

	if len(parts) == 0 {
		return "No changes."
	}

	return "Plan: " + strings.Join(parts, ", ") + "."
}

func calculateSummary(changes []*ResourceChange) SummaryCounts {
	var counts SummaryCounts
	for _, rc := range changes {
		_, actionType := getActionDetails(rc.Change.Actions)
		switch actionType {
		case "create":
			counts.Create++
		case "update":
			counts.Update++
		case "delete":
			counts.Delete++
		case "replace":
			counts.Replace++
		}
	}
	return counts
}

var (
	colorAdd     = color.New(color.FgGreen).SprintFunc()
	colorDelete  = color.New(color.FgRed).SprintFunc()
	colorUpdate  = color.New(color.FgYellow).SprintFunc()
	colorReplace = color.New(color.FgMagenta).SprintFunc()
	colorComment = color.New(color.FgCyan).SprintFunc()

	selectedStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("205")).Bold(true)
	normalStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("252"))
)

// formatValue serializes property values into formatted, untruncated JSON strings.
func formatValue(v any) string {
	if v == nil {
		return "null"
	}
	b, err := json.MarshalIndent(v, "      ", "  ")
	if err != nil {
		return fmt.Sprintf("%v", v)
	}
	return string(b)
}

// isUnknown checks whether an attribute key or nested field is marked as unknown in after_unknown.
func isUnknown(unknownMap map[string]any, key string) bool {
	if unknownMap == nil {
		return false
	}
	val, ok := unknownMap[key]
	if !ok {
		return false
	}
	if b, ok := val.(bool); ok {
		return b
	}
	return val != nil
}

// forcesReplacement checks if a top-level key or path is listed in replace_paths.
func forcesReplacement(replacePaths any, key string) bool {
	if replacePaths == nil {
		return false
	}

	paths, ok := replacePaths.([]any)
	if !ok {
		return false
	}

	for _, p := range paths {
		switch path := p.(type) {
		case string:
			if path == key {
				return true
			}
		case []any:
			if len(path) > 0 {
				if firstElem, ok := path[0].(string); ok && firstElem == key {
					return true
				}
			}
		}
	}
	return false
}

// getActionDetails returns the symbol, description, and action type for styling.
func getActionDetails(actions ActionList) (string, string) {
    if len(actions) == 0 {
        return "#", "noop"
    }

    var hasCreate, hasDelete bool
    for _, a := range actions {
        if a == ActionCreate {
            hasCreate = true
        }
        if a == ActionDelete {
            hasDelete = true
        }
    }

    if hasCreate && hasDelete {
        return "-/+", "replace"
    }

    switch actions[0] {
    case ActionCreate:
        return "+", "create"
    case ActionDelete:
        return "-", "delete"
    case ActionUpdate:
        return "~", "update"
    default:
        return "~", "update"
    }
}

// renderResourceDiff formats and returns a single resource change diff.
func renderResourceDiff(rc *ResourceChange, useColor bool) string {
	var sb strings.Builder
  symbol, actionType := getActionDetails(rc.Change.Actions)

  var actionDesc string
  switch actionType {
  case "create":
      actionDesc = "will be created"
  case "delete":
      actionDesc = "will be destroyed"
  case "replace":
      actionDesc = "must be replaced"
  case "update":
      actionDesc = "will be updated in-place"
  default:
      actionDesc = "will be modified"
  }

	headerComment := fmt.Sprintf("# %s %s\n", rc.Address, actionDesc)
	resourceHeader := fmt.Sprintf("  %s resource %q %q {\n", symbol, rc.Type, rc.Name)

	if useColor {
		sb.WriteString(colorComment(headerComment))
		switch actionType {
		case "create":
			sb.WriteString(colorAdd(resourceHeader))
		case "delete":
			sb.WriteString(colorDelete(resourceHeader))
		case "replace":
			sb.WriteString(colorReplace(resourceHeader))
		default:
			sb.WriteString(colorUpdate(resourceHeader))
		}
	} else {
		sb.WriteString(headerComment)
		sb.WriteString(resourceHeader)
	}

	before := rc.Change.Before
	after := rc.Change.After
	afterUnknown := rc.Change.AfterUnknown
	replacePaths := rc.Change.ReplacePaths

	keySet := make(map[string]struct{})
	for k := range before {
		keySet[k] = struct{}{}
	}
	for k := range after {
		keySet[k] = struct{}{}
	}
	for k := range afterUnknown {
		keySet[k] = struct{}{}
	}

	keys := make([]string, 0, len(keySet))
	for k := range keySet {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, k := range keys {
		vBefore, inBefore := before[k]
		vAfter, inAfter := after[k]
		unknown := isUnknown(afterUnknown, k)
		isReplaceKey := forcesReplacement(replacePaths, k)

		forcesSuffix := ""
		if isReplaceKey {
			forcesSuffix = " # forces replacement"
		}

		var line string
		var lineType string

		switch {
		case inBefore && !inAfter && !unknown:
			line = fmt.Sprintf("      - %s = %s%s\n", k, formatValue(vBefore), forcesSuffix)
			lineType = "delete"

		case (!inBefore || vBefore == nil) && unknown:
			line = fmt.Sprintf("      + %s = (known after apply)%s\n", k, forcesSuffix)
			lineType = "create"

		case !inBefore && inAfter:
			line = fmt.Sprintf("      + %s = %s%s\n", k, formatValue(vAfter), forcesSuffix)
			lineType = "create"

		case inBefore && unknown:
			sBefore := formatValue(vBefore)
			line = fmt.Sprintf("      ~ %s = %s -> (known after apply)%s\n", k, sBefore, forcesSuffix)
			lineType = "update"

		case inBefore && inAfter:
			sBefore := formatValue(vBefore)
			sAfter := formatValue(vAfter)

			if sBefore != sAfter {
				if strings.Contains(sBefore, "\n") || strings.Contains(sAfter, "\n") {
					line = fmt.Sprintf("      ~ %s = %s\n        -> %s%s\n", k, sBefore, sAfter, forcesSuffix)
				} else {
					line = fmt.Sprintf("      ~ %s = %s -> %s%s\n", k, sBefore, sAfter, forcesSuffix)
				}
				lineType = "update"
			} else {
				line = fmt.Sprintf("        %s = %s\n", k, formatValue(vBefore))
				lineType = "noop"
			}
		}

		if useColor {
			switch lineType {
			case "create":
				sb.WriteString(colorAdd(line))
			case "delete":
				sb.WriteString(colorDelete(line))
			case "update":
				sb.WriteString(colorUpdate(line))
			default:
				sb.WriteString(line)
			}
		} else {
			sb.WriteString(line)
		}
	}

	sb.WriteString("    }\n\n")
	return sb.String()
}

// renderTableSummary outputs a tf-summarize style table representation.
func renderTableSummary(w io.Writer, changes []*ResourceChange, useColor bool) {
	table := tablewriter.NewWriter(w)
	table.Header([]string{"Change", "Address"})

	for _, rc := range changes {
		_, actionType := getActionDetails(rc.Change.Actions)

		var changeStr string
		switch actionType {
		case "create":
			changeStr = "CREATE"
			if useColor {
				changeStr = colorAdd(changeStr)
			}
		case "delete":
			changeStr = "DELETE"
			if useColor {
				changeStr = colorDelete(changeStr)
			}
		case "update":
			changeStr = "UPDATE"
			if useColor {
				changeStr = colorUpdate(changeStr)
			}
		case "replace":
			changeStr = "REPLACE"
			if useColor {
				changeStr = colorReplace(changeStr)
			}
		default:
			changeStr = "NOOP"
		}

		table.Append(changeStr, rc.Address)
	}

	table.Render()
summary := calculateSummary(changes)
    _, err := fmt.Fprintln(w, summary.Summary(useColor))
	if err != nil {
		log.Fatal(err)
	}
}

// renderMarkdownTable renders resource changes into a formatted Markdown table.
func renderMarkdownTable(w io.Writer, changes []*ResourceChange) {
	table := tablewriter.NewTable(w,
		tablewriter.WithRenderer(renderer.NewMarkdown()),
	)
	table.Header([]string{"Change", "Resource Type", "Resource Name", "Address"})

	for _, rc := range changes {
		_, actionType := getActionDetails(rc.Change.Actions)

		var changeStr string
		switch actionType {
		case "create":
			changeStr = "CREATE"
		case "delete":
			changeStr = "DELETE"
		case "update":
			changeStr = "UPDATE"
		case "replace":
			changeStr = "REPLACE"
		default:
			changeStr = "NOOP"
		}
		table.Append(changeStr, rc.Type, rc.Name, rc.Address)
	}
	table.Render()

	summary := calculateSummary(changes)
	_, err := fmt.Fprintln(w)
	if err != nil {
		log.Fatal(err)
	}
	_, err = fmt.Fprintln(w, summary.Summary(false))
	if err != nil {
		log.Fatal(err)
	}
}

type model struct {
	changes    []*ResourceChange
	cursor     int
	showDetail bool
	viewport   viewport.Model
	width      int
	height     int
	useColor   bool
	summary    SummaryCounts
}

// initialModel initializes the Bubble Tea TUI state.
func initialModel(changes []*ResourceChange, useColor bool) model {
	return model{
		changes:  changes,
		cursor:   0,
		useColor: useColor,
		summary:  calculateSummary(changes),
	}
}

// Init defines the initial command for the TUI model.
func (m model) Init() tea.Cmd {
	return nil
}

// Update handles incoming TUI events and user keyboard navigation.
func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		if m.showDetail {
			switch msg.String() {
			case " ", "q", "esc":
				m.showDetail = false
				return m, nil
			default:
				m.viewport, cmd = m.viewport.Update(msg)
				return m, cmd
			}
		}

		switch msg.String() {
		case "ctrl+c", "q", "esc":
			return m, tea.Quit
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.cursor < len(m.changes)-1 {
				m.cursor++
			}
		case " ":
			if len(m.changes) > 0 {
				m.showDetail = true
				rc := m.changes[m.cursor]
				content := renderResourceDiff(rc, m.useColor)

				m.viewport = viewport.New(m.width, m.height-3)
				m.viewport.SetContent(content)
			}
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		if m.showDetail {
			m.viewport.Width = msg.Width
			m.viewport.Height = msg.Height - 3
		}
	}

	return m, nil
}

// View renders the current state of the TUI interface.
func (m model) View() string {
	if len(m.changes) == 0 {
		return "No changes found in plan.\nPress 'q' to exit."
	}

	if m.showDetail {
		header := lipgloss.NewStyle().Bold(true).Render("Resource Change Details (Press Space or 'q' to close):")
		return fmt.Sprintf("%s\n\n%s", header, m.viewport.View())
	}

	var sb strings.Builder
	s := fmt.Sprintf("%s%s", m.summary.Summary(m.useColor), "\n")
	sb.WriteString(s)
	sb.WriteString("Terraform Plan Changes (Use ↑/↓ to navigate, Space to view full change, 'q' to quit):\n\n")

	for i, rc := range m.changes {
		symbol, actionType := getActionDetails(rc.Change.Actions)

		var actionFormatted string
		switch actionType {
		case "create":
			actionFormatted = symbol + " CREATE"
			if m.useColor {
				actionFormatted = colorAdd(actionFormatted)
			}
		case "delete":
			actionFormatted = symbol + " DELETE"
			if m.useColor {
				actionFormatted = colorDelete(actionFormatted)
			}
		case "update":
			actionFormatted = symbol + " UPDATE"
			if m.useColor {
				actionFormatted = colorUpdate(actionFormatted)
			}
		case "replace":
			actionFormatted = symbol + " REPLACE"
			if m.useColor {
				actionFormatted = colorReplace(actionFormatted)
			}
		}

		line := fmt.Sprintf("[%s] %s", actionFormatted, rc.Address)
		if i == m.cursor {
			sb.WriteString(selectedStyle.Render("> " + line))
		} else {
			sb.WriteString(normalStyle.Render("  " + line))
		}
		sb.WriteString("\n")
	}

	return sb.String()
}

// parsePlan reads JSON data from an io.Reader and unmarshals it into resource changes and returns it.
func parsePlan(r io.Reader) ([]*ResourceChange, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("reading input: %w", err)
	}

    var plan Plan
    if err := json.Unmarshal(data, &plan); err != nil {
        return nil, fmt.Errorf("parsing plan JSON: %w", err)
    }

    var changed []*ResourceChange
    for _, rc := range plan.ResourceChanges {
        if rc.Change == nil || rc.Change.Actions.IsNoOp() {
            continue
        }
        changed = append(changed, rc)
    }

    return changed, nil
}

func main() {
	modeFlag := flag.String("mode", "tui", "Display mode: 'tui', 'table', 'md', or 'text'")
	noColorFlag := flag.Bool("no-color", false, "Disable color output")

// Program version and verbose mode
versionFlag := flag.Bool("v", false, "Print program version")
verboseFlag := flag.Bool("verbose", false, "Enable verbose logging")
	flag.Parse()

    // Handle version flag
    if *versionFlag {
        fmt.Println(version)
        os.Exit(0)
    }
    // Handle verbose logging
    if *verboseFlag {
        log.SetFlags(log.LstdFlags)
    } else {
        log.SetFlags(0)
    }

	useColor := !*noColorFlag
	color.NoColor = *noColorFlag

    // Read all input into data
    var data []byte
    if flag.NArg() > 0 {
        file, err := os.Open(flag.Arg(0))
        if err != nil {
            fmt.Fprintf(os.Stderr, "Error opening file: %v\n", err)
            os.Exit(1)
        }
        data, err = io.ReadAll(file)
        if err != nil {
            fmt.Fprintf(os.Stderr, "Error reading file: %v\n", err)
            os.Exit(1)
        }
        if err = file.Close(); err != nil {
            fmt.Fprintf(os.Stderr, "Error closing file: %v\n", err)
            os.Exit(1)
        }
    } else {
        var err error
        data, err = io.ReadAll(os.Stdin)
        if err != nil {
            fmt.Fprintf(os.Stderr, "Error reading stdin: %v\n", err)
            os.Exit(1)
        }
    }

    // Parse changes
    changes, err := parsePlan(bytes.NewReader(data))
    if err != nil {
        fmt.Fprintf(os.Stderr, "Error: %v\n", err)
        os.Exit(1)
    }

    // Unmarshal full plan for JSON mode
    var plan Plan
    if err := json.Unmarshal(data, &plan); err != nil {
        fmt.Fprintf(os.Stderr, "Error parsing plan JSON: %v\n", err)
        os.Exit(1)
    }

  switch *modeFlag {
  case "table":
      renderTableSummary(os.Stdout, changes, useColor)
  case "md":
      renderMarkdownTable(os.Stdout, changes)
  case "text":
      for _, rc := range changes {
          fmt.Print(renderResourceDiff(rc, useColor))
      }
      summary := calculateSummary(changes)
      fmt.Println(summary.Summary(useColor))
  case "json":
      // Output the filtered plan as JSON
        filteredPlan := plan
      filteredPlan.ResourceChanges = changes
      encoder := json.NewEncoder(os.Stdout)
      encoder.SetIndent("", "  ")
      if err := encoder.Encode(filteredPlan); err != nil {
          fmt.Fprintf(os.Stderr, "Error encoding JSON: %v\n", err)
          os.Exit(1)
      }
  case "tui":
      p := tea.NewProgram(initialModel(changes, useColor), tea.WithAltScreen())
      if _, err := p.Run(); err != nil {
          fmt.Fprintf(os.Stderr, "Error running TUI: %v\n", err)
          os.Exit(1)
      }
  default:
      fmt.Fprintf(os.Stderr, "Invalid mode: %s. Valid choices are 'tui', 'table', 'md', 'text', 'json'\n", *modeFlag)
      os.Exit(1)
  }

}
