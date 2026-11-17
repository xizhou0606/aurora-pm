package designreview

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/xizhou0606/aurora-pm/backend/internal/generated/accesscatalog"
)

const maxEvidenceBytes = 20 << 20

type Service struct {
	repository   Repository
	projects     ProjectAccess
	actions      EffectiveActionReader
	objects      ObjectStore
	figma        FigmaPreviewer
	normalizer   DOMNormalizer
	availability Availability
	now          func() time.Time
}

func NewService(repository Repository, projects ProjectAccess, actions EffectiveActionReader, objects ObjectStore, figma FigmaPreviewer, normalizer DOMNormalizer, availability Availability) (*Service, error) {
	if repository == nil || projects == nil {
		return nil, ErrUnavailable
	}
	return &Service{repository: repository, projects: projects, actions: actions, objects: objects, figma: figma, normalizer: normalizer, availability: availability, now: func() time.Time { return time.Now().UTC() }}, nil
}

func (service *Service) Capabilities(ctx context.Context, actor ActorIdentity) ReviewCapabilities {
	capabilities := service.environmentCapabilities()
	if service.actions == nil || !actor.Valid() {
		return capabilities
	}
	codes, err := service.actions.EffectiveActionCodes(ctx, actor.PrincipalUserID)
	if err != nil {
		return capabilities
	}
	effective := make(map[string]struct{}, len(codes))
	for _, code := range codes {
		effective[code] = struct{}{}
	}
	has := func(action accesscatalog.ActionCode) bool {
		for _, granting := range accesscatalog.GrantingActions(action) {
			if _, ok := effective[string(granting)]; ok {
				capabilities.AllowedActions = append(capabilities.AllowedActions, string(action))
				return true
			}
		}
		return false
	}
	capabilities.CanView = has(accesscatalog.ActionDesignReviewsReviewsView)
	capabilities.CanCreate = has(accesscatalog.ActionDesignReviewsReviewsCreate)
	capabilities.CanManageDifferences = has(accesscatalog.ActionDesignReviewsDifferencesManage)
	capabilities.CanManageAnalysis = has(accesscatalog.ActionDesignReviewsAnalysisManage)
	capabilities.CanDelete = has(accesscatalog.ActionDesignReviewsReviewsDelete)
	capabilities.CanManageCollectors = has(accesscatalog.ActionDesignReviewsMobileCollectorsManage)
	return capabilities
}

func (service *Service) requireAction(ctx context.Context, actor ActorIdentity, action accesscatalog.ActionCode) error {
	if service.actions == nil || !actor.Valid() {
		return ErrForbidden
	}
	codes, err := service.actions.EffectiveActionCodes(ctx, actor.PrincipalUserID)
	if err != nil {
		return ErrForbidden
	}
	effective := make(map[string]struct{}, len(codes))
	for _, code := range codes {
		effective[code] = struct{}{}
	}
	for _, granting := range accesscatalog.GrantingActions(action) {
		if _, ok := effective[string(granting)]; ok {
			return nil
		}
	}
	return ErrForbidden
}

func (service *Service) environmentCapabilities() ReviewCapabilities {
	reasons := make([]string, 0, 3)
	if !service.availability.ObjectStorage {
		reasons = append(reasons, "对象存储未配置")
	}
	if !service.availability.VisualModel {
		reasons = append(reasons, "设计走查视觉模型未配置")
	}
	if !service.availability.Figma {
		reasons = append(reasons, "Figma 共享凭证未配置")
	}
	return ReviewCapabilities{
		CreationAvailable: len(reasons) == 0, FigmaPreviewAvailable: service.availability.Figma,
		AnalysisAvailable:      service.availability.VisualModel && service.availability.ObjectStorage,
		ObjectStorageAvailable: service.availability.ObjectStorage, UnavailableReasons: reasons,
		AllowedActions: []string{},
	}
}

func (service *Service) ListRequirements(ctx context.Context, workspaceID uint, actor ActorIdentity) ([]RequirementOption, error) {
	if err := service.requireAction(ctx, actor, accesscatalog.ActionDesignReviewsReviewsView); err != nil {
		return nil, err
	}
	items, err := service.repository.ListRequirementOptions(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	out := make([]RequirementOption, 0, len(items))
	for _, item := range items {
		allowed, accessErr := service.projects.CanAccessProject(ctx, workspaceID, item.ProjectSetID, actor.FactUserID)
		if accessErr == nil && allowed {
			out = append(out, item)
		}
	}
	return out, nil
}

func (service *Service) ListReviews(ctx context.Context, workspaceID uint, actor ActorIdentity, filter ReviewListFilter) (ReviewPage, error) {
	if err := service.requireAction(ctx, actor, accesscatalog.ActionDesignReviewsReviewsView); err != nil {
		return ReviewPage{}, err
	}
	if filter.Page < 1 {
		filter.Page = 1
	}
	if filter.PageSize < 1 || filter.PageSize > 100 {
		filter.PageSize = 20
	}
	items, _, err := service.repository.ListReviews(ctx, workspaceID, filter)
	if err != nil {
		return ReviewPage{}, err
	}
	admin, _ := service.projects.IsAccessAdministrator(ctx, workspaceID, actor.FactUserID)
	capabilities := service.Capabilities(ctx, actor)
	visible := make([]DesignReviewSummary, 0, len(items))
	for _, item := range items {
		allowed, accessErr := service.projects.CanAccessProject(ctx, workspaceID, item.ProjectSetID, actor.FactUserID)
		if (accessErr == nil && allowed) || admin {
			item.CanDelete = capabilities.CanDelete && item.RoundNumber == 1 && item.CreatedByUserID == actor.FactUserID
			visible = append(visible, item)
		}
	}
	start := (filter.Page - 1) * filter.PageSize
	if start > len(visible) {
		start = len(visible)
	}
	end := start + filter.PageSize
	if end > len(visible) {
		end = len(visible)
	}
	return ReviewPage{Items: visible[start:end], Page: filter.Page, PageSize: filter.PageSize, Total: int64(len(visible))}, nil
}

func (service *Service) GetReview(ctx context.Context, workspaceID uint, actor ActorIdentity, reviewID string) (DesignReviewDetail, error) {
	if err := service.requireAction(ctx, actor, accesscatalog.ActionDesignReviewsReviewsView); err != nil {
		return DesignReviewDetail{}, err
	}
	if uuid.Validate(reviewID) != nil {
		return DesignReviewDetail{}, ErrInvalid
	}
	detail, err := service.repository.GetReview(ctx, workspaceID, reviewID)
	if err != nil {
		return DesignReviewDetail{}, err
	}
	if err := service.requireProject(ctx, workspaceID, detail.ProjectSetID, actor.FactUserID); err != nil {
		return DesignReviewDetail{}, err
	}
	detail.Capabilities = service.Capabilities(ctx, actor)
	detail.Capabilities.CanDelete = detail.Capabilities.CanDelete && detail.RoundNumber == 1 && detail.CreatedByUserID == actor.FactUserID
	mutable := reviewIsLatestMutableRound(detail)
	detail.Capabilities.CanManageDifferences = detail.Capabilities.CanManageDifferences && mutable
	detail.Capabilities.CanManageAnalysis = detail.Capabilities.CanManageAnalysis && mutable
	for index := range detail.Differences {
		difference := &detail.Differences[index]
		manual := difference.Source == "manual" || difference.Source == "manual_reopen"
		manualCreator := detail.CreatedByUserID == actor.FactUserID || (difference.CreatedByUserID != nil && *difference.CreatedByUserID == actor.FactUserID)
		difference.CanEdit = detail.Capabilities.CanManageDifferences && (!manual || manualCreator)
		difference.CanDelete = detail.Capabilities.CanManageDifferences && manual && manualCreator
		difference.CanVerify = detail.Capabilities.CanManageDifferences
		difference.CanResolve = detail.Capabilities.CanManageDifferences && difference.ResolutionStatus == "unresolved"
		difference.CanReopen = detail.Capabilities.CanManageDifferences && difference.ResolutionStatus == "resolved"
	}
	for index := range detail.DeepJobs {
		job := &detail.DeepJobs[index]
		creator := job.CreatedByUserID == actor.FactUserID || detail.CreatedByUserID == actor.FactUserID
		job.CanRetry = detail.Capabilities.CanManageAnalysis && creator && (job.Status == "failed" || job.Status == "cancelled")
		job.CanCancel = detail.Capabilities.CanManageAnalysis && creator && (job.Status == "pending" || job.Status == "running")
	}
	return detail, nil
}

func reviewIsLatestMutableRound(detail DesignReviewDetail) bool {
	for _, round := range detail.Rounds {
		if round.RoundNumber > detail.RoundNumber && round.Status != "abandoned" {
			return false
		}
	}
	return true
}

func (service *Service) CreateReview(ctx context.Context, workspaceID uint, actor ActorIdentity, userName string, input CreateReviewInput) (DesignReviewDetail, error) {
	capabilities := service.Capabilities(ctx, actor)
	if !capabilities.CanCreate {
		return DesignReviewDetail{}, ErrForbidden
	}
	if !capabilities.CreationAvailable {
		return DesignReviewDetail{}, ErrUnavailable
	}
	requirements, err := service.ListRequirements(ctx, workspaceID, actor)
	if err != nil {
		return DesignReviewDetail{}, err
	}
	var requirement RequirementOption
	for _, item := range requirements {
		if item.ID == input.RequirementID {
			requirement = item
			break
		}
	}
	if requirement.ID == 0 {
		return DesignReviewDetail{}, ErrRequirement
	}
	fileKey, nodeID, err := ParseFigmaNodeURL(input.FigmaURL)
	if err != nil || !input.ImplementationTarget.Valid() || input.ViewportWidth < 240 || input.ViewportWidth > 7680 ||
		input.ViewportHeight < 240 || input.ViewportHeight > 7680 || input.DevicePixelRatio <= 0 || input.DevicePixelRatio > 8 ||
		(input.PageZoom != 0 && input.PageZoom < 0.1) || input.PageZoom > 5 {
		return DesignReviewDetail{}, ErrInvalid
	}
	if _, err := ParseMobileTopbar(input.MobileTopbar, input.ImplementationTarget.Kind); err != nil {
		return DesignReviewDetail{}, err
	}
	if input.PageZoom == 0 {
		input.PageZoom = 1
	}
	domNormalization, err := ParseDOMNormalizationConfig(input.DOMNormalization, input.ImplementationTarget.Kind)
	if err != nil {
		return DesignReviewDetail{}, err
	}
	input.DOMNormalization = domNormalization
	if input.SourceReviewID != nil {
		return service.CreateRecheck(ctx, workspaceID, actor, userName, strings.TrimSpace(*input.SourceReviewID), input)
	}
	id := uuid.NewString()
	preview, initialAssets, err := service.prepareFigmaAssets(ctx, workspaceID, actor.FactUserID, id, fileKey, nodeID)
	if err != nil {
		return DesignReviewDetail{}, err
	}
	if !input.AnalysisScope.Valid(preview.Width, preview.Height, input.ViewportWidth, input.ViewportHeight) {
		service.removePreparedAssets(initialAssets)
		return DesignReviewDetail{}, ErrInvalid
	}
	result, err := service.repository.CreateReview(ctx, CreateReviewRecord{
		ID: id, WorkspaceID: workspaceID, Requirement: requirement, ChainRootID: id, RoundNumber: 1,
		CreatedByUserID: actor.FactUserID, CreatedByName: strings.TrimSpace(userName), FigmaURL: strings.TrimSpace(input.FigmaURL),
		FigmaFileKey: fileKey, FigmaNodeID: nodeID, FigmaNodeName: preview.NodeName, ImplementationTarget: input.ImplementationTarget,
		ViewportWidth: input.ViewportWidth, ViewportHeight: input.ViewportHeight, DevicePixelRatio: input.DevicePixelRatio,
		PageZoom: input.PageZoom, AnalysisScope: input.AnalysisScope, MobileTopbar: input.MobileTopbar, DOMNormalization: input.DOMNormalization,
		InitialAssets: initialAssets,
	})
	if err != nil {
		service.removePreparedAssets(initialAssets)
	}
	return result, err
}

func (service *Service) CreateRecheck(ctx context.Context, workspaceID uint, actor ActorIdentity, userName, sourceID string, input CreateReviewInput) (DesignReviewDetail, error) {
	source, err := service.GetReview(ctx, workspaceID, actor, sourceID)
	if err != nil {
		return DesignReviewDetail{}, err
	}
	if source.Status != "completed" && source.Status != "failed" && source.Status != "abandoned" {
		return DesignReviewDetail{}, ErrConflict
	}
	if source.RequirementID == nil || *source.RequirementID != input.RequirementID || source.ImplementationKind != input.ImplementationTarget.Kind {
		return DesignReviewDetail{}, ErrConflict
	}
	fileKey, nodeID, err := ParseFigmaNodeURL(input.FigmaURL)
	if err != nil {
		return DesignReviewDetail{}, err
	}
	id := uuid.NewString()
	preview, initialAssets, err := service.prepareFigmaAssets(ctx, workspaceID, actor.FactUserID, id, fileKey, nodeID)
	if err != nil {
		return DesignReviewDetail{}, err
	}
	if !input.AnalysisScope.Valid(preview.Width, preview.Height, input.ViewportWidth, input.ViewportHeight) {
		service.removePreparedAssets(initialAssets)
		return DesignReviewDetail{}, ErrInvalid
	}
	result, err := service.repository.CreateRecheck(ctx, RecheckRecord{
		ID: id, WorkspaceID: workspaceID, SourceReviewID: source.ID, ChainRootID: source.ChainRootID,
		RoundNumber: source.RoundNumber + 1, CreatedByUserID: actor.FactUserID, CreatedByName: userName, Input: input,
		FigmaFileKey: fileKey, FigmaNodeID: nodeID, FigmaNodeName: preview.NodeName, InitialAssets: initialAssets,
	})
	if err != nil {
		service.removePreparedAssets(initialAssets)
	}
	return result, err
}

func (service *Service) prepareFigmaAssets(ctx context.Context, workspaceID, userID uint, reviewID, fileKey, nodeID string) (FigmaPreview, []AssetRecord, error) {
	if !service.availability.Figma || !service.availability.ObjectStorage || service.figma == nil || service.objects == nil {
		return FigmaPreview{}, nil, ErrUnavailable
	}
	preview, original, contentType, err := service.figma.Evidence(ctx, fileKey, nodeID)
	if err != nil {
		return FigmaPreview{}, nil, err
	}
	if err := ValidateDesignStructure(preview.Structure); err != nil {
		return FigmaPreview{}, nil, err
	}
	normalized, cfg, err := NormalizePNG(original)
	if err != nil {
		return FigmaPreview{}, nil, err
	}
	makeRecord := func(kind, mediaType string, raw []byte) AssetRecord {
		id := uuid.NewString()
		sum := sha256.Sum256(raw)
		return AssetRecord{ID: id, WorkspaceID: workspaceID, ReviewID: reviewID, Kind: kind,
			ObjectKey:   fmt.Sprintf("workspace/%d/design-reviews/%s/%s-%s", workspaceID, reviewID, id, safeAssetExtension(mediaType)),
			ContentType: mediaType, ByteSize: int64(len(raw)), SHA256: hex.EncodeToString(sum[:]), Width: &cfg.Width, Height: &cfg.Height,
			CreatedAt: service.now(), ActorUserID: userID}
	}
	designStructure := makeRecord("design_structure", "application/json", preview.Structure)
	designStructure.Width, designStructure.Height = nil, nil
	assets := []AssetRecord{makeRecord("design_original", contentType, original), makeRecord("design_normalized", "image/png", normalized), designStructure}
	for index, asset := range assets {
		raw := original
		if asset.Kind == "design_normalized" {
			raw = normalized
		} else if asset.Kind == "design_structure" {
			raw = preview.Structure
		}
		if err := service.objects.PutObject(ctx, asset.ObjectKey, bytes.NewReader(raw), int64(len(raw)), asset.ContentType); err != nil {
			service.removePreparedAssets(assets[:index])
			return FigmaPreview{}, nil, err
		}
	}
	return preview, assets, nil
}

func (service *Service) removePreparedAssets(assets []AssetRecord) {
	if service.objects == nil {
		return
	}
	for _, asset := range assets {
		_ = service.objects.RemoveObject(context.Background(), asset.ObjectKey)
	}
}

func (service *Service) PreviewFigma(ctx context.Context, actor ActorIdentity, rawURL string) (FigmaPreview, error) {
	if err := service.requireAction(ctx, actor, accesscatalog.ActionDesignReviewsReviewsCreate); err != nil {
		return FigmaPreview{}, err
	}
	if !service.availability.Figma || service.figma == nil {
		return FigmaPreview{}, ErrUnavailable
	}
	fileKey, nodeID, err := ParseFigmaNodeURL(rawURL)
	if err != nil {
		return FigmaPreview{}, err
	}
	return service.figma.Preview(ctx, fileKey, nodeID)
}

func (service *Service) UploadAsset(ctx context.Context, workspaceID uint, actor ActorIdentity, reviewID, kind, contentType string, body io.Reader) (EvidenceAsset, error) {
	if err := service.requireAction(ctx, actor, accesscatalog.ActionDesignReviewsReviewsCreate); err != nil {
		return EvidenceAsset{}, err
	}
	if !service.availability.ObjectStorage || service.objects == nil {
		return EvidenceAsset{}, ErrUnavailable
	}
	detail, err := service.GetReview(ctx, workspaceID, actor, reviewID)
	if err != nil {
		return EvidenceAsset{}, err
	}
	if detail.Status != "draft" {
		return EvidenceAsset{}, ErrConflict
	}
	if !validEvidenceKind(kind) {
		return EvidenceAsset{}, ErrInvalid
	}
	raw, err := io.ReadAll(io.LimitReader(body, maxEvidenceBytes+1))
	if err != nil || len(raw) == 0 || len(raw) > maxEvidenceBytes {
		return EvidenceAsset{}, ErrInvalid
	}
	var normalized []byte
	var normalizedWidth, normalizedHeight int
	var implementationTopbar MobileTopbarConfig
	var structureComplete *bool
	var captureWarnings json.RawMessage
	if kind == "design_structure" {
		if contentType != "application/json" || ValidateDesignStructure(raw) != nil {
			return EvidenceAsset{}, ErrInvalid
		}
	}
	if kind == "implementation_structure" || kind == "implementation_structure_original" {
		complete, warnings, validationErr := ValidateImplementationStructure(raw, detail.ImplementationKind, detail.ViewportWidth, detail.ViewportHeight)
		if contentType != "application/json" || validationErr != nil {
			return EvidenceAsset{}, ErrInvalid
		}
		if kind == "implementation_structure" {
			structureComplete, captureWarnings = &complete, warnings
		}
	}
	var domNormalization json.RawMessage
	if kind == "dom_normalization_manifest" {
		if contentType != "application/json" || !DOMNormalizationEnabled(detail.DOMNormalization) {
			return EvidenceAsset{}, ErrInvalid
		}
		domNormalization, err = ParseDOMNormalizationManifest(raw)
		if err != nil {
			return EvidenceAsset{}, err
		}
	}
	if kind == "design_original" || kind == "implementation_original" || kind == "implementation_normalized" {
		var normalizedConfig image.Config
		if kind == "implementation_original" || kind == "implementation_normalized" {
			implementationTopbar, err = ParseMobileTopbar(detail.MobileTopbar, detail.ImplementationKind)
			if err != nil {
				return EvidenceAsset{}, err
			}
			contentHeight := detail.ViewportHeight - implementationTopbar.Height
			normalized, normalizedConfig, err = NormalizePNGTo(raw, detail.ViewportWidth, contentHeight)
		} else {
			normalized, normalizedConfig, err = NormalizePNG(raw)
		}
		if err != nil {
			return EvidenceAsset{}, err
		}
		normalizedWidth, normalizedHeight = normalizedConfig.Width, normalizedConfig.Height
		if kind == "implementation_original" || kind == "implementation_normalized" {
			if implementationTopbar.Enabled {
				decoded, _, decodeErr := image.Decode(bytes.NewReader(normalized))
				if decodeErr != nil {
					return EvidenceAsset{}, ErrInvalid
				}
				background, colorErr := parseNRGBA(implementationTopbar.BackgroundColor)
				if colorErr != nil {
					return EvidenceAsset{}, colorErr
				}
				composed, composeErr := ComposeIOSTopbar(decoded, background, implementationTopbar.IconTheme == "dark")
				if composeErr != nil {
					return EvidenceAsset{}, composeErr
				}
				normalized, composeErr = EncodePNG(composed)
				if composeErr != nil {
					return EvidenceAsset{}, composeErr
				}
				normalizedWidth, normalizedHeight = composed.Bounds().Dx(), composed.Bounds().Dy()
			}
		}
		if kind == "implementation_normalized" {
			raw, contentType = normalized, "image/png"
		}
	}
	sum := sha256.Sum256(raw)
	id := uuid.NewString()
	objectKey := fmt.Sprintf("workspace/%d/design-reviews/%s/%s-%s", workspaceID, reviewID, id, safeAssetExtension(contentType))
	var width, height *int
	if cfg, _, decodeErr := image.DecodeConfig(bytes.NewReader(raw)); decodeErr == nil {
		width, height = &cfg.Width, &cfg.Height
	}
	record := AssetRecord{ID: id, WorkspaceID: workspaceID, ReviewID: reviewID, Kind: kind, ObjectKey: objectKey, ContentType: contentType, ByteSize: int64(len(raw)), SHA256: hex.EncodeToString(sum[:]), Width: width, Height: height, CreatedAt: service.now(), ActorUserID: actor.FactUserID, StructureComplete: structureComplete, CaptureWarnings: captureWarnings, DOMNormalization: domNormalization}
	if kind == "design_original" || kind == "implementation_original" {
		normalizedKind := strings.TrimSuffix(kind, "_original") + "_normalized"
		normalizedRecord := service.assetRecord(workspaceID, actor.FactUserID, reviewID, normalizedKind, "image/png", normalized, normalizedWidth, normalizedHeight)
		return service.persistAssetBatch(ctx, record, raw, normalizedRecord, normalized)
	}
	if err := service.objects.PutObject(ctx, objectKey, bytes.NewReader(raw), int64(len(raw)), contentType); err != nil {
		return EvidenceAsset{}, err
	}
	oldKey, err := service.repository.SaveAsset(ctx, record)
	if err != nil {
		_ = service.objects.RemoveObject(context.WithoutCancel(ctx), objectKey)
		return EvidenceAsset{}, err
	}
	if oldKey != nil && *oldKey != objectKey {
		service.cleanupReplacedObject(ctx, workspaceID, *oldKey)
	}
	return EvidenceAsset{ID: id, Kind: kind, ContentType: contentType, ByteSize: int64(len(raw)), SHA256: record.SHA256, Width: width, Height: height, CreatedAt: record.CreatedAt}, nil
}

func parseNRGBA(value string) (color.NRGBA, error) {
	if !validHexColor(value) {
		return color.NRGBA{}, ErrInvalid
	}
	raw, err := hex.DecodeString(value[1:])
	if err != nil || len(raw) != 3 {
		return color.NRGBA{}, ErrInvalid
	}
	return color.NRGBA{R: raw[0], G: raw[1], B: raw[2], A: 255}, nil
}

func (service *Service) assetRecord(workspaceID, userID uint, reviewID, kind, contentType string, raw []byte, width, height int) AssetRecord {
	id := uuid.NewString()
	sum := sha256.Sum256(raw)
	objectKey := fmt.Sprintf("workspace/%d/design-reviews/%s/%s-%s", workspaceID, reviewID, id, safeAssetExtension(contentType))
	return AssetRecord{ID: id, WorkspaceID: workspaceID, ReviewID: reviewID, Kind: kind, ObjectKey: objectKey, ContentType: contentType, ByteSize: int64(len(raw)), SHA256: hex.EncodeToString(sum[:]), Width: &width, Height: &height, CreatedAt: service.now(), ActorUserID: userID}
}

func (service *Service) persistAssetBatch(ctx context.Context, original AssetRecord, originalRaw []byte, normalized AssetRecord, normalizedRaw []byte) (EvidenceAsset, error) {
	newAssets := []struct {
		record AssetRecord
		raw    []byte
	}{{original, originalRaw}, {normalized, normalizedRaw}}
	for index, asset := range newAssets {
		if err := service.objects.PutObject(ctx, asset.record.ObjectKey, bytes.NewReader(asset.raw), int64(len(asset.raw)), asset.record.ContentType); err != nil {
			for _, uploaded := range newAssets[:index] {
				_ = service.objects.RemoveObject(context.WithoutCancel(ctx), uploaded.record.ObjectKey)
			}
			return EvidenceAsset{}, err
		}
	}
	oldKeys, err := service.repository.SaveAssets(ctx, []AssetRecord{original, normalized})
	if err != nil {
		for _, asset := range newAssets {
			_ = service.objects.RemoveObject(context.WithoutCancel(ctx), asset.record.ObjectKey)
		}
		return EvidenceAsset{}, err
	}
	for _, oldKey := range oldKeys {
		if oldKey != original.ObjectKey && oldKey != normalized.ObjectKey {
			service.cleanupReplacedObject(ctx, original.WorkspaceID, oldKey)
		}
	}
	return EvidenceAsset{ID: original.ID, Kind: original.Kind, ContentType: original.ContentType, ByteSize: original.ByteSize, SHA256: original.SHA256, Width: original.Width, Height: original.Height, CreatedAt: original.CreatedAt}, nil
}

func (service *Service) cleanupReplacedObject(ctx context.Context, workspaceID uint, objectKey string) {
	cleanupCtx := context.WithoutCancel(ctx)
	if err := service.objects.RemoveObject(cleanupCtx, objectKey); err == nil {
		_ = service.repository.CompleteObjectCleanup(cleanupCtx, workspaceID, objectKey)
	}
}

func (service *Service) Asset(ctx context.Context, workspaceID uint, actor ActorIdentity, reviewID, assetID string) (AssetRecord, io.ReadCloser, error) {
	if err := service.requireAction(ctx, actor, accesscatalog.ActionDesignReviewsReviewsView); err != nil {
		return AssetRecord{}, nil, err
	}
	if !service.availability.ObjectStorage || service.objects == nil {
		return AssetRecord{}, nil, ErrUnavailable
	}
	if _, err := service.GetReview(ctx, workspaceID, actor, reviewID); err != nil {
		return AssetRecord{}, nil, err
	}
	record, err := service.repository.GetAsset(ctx, workspaceID, reviewID, assetID)
	if err != nil {
		return AssetRecord{}, nil, err
	}
	reader, err := service.objects.GetObject(ctx, record.ObjectKey)
	return record, reader, err
}

func (service *Service) QueueAnalysis(ctx context.Context, workspaceID uint, actor ActorIdentity, reviewID string) error {
	if err := service.requireAction(ctx, actor, accesscatalog.ActionDesignReviewsReviewsCreate); err != nil {
		return ErrForbidden
	}
	return service.queueAnalysis(ctx, workspaceID, actor, reviewID, "draft")
}

func (service *Service) RetryAnalysis(ctx context.Context, workspaceID uint, actor ActorIdentity, reviewID string) error {
	if err := service.requireAction(ctx, actor, accesscatalog.ActionDesignReviewsAnalysisManage); err != nil {
		return ErrForbidden
	}
	return service.queueAnalysis(ctx, workspaceID, actor, reviewID, "failed")
}

func (service *Service) queueAnalysis(ctx context.Context, workspaceID uint, actor ActorIdentity, reviewID, expectedStatus string) error {
	if !service.availability.VisualModel || !service.availability.ObjectStorage {
		return ErrUnavailable
	}
	detail, err := service.GetReview(ctx, workspaceID, actor, reviewID)
	if err != nil {
		return err
	}
	if detail.Status != expectedStatus {
		return ErrConflict
	}
	kinds, err := service.repository.ListAssetKinds(ctx, workspaceID, reviewID)
	if err != nil {
		return err
	}
	hasDesign, hasImplementation, hasDesignStructure, hasImplementationStructure, hasNormalizationManifest := false, false, false, false, false
	for _, kind := range kinds {
		hasDesign = hasDesign || kind == "design_original" || kind == "design_normalized"
		hasImplementation = hasImplementation || kind == "implementation_original" || kind == "implementation_normalized"
		hasDesignStructure = hasDesignStructure || kind == "design_structure"
		hasImplementationStructure = hasImplementationStructure || kind == "implementation_structure"
		hasNormalizationManifest = hasNormalizationManifest || kind == "dom_normalization_manifest"
	}
	if !hasDesign || !hasImplementation || !hasDesignStructure || !hasImplementationStructure {
		return ErrEvidenceRequired
	}
	if DOMNormalizationEnabled(detail.DOMNormalization) && !hasNormalizationManifest {
		return ErrEvidenceRequired
	}
	var implementationWidth, implementationHeight int
	for _, asset := range detail.Assets {
		if (asset.Kind == "implementation_normalized" || (implementationWidth == 0 && asset.Kind == "implementation_original")) && asset.Width != nil && asset.Height != nil {
			implementationWidth, implementationHeight = *asset.Width, *asset.Height
		}
	}
	if !detail.AnalysisScope.Valid(detail.DesignWidth, detail.DesignHeight, implementationWidth, implementationHeight) {
		return ErrInvalid
	}
	return service.repository.QueueAnalysis(ctx, workspaceID, reviewID, actor.FactUserID)
}

func (service *Service) DeleteReview(ctx context.Context, workspaceID uint, actor ActorIdentity, reviewID string) error {
	if err := service.requireAction(ctx, actor, accesscatalog.ActionDesignReviewsReviewsDelete); err != nil {
		return err
	}
	detail, err := service.GetReview(ctx, workspaceID, actor, reviewID)
	if err != nil {
		return err
	}
	if detail.RoundNumber != 1 || detail.CreatedByUserID != actor.FactUserID {
		return ErrCreatorOnly
	}
	return service.repository.MarkDeleting(ctx, workspaceID, reviewID, actor.FactUserID)
}

func (service *Service) CreateDifference(ctx context.Context, workspaceID uint, actor ActorIdentity, reviewID string, input DifferenceInput) (DesignDifference, error) {
	if err := service.requireAction(ctx, actor, accesscatalog.ActionDesignReviewsDifferencesManage); err != nil {
		return DesignDifference{}, err
	}
	if !input.Valid() {
		return DesignDifference{}, ErrInvalid
	}
	if _, err := service.GetReview(ctx, workspaceID, actor, reviewID); err != nil {
		return DesignDifference{}, err
	}
	return service.repository.CreateDifference(ctx, DifferenceRecord{ID: uuid.NewString(), WorkspaceID: workspaceID, ReviewID: reviewID, ActorUserID: actor.FactUserID, Input: input})
}

func (service *Service) UpdateDifference(ctx context.Context, workspaceID uint, actor ActorIdentity, reviewID, differenceID string, input DifferenceInput) (DesignDifference, error) {
	if err := service.requireAction(ctx, actor, accesscatalog.ActionDesignReviewsDifferencesManage); err != nil {
		return DesignDifference{}, err
	}
	if !input.Valid() {
		return DesignDifference{}, ErrInvalid
	}
	if _, err := service.GetReview(ctx, workspaceID, actor, reviewID); err != nil {
		return DesignDifference{}, err
	}
	return service.repository.UpdateDifference(ctx, workspaceID, reviewID, differenceID, actor.FactUserID, input)
}

func (service *Service) TransitionDifference(ctx context.Context, workspaceID uint, actor ActorIdentity, reviewID, differenceID, operation, status string) (DesignDifference, error) {
	if err := service.requireAction(ctx, actor, accesscatalog.ActionDesignReviewsDifferencesManage); err != nil {
		return DesignDifference{}, err
	}
	if _, err := service.GetReview(ctx, workspaceID, actor, reviewID); err != nil {
		return DesignDifference{}, err
	}
	return service.repository.TransitionDifference(ctx, workspaceID, reviewID, differenceID, actor.FactUserID, operation, status)
}

func (service *Service) requireProject(ctx context.Context, workspaceID, projectSetID, userID uint) error {
	allowed, err := service.projects.CanAccessProject(ctx, workspaceID, projectSetID, userID)
	if err == nil && allowed {
		return nil
	}
	admin, adminErr := service.projects.IsAccessAdministrator(ctx, workspaceID, userID)
	if adminErr == nil && admin {
		return nil
	}
	return ErrForbidden
}

func validEvidenceKind(kind string) bool {
	_, ok := setOf("design_original", "design_normalized", "implementation_original", "implementation_normalized", "pixel_heatmap", "design_structure", "implementation_structure", "implementation_raw_normalized", "implementation_structure_original", "pixel_heatmap_original", "dom_normalization_manifest")[kind]
	return ok
}

func safeAssetExtension(contentType string) string {
	switch strings.ToLower(strings.TrimSpace(contentType)) {
	case "image/png":
		return "png"
	case "image/jpeg":
		return "jpg"
	case "application/json":
		return "json"
	default:
		return "bin"
	}
}

func randomCode() (string, error) {
	buffer := make([]byte, 4)
	if _, err := randomReader(buffer); err != nil {
		return "", err
	}
	value := (uint(buffer[0])<<16 | uint(buffer[1])<<8 | uint(buffer[2])) % 1000000
	return fmt.Sprintf("%06d", value), nil
}

func randomReader(buffer []byte) (int, error) { return rand.Read(buffer) }

func encodeJSON(value any) []byte {
	raw, _ := json.Marshal(value)
	return raw
}
