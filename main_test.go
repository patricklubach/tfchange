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
    },
    {
      "address": "aws_instance.web",
      "type": "aws_instance",
      "name": "web",
      "change": {
        "actions": ["create"],
        "before": null,
        "after": {"instance_type": "t3.micro"},
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

	if len(changes) != 2 {
		t.Fatalf("expected 2 changes, got %d", len(changes))
	}
}

func TestCalculateSummary(t *testing.T) {
	r := strings.NewReader(samplePlanJSON)
	changes, err := parsePlan(r)
	if err != nil {
		t.Fatalf("unexpected error parsing plan: %v", err)
	}

	summary := calculateSummary(changes)
	if summary.Create != 1 || summary.Update != 1 {
		t.Errorf("expected 1 create and 1 update, got: %+v", summary)
	}

	summaryString := summary.String(false)
	expected := "Plan: 1 to be created, 1 to be updated."
	if summaryString != expected {
		t.Errorf("expected '%s', got '%s'", expected, summaryString)
	}
}

func TestRenderTableSummary(t *testing.T) {
	r := strings.NewReader(samplePlanJSON)
	changes, err := parsePlan(r)
	if err != nil {
		t.Fatalf("unexpected error parsing plan: %v", err)
	}

	var buf bytes.Buffer
	renderTableSummary(&buf, changes, false)
	out := buf.String()

	if !strings.Contains(out, "aws_security_group") {
		t.Errorf("table output missing expected resource type")
	}

	if !strings.Contains(out, "Plan: 1 to be created, 1 to be updated.") {
		t.Errorf("table output missing summary line")
	}
}

func TestRenderMarkdownTable(t *testing.T) {
	r := strings.NewReader(samplePlanJSON)
	changes, err := parsePlan(r)
	if err != nil {
		t.Fatalf("unexpected error parsing plan: %v", err)
	}

	var buf bytes.Buffer
	renderMarkdownTable(&buf, changes, false)
	out := buf.String()

	if !strings.Contains(out, "| Change | Resource Type | Resource Name | Address |") {
		t.Errorf("markdown table missing header")
	}

	if !strings.Contains(out, "| UPDATE | aws_security_group | app_sg | aws_security_group.app_sg |") {
		t.Errorf("markdown table missing row entry")
	}

	if !strings.Contains(out, "Plan: 1 to be created, 1 to be updated.") {
		t.Errorf("markdown output missing summary line")
	}
}
