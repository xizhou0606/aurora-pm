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
	requirementgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/requirement/gorm"
	"gorm.io/gorm"
)

func (s *Server) patchQualityBug(c *gin.Context) {
	id, okID := paramID(c, "id")
	if !okID {
		return
	}
	bug, project, found := s.qualityBugByID(c, id)
	if !found {
		return
	}
	var body qualityBugBody
	routeTemplate := CanonicalRouteTemplate(c.FullPath())
	statusRoute := routeTemplate == "/api/v1/platform/quality/bugs/:id/status"
	assignmentRoute := routeTemplate == "/api/v1/platform/quality/bugs/:id/assignment"
	var decodeErr error
	if statusRoute || assignmentRoute {
		decodeErr = decodeStrictTaskNativeJSON(c, &body)
	} else {
		decodeErr = c.ShouldBindJSON(&body)
	}
	if decodeErr != nil {
		bad(c, "invalid body")
		return
	}
	if statusRoute && !qualityBodyTransitionOnly(body) {
		bad(c, "status is required and only workflow transition fields are allowed")
		return
	}
	if assignmentRoute && !qualityBodyAssignmentOnly(body) {
		bad(c, "assignee is required and only assignment fields are allowed")
		return
	}
	targetProject := project
	var targetRequirement *requirementgorm.Requirement
	peopleIDs := qualityBodyUserFactIDs(body, true)
	if body.RequirementID > 0 {
		var requirement requirementgorm.Requirement
		if s.db.Where("id = ? AND workspace_id = ?", body.RequirementID, bug.WorkspaceID).
			First(&requirement).Error != nil {
			fail(c, http.StatusNotFound, "requirement not found")
			return
		}
		var resolvedProject projectgorm.ProjectSet
		if s.db.Where("id = ? AND workspace_id = ?", requirement.ProjectSetID, bug.WorkspaceID).
			First(&resolvedProject).Error != nil {
			fail(c, http.StatusNotFound, "project set not found")
			return
		}
		if !s.userCanAccessProject(c, bug.WorkspaceID, resolvedProject.ID) {
			fail(c, http.StatusNotFound, "project set not found")
			return
		}
		targetProject = resolvedProject
		targetRequirement = &requirement
		if err := s.validateWorkspaceHierarchy(bug.WorkspaceID, targetProject, requirement); err != nil {
			bad(c, err.Error())
			return
		}
		for _, userID := range []*uint{bug.AssigneeUserID, bug.ReviewerUserID} {
			if userID != nil {
				peopleIDs = append(peopleIDs, *userID)
			}
		}
	}
	if err := s.validateActivePMUserReferences(c, peopleIDs); err != nil {
		respondPMUserRegistryValidationError(c, err)
		return
	}
	workflowPayload, err := s.normalizeQualityWorkflowBody(
		bug, &body,
	)
	if err != nil {
		if respondQualityNoteLengthError(c, err) {
			return
		}
		bad(c, err.Error())
		return
	}
	updates, err := s.qualityBugUpdates(body)
	if err != nil {
		if respondQualityNoteLengthError(c, err) {
			return
		}
		bad(c, err.Error())
		return
	}
	if targetRequirement != nil {
		updates["team_id"] = targetProject.TeamID
		updates["project_set_id"] = targetProject.ID
		updates["project_id"] = targetRequirement.PlanUnitID
		updates["requirement_id"] = targetRequirement.ID
	}
	if title, ok := updates["title"].(string); ok && title == "" {
		bad(c, "title cannot be empty")
		return
	}
	effectiveStatus := normalizeQualityBugStatus(bug.Status)
	if value, ok := updates["status"].(string); ok {
		effectiveStatus = value
	}
	effectiveReason := quality.NormalizeBugResolutionReason(bug.ResolutionReason)
	if value, ok := updates["resolution_reason"].(string); ok {
		effectiveReason = value
	}
	if (body.Status != nil || body.ResolutionReason != nil) &&
		(effectiveStatus == "review" || effectiveStatus == "done") &&
		effectiveReason == "" {
		bad(c, "resolution_reason required")
		return
	}
	if body.Status != nil && (effectiveStatus == "review" || effectiveStatus == "done") &&
		!quality.ValidBugResolutionReason(effectiveReason) {
		bad(c, "resolution_reason is invalid")
		return
	}
	if !qualityPatchHasChanges(updates, workflowPayload.shouldRecordAssignment()) {
		ctx := s.loadWorkspaceContext(
			bug.WorkspaceID,
			workspaceContextReferences{}.withQualityBug(bug),
		)
		ok(c, qualityBugDTO(bug, ctx))
		return
	}
	actor, _ := currentActor(c)
	applyQualityStatusTimestamps(&bug, updates, factUserID(actor.User))
	before := bug
	notificationContext := s.loadWorkspaceContext(
		bug.WorkspaceID,
		workspaceContextReferences{}.withQualityBug(bug),
	)
	activitySummary := ""
	err = s.db.Transaction(func(tx *gorm.DB) error {
		lockedProjects, lockErr := lockProjectSetsTx(
			tx, bug.WorkspaceID, []uint{project.ID, targetProject.ID},
		)
		if lockErr != nil {
			return lockErr
		}
		lockedProject, exists := lockedProjects[project.ID]
		if !exists {
			return gorm.ErrRecordNotFound
		}
		project = lockedProject
		lockedTargetProject, exists := lockedProjects[targetProject.ID]
		if !exists {
			return gorm.ErrRecordNotFound
		}
		targetProject = lockedTargetProject
		if targetRequirement != nil {
			lockedRequirement, err := lockProjectRequirementTx(
				tx, bug.WorkspaceID, targetRequirement.ID, targetProject.ID,
			)
			if err != nil {
				return err
			}
			targetRequirement = &lockedRequirement
			if !requirementMatchesProject(*targetRequirement, targetProject) {
				return errProjectChildScopeChanged
			}
			updates["team_id"] = targetRequirement.TeamID
			updates["project_set_id"] = targetProject.ID
			updates["project_id"] = targetRequirement.PlanUnitID
			updates["requirement_id"] = targetRequirement.ID
		}
		lockedBug, rowLockErr := lockProjectQualityBugTx(tx, bug.WorkspaceID, before.ID, project.ID)
		if rowLockErr != nil {
			return rowLockErr
		}
		if !sameQualityBugScope(lockedBug, before) {
			return errProjectChildScopeChanged
		}
		if len(updates) > 0 {
			result := tx.Model(&qualitygorm.QualityBug{}).
				Where("id = ? AND workspace_id = ?", bug.ID, bug.WorkspaceID).
				Updates(updates)
			if result.Error != nil {
				return result.Error
			}
		}
		if err := tx.Where("id = ? AND workspace_id = ?", bug.ID, bug.WorkspaceID).
			First(&bug).Error; err != nil {
			return err
		}
		a, _ := currentActor(c)
		var err error
		activitySummary, err = recordQualityPatchActivity(
			tx, before, bug, factUserID(a.User), workflowPayload,
		)
		if err != nil {
			return err
		}
		if activitySummary == "" {
			activitySummary = qualityActivitySummary(updates, "")
		}
		if err := s.recordMutationWithDetail(
			tx, c, targetProject.ID, "quality_bug", bug.ID, "updated", bug.Title, activitySummary, updates,
		); err != nil {
			return err
		}
		return s.recordQualityPatchNotification(
			tx, c, before, bug, workflowPayload, notificationContext,
		)
	})
	if err != nil {
		if respondProjectChildScopeConflict(c, err) {
			return
		}
		dbFail(c)
		return
	}
	ctx := s.loadWorkspaceContext(
		bug.WorkspaceID,
		workspaceContextReferences{}.withQualityBug(bug),
	)
	ok(c, qualityBugDTO(bug, ctx))
}

func qualityBodyTransitionOnly(body qualityBugBody) bool {
	return body.Status != nil && body.RequirementID == 0 && body.Title == nil && body.Priority == nil &&
		body.BugType == nil && body.SeverityKey == nil && !body.DueAt.Set && !body.DueDate.Set &&
		body.FoundEnvironment == nil && body.OperatingSystem == nil && body.Browser == nil &&
		body.FoundVersion == nil && body.ReproductionSteps == nil && body.ExpectedResult == nil &&
		body.ActualResult == nil && !body.ReproductionMaterials.Set && !body.AssigneeUserID.Set &&
		!body.DefectOwnerUserID.Set && !body.SourceExecutionID.Set && !body.SourceTestID.Set &&
		body.AssignmentNote == nil && !body.AssignmentMaterials.Set
}

func qualityBodyAssignmentOnly(body qualityBugBody) bool {
	hasAssignee := body.AssigneeUserID.Set || body.DefectOwnerUserID.Set
	return hasAssignee && body.RequirementID == 0 && body.Title == nil && body.Status == nil &&
		body.Priority == nil && body.BugType == nil && body.DefectDomain == nil &&
		body.SeverityKey == nil && !body.DueAt.Set && !body.DueDate.Set &&
		body.FoundEnvironment == nil && body.OperatingSystem == nil && body.Browser == nil &&
		body.FoundVersion == nil && body.ReproductionSteps == nil && body.ExpectedResult == nil &&
		body.ActualResult == nil && !body.ReproductionMaterials.Set &&
		body.ResolutionReason == nil && body.ResolutionNote == nil &&
		!body.ResolutionMaterials.Set && !body.RegressionMaterials.Set &&
		!body.ReviewerUserID.Set && !body.VerifiedByUserID.Set &&
		!body.SourceExecutionID.Set && !body.SourceTestID.Set &&
		body.RegressionNote == nil && body.ResolvedNote == nil && body.ReviewNote == nil &&
		body.RejectNote == nil && body.ReturnNote == nil
}

func applyQualityPointers(bug *qualitygorm.QualityBug, updates map[string]any) {
	if value, exists := updates["due_at"]; exists {
		bug.DueAt, _ = value.(*time.Time)
	}
	if value, exists := updates["assignee_user_id"]; exists {
		bug.AssigneeUserID, _ = value.(*uint)
	}
	if value, exists := updates["reviewer_user_id"]; exists {
		bug.ReviewerUserID, _ = value.(*uint)
	}
	if value, exists := updates["source_execution_id"]; exists {
		bug.SourceExecutionID, _ = value.(*uint)
	}
	if value, exists := updates["source_test_id"]; exists {
		bug.SourceTestID, _ = value.(*uint)
	}
	for key, destination := range map[string]**time.Time{
		"resolved_at": &bug.ResolvedAt, "reviewed_at": &bug.ReviewedAt,
		"completed_at": &bug.CompletedAt, "canceled_at": &bug.CanceledAt,
	} {
		if value, exists := updates[key]; exists {
			*destination, _ = value.(*time.Time)
		}
	}
}

func applyQualityStatusTimestamps(bug *qualitygorm.QualityBug, updates map[string]any, actorID uint) {
	status, changed := updates["status"].(string)
	status = normalizeQualityBugStatus(status)
	if !changed || status == normalizeQualityBugStatus(bug.Status) {
		return
	}
	now := time.Now().UTC()
	updates["status_changed_at"] = now
	switch status {
	case "review":
		updates["resolved_at"] = &now
		updates["reviewed_at"] = nil
		updates["completed_at"] = nil
		updates["canceled_at"] = nil
		if _, set := updates["reviewer_user_id"]; !set && bug.ReporterUserID != nil {
			updates["reviewer_user_id"] = bug.ReporterUserID
		}
	case "done":
		updates["completed_at"] = &now
		updates["reviewed_at"] = &now
		updates["canceled_at"] = nil
		if _, set := updates["reviewer_user_id"]; !set && bug.ReviewerUserID == nil && actorID > 0 {
			updates["reviewer_user_id"] = &actorID
		}
	case "canceled":
		updates["canceled_at"] = &now
		updates["completed_at"] = nil
	case "triage":
		updates["resolution_reason"] = ""
		updates["resolution_note"] = ""
		updates["resolution_materials_json"] = "[]"
		updates["resolved_at"] = nil
		updates["reviewed_at"] = nil
		updates["reviewer_user_id"] = nil
		updates["completed_at"] = nil
		updates["canceled_at"] = nil
	case "todo", "in_progress", "reopened":
		updates["completed_at"] = nil
		updates["canceled_at"] = nil
	}
}

func qualityActivitySnapshot(bug qualitygorm.QualityBug) map[string]any {
	return map[string]any{
		"title": bug.Title, "status": bug.Status, "priority": bug.Priority,
		"severity_key": bug.SeverityKey, "assignee_user_id": bug.AssigneeUserID,
		"reviewer_user_id": bug.ReviewerUserID, "resolution_reason": bug.ResolutionReason,
		"project_set_id": bug.ProjectSetID, "project_id": bug.ProjectID,
		"requirement_id": bug.RequirementID,
	}
}

func qualityActivitySummary(updates map[string]any, note string) string {
	parts := make([]string, 0, 3)
	if status, ok := updates["status"].(string); ok {
		parts = append(parts, "状态变更为 "+status)
	}
	if _, ok := updates["assignee_user_id"]; ok {
		parts = append(parts, "更新负责人")
	}
	if _, ok := updates["requirement_id"]; ok {
		parts = append(parts, "更新关联需求")
	}
	if note != "" {
		parts = append(parts, "备注："+note)
	}
	if len(parts) == 0 {
		return fmt.Sprintf("更新 %d 个字段", len(updates))
	}
	return strings.Join(parts, "；")
}
