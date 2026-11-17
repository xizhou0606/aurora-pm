package httpapi

import (
	"net/http/httptest"
	"testing"

	workflowgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/workflow/gorm"
)

func workflowActionNeedsNextAssignee(action string) bool {
	switch action {
	case "complete", "advance", "approve", "override_ai", "condition_yes", "condition_no":
		return true
	default:
		return false
	}
}

func (a *testApp) workflowAction(
	t *testing.T,
	workspaceID uint,
	token string,
	instanceID uint,
	body map[string]any,
) *httptest.ResponseRecorder {
	t.Helper()
	payload := make(map[string]any, len(body)+1)
	for key, value := range body {
		payload[key] = value
	}
	action, _ := payload["action"].(string)
	if _, exists := payload["next_node_assignees"]; !exists && workflowActionNeedsNextAssignee(action) {
		nodeID, _ := payload["node_id"].(string)
		if assignees := a.explicitNextAssignees(t, workspaceID, instanceID, nodeID, action); len(assignees) > 0 {
			payload["next_node_assignees"] = assignees
		}
	}
	return a.request(
		t, "POST",
		"/api/v1/platform/pm/workflows/instances/"+uintString(instanceID)+"/actions",
		token, workspaceID, payload,
	)
}

func (a *testApp) explicitNextAssignees(
	t *testing.T,
	workspaceID, instanceID uint,
	nodeID, action string,
) []map[string]any {
	t.Helper()
	var instance workflowgorm.WorkflowInstance
	if err := a.db.Where("id = ? AND workspace_id = ?", instanceID, workspaceID).First(&instance).Error; err != nil {
		return nil
	}
	var version workflowgorm.WorkflowTemplateVersion
	if err := a.db.Where("id = ? AND workspace_id = ?", instance.TemplateVersionID, workspaceID).First(&version).Error; err != nil {
		return nil
	}
	definition, issues := compileWorkflowDefinition(version.Definition)
	if len(issues) > 0 {
		return nil
	}
	stripWorkflowTemplateAssignees(&definition)
	canonical := action
	if canonical == "advance" {
		canonical = "complete"
	}
	next, err := workflowNextNodesForInstanceTx(a.db, definition, instance, nodeID, canonical)
	if err != nil {
		return nil
	}
	assigneeID := instance.StartedByUserID
	if assigneeID == 0 {
		assigneeID = 1
	}
	items := make([]map[string]any, 0)
	for _, requiredID := range workflowAssigneeRequiredNodeIDs(definition, next) {
		node, found := workflowNodeByID(definition, requiredID)
		if found && instance.StartedByUserID > 0 && workflowNodeIsEntryAssignee(definition, node.ID) {
			continue
		}
		nextAssigneeID := assigneeID
		if planned, exists, err := workflowNodePlanAssignee(instance, requiredID); err != nil {
			t.Fatal(err)
		} else if exists && planned != nil && *planned > 0 {
			nextAssigneeID = *planned
		}
		items = append(items, map[string]any{"node_id": requiredID, "assignee_user_id": nextAssigneeID})
	}
	return items
}
