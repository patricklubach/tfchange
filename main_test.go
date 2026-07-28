package main

import (
	"bytes"
	"strings"
	"testing"
)

const samplePlanJSON = `{
  "resource_changes": [
    {
      "address": "aws_security_group.app_sg",
      "type": "aws_security_group",
      "name": "app_sg",
      "change": {
        "actions": ["update"],
        "before": {"description": "Old SG"},
        "after": {"description": "New SG"},
        "after_unknown": {"id": true}
      }
    }
  ]
}`

func TestParsePlan(t *testing.T) {
	r := strings.NewReader(samplePlanJSON)
	changes, err := parsePlan(r)
	if err != nil {
		t.Fatalf("unexpected error parsing plan: %v", err)
	}

	if len(changes) != 1 {
		t.Fatalf("expected 1 change, got %d", len(changes))
	}

	if changes[0].Address != "aws_security_group.app_sg" {
		t.Errorf("expected address 'aws_security_group.app_sg', got '%s'", changes[0].Address)
	}
}

func TestRenderTableSummary(t *testing.T) {
	r := strings.NewReader(samplePlanJSON)
	changes, err := parsePlan(r)
	if err != nil {
		t.Fatalf("unexpected error parsing plan: %v", err)
	}

	var buf bytes.Buffer
	renderTableSummary(&buf, changes)
	out := buf.String()

	if !strings.Contains(out, "aws_security_group") {
		t.Errorf("table output missing expected resource type")
	}
}
