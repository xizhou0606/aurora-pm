package httpapi

import (
	"sort"

	"github.com/gin-gonic/gin"
	aigorm "github.com/xizhou0606/aurora-pm/backend/internal/module/ai/gorm"
	qualitygorm "github.com/xizhou0606/aurora-pm/backend/internal/module/quality/gorm"
	requirementgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/requirement/gorm"
	taskgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/task/gorm"
	workflowgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/workflow/gorm"
	"gorm.io/gorm"
)

func (s *Server) getRequirement(c *gin.Context) {
	id, valid := paramID(c, "id")
	if !valid {
		return
	}
	requirement, _, allowed := s.requirementByID(c, id)
	if !allowed {
		return
	}
	ok(c, s.requirementDTO(requirement))
}

func (s *Server) getRequirementDeletionImpact(c *gin.Context) {
	id, valid := paramID(c, "id")
	if !valid {
		return
	}
	requirement, _, allowed := s.requirementByID(c, id)
	if !allowed {
		return
	}
	impact, err := s.requirementDeletionImpact(requirement)
	if err != nil {
		dbFail(c)
		return
	}
	ok(c, impact)
}

func (s *Server) requirementDeletionImpact(requirement requirementgorm.Requirement) (gin.H, error) {
	workspaceID := requirement.WorkspaceID
	taskIDs, err := pluckScopedIDs(
		s.db.Model(&taskgorm.WorkTask{}), "workspace_id = ? AND requirement_id = ?", workspaceID, requirement.ID,
	)
	if err != nil {
		return nil, err
	}
	bugIDs, err := pluckScopedIDs(
		s.db.Unscoped().Model(&qualitygorm.QualityBug{}),
		"workspace_id = ? AND requirement_id = ?", workspaceID, requirement.ID,
	)
	if err != nil {
		return nil, err
	}
	taskCommentIDs, err := pluckDependentIDs(
		s.db.Model(&taskgorm.WorkTaskComment{}), workspaceID, "work_task_id", taskIDs,
	)
	if err != nil {
		return nil, err
	}
	taskAttachmentIDs, err := pluckDependentIDs(
		s.db.Model(&taskgorm.WorkTaskAttachment{}), workspaceID, "work_task_id", taskIDs,
	)
	if err != nil {
		return nil, err
	}
	taskActivityIDs, err := pluckDependentIDs(
		s.db.Model(&taskgorm.WorkTaskActivity{}), workspaceID, "work_task_id", taskIDs,
	)
	if err != nil {
		return nil, err
	}
	taskWatcherIDs, err := pluckDependentIDs(
		s.db.Model(&taskgorm.WorkTaskWatcher{}), workspaceID, "work_task_id", taskIDs,
	)
	if err != nil {
		return nil, err
	}
	bugActivityIDs, err := pluckDependentIDs(
		s.db.Model(&qualitygorm.QualityBugActivity{}), workspaceID, "quality_bug_id", bugIDs,
	)
	if err != nil {
		return nil, err
	}
	qualitySourceReferenceIDs, err := requirementTaskQualityReferenceIDs(
		s.db, workspaceID, taskIDs, bugIDs,
	)
	if err != nil {
		return nil, err
	}
	workflowInstanceIDs, err := pluckScopedIDs(
		s.db.Model(&workflowgorm.WorkflowInstance{}),
		"workspace_id = ? AND subject_type = ? AND subject_id = ?", workspaceID, "requirement", requirement.ID,
	)
	if err != nil {
		return nil, err
	}
	aiRunIDs, err := pluckScopedIDs(
		s.db.Model(&aigorm.AIRun{}),
		"workspace_id = ? AND subject_type = ? AND subject_id = ?", workspaceID, "requirement", requirement.ID,
	)
	if err != nil {
		return nil, err
	}
	requirementMaterialIDs, err := pluckScopedIDs(
		s.db.Model(&requirementgorm.RequirementMaterial{}),
		"workspace_id = ? AND requirement_id = ?", workspaceID, requirement.ID,
	)
	if err != nil {
		return nil, err
	}

	counts := gin.H{
		"requirements": 1,
		"tasks":        len(taskIDs), "bugs": len(bugIDs),
		"task_comments": len(taskCommentIDs), "task_attachments": len(taskAttachmentIDs),
		"task_activities": len(taskActivityIDs), "task_watchers": len(taskWatcherIDs),
		"bug_activities":            len(bugActivityIDs),
		"quality_source_references": len(qualitySourceReferenceIDs),
		"workflow_instances":        len(workflowInstanceIDs),
		"ai_runs":                   len(aiRunIDs),
		"requirement_materials":     len(requirementMaterialIDs),
	}
	affectedCount := 1 + len(taskIDs) + len(bugIDs) + len(taskCommentIDs) +
		len(taskAttachmentIDs) + len(taskActivityIDs) + len(taskWatcherIDs) + len(bugActivityIDs) +
		len(qualitySourceReferenceIDs) + len(workflowInstanceIDs) + len(aiRunIDs) + len(requirementMaterialIDs)
	affectedIDs := gin.H{
		"requirements": []uint{requirement.ID},
		"tasks":        taskIDs, "bugs": bugIDs,
		"task_comments": taskCommentIDs, "task_attachments": taskAttachmentIDs,
		"task_activities": taskActivityIDs, "task_watchers": taskWatcherIDs,
		"bug_activities":            bugActivityIDs,
		"quality_source_references": qualitySourceReferenceIDs,
		"workflow_instances":        workflowInstanceIDs,
		"ai_runs":                   aiRunIDs,
		"requirement_materials":     requirementMaterialIDs,
	}
	return gin.H{
		"requirement_id": requirement.ID,
		"requirement":    s.requirementDTO(requirement),
		"task_ids":       taskIDs,
		"bug_ids":        bugIDs,
		"counts":         counts,
		"affected_ids":   affectedIDs,
		"affected_count": affectedCount,
		"cascade_required": len(taskIDs) > 0 || len(bugIDs) > 0 || len(workflowInstanceIDs) > 0 ||
			len(aiRunIDs) > 0 || len(requirementMaterialIDs) > 0,
	}, nil
}

// requirementTaskQualityReferenceIDs mirrors the cross-requirement mutation in
// deleteWorkTaskDependents: deleting a requirement clears task source pointers
// on live quality bugs that are not themselves deleted with the requirement.
func requirementTaskQualityReferenceIDs(
	db *gorm.DB,
	workspaceID uint,
	taskIDs []uint,
	deletedBugIDs []uint,
) ([]uint, error) {
	if len(taskIDs) == 0 {
		return []uint{}, nil
	}
	query := db.Model(&qualitygorm.QualityBug{}).Where(
		"workspace_id = ? AND (source_execution_id IN ? OR source_test_id IN ?)",
		workspaceID, taskIDs, taskIDs,
	)
	if len(deletedBugIDs) > 0 {
		query = query.Where("id NOT IN ?", deletedBugIDs)
	}
	return pluckScopedIDs(query, "1 = 1")
}

func pluckScopedIDs(query *gorm.DB, condition string, values ...any) ([]uint, error) {
	ids := make([]uint, 0)
	if err := query.Where(condition, values...).Pluck("id", &ids).Error; err != nil {
		return nil, err
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	return ids, nil
}

func pluckDependentIDs(query *gorm.DB, workspaceID uint, foreignKey string, ownerIDs []uint) ([]uint, error) {
	if len(ownerIDs) == 0 {
		return []uint{}, nil
	}
	return pluckScopedIDs(
		query, "workspace_id = ? AND "+foreignKey+" IN ?", workspaceID, ownerIDs,
	)
}
