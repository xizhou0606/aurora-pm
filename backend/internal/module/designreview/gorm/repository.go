package designreviewgorm

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"gorm.io/gorm"

	designreview "github.com/xizhou0606/aurora-pm/backend/internal/module/designreview"
)

type Repository struct {
	db  *gorm.DB
	now func() time.Time
}

func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db, now: func() time.Time { return time.Now().UTC() }}
}

func (repository *Repository) ListRequirementOptions(ctx context.Context, workspaceID uint) ([]designreview.RequirementOption, error) {
	var rows []designreview.RequirementOption
	err := repository.db.WithContext(ctx).Table("requirements r").
		Select("r.id, r.title, r.project_new_id AS project_set_id, ps.name AS project_set_name, r.version_task_new_id AS plan_unit_id, pu.title AS plan_unit_title").
		Joins("JOIN project_sets ps ON ps.id = r.project_new_id AND ps.workspace_id = r.workspace_id").
		Joins("JOIN plan_units pu ON pu.id = r.version_task_new_id AND pu.workspace_id = r.workspace_id AND pu.project_new_id = r.project_new_id").
		Where("r.workspace_id = ? AND r.version_task_new_id IS NOT NULL AND COALESCE(r.mutation_state, '') = ''", workspaceID).
		Order("ps.name, pu.title, r.title, r.id").Scan(&rows).Error
	return rows, err
}

type reviewSummaryRow struct {
	Review
	DifferenceCount           int64
	UnresolvedDifferenceCount int64
}

func (repository *Repository) ListReviews(ctx context.Context, workspaceID uint, filter designreview.ReviewListFilter) ([]designreview.DesignReviewSummary, int64, error) {
	query := repository.db.WithContext(ctx).Table("design_reviews dr").
		Select("dr.*, (SELECT COUNT(*) FROM design_review_differences d WHERE d.workspace_id = dr.workspace_id AND d.review_id = dr.id) AS difference_count, (SELECT COUNT(*) FROM design_review_differences d WHERE d.workspace_id = dr.workspace_id AND d.review_id = dr.id AND d.verification_status <> 'false_positive' AND d.resolution_status = 'unresolved') AS unresolved_difference_count").
		Where("dr.workspace_id = ?", workspaceID).
		Where("NOT EXISTS (SELECT 1 FROM design_reviews newer WHERE newer.workspace_id = dr.workspace_id AND newer.chain_root_id = dr.chain_root_id AND newer.round_number > dr.round_number AND newer.deleting_at IS NULL AND newer.abandoned_at IS NULL)").
		Where("dr.deleting_at IS NULL AND dr.abandoned_at IS NULL")
	if keyword := strings.TrimSpace(filter.Keyword); keyword != "" {
		like := "%" + keyword + "%"
		query = query.Where("dr.requirement_title_snapshot LIKE ? OR dr.project_set_name_snapshot LIKE ? OR dr.plan_unit_title_snapshot LIKE ? OR dr.figma_node_name LIKE ?", like, like, like, like)
	}
	if filter.ProjectSetID > 0 {
		query = query.Where("dr.project_set_id_snapshot = ?", filter.ProjectSetID)
	}
	if filter.PlanUnitID > 0 {
		query = query.Where("dr.plan_unit_id_snapshot = ?", filter.PlanUnitID)
	}
	if filter.RequirementID > 0 {
		query = query.Where("dr.requirement_id = ?", filter.RequirementID)
	}
	if strings.TrimSpace(filter.Status) != "" {
		query = query.Where("dr.status = ?", strings.TrimSpace(filter.Status))
	}
	if strings.TrimSpace(filter.Conclusion) != "" {
		query = query.Where("dr.conclusion = ?", strings.TrimSpace(filter.Conclusion))
	}
	var rows []reviewSummaryRow
	if err := query.Order("dr.created_at DESC, dr.id DESC").Scan(&rows).Error; err != nil {
		return nil, 0, err
	}
	items := make([]designreview.DesignReviewSummary, 0, len(rows))
	for _, row := range rows {
		item := summaryFromRecord(row.Review)
		item.DifferenceCount = row.DifferenceCount
		item.UnresolvedDifferenceCount = row.UnresolvedDifferenceCount
		items = append(items, item)
	}
	return items, int64(len(items)), nil
}

func (repository *Repository) GetReview(ctx context.Context, workspaceID uint, reviewID string) (designreview.DesignReviewDetail, error) {
	var row Review
	if err := repository.db.WithContext(ctx).Where("workspace_id = ? AND id = ? AND deleting_at IS NULL", workspaceID, reviewID).First(&row).Error; err != nil {
		return designreview.DesignReviewDetail{}, mapNotFound(err)
	}
	detail := detailFromRecord(row)
	var assets []Asset
	if err := repository.db.WithContext(ctx).Where("workspace_id = ? AND review_id = ?", workspaceID, reviewID).Order("created_at, id").Find(&assets).Error; err != nil {
		return designreview.DesignReviewDetail{}, err
	}
	for _, asset := range assets {
		detail.Assets = append(detail.Assets, designreview.EvidenceAsset{ID: asset.ID, Kind: asset.Kind, ContentType: asset.ContentType, ByteSize: asset.ByteSize, SHA256: asset.SHA256, Width: asset.Width, Height: asset.Height, CreatedAt: asset.CreatedAt})
	}
	var differences []Difference
	if err := repository.db.WithContext(ctx).Where("workspace_id = ? AND review_id = ?", workspaceID, reviewID).Order("ordinal, id").Find(&differences).Error; err != nil {
		return designreview.DesignReviewDetail{}, err
	}
	for _, difference := range differences {
		detail.Differences = append(detail.Differences, differenceFromRecord(difference))
	}
	rounds, err := repository.ListRounds(ctx, workspaceID, detail.ChainRootID)
	if err != nil {
		return designreview.DesignReviewDetail{}, err
	}
	detail.Rounds = rounds
	detail.DeepJobs, err = repository.ListDeepJobs(ctx, workspaceID, reviewID)
	if err != nil {
		return designreview.DesignReviewDetail{}, err
	}
	if err := repository.loadRecheckState(ctx, workspaceID, &detail); err != nil {
		return designreview.DesignReviewDetail{}, err
	}
	detail.DifferenceCount = int64(len(detail.Differences))
	for _, difference := range detail.Differences {
		if difference.VerificationStatus != "false_positive" && difference.ResolutionStatus == "unresolved" {
			detail.UnresolvedDifferenceCount++
		}
	}
	return detail, nil
}

func (repository *Repository) GetReviewForUpdate(ctx context.Context, workspaceID uint, reviewID string) (designreview.DesignReviewDetail, error) {
	return repository.GetReview(ctx, workspaceID, reviewID)
}

func (repository *Repository) CreateReview(ctx context.Context, input designreview.CreateReviewRecord) (designreview.DesignReviewDetail, error) {
	row := recordFromCreate(input)
	err := repository.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&row).Error; err != nil {
			return err
		}
		for _, asset := range input.InitialAssets {
			if err := createInitialAsset(tx, asset); err != nil {
				return err
			}
		}
		return appendAudit(tx, input.WorkspaceID, input.CreatedByUserID, "design_review", input.ID, "created", input.Requirement.ProjectSetID)
	})
	if err != nil {
		return designreview.DesignReviewDetail{}, err
	}
	return repository.GetReview(ctx, input.WorkspaceID, input.ID)
}

func (repository *Repository) CreateRecheck(ctx context.Context, input designreview.RecheckRecord) (designreview.DesignReviewDetail, error) {
	err := repository.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var source Review
		if err := tx.Where("workspace_id = ? AND id = ?", input.WorkspaceID, input.SourceReviewID).First(&source).Error; err != nil {
			return mapNotFound(err)
		}
		var newer int64
		if err := tx.Model(&Review{}).Where("workspace_id = ? AND chain_root_id = ? AND round_number > ? AND deleting_at IS NULL AND abandoned_at IS NULL", input.WorkspaceID, input.ChainRootID, source.RoundNumber).Count(&newer).Error; err != nil {
			return err
		}
		if newer > 0 {
			return designreview.ErrConflict
		}
		targetRaw, _ := json.Marshal(input.Input.ImplementationTarget)
		row := Review{
			ID: input.ID, WorkspaceID: input.WorkspaceID, RequirementID: source.RequirementID,
			RequirementTitleSnapshot: source.RequirementTitleSnapshot, ProjectSetIDSnapshot: source.ProjectSetIDSnapshot,
			ProjectSetNameSnapshot: source.ProjectSetNameSnapshot, PlanUnitIDSnapshot: source.PlanUnitIDSnapshot,
			PlanUnitTitleSnapshot: source.PlanUnitTitleSnapshot, ChainRootID: &input.ChainRootID, SourceReviewID: &input.SourceReviewID,
			RoundNumber: input.RoundNumber, CreatedByUserID: input.CreatedByUserID, CreatedByName: input.CreatedByName,
			ImplementationKind: input.Input.ImplementationTarget.Kind, ImplementationTarget: targetRaw,
			FigmaURL: input.Input.FigmaURL, FigmaFileKey: input.FigmaFileKey, FigmaNodeID: input.FigmaNodeID,
			FigmaNodeName: input.FigmaNodeName,
			ViewportWidth: input.Input.ViewportWidth, ViewportHeight: input.Input.ViewportHeight, DevicePixelRatio: input.Input.DevicePixelRatio,
			PageZoom: input.Input.PageZoom, CaptureWarnings: []byte("[]"), AnalysisScope: jsonBytes(input.Input.AnalysisScope),
			MobileTopbar: nonemptyJSON(input.Input.MobileTopbar, `{"enabled":false}`), DOMNormalization: nonemptyJSON(input.Input.DOMNormalization, `{"enabled":false}`), Status: "draft",
		}
		for _, asset := range input.InitialAssets {
			if asset.Kind == "design_normalized" && asset.Width != nil && asset.Height != nil {
				row.DesignWidth, row.DesignHeight = *asset.Width, *asset.Height
			}
			row.DesignStructureAvailable = row.DesignStructureAvailable || asset.Kind == "design_structure"
		}
		if err := tx.Create(&row).Error; err != nil {
			return mapConflict(err)
		}
		for _, asset := range input.InitialAssets {
			if err := createInitialAsset(tx, asset); err != nil {
				return err
			}
		}
		return appendAudit(tx, input.WorkspaceID, input.CreatedByUserID, "design_review", input.ID, "rechecked", source.ProjectSetIDSnapshot)
	})
	if err != nil {
		return designreview.DesignReviewDetail{}, err
	}
	return repository.GetReview(ctx, input.WorkspaceID, input.ID)
}

func (repository *Repository) ListRounds(ctx context.Context, workspaceID uint, chainRootID string) ([]designreview.ReviewRound, error) {
	var rows []Review
	if err := repository.db.WithContext(ctx).Where("workspace_id = ? AND chain_root_id = ? AND deleting_at IS NULL", workspaceID, chainRootID).Order("round_number").Find(&rows).Error; err != nil {
		return nil, err
	}
	result := make([]designreview.ReviewRound, 0, len(rows))
	for _, row := range rows {
		result = append(result, designreview.ReviewRound{ID: row.ID, RoundNumber: row.RoundNumber, Status: row.Status, Conclusion: row.Conclusion, CreatedAt: row.CreatedAt, CompletedAt: row.CompletedAt})
	}
	return result, nil
}

func recordFromCreate(input designreview.CreateReviewRecord) Review {
	targetRaw, _ := json.Marshal(input.ImplementationTarget)
	chainRoot := input.ChainRootID
	row := Review{
		ID: input.ID, WorkspaceID: input.WorkspaceID, RequirementID: &input.Requirement.ID,
		RequirementTitleSnapshot: input.Requirement.Title, ProjectSetIDSnapshot: input.Requirement.ProjectSetID,
		ProjectSetNameSnapshot: input.Requirement.ProjectSetName, PlanUnitIDSnapshot: input.Requirement.PlanUnitID,
		PlanUnitTitleSnapshot: input.Requirement.PlanUnitTitle, ChainRootID: &chainRoot, SourceReviewID: input.SourceReviewID,
		RoundNumber: input.RoundNumber, CreatedByUserID: input.CreatedByUserID, CreatedByName: input.CreatedByName,
		ImplementationKind: input.ImplementationTarget.Kind, ImplementationTarget: targetRaw,
		FigmaURL: input.FigmaURL, FigmaFileKey: input.FigmaFileKey, FigmaNodeID: input.FigmaNodeID, FigmaNodeName: input.FigmaNodeName,
		ViewportWidth: input.ViewportWidth, ViewportHeight: input.ViewportHeight, DevicePixelRatio: input.DevicePixelRatio,
		PageZoom: input.PageZoom, CaptureWarnings: []byte("[]"), AnalysisScope: jsonBytes(input.AnalysisScope),
		MobileTopbar: nonemptyJSON(input.MobileTopbar, `{"enabled":false}`), DOMNormalization: nonemptyJSON(input.DOMNormalization, `{"enabled":false}`), Status: "draft",
	}
	for _, asset := range input.InitialAssets {
		if asset.Kind == "design_normalized" && asset.Width != nil && asset.Height != nil {
			row.DesignWidth, row.DesignHeight = *asset.Width, *asset.Height
		}
		row.DesignStructureAvailable = row.DesignStructureAvailable || asset.Kind == "design_structure"
	}
	if input.ImplementationTarget.Kind == "web" {
		row.ImplementationURL = stringPtr(input.ImplementationTarget.OriginalURL)
		row.ImplementationActualURL = stringPtr(input.ImplementationTarget.ActualURL)
	}
	return row
}

func createInitialAsset(tx *gorm.DB, input designreview.AssetRecord) error {
	row := Asset{ID: input.ID, WorkspaceID: input.WorkspaceID, ReviewID: input.ReviewID, Kind: input.Kind, ObjectKey: input.ObjectKey, ContentType: input.ContentType, ByteSize: input.ByteSize, SHA256: input.SHA256, Width: input.Width, Height: input.Height, CreatedAt: input.CreatedAt}
	if err := tx.Create(&row).Error; err != nil {
		return err
	}
	return appendAudit(tx, input.WorkspaceID, input.ActorUserID, "design_review_asset", input.ID, "uploaded", 0)
}

func summaryFromRecord(row Review) designreview.DesignReviewSummary {
	var target designreview.DesignReviewTarget
	_ = json.Unmarshal(row.ImplementationTarget, &target)
	root := row.ID
	if row.ChainRootID != nil && *row.ChainRootID != "" {
		root = *row.ChainRootID
	}
	return designreview.DesignReviewSummary{
		ID: row.ID, ChainRootID: root, SourceReviewID: row.SourceReviewID, RoundNumber: row.RoundNumber,
		RequirementID: row.RequirementID, RequirementTitle: row.RequirementTitleSnapshot,
		ProjectSetID: row.ProjectSetIDSnapshot, ProjectSetName: row.ProjectSetNameSnapshot,
		PlanUnitID: row.PlanUnitIDSnapshot, PlanUnitTitle: row.PlanUnitTitleSnapshot,
		ImplementationKind: row.ImplementationKind, ImplementationTarget: target, FigmaURL: row.FigmaURL,
		FigmaNodeName: row.FigmaNodeName, Status: row.Status, Conclusion: row.Conclusion,
		CreatedByUserID: row.CreatedByUserID, CreatedByName: row.CreatedByName, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
	}
}

func detailFromRecord(row Review) designreview.DesignReviewDetail {
	var scope designreview.AnalysisScope
	_ = json.Unmarshal(row.AnalysisScope, &scope)
	return designreview.DesignReviewDetail{
		DesignReviewSummary: summaryFromRecord(row), ViewportWidth: row.ViewportWidth, ViewportHeight: row.ViewportHeight,
		DevicePixelRatio: row.DevicePixelRatio, PageZoom: row.PageZoom, DesignWidth: row.DesignWidth, DesignHeight: row.DesignHeight,
		DimensionsMatch: row.DimensionsMatch, DesignStructureAvailable: row.DesignStructureAvailable,
		ImplementationStructureComplete: row.ImplementationStructureComplete, CaptureWarnings: row.CaptureWarnings,
		AnalysisScope: scope, MobileTopbar: row.MobileTopbar, DOMNormalization: row.DOMNormalization,
		AnalysisSummary: row.AnalysisSummary, Model: row.Model, ReasoningEffort: row.ReasoningEffort,
		PromptVersion: row.PromptVersion, ResponseID: row.ResponseID, AnalysisDurationMS: row.AnalysisDurationMS,
		AnalysisAttempts: row.AnalysisAttempts, LastErrorStage: row.LastErrorStage, LastError: row.LastError,
		Assets: []designreview.EvidenceAsset{}, Differences: []designreview.DesignDifference{}, Rounds: []designreview.ReviewRound{},
	}
}

func jsonBytes(value any) []byte { raw, _ := json.Marshal(value); return raw }
func nonemptyJSON(raw []byte, fallback string) []byte {
	if len(raw) == 0 {
		return []byte(fallback)
	}
	return raw
}
func stringPtr(value string) *string {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	return &value
}
func mapNotFound(err error) error {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return designreview.ErrNotFound
	}
	return err
}
func mapConflict(err error) error {
	if err != nil && (strings.Contains(strings.ToLower(err.Error()), "unique") || strings.Contains(strings.ToLower(err.Error()), "duplicate")) {
		return designreview.ErrConflict
	}
	return err
}
