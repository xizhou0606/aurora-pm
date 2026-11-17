package httpapi

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	identitygorm "github.com/xizhou0606/aurora-pm/backend/internal/module/identity/gorm"
	projectgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/project/gorm"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/task"
)

func (s *Server) listIssueKinds(c *gin.Context) {
	if !s.validateTaskWorkflowScope(c) {
		return
	}
	kinds := task.StandardWorkflowKinds()
	rows := make([]gin.H, 0, len(kinds))
	for _, kind := range kinds {
		rows = append(rows, gin.H{"key": kind.Key, "kind_key": kind.KindKey, "label": kind.Label})
	}
	ok(c, rows)
}

func (s *Server) listIssueStatuses(c *gin.Context) {
	if !validTaskWorkflowKind(c.Query("kind")) {
		bad(c, "kind must be task, story, bug, epic, or subtask")
		return
	}
	if !s.validateTaskWorkflowScope(c) {
		return
	}
	statuses := task.StandardWorkflowStatuses()
	rows := make([]gin.H, len(statuses))
	for index, row := range statuses {
		rows[index] = gin.H{
			"status_key": row.StatusKey, "label": row.Label,
			"sort_order": row.SortOrder, "color": row.Color,
		}
	}
	ok(c, rows)
}

func (s *Server) listIssueWorkflowTransitions(c *gin.Context) {
	if !validTaskWorkflowKind(c.Query("kind")) {
		bad(c, "kind must be task, story, bug, epic, or subtask")
		return
	}
	if !s.validateTaskWorkflowScope(c) {
		return
	}
	profile := task.StandardWorkflowTransitionProfile()
	transitions := make([]gin.H, 0, len(profile.Transitions))
	for _, transition := range profile.Transitions {
		transitions = append(transitions, gin.H{
			"from_status": transition.FromStatus, "to_status": transition.ToStatus,
		})
	}
	// The production source's built-in profile is informational unless an
	// explicit enforce policy exists. Workflow editing is hidden on this page,
	// so PM preserves that standard-builtin behavior.
	ok(c, gin.H{
		"enabled": profile.Enabled, "enforce": profile.Enforce, "standard": profile.Standard,
		"workflow_profile": profile.WorkflowProfile, "transitions": transitions,
	})
}

func validTaskWorkflowKind(raw string) bool {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "task", "story", "bug", "epic", "subtask", "requirement", "req":
		return true
	default:
		return false
	}
}

func (s *Server) validateTaskWorkflowScope(c *gin.Context) bool {
	workspace, _ := currentWorkspace(c)
	teamID := queryUint(c, "team_id")
	projectID := queryUint(c, "project_id")
	if teamID == 0 && projectID == 0 {
		return true
	}
	if teamID == 0 {
		bad(c, "team_id required with project_id")
		return false
	}
	var team identitygorm.Team
	if s.db.Where("id = ? AND workspace_id = ?", teamID, workspace.ID).First(&team).Error != nil {
		fail(c, http.StatusNotFound, "team not found")
		return false
	}
	if projectID == 0 {
		return true
	}
	var plan projectgorm.PlanUnit
	if s.db.Where("id = ? AND workspace_id = ? AND team_id = ?", projectID, workspace.ID, team.ID).
		First(&plan).Error != nil {
		fail(c, http.StatusNotFound, "project not found")
		return false
	}
	var projectCount int64
	if s.db.Model(&projectgorm.ProjectSet{}).
		Where("id = ? AND workspace_id = ? AND team_id = ?", plan.ProjectSetID, workspace.ID, team.ID).
		Count(&projectCount).Error != nil || projectCount != 1 {
		fail(c, http.StatusNotFound, "project set not found")
		return false
	}
	return true
}

func standardAllowedTaskStatuses() []string {
	statuses := task.StandardWorkflowStatuses()
	result := make([]string, 0, len(statuses))
	for _, row := range statuses {
		result = append(result, row.StatusKey)
	}
	return result
}
