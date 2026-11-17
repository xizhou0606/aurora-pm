package projectgorm

import (
	"context"
	"fmt"

	"github.com/xizhou0606/aurora-pm/backend/internal/module/project"
	"gorm.io/gorm"
)

// RemovalImpactRepository is the GORM adapter for inspecting outstanding
// responsibilities that block a participant removal.
type RemovalImpactRepository struct{ db *gorm.DB }

// NewRemovalImpactRepository creates a new RemovalImpactRepository.
func NewRemovalImpactRepository(db *gorm.DB) *RemovalImpactRepository {
	return &RemovalImpactRepository{db: db}
}

// InspectRemovalImpact counts open tasks, open bugs, active workflow
// instances, and plan units owned by userID within the given workspace and
// project set. It returns at most 10 responsible items.
func (repository *RemovalImpactRepository) InspectRemovalImpact(
	ctx context.Context,
	workspaceID, projectSetID, userID uint,
) (project.RemovalImpact, error) {
	if repository == nil || repository.db == nil || workspaceID == 0 || projectSetID == 0 || userID == 0 {
		return project.RemovalImpact{}, project.ErrParticipantInvalid
	}
	db := repository.db.WithContext(ctx)

	var (
		openTaskCount       int64
		openBugCount        int64
		activeWorkflowCount int64
		planUnitOwnerCount  int64
	)

	// Count open tasks where the user is assignee, excluding terminal statuses.
	if err := db.Table("work_tasks").
		Where("workspace_id = ? AND project_set_id = ? AND assignee_id = ? AND status NOT IN ?",
			workspaceID, projectSetID, userID, []string{"completed", "closed", "cancelled"}).
		Count(&openTaskCount).Error; err != nil {
		return project.RemovalImpact{}, fmt.Errorf("count open tasks: %w", err)
	}

	// Count open bugs where the user is assignee, excluding terminal statuses.
	if err := db.Table("quality_bugs").
		Where("workspace_id = ? AND project_set_id = ? AND assignee_user_id = ? AND status NOT IN ?",
			workspaceID, projectSetID, userID, []string{"resolved", "closed", "rejected"}).
		Count(&openBugCount).Error; err != nil {
		return project.RemovalImpact{}, fmt.Errorf("count open bugs: %w", err)
	}

	// Count active workflow instances where the user is involved as starter
	// or has an active approval token assigned.
	var startedInstances int64
	if err := db.Table("workflow_instances").
		Where("workspace_id = ? AND started_by_user_id = ? AND status = ?",
			workspaceID, userID, "running").
		Count(&startedInstances).Error; err != nil {
		return project.RemovalImpact{}, fmt.Errorf("count started workflow instances: %w", err)
	}

	var tokenAssignedInstances int64
	if err := db.Table("workflow_tokens").
		Joins("JOIN workflow_instances ON workflow_instances.id = workflow_tokens.instance_id").
		Where("workflow_tokens.workspace_id = ? AND workflow_tokens.assignee_user_id = ? AND workflow_tokens.status = ? AND workflow_instances.status = ?",
			workspaceID, userID, "active", "running").
		Count(&tokenAssignedInstances).Error; err != nil {
		return project.RemovalImpact{}, fmt.Errorf("count token-assigned workflow instances: %w", err)
	}
	activeWorkflowCount = startedInstances + tokenAssignedInstances

	// Count plan units where the user is the owner.
	if err := db.Table("plan_units").
		Where("workspace_id = ? AND project_new_id = ? AND owner_user_id = ?",
			workspaceID, projectSetID, userID).
		Count(&planUnitOwnerCount).Error; err != nil {
		return project.RemovalImpact{}, fmt.Errorf("count plan unit owner: %w", err)
	}

	// Build at most 10 responsible items.
	impact := project.RemovalImpact{
		OpenTaskCount:       openTaskCount,
		OpenBugCount:        openBugCount,
		ActiveWorkflowCount: activeWorkflowCount,
		PlanUnitOwnerCount:  planUnitOwnerCount,
		ResponsibleItems:    make([]project.ImpactItem, 0, 10),
	}

	items, err := repository.collectResponsibleItems(ctx, db, workspaceID, projectSetID, userID, 10)
	if err != nil {
		return project.RemovalImpact{}, err
	}
	impact.ResponsibleItems = items

	return impact, nil
}

// collectResponsibleItems gathers up to limit responsible items from tasks,
// bugs, workflows, and plan units where the user has an outstanding
// responsibility.
func (repository *RemovalImpactRepository) collectResponsibleItems(
	ctx context.Context,
	db *gorm.DB,
	workspaceID, projectSetID, userID uint,
	limit int,
) ([]project.ImpactItem, error) {
	items := make([]project.ImpactItem, 0, limit)

	// Collect open tasks the user is assigned to.
	var tasks []struct {
		ID     uint
		Title  string
		Status string
	}
	if err := db.Table("work_tasks").
		Where("workspace_id = ? AND project_set_id = ? AND assignee_id = ? AND status NOT IN ?",
			workspaceID, projectSetID, userID, []string{"completed", "closed", "cancelled"}).
		Limit(limit).Find(&tasks).Error; err != nil {
		return nil, fmt.Errorf("query open tasks: %w", err)
	}
	for _, t := range tasks {
		items = append(items, project.ImpactItem{
			ResourceType: "task",
			ResourceID:   t.ID,
			Title:        t.Title,
			Status:       t.Status,
			Relation:     "assignee",
		})
		if len(items) >= limit {
			return items, nil
		}
	}

	// Collect open bugs the user is assigned to.
	var bugs []struct {
		ID     uint
		Title  string
		Status string
	}
	if err := db.Table("quality_bugs").
		Where("workspace_id = ? AND project_set_id = ? AND assignee_user_id = ? AND status NOT IN ?",
			workspaceID, projectSetID, userID, []string{"resolved", "closed", "rejected"}).
		Limit(limit - len(items)).Find(&bugs).Error; err != nil {
		return nil, fmt.Errorf("query open bugs: %w", err)
	}
	for _, b := range bugs {
		items = append(items, project.ImpactItem{
			ResourceType: "bug",
			ResourceID:   b.ID,
			Title:        b.Title,
			Status:       b.Status,
			Relation:     "assignee",
		})
		if len(items) >= limit {
			return items, nil
		}
	}

	// Collect active workflow instances the user started.
	var instances []struct {
		ID     uint
		Status string
	}
	if err := db.Table("workflow_instances").
		Where("workspace_id = ? AND started_by_user_id = ? AND status = ?",
			workspaceID, userID, "running").
		Limit(limit - len(items)).Find(&instances).Error; err != nil {
		return nil, fmt.Errorf("query workflow instances: %w", err)
	}
	for _, inst := range instances {
		items = append(items, project.ImpactItem{
			ResourceType: "workflow_instance",
			ResourceID:   inst.ID,
			Title:        fmt.Sprintf("Workflow #%d", inst.ID),
			Status:       inst.Status,
			Relation:     "starter",
		})
		if len(items) >= limit {
			return items, nil
		}
	}

	// Collect plan units the user owns.
	var planUnits []struct {
		ID     uint
		Title  string
		Status string
	}
	if err := db.Table("plan_units").
		Where("workspace_id = ? AND project_new_id = ? AND owner_user_id = ?",
			workspaceID, projectSetID, userID).
		Limit(limit - len(items)).Find(&planUnits).Error; err != nil {
		return nil, fmt.Errorf("query plan units: %w", err)
	}
	for _, pu := range planUnits {
		items = append(items, project.ImpactItem{
			ResourceType: "plan_unit",
			ResourceID:   pu.ID,
			Title:        pu.Title,
			Status:       pu.Status,
			Relation:     "owner",
		})
		if len(items) >= limit {
			return items, nil
		}
	}

	return items, nil
}

// InspectPlanUnitRemovalImpact counts outstanding responsibilities scoped to
// ONE plan unit: open tasks assigned to the user within the unit
// (work_tasks.project_id) and open bugs linked to the unit's requirements.
// Workflow instances are not unit-attributable and are not counted.
func (repository *RemovalImpactRepository) InspectPlanUnitRemovalImpact(
	ctx context.Context,
	workspaceID, planUnitID, userID uint,
) (project.RemovalImpact, error) {
	if repository == nil || repository.db == nil || workspaceID == 0 || planUnitID == 0 || userID == 0 {
		return project.RemovalImpact{}, project.ErrParticipantInvalid
	}
	db := repository.db.WithContext(ctx)

	openTaskStatuses := []string{"completed", "closed", "cancelled"}
	openBugStatuses := []string{"resolved", "closed", "rejected"}

	var openTaskCount int64
	if err := db.Table("work_tasks").
		Where("workspace_id = ? AND project_id = ? AND assignee_id = ? AND status NOT IN ?",
			workspaceID, planUnitID, userID, openTaskStatuses).
		Count(&openTaskCount).Error; err != nil {
		return project.RemovalImpact{}, fmt.Errorf("count open tasks: %w", err)
	}

	var openBugCount int64
	if err := db.Table("quality_bugs").
		Joins("JOIN requirements r ON r.id = quality_bugs.requirement_id").
		Where("quality_bugs.workspace_id = ? AND r.version_task_new_id = ? AND quality_bugs.assignee_user_id = ? AND quality_bugs.status NOT IN ?",
			workspaceID, planUnitID, userID, openBugStatuses).
		Count(&openBugCount).Error; err != nil {
		return project.RemovalImpact{}, fmt.Errorf("count open bugs: %w", err)
	}

	impact := project.RemovalImpact{
		OpenTaskCount:    openTaskCount,
		OpenBugCount:     openBugCount,
		ResponsibleItems: make([]project.ImpactItem, 0, 10),
	}

	limit := 10
	var tasks []struct {
		ID     uint
		Title  string
		Status string
	}
	if err := db.Table("work_tasks").
		Where("workspace_id = ? AND project_id = ? AND assignee_id = ? AND status NOT IN ?",
			workspaceID, planUnitID, userID, openTaskStatuses).
		Limit(limit).Find(&tasks).Error; err != nil {
		return project.RemovalImpact{}, fmt.Errorf("query open tasks: %w", err)
	}
	for _, t := range tasks {
		impact.ResponsibleItems = append(impact.ResponsibleItems, project.ImpactItem{
			ResourceType: "task", ResourceID: t.ID, Title: t.Title, Status: t.Status, Relation: "assignee",
		})
	}

	var bugs []struct {
		ID     uint
		Title  string
		Status string
	}
	if err := db.Table("quality_bugs").
		Joins("JOIN requirements r ON r.id = quality_bugs.requirement_id").
		Where("quality_bugs.workspace_id = ? AND r.version_task_new_id = ? AND quality_bugs.assignee_user_id = ? AND quality_bugs.status NOT IN ?",
			workspaceID, planUnitID, userID, openBugStatuses).
		Limit(limit - len(impact.ResponsibleItems)).Find(&bugs).Error; err != nil {
		return project.RemovalImpact{}, fmt.Errorf("query open bugs: %w", err)
	}
	for _, b := range bugs {
		if len(impact.ResponsibleItems) >= limit {
			break
		}
		impact.ResponsibleItems = append(impact.ResponsibleItems, project.ImpactItem{
			ResourceType: "bug", ResourceID: b.ID, Title: b.Title, Status: b.Status, Relation: "assignee",
		})
	}

	return impact, nil
}

// Compile-time check: RemovalImpactRepository implements the plan unit
// responsibility inspector.
var _ project.PlanUnitResponsibilityInspector = (*RemovalImpactRepository)(nil)
