package toolruntimegorm

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"time"

	"gorm.io/gorm"

	"github.com/xizhou0606/aurora-pm/backend/internal/module/toolruntime"
)

type DeviceAuthorizationRepository struct {
	db *gorm.DB
}

func NewDeviceAuthorizationRepository(db *gorm.DB) *DeviceAuthorizationRepository {
	return &DeviceAuthorizationRepository{db: db}
}

func (repository *DeviceAuthorizationRepository) Create(ctx context.Context, value toolruntime.DeviceAuthorization) error {
	row, ok := deviceAuthorizationRecord(value)
	if repository == nil || repository.db == nil || !ok {
		return toolruntime.ErrDeviceAuthorizationInvalid
	}
	var randomID [16]byte
	if _, err := rand.Read(randomID[:]); err != nil {
		return err
	}
	id := "d_" + hex.EncodeToString(randomID[:])
	row.RecordID = &id
	if value.UserCode != "" {
		row.UserCode = &value.UserCode
	}
	if err := repository.db.WithContext(ctx).Create(&row).Error; err != nil {
		return toolruntime.ErrDeviceAuthorizationInvalid
	}

	return nil
}

func (repository *DeviceAuthorizationRepository) FindByUserCode(ctx context.Context, digest string) (toolruntime.DeviceAuthorization, error) {
	if repository == nil || repository.db == nil || !credentialDigestPattern.MatchString(digest) {
		return toolruntime.DeviceAuthorization{}, toolruntime.ErrDeviceAuthorizationInvalid
	}
	var row ExternalDeviceAuthorization
	if err := repository.db.WithContext(ctx).First(&row, "user_code_digest = ?", digest).Error; err != nil {
		return toolruntime.DeviceAuthorization{}, toolruntime.ErrDeviceAuthorizationInvalid
	}
	value, ok := deviceAuthorizationDomain(row)
	if !ok {
		return toolruntime.DeviceAuthorization{}, toolruntime.ErrDeviceAuthorizationInvalid
	}
	return value, nil
}

func (repository *DeviceAuthorizationRepository) ClaimDecision(
	ctx context.Context,
	digest string,
	workspaceID uint,
	decision toolruntime.AuthorizationDecision,
	now time.Time,
) (toolruntime.DeviceAuthorization, error) {
	if repository == nil || repository.db == nil || !credentialDigestPattern.MatchString(digest) || workspaceID == 0 || now.IsZero() ||
		(decision != toolruntime.AuthorizationAuthorize && decision != toolruntime.AuthorizationDeny) {
		return toolruntime.DeviceAuthorization{}, toolruntime.ErrDeviceAuthorizationInvalid
	}
	status := toolruntime.DeviceAuthorizationDenied
	if decision == toolruntime.AuthorizationAuthorize {
		status = "approving"
	}
	result := repository.db.WithContext(ctx).Model(&ExternalDeviceAuthorization{}).
		Where("user_code_digest = ? AND workspace_id = ? AND status = ? AND expires_at > ?", digest, workspaceID, toolruntime.DeviceAuthorizationPending, now.UTC()).
		Updates(map[string]any{"status": status, "decided_at": now.UTC(), "updated_at": now.UTC()})
	if result.Error != nil || result.RowsAffected != 1 {
		return toolruntime.DeviceAuthorization{}, repository.decisionConflict(ctx, digest, workspaceID, now)
	}
	return repository.FindByUserCode(ctx, digest)
}

func (repository *DeviceAuthorizationRepository) CompleteApproval(
	ctx context.Context,
	userDigest string,
	binding toolruntime.CredentialBinding,
	requestID string,
	now time.Time,
) (toolruntime.DeviceAuthorization, error) {
	bindingRow, bindingOK := bindingRecord(binding)
	if repository == nil || repository.db == nil || !credentialDigestPattern.MatchString(userDigest) ||
		!bindingOK || !authorizationIDPattern.MatchString(requestID) || now.IsZero() {
		return toolruntime.DeviceAuthorization{}, toolruntime.ErrDeviceAuthorizationInvalid
	}
	result := repository.db.WithContext(ctx).Model(&ExternalDeviceAuthorization{}).
		Where("user_code_digest = ? AND status = ? AND workspace_id = ? AND client_id = ? AND channel = ? AND audience = ? AND expires_at > ?",
			userDigest, "approving", binding.WorkspaceID, binding.ClientID, binding.Channel, binding.Audience, now.UTC()).
		Updates(map[string]any{
			"status": toolruntime.DeviceAuthorizationApproved, "authorization_request_id": requestID,
			"principal_id": bindingRow.PrincipalID, "grant_id": bindingRow.GrantID, "updated_at": now.UTC(),
		})
	if result.Error != nil || result.RowsAffected != 1 {
		return toolruntime.DeviceAuthorization{}, toolruntime.ErrDeviceAuthorizationInvalid
	}
	return repository.FindByUserCode(ctx, userDigest)
}

func (repository *DeviceAuthorizationRepository) FailApproval(ctx context.Context, digest string, now time.Time) error {
	if repository == nil || repository.db == nil || !credentialDigestPattern.MatchString(digest) || now.IsZero() {
		return toolruntime.ErrDeviceAuthorizationInvalid
	}
	result := repository.db.WithContext(ctx).Model(&ExternalDeviceAuthorization{}).
		Where("user_code_digest = ? AND status = ?", digest, "approving").
		Updates(map[string]any{"status": toolruntime.DeviceAuthorizationDenied, "updated_at": now.UTC()})
	if result.Error != nil || result.RowsAffected != 1 {
		return toolruntime.ErrDeviceAuthorizationInvalid
	}
	return nil
}

func (repository *DeviceAuthorizationRepository) Poll(ctx context.Context, digest string, now time.Time) (toolruntime.DeviceAuthorization, error) {
	if repository == nil || repository.db == nil || !credentialDigestPattern.MatchString(digest) || now.IsZero() {
		return toolruntime.DeviceAuthorization{}, toolruntime.ErrDeviceAuthorizationInvalid
	}
	var value toolruntime.DeviceAuthorization
	var protocolErr error
	err := repository.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var row ExternalDeviceAuthorization
		if err := tx.First(&row, "device_code_digest = ?", digest).Error; err != nil {
			return toolruntime.ErrDeviceAuthorizationInvalid
		}
		current, ok := deviceAuthorizationDomain(row)
		if !ok {
			return toolruntime.ErrDeviceAuthorizationInvalid
		}
		if !now.UTC().Before(current.ExpiresAt) {
			protocolErr = toolruntime.ErrDeviceAuthorizationExpired
			return nil
		}
		switch current.Status {
		case toolruntime.DeviceAuthorizationPending, "approving":
			interval := current.PollInterval
			tooSoon := current.LastPolledAt != nil && now.UTC().Before(current.LastPolledAt.Add(interval))
			if tooSoon {
				interval += 5 * time.Second
				if interval > 60*time.Second {
					interval = 60 * time.Second
				}
			}
			if err := tx.Model(&ExternalDeviceAuthorization{}).Where("device_code_digest = ?", digest).
				Updates(map[string]any{"last_polled_at": now.UTC(), "poll_interval_seconds": uint16(interval / time.Second), "updated_at": now.UTC()}).Error; err != nil {
				return err
			}
			if tooSoon {
				protocolErr = toolruntime.ErrDeviceAuthorizationSlowDown
				return nil
			}
			protocolErr = toolruntime.ErrDeviceAuthorizationPending
			return nil
		case toolruntime.DeviceAuthorizationDenied:
			protocolErr = toolruntime.ErrDeviceAuthorizationDenied
			return nil
		case toolruntime.DeviceAuthorizationConsumed:
			return toolruntime.ErrDeviceAuthorizationInvalid
		case toolruntime.DeviceAuthorizationApproved:
			value = current
			return nil
		default:
			return toolruntime.ErrDeviceAuthorizationInvalid
		}
	})
	if err != nil {
		return toolruntime.DeviceAuthorization{}, err
	}
	return value, protocolErr
}

func (repository *DeviceAuthorizationRepository) decisionConflict(ctx context.Context, digest string, workspaceID uint, now time.Time) error {
	var row ExternalDeviceAuthorization
	if err := repository.db.WithContext(ctx).First(&row, "user_code_digest = ? AND workspace_id = ?", digest, workspaceID).Error; err != nil {
		return toolruntime.ErrDeviceAuthorizationInvalid
	}
	if !now.UTC().Before(row.ExpiresAt) {
		return toolruntime.ErrDeviceAuthorizationExpired
	}
	return toolruntime.ErrDeviceAuthorizationInvalid
}

func (repository *DeviceAuthorizationRepository) Consume(
	ctx context.Context,
	digest string,
	consumedAt time.Time,
	family toolruntime.CredentialFamily,
	access toolruntime.AccessCredential,
	refresh toolruntime.RefreshCredential,
) error {
	familyRow, familyOK := credentialFamilyRecord(family)
	accessRow, accessOK := accessCredentialRecord(access)
	refreshRow, refreshOK := refreshCredentialRecord(refresh)
	if repository == nil || repository.db == nil || !credentialDigestPattern.MatchString(digest) || consumedAt.IsZero() ||
		!familyOK || !accessOK || !refreshOK || family.ID != access.FamilyID || family.ID != refresh.FamilyID ||
		family.Binding != access.Binding || family.Binding != refresh.Binding {
		return toolruntime.ErrDeviceAuthorizationInvalid
	}
	err := repository.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		result := tx.Model(&ExternalDeviceAuthorization{}).
			Where("device_code_digest = ? AND status = ? AND consumed_at IS NULL AND expires_at > ? AND principal_id = ? AND grant_id = ?",
				digest, toolruntime.DeviceAuthorizationApproved, consumedAt.UTC(), family.Binding.PrincipalID, family.Binding.GrantID).
			Updates(map[string]any{"status": toolruntime.DeviceAuthorizationConsumed, "consumed_at": consumedAt.UTC(), "updated_at": consumedAt.UTC()})
		if result.Error != nil || result.RowsAffected != 1 {
			return toolruntime.ErrDeviceAuthorizationInvalid
		}
		if err := tx.Create(&familyRow).Error; err != nil {
			return err
		}
		if err := tx.Create(&accessRow).Error; err != nil {
			return err
		}
		return tx.Create(&refreshRow).Error
	})
	if err != nil {
		return toolruntime.ErrDeviceAuthorizationInvalid
	}
	return nil
}

func deviceAuthorizationRecord(value toolruntime.DeviceAuthorization) (ExternalDeviceAuthorization, bool) {
	if !credentialDigestPattern.MatchString(value.DeviceCodeDigest) || !credentialDigestPattern.MatchString(value.UserCodeDigest) ||
		value.WorkspaceID == 0 || value.ClientID != toolruntime.OAuthClientCLI || value.Channel != toolruntime.DeliveryCLI ||
		value.Audience != toolruntime.AudienceCLI || value.Resource == "" || value.Scope == "" ||
		value.Status != toolruntime.DeviceAuthorizationPending || value.ExpiresAt.IsZero() || value.CreatedAt.IsZero() ||
		!value.CreatedAt.Before(value.ExpiresAt) || value.PollInterval < time.Second || value.PollInterval > 60*time.Second {
		return ExternalDeviceAuthorization{}, false
	}
	return ExternalDeviceAuthorization{
		DeviceCodeDigest: value.DeviceCodeDigest, UserCodeDigest: value.UserCodeDigest, WorkspaceID: value.WorkspaceID,
		ClientID: value.ClientID, Channel: string(value.Channel), Audience: string(value.Audience),
		Resource: value.Resource, Scope: value.Scope, Status: string(value.Status), ExpiresAt: value.ExpiresAt.UTC(),
		PollIntervalSeconds: uint16(value.PollInterval / time.Second), CreatedAt: value.CreatedAt.UTC(), UpdatedAt: value.UpdatedAt.UTC(),
	}, true
}

func deviceAuthorizationDomain(row ExternalDeviceAuthorization) (toolruntime.DeviceAuthorization, bool) {
	if !credentialDigestPattern.MatchString(row.DeviceCodeDigest) || !credentialDigestPattern.MatchString(row.UserCodeDigest) ||
		row.WorkspaceID == 0 || row.ClientID != toolruntime.OAuthClientCLI || row.Channel != string(toolruntime.DeliveryCLI) ||
		row.Audience != string(toolruntime.AudienceCLI) || row.Resource == "" || row.Scope == "" || row.ExpiresAt.IsZero() ||
		row.PollIntervalSeconds == 0 || row.CreatedAt.IsZero() || row.UpdatedAt.IsZero() {
		return toolruntime.DeviceAuthorization{}, false
	}
	status := toolruntime.DeviceAuthorizationStatus(row.Status)
	if status != toolruntime.DeviceAuthorizationPending && status != "approving" && status != toolruntime.DeviceAuthorizationApproved &&
		status != toolruntime.DeviceAuthorizationDenied && status != toolruntime.DeviceAuthorizationConsumed ||
		!validDeviceAuthorizationState(row, status) {
		return toolruntime.DeviceAuthorization{}, false
	}
	value := toolruntime.DeviceAuthorization{
		DeviceCodeDigest: row.DeviceCodeDigest, UserCodeDigest: row.UserCodeDigest, WorkspaceID: row.WorkspaceID,
		ClientID: row.ClientID, Channel: toolruntime.DeliveryChannel(row.Channel), Audience: toolruntime.TokenAudience(row.Audience),
		Resource: row.Resource, Scope: row.Scope, Status: status, ExpiresAt: row.ExpiresAt.UTC(),
		PollInterval: time.Duration(row.PollIntervalSeconds) * time.Second, LastPolledAt: utcTimePointer(row.LastPolledAt),
		DecidedAt: utcTimePointer(row.DecidedAt), ConsumedAt: utcTimePointer(row.ConsumedAt), CreatedAt: row.CreatedAt.UTC(), UpdatedAt: row.UpdatedAt.UTC(),
	}
	if row.AuthorizationRequestID != nil {
		value.AuthorizationRequestID = *row.AuthorizationRequestID
	}
	if row.PrincipalID != nil {
		value.PrincipalID = *row.PrincipalID
	}
	if row.GrantID != nil {
		value.GrantID = *row.GrantID
	}
	return value, true
}

func validDeviceAuthorizationState(row ExternalDeviceAuthorization, status toolruntime.DeviceAuthorizationStatus) bool {
	if !validOptionalTime(row.LastPolledAt, row.CreatedAt) || !validOptionalTime(row.DecidedAt, row.CreatedAt) ||
		!validOptionalTime(row.ConsumedAt, row.CreatedAt) {
		return false
	}
	hasApproval := row.AuthorizationRequestID != nil && authorizationIDPattern.MatchString(*row.AuthorizationRequestID) &&
		row.PrincipalID != nil && *row.PrincipalID != 0 && row.GrantID != nil && *row.GrantID != 0
	hasAnyApproval := row.AuthorizationRequestID != nil || row.PrincipalID != nil || row.GrantID != nil
	switch status {
	case toolruntime.DeviceAuthorizationPending:
		return row.DecidedAt == nil && !hasAnyApproval && row.ConsumedAt == nil
	case "approving", toolruntime.DeviceAuthorizationDenied:
		return row.DecidedAt != nil && !hasAnyApproval && row.ConsumedAt == nil
	case toolruntime.DeviceAuthorizationApproved:
		return row.DecidedAt != nil && hasApproval && row.ConsumedAt == nil
	case toolruntime.DeviceAuthorizationConsumed:
		return row.DecidedAt != nil && hasApproval && row.ConsumedAt != nil
	default:
		return false
	}
}

var _ toolruntime.DeviceAuthorizationRepository = (*DeviceAuthorizationRepository)(nil)
