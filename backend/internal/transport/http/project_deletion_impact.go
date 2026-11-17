package httpapi

import (
	"github.com/gin-gonic/gin"
	projectgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/project/gorm"
	qualitygorm "github.com/xizhou0606/aurora-pm/backend/internal/module/quality/gorm"
	requirementgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/requirement/gorm"
	taskgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/task/gorm"
	"gorm.io/gorm"
)

func (s *Server) getProjectSetDeletionImpact(c *gin.Context) {
	project, allowed := s.projectFromParam(c)
	if !allowed {
		return
	}
	impact, err := s.projectSetDeletionImpact(project)
	if err != nil {
		dbFail(c)
		return
	}
	ok(c, impact)
}

func (s *Server) projectSetDeletionImpact(project projectgorm.ProjectSet) (gin.H, error) {
	workspaceID, projectID := project.WorkspaceID, project.ID
	affected := gin.H{"project_sets": []uint{projectID}}
	counts := gin.H{"project_sets": 1}
	affectedCount := 1
	collect := func(key string, query *gorm.DB, condition string, values ...any) ([]uint, error) {
		ids, err := pluckScopedIDs(query, condition, values...)
		if err != nil {
			return nil, err
		}
		affected[key] = ids
		counts[key] = len(ids)
		affectedCount += len(ids)
		return ids, nil
	}

	planUnitIDs, err := collect(
		"plan_units", s.db.Model(&projectgorm.PlanUnit{}),
		"workspace_id = ? AND project_new_id = ?", workspaceID, projectID,
	)
	if err != nil {
		return nil, err
	}
	requirementIDs, err := collect(
		"requirements", s.db.Model(&requirementgorm.Requirement{}),
		"workspace_id = ? AND project_new_id = ?", workspaceID, projectID,
	)
	if err != nil {
		return nil, err
	}
	taskIDs, err := collect(
		"tasks", s.db.Model(&taskgorm.WorkTask{}),
		"workspace_id = ? AND project_set_id = ?", workspaceID, projectID,
	)
	if err != nil {
		return nil, err
	}
	bugIDs, err := collect(
		"quality_bugs", s.db.Unscoped().Model(&qualitygorm.QualityBug{}),
		"workspace_id = ? AND project_set_id = ?", workspaceID, projectID,
	)
	if err != nil {
		return nil, err
	}

	for _, dependency := range []struct {
		key        string
		query      *gorm.DB
		foreignKey string
		ownerIDs   []uint
	}{
		{"delivery_nodes", s.db.Model(&projectgorm.DeliveryNode{}), "plan_unit_id", planUnitIDs},
		{"task_comments", s.db.Model(&taskgorm.WorkTaskComment{}), "work_task_id", taskIDs},
		{"task_attachments", s.db.Model(&taskgorm.WorkTaskAttachment{}), "work_task_id", taskIDs},
		{"task_activities", s.db.Model(&taskgorm.WorkTaskActivity{}), "work_task_id", taskIDs},
		{"task_watchers", s.db.Model(&taskgorm.WorkTaskWatcher{}), "work_task_id", taskIDs},
		{"task_test_definitions", s.db.Model(&taskgorm.TaskTestDefinition{}), "work_task_id", taskIDs},
		{"task_test_executions", s.db.Model(&taskgorm.TaskTestExecution{}), "work_task_id", taskIDs},
		{"task_test_execution_runs", s.db.Model(&taskgorm.TaskTestExecutionRun{}), "work_task_id", taskIDs},
		{"quality_bug_activities", s.db.Model(&qualitygorm.QualityBugActivity{}), "quality_bug_id", bugIDs},
	} {
		ids, dependencyErr := pluckDependentIDs(
			dependency.query, workspaceID, dependency.foreignKey, dependency.ownerIDs,
		)
		if dependencyErr != nil {
			return nil, dependencyErr
		}
		affected[dependency.key] = ids
		counts[dependency.key] = len(ids)
		affectedCount += len(ids)
	}

	for _, direct := range []struct {
		key       string
		query     *gorm.DB
		condition string
		values    []any
	}{
		{"delivery_node_labels", s.db.Model(&projectgorm.DeliveryNodeLabel{}),
			"workspace_id = ? AND project_set_id = ?", []any{workspaceID, projectID}},
		{"project_news", s.db.Model(&projectgorm.ProjectNews{}),
			"workspace_id = ? AND project_set_id = ?", []any{workspaceID, projectID}},
		{"key_sequences", s.db.Model(&taskgorm.KeySequence{}),
			"workspace_id = ? AND project_set_id = ?", []any{workspaceID, projectID}},
		{"legacy_project_permissions", s.db.Model(&projectgorm.ProjectPermission{}),
			"workspace_id = ? AND project_new_id = ?", []any{workspaceID, projectID}},
	} {
		if _, err := collect(direct.key, direct.query, direct.condition, direct.values...); err != nil {
			return nil, err
		}
	}
	permissionIDs := affected["legacy_project_permissions"].([]uint)
	roleIDs, err := pluckDependentIDs(
		s.db.Model(&projectgorm.ProjectMemberRole{}), workspaceID, "project_permission_id", permissionIDs,
	)
	if err != nil {
		return nil, err
	}
	affected["legacy_project_member_roles"] = roleIDs
	counts["legacy_project_member_roles"] = len(roleIDs)
	affectedCount += len(roleIDs)

	qualitySourceReferences, err := requirementTaskQualityReferenceIDs(
		s.db, workspaceID, taskIDs, bugIDs,
	)
	if err != nil {
		return nil, err
	}
	affected["quality_source_references"] = qualitySourceReferences
	counts["quality_source_references"] = len(qualitySourceReferences)
	affectedCount += len(qualitySourceReferences)

	return gin.H{
		"project_set_id": projectID,
		"project_set":    gin.H{"id": project.ID, "code": project.Code, "name": project.Name},
		"plan_unit_ids":  planUnitIDs, "requirement_ids": requirementIDs,
		"task_ids": taskIDs, "quality_bug_ids": bugIDs,
		"counts": counts, "affected_ids": affected, "affected_count": affectedCount,
		"cascade_required":                  affectedCount > 1,
		"legacy_authorization_cleanup_only": true,
	}, nil
}

func (s *Server) getPlanUnitDeletionImpact(c *gin.Context) {
	id, valid := paramID(c, "id")
	if !valid {
		return
	}
	unit, project, allowed := s.planUnitByID(c, id)
	if !allowed {
		return
	}
	impact, err := s.planUnitDeletionImpact(unit, project)
	if err != nil {
		dbFail(c)
		return
	}
	ok(c, impact)
}

func (s *Server) planUnitDeletionImpact(unit projectgorm.PlanUnit, project projectgorm.ProjectSet) (gin.H, error) {
	requirementIDs, err := pluckScopedIDs(
		s.db.Model(&requirementgorm.Requirement{}),
		"workspace_id = ? AND project_new_id = ? AND version_task_new_id = ?",
		unit.WorkspaceID, project.ID, unit.ID,
	)
	if err != nil {
		return nil, err
	}
	labelIDs, err := pluckScopedIDs(
		s.db.Model(&projectgorm.DeliveryNodeLabel{}),
		"workspace_id = ? AND project_set_id = ? AND plan_unit_id = ?",
		unit.WorkspaceID, project.ID, unit.ID,
	)
	if err != nil {
		return nil, err
	}
	nodeIDs, err := pluckScopedIDs(
		s.db.Model(&projectgorm.DeliveryNode{}),
		"workspace_id = ? AND plan_unit_id = ?", unit.WorkspaceID, unit.ID,
	)
	if err != nil {
		return nil, err
	}
	blocked := len(requirementIDs) > 0
	reason := ""
	if blocked {
		reason = "plan unit has requirements"
	}
	return gin.H{
		"plan_unit_id": unit.ID,
		"plan_unit":    gin.H{"id": unit.ID, "title": unit.Title, "type": unit.Type},
		"project_set":  gin.H{"id": project.ID, "code": project.Code, "name": project.Name},
		"blocked":      blocked, "blocked_reason": reason,
		"requirement_ids": requirementIDs,
		"counts": gin.H{
			"plan_units": 1, "requirements": len(requirementIDs),
			"delivery_node_labels": len(labelIDs), "delivery_nodes": len(nodeIDs),
		},
		"affected_ids": gin.H{
			"plan_units": []uint{unit.ID}, "requirements": requirementIDs,
			"delivery_node_labels": labelIDs, "delivery_nodes": nodeIDs,
		},
		"affected_count":   1 + len(labelIDs) + len(nodeIDs),
		"cascade_required": len(labelIDs) > 0 || len(nodeIDs) > 0,
	}, nil
}

func (s *Server) getDeliveryLabelDeletionImpact(c *gin.Context) {
	label, project, allowed := s.deliveryLabelByID(c)
	if !allowed {
		return
	}
	nodeIDs, err := pluckScopedIDs(
		s.db.Model(&projectgorm.DeliveryNode{}),
		"workspace_id = ? AND plan_unit_id = ? AND label_id = ?",
		label.WorkspaceID, label.PlanUnitID, label.ID,
	)
	if err != nil {
		dbFail(c)
		return
	}
	ok(c, gin.H{
		"delivery_label_id": label.ID,
		"delivery_label": gin.H{
			"id": label.ID, "client_key": label.ClientKey, "name": label.Name,
			"deleted": label.Deleted,
		},
		"project_set": gin.H{"id": project.ID, "code": project.Code, "name": project.Name},
		"soft_delete": true, "already_deleted": label.Deleted,
		"soft_delete_effect":       "the label is hidden from active label lists; linked delivery nodes remain unchanged",
		"linked_delivery_node_ids": nodeIDs,
		"counts":                   gin.H{"delivery_node_labels": 1, "linked_delivery_nodes": len(nodeIDs)},
		"affected_ids":             gin.H{"delivery_node_labels": []uint{label.ID}},
		"affected_count":           1, "cascade_required": false,
	})
}
