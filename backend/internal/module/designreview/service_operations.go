package designreview

import (
	"context"
	"strings"

	"github.com/google/uuid"

	"github.com/xizhou0606/aurora-pm/backend/internal/generated/accesscatalog"
)

func (service *Service) DeleteDifference(ctx context.Context, workspaceID uint, actor ActorIdentity, reviewID, differenceID string) error {
	if err := service.requireAction(ctx, actor, accesscatalog.ActionDesignReviewsDifferencesManage); err != nil {
		return err
	}
	if _, err := service.GetReview(ctx, workspaceID, actor, reviewID); err != nil {
		return err
	}
	return service.repository.DeleteDifference(ctx, workspaceID, reviewID, differenceID, actor.FactUserID)
}

func (service *Service) AbandonAnalysis(ctx context.Context, workspaceID uint, actor ActorIdentity, reviewID string) error {
	if err := service.requireAction(ctx, actor, accesscatalog.ActionDesignReviewsAnalysisManage); err != nil {
		return err
	}
	detail, err := service.GetReview(ctx, workspaceID, actor, reviewID)
	if err != nil {
		return err
	}
	if detail.CreatedByUserID != actor.FactUserID {
		return ErrCreatorOnly
	}
	if detail.SourceReviewID == nil {
		return ErrConflict
	}
	return service.repository.AbandonAnalysis(ctx, workspaceID, reviewID, actor.FactUserID)
}

func (service *Service) CreateDeepReview(ctx context.Context, workspaceID uint, actor ActorIdentity, reviewID string, design, implementation Region, focus string, retryOfID *string) (DeepReviewJob, error) {
	capabilities := service.Capabilities(ctx, actor)
	if !capabilities.CanManageAnalysis {
		return DeepReviewJob{}, ErrForbidden
	}
	if !capabilities.AnalysisAvailable {
		return DeepReviewJob{}, ErrUnavailable
	}
	if !design.Valid() || !implementation.Valid() || len([]rune(focus)) > 500 || retryOfID != nil {
		return DeepReviewJob{}, ErrInvalid
	}
	detail, err := service.GetReview(ctx, workspaceID, actor, reviewID)
	if err != nil {
		return DeepReviewJob{}, err
	}
	if detail.Status != "completed" || detail.DesignWidth <= 0 || detail.DesignHeight <= 0 || detail.ViewportWidth <= 0 || detail.ViewportHeight <= 0 {
		return DeepReviewJob{}, ErrConflict
	}
	if design.Width*float64(detail.DesignWidth) < 16 || design.Height*float64(detail.DesignHeight) < 16 ||
		implementation.Width*float64(detail.ViewportWidth) < 16 || implementation.Height*float64(detail.ViewportHeight) < 16 {
		return DeepReviewJob{}, ErrInvalid
	}
	return service.repository.CreateDeepJob(ctx, DeepJobRecord{ID: uuid.NewString(), WorkspaceID: workspaceID, ReviewID: reviewID, CreatedByUserID: actor.FactUserID, DesignRegion: design, ImplementationRegion: implementation, Focus: strings.TrimSpace(focus), RetryOfID: retryOfID})
}

func (service *Service) TransitionDeepReview(ctx context.Context, workspaceID uint, actor ActorIdentity, reviewID, jobID, operation string) error {
	if err := service.requireAction(ctx, actor, accesscatalog.ActionDesignReviewsAnalysisManage); err != nil {
		return err
	}
	if _, err := service.GetReview(ctx, workspaceID, actor, reviewID); err != nil {
		return err
	}
	if operation != "cancel" && operation != "retry" {
		return ErrInvalid
	}
	return service.repository.TransitionDeepJob(ctx, workspaceID, reviewID, jobID, actor.FactUserID, operation)
}

func (service *Service) ResolveRecheckMatch(ctx context.Context, workspaceID uint, actor ActorIdentity, reviewID, matchID, resolution string) error {
	if err := service.requireAction(ctx, actor, accesscatalog.ActionDesignReviewsDifferencesManage); err != nil {
		return err
	}
	if uuid.Validate(matchID) != nil || (resolution != "inherit" && resolution != "new") {
		return ErrInvalid
	}
	if _, err := service.GetReview(ctx, workspaceID, actor, reviewID); err != nil {
		return err
	}
	return service.repository.ResolveRecheckMatch(ctx, workspaceID, reviewID, matchID, actor.FactUserID, resolution)
}

func (service *Service) OverrideRecheckVerification(ctx context.Context, workspaceID uint, actor ActorIdentity, reviewID, verificationID string, status *string) error {
	if err := service.requireAction(ctx, actor, accesscatalog.ActionDesignReviewsDifferencesManage); err != nil {
		return err
	}
	if uuid.Validate(verificationID) != nil || (status != nil && !validHistoricalStatus(*status)) {
		return ErrInvalid
	}
	if _, err := service.GetReview(ctx, workspaceID, actor, reviewID); err != nil {
		return err
	}
	return service.repository.OverrideRecheckVerification(ctx, workspaceID, reviewID, verificationID, actor.FactUserID, status)
}
