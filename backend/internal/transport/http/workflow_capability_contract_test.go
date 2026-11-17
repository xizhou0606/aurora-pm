package httpapi

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestAICapabilityResponseUsesArraysForEmptyFields(t *testing.T) {
	raw, err := json.Marshal(aiCapabilitiesForResponse())
	if err != nil {
		t.Fatal(err)
	}
	var capabilities []map[string]any
	if err := json.Unmarshal(raw, &capabilities); err != nil {
		t.Fatal(err)
	}
	for _, capability := range capabilities {
		if capability["subject_types"] == nil || capability["workflow_subject_types"] == nil || capability["allowed_fields"] == nil {
			t.Fatalf("capability arrays must not be null: %v", capability)
		}
	}
}

func TestWorkflowNodeCapabilitiesExposeOnlyOpenedBusinessLoops(t *testing.T) {
	got := map[string][]string{}
	for _, capability := range aiCapabilities {
		if len(capability.WorkflowSubjectTypes) > 0 {
			got[capability.Key] = capability.WorkflowSubjectTypes
		}
	}
	want := map[string][]string{
		"requirement.completeness": {"requirement"},
		"requirement.task_plan":    {"requirement"},
		"delivery.risk_monitor":    {"plan_unit"},
	}
	if !sameStringSliceMap(got, want) {
		t.Fatalf("workflow capability contract=%v, want %v", got, want)
	}
}

func TestWorkflowAuthoringCapabilitiesMatchSemanticNodeContract(t *testing.T) {
	tests := []struct {
		name        string
		nodeType    string
		capability  string
		subjectType string
		wantIssue   string
	}{
		{name: "analysis check", nodeType: "ai_check", capability: "requirement.completeness"},
		{name: "artifact generation", nodeType: "ai_generate", capability: "requirement.task_plan"},
		{name: "plan unit risk check", nodeType: "ai_check", capability: "delivery.risk_monitor", subjectType: "plan_unit"},
		{name: "workflow helper", nodeType: "ai_generate", capability: "workflow.generate", wantIssue: "未开放为流程节点"},
		{name: "test case workbench capability", nodeType: "ai_generate", capability: "qa.test_cases", wantIssue: "未开放为流程节点"},
		{name: "field mutation proposal", nodeType: "ai_generate", capability: "requirement.draft", wantIssue: "未开放为流程节点"},
		{name: "generic artifact", nodeType: "ai_generate", capability: "requirement.review_pack", wantIssue: "未开放为流程节点"},
		{name: "detail-only design classify", nodeType: "ai_check", capability: "requirement.design_classify", wantIssue: "未开放为流程节点"},
		{name: "version-page scope proposal", nodeType: "ai_generate", capability: "version.scope_proposal", subjectType: "plan_unit", wantIssue: "未开放为流程节点"},
		{name: "risk not opened for requirement workflow", nodeType: "ai_check", capability: "delivery.risk_monitor", wantIssue: "未开放为流程节点"},
		{name: "analysis on generate node", nodeType: "ai_generate", capability: "delivery.risk_monitor", subjectType: "plan_unit", wantIssue: "与节点类型不匹配"},
		{name: "artifact on check node", nodeType: "ai_check", capability: "requirement.task_plan", wantIssue: "与节点类型不匹配"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			subjectType := test.subjectType
			if subjectType == "" {
				subjectType = "requirement"
			}
			issues := validateWorkflowCapabilities(workflowDefinition{Nodes: []workflowNode{{
				ID: "ai", Type: test.nodeType, Name: "AI 节点", Capability: test.capability,
			}}}, subjectType)
			if test.wantIssue == "" {
				if len(issues) != 0 {
					t.Fatalf("valid capability rejected: %v", issues)
				}
				return
			}
			if !containsIssue(issues, test.wantIssue) {
				t.Fatalf("issues=%v, want substring %q", issues, test.wantIssue)
			}
		})
	}
}

func sameStringSliceMap(left, right map[string][]string) bool {
	if len(left) != len(right) {
		return false
	}
	for key, leftValues := range left {
		rightValues, ok := right[key]
		if !ok || len(leftValues) != len(rightValues) {
			return false
		}
		for index := range leftValues {
			if leftValues[index] != rightValues[index] {
				return false
			}
		}
	}
	return true
}

func containsIssue(issues []string, substring string) bool {
	for _, issue := range issues {
		if strings.Contains(issue, substring) {
			return true
		}
	}
	return false
}
