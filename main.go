package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/fatih/color"
	"github.com/olekukonko/tablewriter"
)

// Action represents an individual state modification type in a plan.
type Action string

const (
	ActionNoOp    Action = "no-op"
	ActionCreate  Action = "create"
	ActionRead    Action = "read"
	ActionUpdate  Action = "update"
	ActionDelete  Action = "delete"
	ActionReplace Action = "replace"
)

// ActionList contains actions planned for a single resource.
type ActionList []Action

// IsNoOp determines if a resource change is a no-operation or read-only.
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

// Change tracks attribute states and unknown dynamic values before and after plan execution.
type Change struct {
	Actions      ActionList             `json:"actions"`
	Before       map[string]interface{} `json:"before"`
	After        map[string]interface{} `json:"after"`
	AfterUnknown map[string]interface{} `json:"after_unknown"`
}

// ResourceChange represents a planned modification to a Terraform resource.
type ResourceChange struct {
	Address string  `json:"address"`
	Type    string  `json:"type"`
	Name    string  `json:"name"`
	Change  *Change `json:"change"`
}

// Plan represents the relevant structure of a Terraform JSON execution plan.
type Plan struct {
	ResourceChanges []*ResourceChange `json:"resource_changes"`
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
func formatValue(v interface{}) string {
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
func isUnknown(unknownMap map[string]interface{}, key string) bool {
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

// getActionDetails returns the symbol, description, and action type for styling.
func getActionDetails(actions ActionList) (string, string, string) {
	if len(actions) == 0 {
		return "#", "no-op", "noop"
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
		return "-/+", "must be replaced", "replace"
	}

	switch actions[0] {
	case ActionCreate:
		return "+", "will be created", "create"
	case ActionDelete:
		return "-", "will be destroyed", "delete"
	case ActionUpdate:
		return "~", "will be updated in-place", "update"
	default:
		return "~", "will be modified", "update"
	}
}

// renderResourceDiff formats and returns a single resource change diff.
func renderResourceDiff(rc *ResourceChange, useColor bool) string {
	var sb strings.Builder
	symbol, actionDesc, actionType := getActionDetails(rc.Change.Actions)

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

		var line string
		var lineType string

		switch {
		case inBefore && !inAfter && !unknown:
			line = fmt.Sprintf("      - %s = %s\n", k, formatValue(vBefore))
			lineType = "delete"

		case (!inBefore || vBefore == nil) && unknown:
			line = fmt.Sprintf("      + %s = (known after apply)\n", k)
			lineType = "create"

		case !inBefore && inAfter:
			line = fmt.Sprintf("      + %s = %s\n", k, formatValue(vAfter))
			lineType = "create"

		case inBefore && unknown:
			sBefore := formatValue(vBefore)
			line = fmt.Sprintf("      ~ %s = %s -> (known after apply)\n", k, sBefore)
			lineType = "update"

		case inBefore && inAfter:
			sBefore := formatValue(vBefore)
			sAfter := formatValue(vAfter)

			if sBefore != sAfter {
				if strings.Contains(sBefore, "\n") || strings.Contains(sAfter, "\n") {
					line = fmt.Sprintf("      ~ %s = %s\n        -> %s\n", k, sBefore, sAfter)
				} else {
					line = fmt.Sprintf("      ~ %s = %s -> %s\n", k, sBefore, sAfter)
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
func renderTableSummary(w io.Writer, changes []*ResourceChange) {
	table := tablewriter.NewWriter(w)
	table.SetHeader([]string{"Change", "Resource Type", "Resource Name", "Address"})
	table.SetBorder(true)
	table.SetAutoWrapText(false)

	for _, rc := range changes {
		_, _, actionType := getActionDetails(rc.Change.Actions)

		var changeStr string
		switch actionType {
		case "create":
			changeStr = colorAdd("CREATE")
		case "delete":
			changeStr = colorDelete("DELETE")
		case "update":
			changeStr = colorUpdate("UPDATE")
		case "replace":
			changeStr = colorReplace("REPLACE")
		default:
			changeStr = "NOOP"
		}

		table.Append([]string{changeStr, rc.Type, rc.Name, rc.Address})
	}

	table.Render()
}

// model represents the state of the interactive TUI view.
type model struct {
	changes    []*ResourceChange
	cursor     int
	showDetail bool
	viewport   viewport.Model
	ready      bool
	width      int
	height     int
	useColor   bool
}

func initialModel(changes []*ResourceChange, useColor bool) model {
	return model{
		changes:  changes,
		cursor:   0,
		useColor: useColor,
	}
}

func (m model) Init() tea.Cmd {
	return nil
}

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

				m.viewport = viewport.New(m.width, m.height-2)
				m.viewport.SetContent(content)
			}
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		if m.showDetail {
			m.viewport.Width = msg.Width
			m.viewport.Height = msg.Height - 2
		}
	}

	return m, nil
}

func (m model) View() string {
	if len(m.changes) == 0 {
		return "No changes found in plan.\nPress 'q' to exit."
	}

	if m.showDetail {
		header := lipgloss.NewStyle().Bold(true).Render("Resource Change Details (Press Space or 'q' to close):")
		return fmt.Sprintf("%s\n\n%s", header, m.viewport.View())
	}

	var sb strings.Builder
	sb.WriteString("Terraform Plan Changes (Use ↑/↓ to navigate, Space to view full change, 'q' to quit):\n\n")

	for i, rc := range m.changes {
		symbol, _, actionType := getActionDetails(rc.Change.Actions)

		var actionFormatted string
		switch actionType {
		case "create":
			actionFormatted = colorAdd(symbol + " CREATE")
		case "delete":
			actionFormatted = colorDelete(symbol + " DELETE")
		case "update":
			actionFormatted = colorUpdate(symbol + " UPDATE")
		case "replace":
			actionFormatted = colorReplace(symbol + " REPLACE")
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
	modeFlag := flag.String("mode", "tui", "Display mode: 'tui', 'table', or 'text'")
	noColorFlag := flag.Bool("no-color", false, "Disable color output")
	flag.Parse()

	useColor := !*noColorFlag

	var input io.Reader = os.Stdin
	if flag.NArg() > 0 {
		file, err := os.Open(flag.Arg(0))
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error opening file: %v\n", err)
			os.Exit(1)
		}
		defer file.Close()
		input = file
	}

	changes, err := parsePlan(input)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	switch *modeFlag {
	case "table":
		renderTableSummary(os.Stdout, changes)
	case "text":
		for _, rc := range changes {
			fmt.Print(renderResourceDiff(rc, useColor))
		}
	case "tui":
		p := tea.NewProgram(initialModel(changes, useColor), tea.WithAltScreen())
		if _, err := p.Run(); err != nil {
			fmt.Fprintf(os.Stderr, "Error running TUI: %v\n", err)
			os.Exit(1)
		}
	default:
		fmt.Fprintf(os.Stderr, "Invalid mode: %s. Valid choices are 'tui', 'table', 'text'\n", *modeFlag)
		os.Exit(1)
	}
}
