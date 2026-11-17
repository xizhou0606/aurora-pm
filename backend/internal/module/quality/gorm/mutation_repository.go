package qualitygorm

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/xizhou0606/aurora-pm/backend/internal/module/quality"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/requirement"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// MutationScopeFactory is an infrastructure composition seam. The application
// supplies a requirement-domain resolver backed by this transaction; neither
// owning domain exposes GORM in its contract.
type MutationScopeFactory func(*gorm.DB) (requirement.MutationScopeResolver, error)
type MutationEffectFactory func(*gorm.DB) (quality.BugMutationEffectSink, error)

type MutationRepository struct {
	db            *gorm.DB
	scopeFactory  MutationScopeFactory
	effectFactory MutationEffectFactory
}

func NewMutationRepository(
	db *gorm.DB,
	scopeFactory MutationScopeFactory,
	effectFactory MutationEffectFactory,
) (*MutationRepository, error) {
	if db == nil || scopeFactory == nil || effectFactory == nil {
		return nil, quality.ErrBugMutationInvalid
	}
	return &MutationRepository{db: db, scopeFactory: scopeFactory, effectFactory: effectFactory}, nil
}

func (repository *MutationRepository) ApplyBugMutation(
	ctx context.Context, command quality.BugMutation,
) (quality.BugMutationResult, error) {
	if repository == nil || repository.db == nil || repository.scopeFactory == nil || repository.effectFactory == nil {
		return quality.BugMutationResult{}, quality.ErrBugMutationInvalid
	}
	if replay, found, err := repository.replay(ctx, command); err != nil || found {
		return replay, err
	}
	var result quality.BugMutationResult
	err := repository.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		ledger := ToolMutationApply{
			WorkspaceID: command.WorkspaceID, ApplyKey: command.ApplyKey, ChangeSetID: command.ChangeSetID,
			Operation: string(command.Operation), InputDigest: command.InputDigest,
		}
		if err := tx.Create(&ledger).Error; err != nil {
			return err
		}
		resolver, err := repository.scopeFactory(tx)
		if err != nil || resolver == nil {
			if err != nil {
				return err
			}
			return quality.ErrBugMutationInvalid
		}
		effects, err := repository.effectFactory(tx)
		if err != nil || effects == nil {
			if err != nil {
				return err
			}
			return quality.ErrBugMutationInvalid
		}
		switch command.Operation {
		case quality.BugCreate:
			var bug quality.BugSnapshot
			bug, err = applyBugCreate(ctx, tx, resolver, command.WorkspaceID, command.PrincipalID, command.Values)
			result.Bug = &bug
			if err == nil {
				err = effects.RecordBugMutation(ctx, quality.BugMutationEffect{
					Operation: command.Operation, WorkspaceID: command.WorkspaceID,
					PrincipalID: command.PrincipalID, After: bug,
				})
			}
		case quality.BugUpdate:
			var bug, before quality.BugSnapshot
			bug, before, err = applyBugUpdate(ctx, tx, resolver, command)
			result.Bug = &bug
			if err == nil {
				err = effects.RecordBugMutation(ctx, quality.BugMutationEffect{
					Operation: command.Operation, WorkspaceID: command.WorkspaceID,
					PrincipalID: command.PrincipalID, Before: &before, After: bug,
				})
			}
		case quality.BugDelete:
			err = applyBugDelete(ctx, tx, resolver, command.WorkspaceID, command.ID)
			result.DeletedID, result.Deleted = command.ID, err == nil
		case quality.BugBatchCreate:
			result.Created = make([]quality.BatchBugMutationResult, 0, len(command.Items))
			for _, item := range command.Items {
				var bug quality.BugSnapshot
				bug, err = applyBugCreate(ctx, tx, resolver, command.WorkspaceID, command.PrincipalID, item.Values)
				if err != nil {
					break
				}
				if err = effects.RecordBugMutation(ctx, quality.BugMutationEffect{
					Operation: quality.BugCreate, WorkspaceID: command.WorkspaceID,
					PrincipalID: command.PrincipalID, After: bug,
				}); err != nil {
					break
				}
				result.Created = append(result.Created, quality.BatchBugMutationResult{ClientID: item.ClientID, Bug: bug})
			}
			result.CreatedCount, result.Atomic = len(result.Created), err == nil
		case quality.BugBatchAssign:
			result.Updated = make([]quality.BugSnapshot, 0, len(command.IDs))
			for _, id := range command.IDs {
				itemCommand := command
				itemCommand.Operation, itemCommand.ID, itemCommand.IDs = quality.BugUpdate, id, nil
				var bug, before quality.BugSnapshot
				bug, before, err = applyBugUpdate(ctx, tx, resolver, itemCommand)
				if err != nil {
					break
				}
				if err = effects.RecordBugMutation(ctx, quality.BugMutationEffect{
					Operation: quality.BugUpdate, WorkspaceID: command.WorkspaceID,
					PrincipalID: command.PrincipalID, Before: &before, After: bug,
				}); err != nil {
					break
				}
				result.Updated = append(result.Updated, bug)
			}
			result.UpdatedCount, result.Atomic = len(result.Updated), err == nil
		case quality.BugBatchDelete:
			for _, id := range command.IDs {
				if err = applyBugDelete(ctx, tx, resolver, command.WorkspaceID, id); err != nil {
					break
				}
				result.DeletedIDs = append(result.DeletedIDs, id)
			}
			result.DeletedCount, result.Atomic = len(result.DeletedIDs), err == nil
		default:
			err = quality.ErrBugMutationInvalid
		}
		if err != nil {
			return err
		}
		encoded, err := json.Marshal(result)
		if err != nil {
			return err
		}
		updated := tx.Model(&ToolMutationApply{}).Where("id = ? AND result_json = ''", ledger.ID).
			Update("result_json", string(encoded))
		if updated.Error != nil {
			return updated.Error
		}
		if updated.RowsAffected != 1 {
			return quality.ErrBugMutationConflict
		}
		return nil
	})
	if err == nil {
		return result, nil
	}
	if replay, found, replayErr := repository.replay(ctx, command); found || replayErr != nil {
		return replay, replayErr
	}
	return quality.BugMutationResult{}, normalizeQualityMutationError(err)
}

func (repository *MutationRepository) replay(
	ctx context.Context, command quality.BugMutation,
) (quality.BugMutationResult, bool, error) {
	var ledger ToolMutationApply
	err := repository.db.WithContext(ctx).
		Where("apply_key = ? OR change_set_id = ?", command.ApplyKey, command.ChangeSetID).
		First(&ledger).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return quality.BugMutationResult{}, false, nil
	}
	if err != nil {
		return quality.BugMutationResult{}, false, err
	}
	if ledger.ApplyKey != command.ApplyKey || ledger.WorkspaceID != command.WorkspaceID || ledger.ChangeSetID != command.ChangeSetID ||
		ledger.Operation != string(command.Operation) || ledger.InputDigest != command.InputDigest || ledger.ResultJSON == "" {
		return quality.BugMutationResult{}, true, quality.ErrBugMutationConflict
	}
	var result quality.BugMutationResult
	if json.Unmarshal([]byte(ledger.ResultJSON), &result) != nil {
		return quality.BugMutationResult{}, true, quality.ErrBugMutationConflict
	}
	return result, true, nil
}

func applyBugCreate(
	ctx context.Context, tx *gorm.DB, resolver requirement.MutationScopeResolver,
	workspaceID, principalID uint, values quality.BugMutationValues,
) (quality.BugSnapshot, error) {
	scope, err := resolver.ResolveMutationScope(ctx, workspaceID, values.RequirementID)
	if err != nil {
		return quality.BugSnapshot{}, normalizeQualityMutationError(err)
	}
	now := time.Now().UTC()
	row := QualityBug{
		ReproductionMaterialsJSON: quality.EncodeMaterials(values.ReproductionMaterials), ResolutionMaterialsJSON: quality.EncodeMaterials(values.ResolutionMaterials),
		WorkspaceID: scope.WorkspaceID, TeamID: scope.TeamID, ProjectSetID: scope.ProjectSetID,
		ProjectID: scope.PlanUnitID, RequirementID: scope.RequirementID, Title: values.Title,
		Status: values.Status, Priority: values.Priority, BugType: values.BugType, DefectDomain: values.DefectDomain,
		SeverityKey: values.SeverityKey,
		DueAt:       values.DueAt, FoundEnvironment: values.FoundEnvironment, OperatingSystem: values.OperatingSystem,
		Browser: values.Browser, FoundVersion: values.FoundVersion, ReproductionSteps: values.ReproductionSteps,
		ExpectedResult: values.ExpectedResult, ActualResult: values.ActualResult,
		ResolutionReason: values.ResolutionReason, ResolutionNote: values.ResolutionNote,
		AssigneeUserID: values.AssigneeUserID, ReporterUserID: &principalID, ReviewerUserID: values.ReviewerUserID,
		SourceExecutionID: values.SourceExecutionID, SourceTestID: values.SourceTestID, StatusChangedAt: now,
	}
	applyStatusTimestamps(&row, "", row.Status, principalID, now)
	if err := tx.Create(&row).Error; err != nil {
		return quality.BugSnapshot{}, err
	}
	row.BugKey = fmt.Sprintf("BUG-%d", row.ID)
	if updated := tx.Model(&QualityBug{}).Where("id = ? AND workspace_id = ?", row.ID, workspaceID).Update("bug_key", row.BugKey); updated.Error != nil || updated.RowsAffected != 1 {
		if updated.Error != nil {
			return quality.BugSnapshot{}, updated.Error
		}
		return quality.BugSnapshot{}, quality.ErrBugMutationConflict
	}
	if err := tx.Create(&QualityBugActivity{WorkspaceID: workspaceID, QualityBugID: row.ID, UserID: principalID, Action: "created", Summary: "创建质量 Bug"}).Error; err != nil {
		return quality.BugSnapshot{}, err
	}
	return bugReadSnapshot(row), nil
}

func applyBugUpdate(
	ctx context.Context, tx *gorm.DB, resolver requirement.MutationScopeResolver, command quality.BugMutation,
) (quality.BugSnapshot, quality.BugSnapshot, error) {
	row, err := lockBug(tx, command.WorkspaceID, command.ID)
	if err != nil {
		return quality.BugSnapshot{}, quality.BugSnapshot{}, err
	}
	before := bugReadSnapshot(row)
	currentScope, err := resolver.ResolveMutationScope(ctx, command.WorkspaceID, row.RequirementID)
	if err != nil {
		return quality.BugSnapshot{}, quality.BugSnapshot{}, normalizeQualityMutationError(err)
	}
	if !bugMatchesScope(row, currentScope) {
		return quality.BugSnapshot{}, quality.BugSnapshot{}, quality.ErrBugMutationConflict
	}
	values := command.Values
	if command.StatusTransition && (!values.Fields["status"] || values.Status == row.Status) {
		return quality.BugSnapshot{}, quality.BugSnapshot{}, quality.ErrBugMutationConflict
	}
	updates := bugMutationUpdates(values)
	effectiveStatus, effectiveReason := row.Status, row.ResolutionReason
	if value, changed := updates["status"].(string); changed {
		effectiveStatus = value
	}
	if value, changed := updates["resolution_reason"].(string); changed {
		effectiveReason = value
	}
	canonicalReason := quality.NormalizeBugResolutionReason(effectiveReason)
	if (effectiveStatus == "review" || effectiveStatus == "done") &&
		(canonicalReason == "" || !quality.ValidBugResolutionReason(canonicalReason)) {
		return quality.BugSnapshot{}, quality.BugSnapshot{}, quality.ErrBugMutationInvalid
	}
	if values.Fields["requirement_id"] && values.RequirementID != row.RequirementID {
		target, resolveErr := resolver.ResolveMutationScope(ctx, command.WorkspaceID, values.RequirementID)
		if resolveErr != nil {
			return quality.BugSnapshot{}, quality.BugSnapshot{}, normalizeQualityMutationError(resolveErr)
		}
		updates["team_id"], updates["project_set_id"], updates["project_id"], updates["requirement_id"] =
			target.TeamID, target.ProjectSetID, target.PlanUnitID, target.RequirementID
	}
	if status, changed := updates["status"].(string); changed && status != row.Status {
		applyStatusUpdate(&row, updates, status, command.PrincipalID, time.Now().UTC())
	}
	if len(updates) > 0 {
		updated := tx.Model(&QualityBug{}).Where("id = ? AND workspace_id = ?", row.ID, command.WorkspaceID).Updates(updates)
		if updated.Error != nil {
			return quality.BugSnapshot{}, quality.BugSnapshot{}, updated.Error
		}
		if updated.RowsAffected != 1 {
			return quality.BugSnapshot{}, quality.BugSnapshot{}, quality.ErrBugMutationConflict
		}
	}
	if err := tx.Where("id = ? AND workspace_id = ?", row.ID, command.WorkspaceID).First(&row).Error; err != nil {
		return quality.BugSnapshot{}, quality.BugSnapshot{}, normalizeQualityMutationError(err)
	}
	if err := tx.Create(bugMutationActivity(before, row, command)).Error; err != nil {
		return quality.BugSnapshot{}, quality.BugSnapshot{}, err
	}
	return bugReadSnapshot(row), before, nil
}

func applyBugDelete(ctx context.Context, tx *gorm.DB, resolver requirement.MutationScopeResolver, workspaceID, id uint) error {
	row, err := lockBug(tx, workspaceID, id)
	if err != nil {
		return err
	}
	scope, err := resolver.ResolveMutationScope(ctx, workspaceID, row.RequirementID)
	if err != nil {
		return normalizeQualityMutationError(err)
	}
	if !bugMatchesScope(row, scope) {
		return quality.ErrBugMutationConflict
	}
	if err := tx.Where("workspace_id = ? AND quality_bug_id = ?", workspaceID, id).Delete(&QualityBugComment{}).Error; err != nil {
		return err
	}
	if err := tx.Where("workspace_id = ? AND quality_bug_id = ?", workspaceID, id).Delete(&QualityBugActivity{}).Error; err != nil {
		return err
	}
	deleted := tx.Where("workspace_id = ? AND id = ?", workspaceID, id).Delete(&QualityBug{})
	if deleted.Error != nil {
		return deleted.Error
	}
	if deleted.RowsAffected != 1 {
		return quality.ErrBugMutationConflict
	}
	return nil
}

func lockBug(tx *gorm.DB, workspaceID, id uint) (QualityBug, error) {
	var row QualityBug
	err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("workspace_id = ? AND id = ?", workspaceID, id).First(&row).Error
	return row, normalizeQualityMutationError(err)
}

func bugMatchesScope(row QualityBug, scope requirement.MutationScope) bool {
	return row.WorkspaceID == scope.WorkspaceID && row.TeamID == scope.TeamID && row.ProjectSetID == scope.ProjectSetID &&
		row.ProjectID == scope.PlanUnitID && row.RequirementID == scope.RequirementID
}

func bugMutationUpdates(values quality.BugMutationValues) map[string]any {
	updates := map[string]any{}
	if values.Fields["reproduction_materials"] {
		updates["reproduction_materials_json"] = quality.EncodeMaterials(values.ReproductionMaterials)
	}
	if values.Fields["resolution_materials"] {
		updates["resolution_materials_json"] = quality.EncodeMaterials(values.ResolutionMaterials)
	}
	strings := map[string]string{
		"title": values.Title, "status": values.Status, "priority": values.Priority, "bug_type": values.BugType,
		"defect_domain": values.DefectDomain,
		"severity_key":  values.SeverityKey, "found_environment": values.FoundEnvironment,
		"operating_system": values.OperatingSystem, "browser": values.Browser, "found_version": values.FoundVersion,
		"reproduction_steps": values.ReproductionSteps, "expected_result": values.ExpectedResult,
		"actual_result": values.ActualResult, "resolution_reason": values.ResolutionReason, "resolution_note": values.ResolutionNote,
	}
	for field, value := range strings {
		if values.Fields[field] {
			updates[field] = value
		}
	}
	pointers := map[string]any{
		"due_at": values.DueAt, "assignee_user_id": values.AssigneeUserID, "reviewer_user_id": values.ReviewerUserID,
		"source_execution_id": values.SourceExecutionID, "source_test_id": values.SourceTestID,
	}
	for field, value := range pointers {
		if values.Fields[field] {
			updates[field] = value
		}
	}
	return updates
}

func applyStatusTimestamps(row *QualityBug, oldStatus, status string, principalID uint, now time.Time) {
	updates := map[string]any{}
	applyStatusUpdate(row, updates, status, principalID, now)
	if value, ok := updates["resolved_at"].(*time.Time); ok {
		row.ResolvedAt = value
	}
	if value, ok := updates["reviewed_at"].(*time.Time); ok {
		row.ReviewedAt = value
	}
	if value, ok := updates["completed_at"].(*time.Time); ok {
		row.CompletedAt = value
	}
	if value, ok := updates["canceled_at"].(*time.Time); ok {
		row.CanceledAt = value
	}
	if value, exists := updates["reviewer_user_id"]; exists {
		row.ReviewerUserID, _ = value.(*uint)
	}
}

func applyStatusUpdate(row *QualityBug, updates map[string]any, status string, principalID uint, now time.Time) {
	updates["status_changed_at"] = now
	switch status {
	case "review":
		updates["resolved_at"], updates["reviewed_at"], updates["completed_at"], updates["canceled_at"] = &now, nil, nil, nil
	case "done":
		updates["reviewed_at"], updates["completed_at"], updates["canceled_at"] = &now, &now, nil
		if row.ReviewerUserID == nil {
			updates["reviewer_user_id"] = &principalID
		}
	case "canceled":
		updates["canceled_at"], updates["completed_at"] = &now, nil
	case "triage":
		updates["resolved_at"], updates["reviewed_at"], updates["completed_at"], updates["canceled_at"] = nil, nil, nil, nil
		updates["resolution_reason"], updates["resolution_note"], updates["reviewer_user_id"] = "", "", nil
	case "todo", "in_progress", "reopened":
		updates["completed_at"], updates["canceled_at"] = nil, nil
	}
}

func normalizeQualityMutationError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, gorm.ErrRecordNotFound) || errors.Is(err, requirement.ErrMutationScopeNotFound) {
		return quality.ErrBugMutationNotFound
	}
	if errors.Is(err, requirement.ErrMutationScopeInvalid) {
		return quality.ErrBugMutationConflict
	}
	return err
}

var _ quality.BugMutationStore = (*MutationRepository)(nil)
