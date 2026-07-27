package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
)

// Action represents an individual state modification type in a plan.
type Action string

const (
	ActionNoOp   Action = "no-op"
	ActionCreate Action = "create"
	ActionRead   Action = "read"
	ActionUpdate Action = "update"
	ActionDelete Action = "delete"
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
	// If it's a map or slice containing unknown nested fields, consider the value unknown/partially unknown
	return val != nil
}

// getActionSymbol determines the Terraform diff prefix for a resource.
func getActionSymbol(actions ActionList) (string, string) {
	if len(actions) == 0 {
		return "#", "no-op"
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
		return "-/+", "must be replaced"
	}

	switch actions[0] {
	case ActionCreate:
		return "+", "will be created"
	case ActionDelete:
		return "-", "will be destroyed"
	case ActionUpdate:
		return "~", "will be updated in-place"
	default:
		return "~", "will be modified"
	}
}

// printHumanDiff formats and outputs resource changes without hiding unchanged attributes.
func printHumanDiff(w io.Writer, rc *ResourceChange) {
	symbol, actionDesc := getActionSymbol(rc.Change.Actions)

	fmt.Fprintf(w, "# %s %s\n", rc.Address, actionDesc)
	fmt.Fprintf(w, "  %s resource %q %q {\n", symbol, rc.Type, rc.Name)

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

		switch {
		case inBefore && !inAfter && !unknown:
			fmt.Fprintf(w, "      - %s = %s\n", k, formatValue(vBefore))

		case (!inBefore || vBefore == nil) && unknown:
			fmt.Fprintf(w, "      + %s = (known after apply)\n", k)

		case !inBefore && inAfter:
			fmt.Fprintf(w, "      + %s = %s\n", k, formatValue(vAfter))

		case inBefore && unknown:
			sBefore := formatValue(vBefore)
			fmt.Fprintf(w, "      ~ %s = %s -> (known after apply)\n", k, sBefore)

		case inBefore && inAfter:
			sBefore := formatValue(vBefore)
			sAfter := formatValue(vAfter)

			if sBefore != sAfter {
				if strings.Contains(sBefore, "\n") || strings.Contains(sAfter, "\n") {
					fmt.Fprintf(w, "      ~ %s = %s\n        -> %s\n", k, sBefore, sAfter)
				} else {
					fmt.Fprintf(w, "      ~ %s = %s -> %s\n", k, sBefore, sAfter)
				}
			} else {
				fmt.Fprintf(w, "        %s = %s\n", k, sBefore)
			}
		}
	}

	fmt.Fprintln(w, "    }")
	fmt.Fprintln(w)
}

// processPlan parses the input execution plan and outputs human-readable diffs.
func processPlan(r io.Reader, w io.Writer) error {
	data, err := io.ReadAll(r)
	if err != nil {
		return fmt.Errorf("reading input: %w", err)
	}

	var plan Plan
	if err := json.Unmarshal(data, &plan); err != nil {
		return fmt.Errorf("parsing plan JSON: %w", err)
	}

	hasChanges := false
	for _, rc := range plan.ResourceChanges {
		if rc.Change == nil || rc.Change.Actions.IsNoOp() {
			continue
		}
		hasChanges = true
		printHumanDiff(w, rc)
	}

	if !hasChanges {
		fmt.Fprintln(w, "No changes. Infrastructure matches configuration.")
	}

	return nil
}

func main() {
	var input io.Reader = os.Stdin

	if len(os.Args) > 1 {
		file, err := os.Open(os.Args[1])
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error opening file: %v\n", err)
			os.Exit(1)
		}
		defer file.Close()
		input = file
	}

	if err := processPlan(input, os.Stdout); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
