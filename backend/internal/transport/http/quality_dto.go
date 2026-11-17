package httpapi

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	projectgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/project/gorm"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/quality"
	qualitygorm "github.com/xizhou0606/aurora-pm/backend/internal/module/quality/gorm"
)

func qualityCurrentOwner(bug qualitygorm.QualityBug) *uint {
	if qualityReviewerOwnsStatus(bug.Status) {
		if bug.ReviewerUserID != nil {
			return bug.ReviewerUserID
		}
		return bug.ReporterUserID
	}
	return bug.AssigneeUserID
}

func qualityListCurrentOwner(bug qualitygorm.QualityBug) *uint {
	if qualityListReviewerOwnsStatus(bug.Status) {
		if bug.ReviewerUserID != nil {
			return bug.ReviewerUserID
		}
		return bug.ReporterUserID
	}
	return bug.AssigneeUserID
}

func qualityBugTypeLabel(value string) string {
	switch normalizeStatus(value) {
	case "code_error":
		return "代码错误"
	case "config":
		return "配置相关"
	case "deployment":
		return "安装部署"
	case "standard":
		return "标准规范"
	case "test_script":
		return "测试脚本"
	case "design_defect":
		return "设计缺陷"
	case "other":
		return "其他"
	case "functional":
		return "功能"
	case "performance":
		return "性能"
	case "security":
		return "安全"
	case "compatibility":
		return "兼容性"
	case "ui", "ux":
		return "体验"
	default:
		return strings.TrimSpace(value)
	}
}

func qualityResolutionReasonLabel(value string) string {
	return quality.ResolutionReasonLabel(value)
}

func qualityBugDTO(bug qualitygorm.QualityBug, ctx workspaceContext) gin.H {
	owner := qualityListCurrentOwner(bug)
	bugKey := strings.TrimSpace(bug.BugKey)
	if bugKey == "" && bug.ID > 0 {
		bugKey = fmt.Sprintf("BUG-%d", bug.ID)
	}
	projectSet := ctx.ProjectSets[bug.ProjectSetID]
	project := ctx.Projects[bug.ProjectID]
	requirement := ctx.Requirements[bug.RequirementID]
	allowedActions := qualityAllowedActions(bug)
	resolutionReason := quality.NormalizeBugResolutionReason(bug.ResolutionReason)
	statusChangedAt := bug.StatusChangedAt
	if statusChangedAt.IsZero() {
		statusChangedAt = bug.CreatedAt
	}
	waitSeconds := int64(0)
	if !statusChangedAt.IsZero() {
		waitSeconds = int64(time.Since(statusChangedAt).Seconds())
		if waitSeconds < 0 {
			waitSeconds = 0
		}
	}
	return gin.H{
		"id": bug.ID, "workspace_id": bug.WorkspaceID, "team_id": bug.TeamID,
		"project_set_id": bug.ProjectSetID, "project_id": bug.ProjectID,
		"project_new_id": bug.ProjectID,
		"requirement_id": bug.RequirementID, "bug_key": bugKey,
		"title": bug.Title, "status": normalizeQualityBugStatus(bug.Status), "priority": bug.Priority,
		"priority_key": bug.Priority, "bug_type": bug.BugType,
		"defect_domain":  quality.NormalizeBugDefectDomain(bug.DefectDomain),
		"bug_type_label": qualityBugTypeLabel(bug.BugType),
		"severity_key":   bug.SeverityKey, "due_at": bug.DueAt, "due_date": bug.DueAt,
		"found_environment": bug.FoundEnvironment, "operating_system": bug.OperatingSystem,
		"browser": bug.Browser, "found_version": bug.FoundVersion,
		"reproduction_steps": bug.ReproductionSteps,
		"expected_result":    bug.ExpectedResult, "actual_result": bug.ActualResult,
		"reproduction_materials":  decodeQualityMaterials(bug.ReproductionMaterialsJSON),
		"resolution_reason":       resolutionReason,
		"resolution_reason_label": qualityResolutionReasonLabel(resolutionReason),
		"resolution_note":         bug.ResolutionNote,
		"resolution_materials":    decodeQualityMaterials(bug.ResolutionMaterialsJSON),
		"assignee_user_id":        bug.AssigneeUserID, "reporter_user_id": bug.ReporterUserID,
		"reviewer_user_id": bug.ReviewerUserID, "current_owner_user_id": owner,
		"defect_owner_user_id": bug.AssigneeUserID, "verified_by_user_id": bug.ReviewerUserID,
		"source_execution_id": bug.SourceExecutionID, "source_test_id": bug.SourceTestID,
		"assignee_name":      userDisplayName(ctx, bug.AssigneeUserID),
		"reporter_name":      userDisplayName(ctx, bug.ReporterUserID),
		"reviewer_name":      userDisplayName(ctx, bug.ReviewerUserID),
		"current_owner_name": userDisplayName(ctx, owner),
		"project_set_name":   projectSet.Name, "project_name": project.Title,
		"requirement_title": requirement.Title, "resolved_at": bug.ResolvedAt,
		"reviewed_at": bug.ReviewedAt, "completed_at": bug.CompletedAt,
		"verified_at": bug.ReviewedAt, "closed_at": bug.CompletedAt,
		"canceled_at": bug.CanceledAt, "allowed_actions": allowedActions,
		"status_changed_at": statusChangedAt, "status_wait_seconds": waitSeconds,
		"created_at": bug.CreatedAt, "updated_at": bug.UpdatedAt,
	}
}

func (s *Server) qualityBugByID(
	c *gin.Context,
	id uint,
) (qualitygorm.QualityBug, projectgorm.ProjectSet, bool) {
	workspace, _ := currentWorkspace(c)
	var bug qualitygorm.QualityBug
	query := s.db.Where("id = ? AND workspace_id = ?", id, workspace.ID)
	query = applyWorkspaceHierarchyScope(query, "quality_bugs", workspace.ID)
	if query.First(&bug).Error != nil {
		fail(c, http.StatusNotFound, "quality bug not found")
		return qualitygorm.QualityBug{}, projectgorm.ProjectSet{}, false
	}
	var project projectgorm.ProjectSet
	if s.db.Where("id = ? AND workspace_id = ?", bug.ProjectSetID, workspace.ID).First(&project).Error != nil {
		fail(c, http.StatusNotFound, "project set not found")
		return qualitygorm.QualityBug{}, projectgorm.ProjectSet{}, false
	}
	if !s.requireProjectAccess(c, workspace.ID, project.ID) {
		return qualitygorm.QualityBug{}, projectgorm.ProjectSet{}, false
	}
	return bug, project, true
}
