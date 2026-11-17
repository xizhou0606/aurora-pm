package requirementgorm

import (
	"context"
	"encoding/json"
	"errors"
	"sort"

	"github.com/xizhou0606/aurora-pm/backend/internal/module/requirement"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type MutationDependencies struct {
	Scopes     requirement.PlanScopeResolver
	Workflow   requirement.WorkflowLifecycle
	Dependents requirement.DependentLifecycle
	Effects    requirement.EffectWriter
	Covers     requirement.CoverLifecycle
	// ScopeRevisions is optional; when present, mutations that change the
	// current requirement set of a plan unit bump its delivery scope revision.
	ScopeRevisions requirement.DeliveryScopeBumper
}
type MutationDependencyFactory func(*gorm.DB) (MutationDependencies, error)
type MutationRepository struct {
	db           *gorm.DB
	dependencies MutationDependencyFactory
}

func NewMutationRepository(db *gorm.DB, dependencies MutationDependencyFactory) (*MutationRepository, error) {
	if db == nil || dependencies == nil {
		return nil, requirement.ErrMutationInvalid
	}
	return &MutationRepository{db: db, dependencies: dependencies}, nil
}
func (repository *MutationRepository) ApplyRequirementMutation(ctx context.Context, command requirement.Mutation) (requirement.ApplyResult, error) {
	if replay, found, err := repository.replay(ctx, command); found || err != nil {
		return replay, err
	}
	var result requirement.ApplyResult
	err := repository.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		ledger := ToolMutationApply{WorkspaceID: command.WorkspaceID, ApplyKey: command.ApplyKey, ChangeSetID: command.ChangeSetID, Operation: string(command.Operation), InputDigest: command.InputDigest, ResultJSON: ""}
		if err := tx.Create(&ledger).Error; err != nil {
			return err
		}
		deps, err := repository.dependencies(tx)
		if err != nil || deps.Scopes == nil || deps.Workflow == nil || deps.Dependents == nil || deps.Effects == nil || deps.Covers == nil {
			return requirement.ErrMutationConflict
		}
		switch command.Operation {
		case requirement.MutationCreate:
			result, err = applyRequirementCreate(ctx, tx, deps, command)
		case requirement.MutationUpdate:
			result, err = applyRequirementUpdate(ctx, tx, deps, command)
		case requirement.MutationBatchMove:
			result, err = applyRequirementBatchMove(ctx, tx, deps, command)
		case requirement.MutationDelete:
			result, err = applyRequirementDelete(ctx, tx, deps, command)
		default:
			err = requirement.ErrMutationInvalid
		}
		if err != nil {
			return err
		}
		raw, _ := json.Marshal(result)
		updated := tx.Model(&ToolMutationApply{}).Where("id = ? AND result_json = ''", ledger.ID).Update("result_json", string(raw))
		if updated.Error != nil {
			return updated.Error
		}
		if updated.RowsAffected != 1 {
			return requirement.ErrMutationConflict
		}
		return nil
	})
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return requirement.ApplyResult{}, requirement.ErrMutationNotFound
		}
		return requirement.ApplyResult{}, err
	}
	return result, nil
}

// bumpRequirementScopeRevisions bumps the delivery scope revision of every
// affected plan unit inside the caller's transaction. Optional dependency:
// without it the revision simply stays unchanged.
func bumpRequirementScopeRevisions(ctx context.Context, deps MutationDependencies, workspaceID uint, unitIDs []uint) error {
	if deps.ScopeRevisions == nil || len(unitIDs) == 0 {
		return nil
	}
	ids := append([]uint(nil), unitIDs...)
	sort.Slice(ids, func(left, right int) bool { return ids[left] < ids[right] })
	for _, unitID := range ids {
		if unitID == 0 {
			continue
		}
		if err := deps.ScopeRevisions.BumpDeliveryScopeRevision(ctx, workspaceID, unitID); err != nil {
			return err
		}
	}
	return nil
}

func applyRequirementCreate(ctx context.Context, tx *gorm.DB, deps MutationDependencies, command requirement.Mutation) (requirement.ApplyResult, error) {
	scope, err := deps.Scopes.ResolveRequirementPlan(ctx, command.WorkspaceID, command.PlanUnitID)
	if err != nil {
		return requirement.ApplyResult{}, err
	}
	// The resolver locked the plan unit row FOR UPDATE, so this fact is
	// authoritative against a concurrent intake-lock confirmation.
	if scope.RequirementsLockedAt != nil {
		return requirement.ApplyResult{}, requirement.ErrRequirementsLocked
	}
	priority := command.Priority
	if priority == "" {
		priority = "p1"
	}
	product := command.ProductOwnerUserID
	if product == nil {
		value := command.PrincipalID
		product = &value
	}
	row := Requirement{WorkspaceID: command.WorkspaceID, TeamID: scope.TeamID, ProjectSetID: scope.ProjectSetID, PlanUnitID: scope.PlanUnitID,
		Title: command.Title, Description: command.Description, ProductDocURL: command.ProductDocURL, ProductOwnerUserID: product,
		ProductOwnerUserIDs: CoalesceOwnerIDs(nil, product), DesignURL: command.DesignURL, DesignOwnerUserID: command.DesignOwnerUserID,
		DesignOwnerUserIDs: CoalesceOwnerIDs(nil, command.DesignOwnerUserID), DeveloperUserIDs: UniqueOwnerIDs(command.DeveloperUserIDs),
		AcceptanceCriteria: command.AcceptanceCriteria, Priority: priority, Status: "backlog"}
	if err := tx.Create(&row).Error; err != nil {
		return requirement.ApplyResult{}, err
	}
	value := requirementResult(row)
	instanceID, status, err := deps.Workflow.StartRequirementWorkflow(ctx, command, value)
	if err != nil || instanceID == 0 || status == "" {
		return requirement.ApplyResult{}, requirement.ErrMutationConflict
	}
	updated := tx.Model(&Requirement{}).Where("workspace_id = ? AND id = ? AND workflow_instance_id IS NULL", row.WorkspaceID, row.ID).Updates(map[string]any{"workflow_instance_id": instanceID, "status": status})
	if updated.Error != nil || updated.RowsAffected != 1 {
		return requirement.ApplyResult{}, requirement.ErrMutationConflict
	}
	row.WorkflowInstanceID, row.Status = &instanceID, status
	value = requirementResult(row)
	if err := deps.Covers.EnqueueRequirementCover(ctx, value); err != nil {
		return requirement.ApplyResult{}, err
	}
	if err := deps.Effects.RecordRequirementMutation(ctx, requirement.MutationEffect{WorkspaceID: command.WorkspaceID, PrincipalID: command.PrincipalID, ProjectSetID: row.ProjectSetID, RequirementID: row.ID, Action: "created", ApplyKey: command.ApplyKey, AffectedPlanUnitIDs: []uint{scope.PlanUnitID}}); err != nil {
		return requirement.ApplyResult{}, err
	}
	if err := bumpRequirementScopeRevisions(ctx, deps, command.WorkspaceID, []uint{scope.PlanUnitID}); err != nil {
		return requirement.ApplyResult{}, err
	}
	return requirement.ApplyResult{Requirement: &value}, nil
}
func applyRequirementUpdate(ctx context.Context, tx *gorm.DB, deps MutationDependencies, command requirement.Mutation) (requirement.ApplyResult, error) {
	var targetScope *requirement.PlanScope
	if command.PlanUnitID > 0 {
		scope, err := deps.Scopes.ResolveRequirementPlan(ctx, command.WorkspaceID, command.PlanUnitID)
		if err != nil {
			return requirement.ApplyResult{}, err
		}
		targetScope = &scope
	}
	var row Requirement
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("workspace_id = ? AND id = ?", command.WorkspaceID, command.ID).First(&row).Error; err != nil {
		return requirement.ApplyResult{}, err
	}
	if row.Version != command.ExpectedVersion || row.MutationState != "" {
		return requirement.ApplyResult{}, requirement.ErrMutationConflict
	}
	updates := map[string]any{}
	moved := false
	if command.TitleSet {
		if command.Title == "" {
			return requirement.ApplyResult{}, requirement.ErrMutationInvalid
		}
		updates["title"] = command.Title
	}
	if command.DescriptionSet {
		updates["description"] = command.Description
	}
	if command.ProductDocURLSet {
		updates["product_doc_url"] = command.ProductDocURL
	}
	if command.DesignURLSet {
		updates["design_url"] = command.DesignURL
	}
	if command.AcceptanceCriteriaSet {
		updates["acceptance_criteria"] = command.AcceptanceCriteria
	}
	if command.PrioritySet {
		updates["priority"] = command.Priority
	}
	if command.ProductOwnerSet {
		updates["product_owner_user_id"] = command.ProductOwnerUserID
		updates["product_owner_user_ids"] = CoalesceOwnerIDs(nil, command.ProductOwnerUserID)
	}
	if command.DesignOwnerSet {
		updates["design_owner_user_id"] = command.DesignOwnerUserID
		updates["design_owner_user_ids"] = CoalesceOwnerIDs(nil, command.DesignOwnerUserID)
	}
	if command.ProductOwnerUserIDsSet {
		ids := UniqueOwnerIDs(command.ProductOwnerUserIDs)
		updates["product_owner_user_ids"] = ids
		updates["product_owner_user_id"] = FirstOwnerID(ids, nil)
	}
	if command.DesignOwnerUserIDsSet {
		ids := UniqueOwnerIDs(command.DesignOwnerUserIDs)
		updates["design_owner_user_ids"] = ids
		updates["design_owner_user_id"] = FirstOwnerID(ids, nil)
	}
	if command.FrontendOwnerUserIDsSet {
		ids := UniqueOwnerIDs(command.FrontendOwnerUserIDs)
		updates["frontend_owner_user_ids"] = ids
		updates["frontend_owner_user_id"] = FirstOwnerID(ids, nil)
	}
	if command.BackendOwnerUserIDsSet {
		ids := UniqueOwnerIDs(command.BackendOwnerUserIDs)
		updates["backend_owner_user_ids"] = ids
		updates["backend_owner_user_id"] = FirstOwnerID(ids, nil)
	}
	if command.TesterUserIDsSet {
		updates["tester_user_ids"] = UniqueOwnerIDs(command.TesterUserIDs)
	}
	if command.DeveloperUserIDsSet {
		updates["developer_user_ids"] = UniqueOwnerIDs(command.DeveloperUserIDs)
	}
	if command.PlanUnitID > 0 && (row.ScheduledPlanUnitID == nil || *row.ScheduledPlanUnitID != command.PlanUnitID) {
		if targetScope == nil {
			return requirement.ApplyResult{}, requirement.ErrMutationConflict
		}
		if err := deps.Workflow.ValidateRequirementSchedule(ctx, command.WorkspaceID, row.ID, command.PlanUnitID, command.ConfirmSubjectSchedule); err != nil {
			return requirement.ApplyResult{}, requirement.ErrMutationConflict
		}
		updates["team_id"], updates["project_new_id"], updates["version_task_new_id"] = targetScope.TeamID, targetScope.ProjectSetID, targetScope.PlanUnitID
		if _, _, err := deps.Dependents.MoveRequirementDependents(ctx, command.WorkspaceID, row.ID, *targetScope); err != nil {
			return requirement.ApplyResult{}, err
		}
		moved = true
	}
	updates["version"] = gorm.Expr("version + 1")
	changed := tx.Model(&Requirement{}).Where("workspace_id = ? AND id = ? AND version = ?", command.WorkspaceID, row.ID, command.ExpectedVersion).Updates(updates)
	if changed.Error != nil || changed.RowsAffected != 1 {
		return requirement.ApplyResult{}, requirement.ErrMutationConflict
	}
	if err := tx.Where("workspace_id = ? AND id = ?", command.WorkspaceID, row.ID).First(&row).Error; err != nil {
		return requirement.ApplyResult{}, err
	}
	value := requirementResult(row)
	if command.ProductDocURLSet {
		if err := deps.Covers.EnqueueRequirementCover(ctx, value); err != nil {
			return requirement.ApplyResult{}, err
		}
	}
	action := "updated"
	if moved {
		action = "moved"
	}
	affectedUnits := []uint{}
	if moved {
		if row.ScheduledPlanUnitID != nil && *row.ScheduledPlanUnitID > 0 {
			affectedUnits = append(affectedUnits, *row.ScheduledPlanUnitID)
		}
		if command.PlanUnitID > 0 && command.PlanUnitID != row.PlanUnitID {
			affectedUnits = append(affectedUnits, command.PlanUnitID)
		}
	}
	if err := deps.Effects.RecordRequirementMutation(ctx, requirement.MutationEffect{WorkspaceID: command.WorkspaceID, PrincipalID: command.PrincipalID, ProjectSetID: row.ProjectSetID, RequirementID: row.ID, Action: action, ApplyKey: command.ApplyKey, AffectedPlanUnitIDs: affectedUnits}); err != nil {
		return requirement.ApplyResult{}, err
	}
	if err := bumpRequirementScopeRevisions(ctx, deps, command.WorkspaceID, affectedUnits); err != nil {
		return requirement.ApplyResult{}, err
	}
	return requirement.ApplyResult{Requirement: &value}, nil
}
func applyRequirementBatchMove(ctx context.Context, tx *gorm.DB, deps MutationDependencies, command requirement.Mutation) (requirement.ApplyResult, error) {
	scope, err := deps.Scopes.ResolveRequirementPlan(ctx, command.WorkspaceID, command.PlanUnitID)
	if err != nil || scope.Type != "version" {
		return requirement.ApplyResult{}, requirement.ErrMutationConflict
	}
	items := append([]requirement.BatchMoveItem(nil), command.Items...)
	sort.Slice(items, func(i, j int) bool { return items[i].RequirementID < items[j].RequirementID })
	output := requirement.BatchMoveResult{Items: []requirement.MutationResult{}}
	affectedUnits := map[uint]bool{scope.PlanUnitID: true}
	for _, item := range items {
		var row Requirement
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("workspace_id = ? AND id = ?", command.WorkspaceID, item.RequirementID).First(&row).Error; err != nil {
			return requirement.ApplyResult{}, err
		}
		if row.Version != item.ExpectedVersion || row.MutationState != "" {
			return requirement.ApplyResult{}, requirement.ErrMutationConflict
		}
		if row.ScheduledPlanUnitID == nil || *row.ScheduledPlanUnitID != scope.PlanUnitID {
			if row.ScheduledPlanUnitID != nil && *row.ScheduledPlanUnitID > 0 {
				affectedUnits[*row.ScheduledPlanUnitID] = true
			}
			if err := deps.Workflow.ValidateRequirementSchedule(ctx, command.WorkspaceID, row.ID, scope.PlanUnitID, command.ConfirmSubjectSchedule); err != nil {
				return requirement.ApplyResult{}, requirement.ErrMutationConflict
			}
			tasks, bugs, err := deps.Dependents.MoveRequirementDependents(ctx, command.WorkspaceID, row.ID, scope)
			if err != nil {
				return requirement.ApplyResult{}, err
			}
			output.AffectedTaskCount += tasks
			output.AffectedQualityCount += bugs
			changed := tx.Model(&Requirement{}).Where("workspace_id = ? AND id = ? AND version = ?", command.WorkspaceID, row.ID, item.ExpectedVersion).Updates(map[string]any{"team_id": scope.TeamID, "project_new_id": scope.ProjectSetID, "version_task_new_id": scope.PlanUnitID, "version": gorm.Expr("version + 1")})
			if changed.Error != nil || changed.RowsAffected != 1 {
				return requirement.ApplyResult{}, requirement.ErrMutationConflict
			}
			output.MovedCount++
			if err := deps.Effects.RecordRequirementMutation(ctx, requirement.MutationEffect{WorkspaceID: command.WorkspaceID, PrincipalID: command.PrincipalID, ProjectSetID: scope.ProjectSetID, RequirementID: row.ID, Action: "moved", ApplyKey: command.ApplyKey}); err != nil {
				return requirement.ApplyResult{}, err
			}
			tx.Where("workspace_id = ? AND id = ?", command.WorkspaceID, row.ID).First(&row)
		}
		output.Items = append(output.Items, requirementResult(row))
	}
	affectedUnitIDs := make([]uint, 0, len(affectedUnits))
	for unitID := range affectedUnits {
		affectedUnitIDs = append(affectedUnitIDs, unitID)
	}
	if err := deps.Effects.RecordRequirementMutation(ctx, requirement.MutationEffect{WorkspaceID: command.WorkspaceID, PrincipalID: command.PrincipalID, ProjectSetID: scope.ProjectSetID, Action: "batch_moved", ApplyKey: command.ApplyKey, AffectedPlanUnitIDs: affectedUnitIDs}); err != nil {
		return requirement.ApplyResult{}, err
	}
	if err := bumpRequirementScopeRevisions(ctx, deps, command.WorkspaceID, affectedUnitIDs); err != nil {
		return requirement.ApplyResult{}, err
	}
	return requirement.ApplyResult{Batch: &output}, nil
}
func applyRequirementDelete(ctx context.Context, tx *gorm.DB, deps MutationDependencies, command requirement.Mutation) (requirement.ApplyResult, error) {
	var row Requirement
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("workspace_id = ? AND id = ?", command.WorkspaceID, command.ID).First(&row).Error; err != nil {
		return requirement.ApplyResult{}, err
	}
	history, err := deps.Workflow.RequirementHasWorkflowHistory(ctx, command.WorkspaceID, row.ID)
	if err != nil || history {
		return requirement.ApplyResult{}, requirement.ErrMutationConflict
	}
	if err := deps.Dependents.ValidateRequirementDeletion(ctx, command.WorkspaceID, row.ID); err != nil {
		return requirement.ApplyResult{}, err
	}
	if err := deps.Workflow.DeleteRequirementWorkflow(ctx, command.WorkspaceID, row.ID); err != nil {
		return requirement.ApplyResult{}, err
	}
	if _, _, err := deps.Dependents.DeleteRequirementDependents(ctx, command.WorkspaceID, row.ID); err != nil {
		return requirement.ApplyResult{}, err
	}
	if err := MarkCoverDeletedTx(tx, command.WorkspaceID, row.ID); err != nil {
		return requirement.ApplyResult{}, err
	}
	if err := tx.Where("workspace_id = ? AND requirement_id = ?", command.WorkspaceID, row.ID).Delete(&RequirementMaterial{}).Error; err != nil {
		return requirement.ApplyResult{}, err
	}
	affectedUnits := []uint{}
	if row.ScheduledPlanUnitID != nil && *row.ScheduledPlanUnitID > 0 {
		affectedUnits = append(affectedUnits, *row.ScheduledPlanUnitID)
	}
	if err := deps.Effects.RecordRequirementMutation(ctx, requirement.MutationEffect{WorkspaceID: command.WorkspaceID, PrincipalID: command.PrincipalID, ProjectSetID: row.ProjectSetID, RequirementID: row.ID, Action: "deleted", ApplyKey: command.ApplyKey, AffectedPlanUnitIDs: affectedUnits}); err != nil {
		return requirement.ApplyResult{}, err
	}
	if err := bumpRequirementScopeRevisions(ctx, deps, command.WorkspaceID, affectedUnits); err != nil {
		return requirement.ApplyResult{}, err
	}
	deleted := tx.Where("workspace_id = ? AND id = ?", command.WorkspaceID, row.ID).Delete(&Requirement{})
	if deleted.Error != nil || deleted.RowsAffected != 1 {
		return requirement.ApplyResult{}, requirement.ErrMutationConflict
	}
	value := requirementResult(row)
	value.Deleted = true
	return requirement.ApplyResult{Requirement: &value}, nil
}
func requirementResult(row Requirement) requirement.MutationResult {
	developers := append([]uint(nil), row.DeveloperUserIDs...)
	return requirement.MutationResult{ID: row.ID, WorkspaceID: row.WorkspaceID, TeamID: row.TeamID, ProjectSetID: row.ProjectSetID, PlanUnitID: row.PlanUnitID, Version: row.Version, WorkflowInstanceID: row.WorkflowInstanceID, Title: row.Title, Description: row.Description, ProductDocURL: row.ProductDocURL, DesignURL: row.DesignURL, AcceptanceCriteria: row.AcceptanceCriteria, Priority: row.Priority, Status: row.Status, ProductOwnerUserID: row.ProductOwnerUserID, DesignOwnerUserID: row.DesignOwnerUserID, DeveloperUserIDs: developers}
}
func (repository *MutationRepository) replay(ctx context.Context, command requirement.Mutation) (requirement.ApplyResult, bool, error) {
	var row ToolMutationApply
	err := repository.db.WithContext(ctx).Where("apply_key = ?", command.ApplyKey).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return requirement.ApplyResult{}, false, nil
	}
	if err != nil {
		return requirement.ApplyResult{}, false, err
	}
	if row.WorkspaceID != command.WorkspaceID || row.ChangeSetID != command.ChangeSetID || row.Operation != string(command.Operation) || row.InputDigest != command.InputDigest || row.ResultJSON == "" {
		return requirement.ApplyResult{}, true, requirement.ErrMutationConflict
	}
	var result requirement.ApplyResult
	if json.Unmarshal([]byte(row.ResultJSON), &result) != nil {
		return requirement.ApplyResult{}, true, requirement.ErrMutationConflict
	}
	return result, true, nil
}

var _ requirement.MutationStore = (*MutationRepository)(nil)
