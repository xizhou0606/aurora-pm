package designreviewgorm

import (
	"context"
	"encoding/json"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	designreview "github.com/xizhou0606/aurora-pm/backend/internal/module/designreview"
)

func (repository *Repository) CreateDeepJob(ctx context.Context, input designreview.DeepJobRecord) (designreview.DeepReviewJob, error) {
	now := repository.now()
	row := DeepJob{ID: input.ID, WorkspaceID: input.WorkspaceID, ReviewID: input.ReviewID, CreatedByUserID: input.CreatedByUserID, RetryOfID: input.RetryOfID, Status: "pending", DesignRegion: jsonBytes(input.DesignRegion), ImplementationRegion: jsonBytes(input.ImplementationRegion), Focus: input.Focus, AvailableAt: now, CreatedAt: now, UpdatedAt: now}
	err := repository.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var review Review
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("workspace_id = ? AND id = ? AND deleting_at IS NULL", input.WorkspaceID, input.ReviewID).First(&review).Error; err != nil {
			return mapNotFound(err)
		}
		if review.Status != "completed" || input.RetryOfID != nil {
			return designreview.ErrConflict
		}
		if err := ensureReviewMutable(tx, review); err != nil {
			return err
		}
		if err := ensureDeepJobCapacity(tx, input.WorkspaceID, input.ReviewID); err != nil {
			return err
		}
		if err := tx.Create(&row).Error; err != nil {
			return err
		}
		return appendAudit(tx, input.WorkspaceID, input.CreatedByUserID, "design_review_deep_job", input.ID, "created", review.ProjectSetIDSnapshot)
	})
	return deepJobFromRecord(row), err
}

func (repository *Repository) ListDeepJobs(ctx context.Context, workspaceID uint, reviewID string) ([]designreview.DeepReviewJob, error) {
	var rows []DeepJob
	if err := repository.db.WithContext(ctx).Where("workspace_id = ? AND review_id = ?", workspaceID, reviewID).Order("created_at DESC, id DESC").Find(&rows).Error; err != nil {
		return nil, err
	}
	result := make([]designreview.DeepReviewJob, 0, len(rows))
	for _, row := range rows {
		result = append(result, deepJobFromRecord(row))
	}
	return result, nil
}

func deepJobFromRecord(row DeepJob) designreview.DeepReviewJob {
	var design, implementation designreview.Region
	_ = json.Unmarshal(row.DesignRegion, &design)
	_ = json.Unmarshal(row.ImplementationRegion, &implementation)
	return designreview.DeepReviewJob{
		ID: row.ID, ReviewID: row.ReviewID, RetryOfID: row.RetryOfID, Status: row.Status,
		CreatedByUserID: row.CreatedByUserID, DesignRegion: design, ImplementationRegion: implementation,
		Focus: row.Focus, Attempts: row.Attempts, Model: row.Model, ReasoningEffort: row.ReasoningEffort,
		PromptVersion: row.PromptVersion, ResponseID: row.ResponseID, DurationMS: row.DurationMS,
		GeneratedDifferenceCount: row.GeneratedDifferenceCount, LastError: row.LastError,
		StartedAt: row.StartedAt, CompletedAt: row.CompletedAt, CreatedAt: row.CreatedAt,
	}
}

func (repository *Repository) TransitionDeepJob(ctx context.Context, workspaceID uint, reviewID, jobID string, actorUserID uint, operation string) error {
	return repository.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var row DeepJob
		if err := tx.Where("workspace_id = ? AND review_id = ? AND id = ?", workspaceID, reviewID, jobID).First(&row).Error; err != nil {
			return mapNotFound(err)
		}
		var review Review
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("workspace_id = ? AND id = ? AND deleting_at IS NULL", workspaceID, reviewID).First(&review).Error; err != nil {
			return mapNotFound(err)
		}
		if row.CreatedByUserID != actorUserID && review.CreatedByUserID != actorUserID {
			return designreview.ErrCreatorOnly
		}
		now := repository.now()
		switch operation {
		case "cancel":
			updated := tx.Model(&row).Where("status IN ?", []string{"pending", "running"}).Updates(map[string]any{"cancel_requested": true, "status": "cancelled", "completed_at": now, "updated_at": now})
			if updated.Error != nil {
				return updated.Error
			}
			if updated.RowsAffected != 1 {
				return designreview.ErrConflict
			}
			return appendAudit(tx, workspaceID, actorUserID, "design_review_deep_job", row.ID, "cancelled", review.ProjectSetIDSnapshot)
		case "retry":
			if row.Status != "failed" && row.Status != "cancelled" {
				return designreview.ErrConflict
			}
			if review.Status != "completed" {
				return designreview.ErrConflict
			}
			if err := ensureReviewMutable(tx, review); err != nil {
				return err
			}
			if err := ensureDeepJobCapacity(tx, workspaceID, reviewID); err != nil {
				return err
			}
			retry := DeepJob{ID: newID(), WorkspaceID: row.WorkspaceID, ReviewID: row.ReviewID, CreatedByUserID: actorUserID, RetryOfID: &row.ID, Status: "pending", DesignRegion: row.DesignRegion, ImplementationRegion: row.ImplementationRegion, Focus: row.Focus, AvailableAt: now, CreatedAt: now, UpdatedAt: now}
			if err := tx.Create(&retry).Error; err != nil {
				return err
			}
			return appendAudit(tx, workspaceID, actorUserID, "design_review_deep_job", retry.ID, "retried", review.ProjectSetIDSnapshot)
		default:
			return designreview.ErrInvalid
		}
	})
}

type ClaimedJob struct {
	ID          string
	WorkspaceID uint
	ReviewID    string
	Kind        string
	Attempts    int
	Payload     json.RawMessage
}

func (repository *Repository) ClaimJob(ctx context.Context, owner string, leaseDuration time.Duration) (*ClaimedJob, error) {
	var claimed *ClaimedJob
	err := repository.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		now := repository.now()
		leaseUntil := now.Add(leaseDuration)
		locking := func(query *gorm.DB) *gorm.DB {
			if tx.Dialector.Name() == "postgres" {
				return query.Clauses(clause.Locking{Strength: "UPDATE", Options: "SKIP LOCKED"})
			}
			return query
		}
		if err := expireExhaustedJobs(tx, now); err != nil {
			return err
		}
		var analysis AnalysisJob
		query := locking(tx).Where("attempts < 3 AND ((status = 'pending' AND available_at <= ?) OR (status = 'running' AND lease_until < ?))", now, now).Order("available_at, id").Limit(1).Find(&analysis)
		if query.Error != nil {
			return query.Error
		}
		if query.RowsAffected == 1 {
			if err := tx.Model(&analysis).Updates(map[string]any{"status": "running", "attempts": gorm.Expr("attempts + 1"), "lease_owner": owner, "lease_until": leaseUntil, "updated_at": now}).Error; err != nil {
				return err
			}
			_ = tx.Model(&Review{}).Where("workspace_id = ? AND id = ?", analysis.WorkspaceID, analysis.ReviewID).Updates(map[string]any{"status": "analyzing", "analysis_attempts": gorm.Expr("analysis_attempts + 1"), "updated_at": now}).Error
			claimed = &ClaimedJob{ID: analysis.ID, WorkspaceID: analysis.WorkspaceID, ReviewID: analysis.ReviewID, Kind: "full", Attempts: analysis.Attempts + 1}
			return nil
		}
		var deep DeepJob
		query = locking(tx).Where("attempts < 3 AND ((status = 'pending' AND available_at <= ?) OR (status = 'running' AND lease_until < ?))", now, now).Order("available_at, id").Limit(1).Find(&deep)
		if query.Error != nil || query.RowsAffected == 0 {
			return query.Error
		}
		if err := tx.Model(&deep).Updates(map[string]any{"status": "running", "attempts": gorm.Expr("attempts + 1"), "lease_owner": owner, "lease_until": leaseUntil, "started_at": now, "updated_at": now}).Error; err != nil {
			return err
		}
		payload, _ := json.Marshal(map[string]any{"design_region": json.RawMessage(deep.DesignRegion), "implementation_region": json.RawMessage(deep.ImplementationRegion), "focus": deep.Focus})
		claimed = &ClaimedJob{ID: deep.ID, WorkspaceID: deep.WorkspaceID, ReviewID: deep.ReviewID, Kind: "deep", Attempts: deep.Attempts + 1, Payload: payload}
		return nil
	})
	return claimed, err
}

func expireExhaustedJobs(tx *gorm.DB, now time.Time) error {
	var analysis []AnalysisJob
	if err := tx.Where("status = 'running' AND attempts >= 3 AND lease_until < ?", now).Find(&analysis).Error; err != nil {
		return err
	}
	message := "analysis lease expired after maximum attempts"
	for _, job := range analysis {
		updated := tx.Model(&AnalysisJob{}).Where("id = ? AND workspace_id = ? AND status = 'running'", job.ID, job.WorkspaceID).
			Updates(map[string]any{"status": "failed", "lease_owner": nil, "lease_until": nil, "last_error": message, "updated_at": now})
		if updated.Error != nil {
			return updated.Error
		}
		if updated.RowsAffected == 1 {
			if err := tx.Model(&Review{}).Where("workspace_id = ? AND id = ?", job.WorkspaceID, job.ReviewID).
				Updates(map[string]any{"status": "failed", "last_error_stage": "analysis", "last_error": message, "updated_at": now}).Error; err != nil {
				return err
			}
		}
	}
	return tx.Model(&DeepJob{}).Where("status = 'running' AND attempts >= 3 AND lease_until < ?", now).
		Updates(map[string]any{"status": "failed", "lease_owner": nil, "lease_until": nil, "last_error": message, "completed_at": now, "updated_at": now}).Error
}

func (repository *Repository) CompleteJob(ctx context.Context, job ClaimedJob, summary string, modelCoverageComplete bool, differences []designreview.AnalyzedDifference, metadata designreview.AnalysisRunMetadata) error {
	metadataUpdates, err := completionMetadata(metadata)
	if err != nil {
		return err
	}
	return repository.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		now := repository.now()
		var review Review
		if err := tx.Where("workspace_id = ? AND id = ?", job.WorkspaceID, job.ReviewID).First(&review).Error; err != nil {
			return mapNotFound(err)
		}
		if review.ChainRootID == nil {
			return designreview.ErrConflict
		}
		for _, analyzed := range differences {
			input := analyzed.Input
			if !input.Valid() {
				return designreview.ErrInvalid
			}
			var ordinal int
			if err := tx.Model(&Difference{}).Where("workspace_id = ? AND review_id = ?", job.WorkspaceID, job.ReviewID).Select("COALESCE(MAX(ordinal), 0)").Scan(&ordinal).Error; err != nil {
				return err
			}
			differenceID, lineageID := newID(), newID()
			if err := tx.Create(&DifferenceLineage{ID: lineageID, WorkspaceID: job.WorkspaceID, ChainRootReviewID: *review.ChainRootID, VerificationStatus: "pending", CreatedAt: now, UpdatedAt: now}).Error; err != nil {
				return err
			}
			difference := Difference{ID: differenceID, WorkspaceID: job.WorkspaceID, ReviewID: job.ReviewID, LineageID: &lineageID, LineageRootDifferenceID: &differenceID, Ordinal: ordinal + 1, Source: "ai", AnalysisKind: job.Kind, Category: input.Category, Severity: input.Severity, Title: input.Title, Description: input.Description, Recommendation: input.Recommendation, Confidence: input.Confidence, DesignRegion: jsonBytes(input.DesignRegion), ImplementationRegion: jsonBytes(input.ImplementationRegion), EvidenceBasis: "visual", EvidenceNotes: input.EvidenceNotes, VerificationStatus: "pending", ResolutionStatus: "unresolved", DesignIdentityAnchor: jsonBytes(analyzed.DesignAnchor), ImplementationIdentityAnchor: jsonBytes(analyzed.ImplementationAnchor), AIOriginal: jsonBytes(input)}
			if job.Kind == "deep" {
				difference.DeepJobID = &job.ID
			}
			if err := tx.Create(&difference).Error; err != nil {
				return err
			}
		}
		if job.Kind == "full" {
			updated := tx.Model(&AnalysisJob{}).Where("id = ? AND workspace_id = ? AND status = 'running'", job.ID, job.WorkspaceID).Updates(map[string]any{"status": "succeeded", "lease_owner": nil, "lease_until": nil, "updated_at": now})
			if updated.Error != nil || updated.RowsAffected != 1 {
				return designreview.ErrConflict
			}
			reviewUpdates := map[string]any{"status": "completed", "analysis_summary": summary, "model_coverage_complete": modelCoverageComplete, "last_error_stage": nil, "last_error": nil, "completed_at": now, "updated_at": now}
			copyCompletionMetadata(reviewUpdates, metadataUpdates, "analysis_duration_ms")
			if err := tx.Model(&Review{}).Where("workspace_id = ? AND id = ?", job.WorkspaceID, job.ReviewID).Updates(reviewUpdates).Error; err != nil {
				return err
			}
		} else {
			deepUpdates := map[string]any{"status": "succeeded", "generated_difference_count": len(differences), "lease_owner": nil, "lease_until": nil, "completed_at": now, "updated_at": now}
			copyCompletionMetadata(deepUpdates, metadataUpdates, "duration_ms")
			updated := tx.Model(&DeepJob{}).Where("id = ? AND workspace_id = ? AND status = 'running'", job.ID, job.WorkspaceID).Updates(deepUpdates)
			if updated.Error != nil || updated.RowsAffected != 1 {
				return designreview.ErrConflict
			}
		}
		return refreshConclusion(tx, job.WorkspaceID, job.ReviewID)
	})
}

func (repository *Repository) FailJob(ctx context.Context, job ClaimedJob, cause error, retryable bool) error {
	now := repository.now()
	message := cause.Error()
	status := "failed"
	availableAt := now
	if retryable && job.Attempts < 3 {
		status = "pending"
		availableAt = now.Add(time.Duration(job.Attempts*job.Attempts) * time.Second)
	}
	model := &AnalysisJob{}
	if job.Kind == "deep" {
		model = nil
		updates := map[string]any{"status": status, "available_at": availableAt, "lease_owner": nil, "lease_until": nil, "last_error": message, "updated_at": now}
		if status == "failed" {
			updates["completed_at"] = now
		}
		return repository.db.WithContext(ctx).Model(&DeepJob{}).Where("id = ? AND workspace_id = ? AND status = 'running'", job.ID, job.WorkspaceID).Updates(updates).Error
	}
	return repository.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		updated := tx.Model(model).Where("id = ? AND workspace_id = ? AND status = 'running'", job.ID, job.WorkspaceID).Updates(map[string]any{"status": status, "available_at": availableAt, "lease_owner": nil, "lease_until": nil, "last_error": message, "updated_at": now})
		if updated.Error != nil || updated.RowsAffected == 0 {
			return updated.Error
		}
		if status == "failed" {
			return tx.Model(&Review{}).Where("workspace_id = ? AND id = ? AND status = 'analyzing'", job.WorkspaceID, job.ReviewID).Updates(map[string]any{"status": "failed", "last_error_stage": "analysis", "last_error": message, "updated_at": now}).Error
		}
		return nil
	})
}
