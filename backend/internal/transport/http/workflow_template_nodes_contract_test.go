package httpapi

import (
	"net/http"
	"testing"
)

func TestWorkflowTemplateListIncludesPublishedBusinessNodes(t *testing.T) {
	app, token, workspace, _, _ := newAIWorkflowFixture(t)
	createPublishedRequirementWorkflow(t, app, token, workspace.ID)

	plain := app.request(t, http.MethodGet, "/api/v1/platform/pm/workflows/templates?subject_type=requirement", token, workspace.ID, nil)
	requireStatus(t, plain, http.StatusOK)
	plainRows := decodeData(t, plain).([]any)
	if len(plainRows) == 0 {
		t.Fatal("expected published requirement templates")
	}
	if _, exists := plainRows[0].(map[string]any)["nodes"]; exists {
		t.Fatalf("list should omit nodes unless requested: %v", plainRows[0])
	}

	listed := app.request(t, http.MethodGet, "/api/v1/platform/pm/workflows/templates?subject_type=requirement&include_nodes=1", token, workspace.ID, nil)
	requireStatus(t, listed, http.StatusOK)
	rows := decodeData(t, listed).([]any)
	got := map[string]string{}
	for _, row := range rows {
		nodes, _ := row.(map[string]any)["nodes"].([]any)
		for _, item := range nodes {
			node := item.(map[string]any)
			id, _ := node["id"].(string)
			name, _ := node["name"].(string)
			got[id] = name
		}
	}
	if got["review"] != "需求评审" {
		t.Fatalf("published business nodes=%v", got)
	}
	if _, exists := got["start"]; exists {
		t.Fatalf("structural start node leaked: %v", got)
	}
	if _, exists := got["end"]; exists {
		t.Fatalf("structural end node leaked: %v", got)
	}
}
