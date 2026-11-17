package designreview

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"image"
	_ "image/png"
	"io"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/xizhou0606/aurora-pm/backend/internal/generated/accesscatalog"
)

const (
	collectorProtocolVersion = "0.1.0"
	mobilewrightVersion      = "0.0.53"
)

var nativeElementID = regexp.MustCompile(`^native-[1-9][0-9]*$`)

type PairingCode struct {
	Code      string    `json:"code"`
	ExpiresAt time.Time `json:"expires_at"`
}

type MobileDevice struct {
	ID        string  `json:"id"`
	Name      string  `json:"name"`
	Platform  string  `json:"platform"`
	Type      string  `json:"type"`
	Model     *string `json:"model"`
	OSVersion *string `json:"os_version"`
}

type CollectorInventory struct {
	Devices []MobileDevice `json:"devices"`
	Doctor  struct {
		Ready   bool   `json:"ready"`
		Summary string `json:"summary"`
	} `json:"doctor"`
}

type PairCollectorInput struct {
	Code                string          `json:"code"`
	Name                string          `json:"name"`
	Version             string          `json:"version"`
	MobilewrightVersion string          `json:"mobilewright_version"`
	Hostname            string          `json:"hostname"`
	Architecture        string          `json:"architecture"`
	Inventory           json.RawMessage `json:"inventory"`
}

type PairCollectorResult struct {
	Collector MobileCollector `json:"collector"`
	Token     string          `json:"token"`
}

type CollectorHeartbeatInput struct {
	Version             string         `json:"version"`
	MobilewrightVersion string         `json:"mobilewright_version"`
	Hostname            string         `json:"hostname"`
	Architecture        string         `json:"architecture"`
	Devices             []MobileDevice `json:"devices"`
	Doctor              struct {
		Ready   bool   `json:"ready"`
		Summary string `json:"summary"`
	} `json:"doctor"`
}

type NativeElement struct {
	ID         string  `json:"id"`
	ParentID   *string `json:"parent_id"`
	Type       string  `json:"native_type"`
	Role       *string `json:"role"`
	Label      *string `json:"label"`
	Text       *string `json:"text"`
	Value      *string `json:"value"`
	Identifier *string `json:"identifier"`
	ResourceID *string `json:"resource_id"`
	Bounds     struct {
		X      float64 `json:"x"`
		Y      float64 `json:"y"`
		Width  float64 `json:"width"`
		Height float64 `json:"height"`
	} `json:"bounds"`
	State struct {
		Visible  bool  `json:"visible"`
		Enabled  bool  `json:"enabled"`
		Selected *bool `json:"selected"`
		Focused  *bool `json:"focused"`
		Checked  *bool `json:"checked"`
	} `json:"state"`
	Clipped     bool   `json:"clipped"`
	Fingerprint string `json:"structural_fingerprint"`
}

type MobileCaptureResult struct {
	CapturedAt time.Time          `json:"captured_at"`
	Target     DesignReviewTarget `json:"target"`
	Elements   []NativeElement    `json:"elements"`
	Truncated  bool               `json:"structure_truncated"`
	Warnings   []struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"warnings"`
	ScreenshotSHA256Before string `json:"screenshot_sha256_before"`
	ScreenshotSHA256After  string `json:"screenshot_sha256_after"`
}

func (service *Service) CreatePairingCode(ctx context.Context, workspaceID uint, actor ActorIdentity) (PairingCode, error) {
	if err := service.requireAction(ctx, actor, accesscatalog.ActionDesignReviewsMobileCollectorsManage); err != nil {
		return PairingCode{}, err
	}
	code, err := randomCode()
	if err != nil {
		return PairingCode{}, err
	}
	digest := sha256.Sum256([]byte(code))
	expires := service.now().Add(10 * time.Minute)
	if err := service.repository.CreatePairingCode(ctx, PairingCodeRecord{ID: uuid.NewString(), WorkspaceID: workspaceID, UserID: actor.FactUserID, CodeHash: hex.EncodeToString(digest[:]), ExpiresAt: expires}); err != nil {
		return PairingCode{}, err
	}
	return PairingCode{Code: code, ExpiresAt: expires}, nil
}

func validCollectorMetadata(version, mobilewright, hostname, architecture string) bool {
	return version == collectorProtocolVersion && mobilewright == mobilewrightVersion && strings.TrimSpace(hostname) != "" && len(hostname) <= 200 && (architecture == "arm64" || architecture == "x64")
}

func validInventory(inventory CollectorInventory) bool {
	if len(inventory.Devices) > 100 || len(inventory.Doctor.Summary) > 1000 {
		return false
	}
	for _, device := range inventory.Devices {
		if strings.TrimSpace(device.ID) == "" || len(device.ID) > 300 || strings.TrimSpace(device.Name) == "" || len(device.Name) > 200 ||
			(device.Platform != "ios" && device.Platform != "android") || (device.Type != "real" && device.Type != "simulator" && device.Type != "emulator") {
			return false
		}
	}
	return true
}

func (service *Service) PairCollector(ctx context.Context, input PairCollectorInput) (PairCollectorResult, error) {
	var inventory CollectorInventory
	if len(input.Code) != 6 || strings.TrimSpace(input.Name) == "" || len(input.Name) > 200 ||
		!validCollectorMetadata(input.Version, input.MobilewrightVersion, input.Hostname, input.Architecture) ||
		json.Unmarshal(input.Inventory, &inventory) != nil || !validInventory(inventory) {
		return PairCollectorResult{}, ErrInvalid
	}
	codeDigest := sha256.Sum256([]byte(input.Code))
	tokenBytes := make([]byte, 32)
	if _, err := randomReader(tokenBytes); err != nil {
		return PairCollectorResult{}, err
	}
	token := hex.EncodeToString(tokenBytes)
	tokenDigest := sha256.Sum256([]byte(token))
	collector, err := service.repository.PairMobileCollector(ctx, hex.EncodeToString(codeDigest[:]), MobileCollectorRecord{
		ID: uuid.NewString(), TokenHash: hex.EncodeToString(tokenDigest[:]), Name: strings.TrimSpace(input.Name),
		Version: input.Version, MobilewrightVersion: input.MobilewrightVersion, Hostname: strings.TrimSpace(input.Hostname),
		Architecture: input.Architecture, Inventory: input.Inventory,
	})
	if err != nil {
		return PairCollectorResult{}, err
	}
	return PairCollectorResult{Collector: collector, Token: token}, nil
}

func (service *Service) AuthenticateCollector(ctx context.Context, token string) (MobileCollector, error) {
	if strings.TrimSpace(token) == "" {
		return MobileCollector{}, ErrForbidden
	}
	digest := sha256.Sum256([]byte(strings.TrimSpace(token)))
	return service.repository.AuthenticateCollector(ctx, hex.EncodeToString(digest[:]))
}

func (service *Service) HeartbeatCollector(ctx context.Context, collector MobileCollector, input CollectorHeartbeatInput) error {
	inventory := CollectorInventory{Devices: input.Devices}
	inventory.Doctor = input.Doctor
	raw, err := json.Marshal(inventory)
	if err != nil || !validCollectorMetadata(input.Version, input.MobilewrightVersion, input.Hostname, input.Architecture) || !validInventory(inventory) {
		return ErrInvalid
	}
	return service.repository.UpdateMobileCollectorHeartbeat(ctx, collector.ID, MobileCollectorHeartbeatRecord{
		Version: input.Version, MobilewrightVersion: input.MobilewrightVersion, Hostname: input.Hostname,
		Architecture: input.Architecture, Inventory: raw, DoctorReady: input.Doctor.Ready, DoctorSummary: input.Doctor.Summary,
	})
}

func (service *Service) ListCollectors(ctx context.Context, workspaceID uint, actor ActorIdentity) ([]MobileCollector, error) {
	if err := service.requireAction(ctx, actor, accesscatalog.ActionDesignReviewsMobileCollectorsManage); err != nil {
		return nil, err
	}
	return service.repository.ListMobileCollectors(ctx, workspaceID, actor.FactUserID)
}

func (service *Service) RevokeCollector(ctx context.Context, workspaceID uint, actor ActorIdentity, collectorID string) error {
	if err := service.requireAction(ctx, actor, accesscatalog.ActionDesignReviewsMobileCollectorsManage); err != nil {
		return err
	}
	return service.repository.RevokeMobileCollector(ctx, workspaceID, collectorID, actor.FactUserID)
}

func (service *Service) CreateCapture(ctx context.Context, workspaceID uint, actor ActorIdentity, collectorID, platform, deviceID string) (MobileCaptureDraft, error) {
	if err := service.requireAction(ctx, actor, accesscatalog.ActionDesignReviewsMobileCollectorsManage); err != nil {
		return MobileCaptureDraft{}, err
	}
	collector, err := service.repository.GetMobileCollector(ctx, workspaceID, actor.FactUserID, collectorID)
	if err != nil {
		return MobileCaptureDraft{}, err
	}
	var inventory CollectorInventory
	if !collector.DoctorReady || json.Unmarshal(collector.Inventory, &inventory) != nil {
		return MobileCaptureDraft{}, ErrUnavailable
	}
	found := false
	for _, device := range inventory.Devices {
		found = found || (device.ID == deviceID && device.Platform == platform)
	}
	if !found {
		return MobileCaptureDraft{}, ErrInvalid
	}
	return service.repository.CreateCaptureDraft(ctx, CaptureDraftRecord{
		ID: uuid.NewString(), WorkspaceID: workspaceID, UserID: actor.FactUserID, CollectorID: collector.ID,
		Platform: platform, DeviceID: strings.TrimSpace(deviceID), ExpiresAt: service.now().Add(30 * time.Minute),
	})
}

func (service *Service) ListCaptures(ctx context.Context, workspaceID uint, actor ActorIdentity) ([]MobileCaptureDraft, error) {
	if err := service.requireAction(ctx, actor, accesscatalog.ActionDesignReviewsMobileCollectorsManage); err != nil {
		return nil, err
	}
	return service.repository.ListCaptureDrafts(ctx, workspaceID, actor.FactUserID)
}

func (service *Service) CaptureAsset(ctx context.Context, workspaceID uint, actor ActorIdentity, captureID string) (MobileCaptureAssetRecord, io.ReadCloser, error) {
	if err := service.requireAction(ctx, actor, accesscatalog.ActionDesignReviewsMobileCollectorsManage); err != nil {
		return MobileCaptureAssetRecord{}, nil, err
	}
	if !service.availability.ObjectStorage || service.objects == nil {
		return MobileCaptureAssetRecord{}, nil, ErrUnavailable
	}
	record, err := service.repository.GetCaptureAsset(ctx, workspaceID, captureID, actor.FactUserID)
	if err != nil {
		return MobileCaptureAssetRecord{}, nil, err
	}
	reader, err := service.objects.GetObject(ctx, record.ObjectKey)
	return record, reader, err
}

func (service *Service) ConsumeCapture(ctx context.Context, workspaceID uint, actor ActorIdentity, captureID string) (MobileCaptureDraft, error) {
	if err := service.requireAction(ctx, actor, accesscatalog.ActionDesignReviewsMobileCollectorsManage); err != nil {
		return MobileCaptureDraft{}, err
	}
	return service.repository.ConsumeCaptureDraft(ctx, workspaceID, captureID, actor.FactUserID)
}

func (service *Service) DeleteCapture(ctx context.Context, workspaceID uint, actor ActorIdentity, captureID string) error {
	if err := service.requireAction(ctx, actor, accesscatalog.ActionDesignReviewsMobileCollectorsManage); err != nil {
		return err
	}
	asset, assetErr := service.repository.GetCaptureAsset(ctx, workspaceID, captureID, actor.FactUserID)
	if assetErr != nil && !errors.Is(assetErr, ErrNotFound) {
		return assetErr
	}
	if assetErr == nil && asset.ObjectKey != "" {
		if service.objects == nil {
			return ErrUnavailable
		}
		if err := service.objects.RemoveObject(ctx, asset.ObjectKey); err != nil {
			return err
		}
	}
	_, err := service.repository.DeleteCaptureDraft(ctx, workspaceID, captureID, actor.FactUserID)
	return err
}

func (service *Service) ClaimCapture(ctx context.Context, collector MobileCollector) (*MobileCaptureDraft, error) {
	return service.repository.ClaimCaptureDraft(ctx, collector.ID)
}

func (service *Service) FailCapture(ctx context.Context, collector MobileCollector, draftID, message string) error {
	message = strings.TrimSpace(message)
	if message == "" || len(message) > 1000 {
		return ErrInvalid
	}
	return service.repository.FailCaptureDraft(ctx, collector.ID, draftID, collector.UserID, message)
}

func validCaptureResult(result MobileCaptureResult, draft MobileCaptureDraft, collector MobileCollector) bool {
	if result.CapturedAt.IsZero() || result.CapturedAt.After(time.Now().Add(5*time.Minute)) || result.Target.Kind != "mobile" ||
		!result.Target.Valid() || result.Target.Platform != draft.Platform || result.Target.Collector != collectorProtocolVersion ||
		result.Target.Mobilewright != mobilewrightVersion || len(result.Elements) > 1500 || len(result.Warnings) > 50 ||
		len(result.Target.AppID) > 300 || len(result.Target.AppName) > 300 || len(result.Target.AppVersion) > 120 {
		return false
	}
	var screen struct {
		Width  int     `json:"width"`
		Height int     `json:"height"`
		Scale  float64 `json:"scale"`
	}
	if json.Unmarshal(result.Target.Screen, &screen) != nil || screen.Width < 240 || screen.Width > 7680 || screen.Height < 240 || screen.Height > 7680 || screen.Scale <= 0 || screen.Scale > 8 {
		return false
	}
	seen := make(map[string]struct{}, len(result.Elements))
	for _, element := range result.Elements {
		if !nativeElementID.MatchString(element.ID) || strings.TrimSpace(element.Type) == "" || len(element.Type) > 120 ||
			len(element.Fingerprint) < 16 || len(element.Fingerprint) > 64 || element.Bounds.Width < 0 || element.Bounds.Height < 0 ||
			element.Bounds.X < 0 || element.Bounds.Y < 0 || element.Bounds.X+element.Bounds.Width > float64(screen.Width)+1 || element.Bounds.Y+element.Bounds.Height > float64(screen.Height)+1 ||
			tooLong(element.Role, 80) || tooLong(element.Label, 500) || tooLong(element.Text, 500) || tooLong(element.Value, 500) || tooLong(element.Identifier, 300) || tooLong(element.ResourceID, 300) {
			return false
		}
		if _, exists := seen[element.ID]; exists {
			return false
		}
		seen[element.ID] = struct{}{}
		if element.ParentID != nil {
			if _, exists := seen[*element.ParentID]; !exists {
				return false
			}
		}
	}
	for _, warning := range result.Warnings {
		if strings.TrimSpace(warning.Code) == "" || strings.TrimSpace(warning.Message) == "" || len(warning.Code) > 80 || len(warning.Message) > 500 {
			return false
		}
	}
	return len(result.ScreenshotSHA256Before) == 64 && len(result.ScreenshotSHA256After) == 64
}

func tooLong(value *string, maximum int) bool {
	return value != nil && len([]rune(*value)) > maximum
}

func (service *Service) CompleteCapture(ctx context.Context, collector MobileCollector, draftID, contentType string, screenshot io.Reader, result MobileCaptureResult) error {
	draft, err := service.repository.GetCaptureDraftForCollector(ctx, collector.ID, draftID)
	if err != nil {
		return err
	}
	if draft.Status != "claimed" || !validCaptureResult(result, draft, collector) || contentType != "image/png" {
		return ErrInvalid
	}
	raw, err := io.ReadAll(io.LimitReader(screenshot, maxEvidenceBytes+1))
	if err != nil || len(raw) == 0 || len(raw) > maxEvidenceBytes {
		return ErrInvalid
	}
	if _, format, decodeErr := image.DecodeConfig(bytes.NewReader(raw)); decodeErr != nil || format != "png" {
		return ErrInvalid
	}
	sum := sha256.Sum256(raw)
	digest := hex.EncodeToString(sum[:])
	if result.ScreenshotSHA256After != digest {
		return ErrInvalid
	}
	target, _ := json.Marshal(result.Target)
	elements, _ := json.Marshal(result.Elements)
	warnings, _ := json.Marshal(result.Warnings)
	objectKey := "workspace/" + strconv.FormatUint(uint64(collector.WorkspaceID), 10) + "/design-reviews/mobile-captures/" + draft.ID + "/screenshot-" + uuid.NewString() + ".png"
	if service.objects == nil || service.objects.PutObject(ctx, objectKey, bytes.NewReader(raw), int64(len(raw)), contentType) != nil {
		return ErrUnavailable
	}
	err = service.repository.CompleteCaptureDraft(ctx, CompleteCaptureRecord{
		ID: draft.ID, CollectorID: collector.ID, WorkspaceID: collector.WorkspaceID, ActorUserID: collector.UserID,
		AppID: result.Target.AppID, Target: target, Elements: elements, StructureTruncated: result.Truncated, Warnings: warnings,
		ScreenshotObjectKey: objectKey, ScreenshotMediaType: contentType, ScreenshotSHA256: digest,
		ScreenshotByteSize: int64(len(raw)), CapturedAt: result.CapturedAt,
	})
	if err != nil {
		_ = service.objects.RemoveObject(context.WithoutCancel(ctx), objectKey)
	}
	return err
}
