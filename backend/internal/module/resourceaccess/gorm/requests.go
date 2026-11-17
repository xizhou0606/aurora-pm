package resourceaccessgorm

import (
	"context"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/documentlibrary"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/resourceaccess"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"strconv"
	"time"
)

type AccessRequestRecord struct {
	TargetKind   string `gorm:"size:24;not null;default:document_resource"`
	PlanUnitID   *uint
	ProjectSetID *uint

	GrantLevel      resourceaccess.Level `gorm:"size:16;not null;default:editor"`
	GrantExpiresAt  *time.Time
	ID              uint                         `gorm:"primaryKey"`
	WorkspaceID     uint                         `gorm:"not null;index:idx_access_requests_scope,priority:1"`
	ResourceID      string                       `gorm:"size:64;index:idx_access_requests_scope,priority:2"`
	ApplicantUserID uint                         `gorm:"not null"`
	Reason          string                       `gorm:"type:text;not null"`
	NotifyOwner     bool                         `gorm:"not null"`
	Status          resourceaccess.RequestStatus `gorm:"size:16;not null"`
	Read            bool                         `gorm:"not null;default:false"`
	CreatedAt       time.Time                    `gorm:"not null"`
	ReviewedAt      *time.Time
	ReviewedBy      *uint
}

func (AccessRequestRecord) TableName() string { return "resource_access_requests" }
func requestView(r AccessRequestRecord) resourceaccess.AccessRequest {
	return resourceaccess.AccessRequest{ID: r.ID, GrantLevel: r.GrantLevel, GrantExpiresAt: r.GrantExpiresAt, ApplicantUserID: r.ApplicantUserID, Reason: r.Reason, NotifyOwner: r.NotifyOwner, Status: r.Status, Read: r.Read, CreatedAt: r.CreatedAt, ExpiresAt: resourceaccess.RequestExpiresAt(r.CreatedAt), ReviewedAt: r.ReviewedAt}
}
func (r *Repository) ListAccessRequests(ctx context.Context, p resourceaccess.Principal, id documentlibrary.ResourceID, owner bool) ([]resourceaccess.AccessRequest, error) {
	query := r.db.WithContext(ctx).Where("workspace_id = ? AND resource_id = ?", p.WorkspaceID, id)
	if !owner {
		query = query.Where("applicant_user_id = ?", p.UserID)
	} else {
		query = query.Order("CASE WHEN status = 'pending' THEN 0 ELSE 1 END ASC")
	}
	var rows []AccessRequestRecord
	if err := query.Order("created_at DESC, id DESC").Limit(101).Find(&rows).Error; err != nil {
		return nil, err
	}
	result := make([]resourceaccess.AccessRequest, 0, len(rows))
	for _, row := range rows {
		result = append(result, requestView(row))
	}
	return result, nil
}
func (r *Repository) CreateAccessRequest(ctx context.Context, m resourceaccess.RequestMutation) (resourceaccess.AccessRequest, error) {
	var result AccessRequestRecord
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if r.requestOwnerGuard == nil {
			return resourceaccess.ErrUnavailable
		}
		if err := r.requestOwnerGuard(tx, m); err != nil {
			return err
		}
		var policy PolicyRecord
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("workspace_id = ? AND resource_id = ?", m.Principal.WorkspaceID, m.ResourceID).First(&policy).Error; err != nil {
			return mapNotFound(err)
		}
		if r.requestClock == nil {
			return resourceaccess.ErrUnavailable
		}
		m.Now = r.requestClock.Now().UTC()
		var rows []AccessRequestRecord
		if err := tx.Model(&AccessRequestRecord{}).Where("workspace_id = ? AND resource_id = ? AND applicant_user_id = ? AND status = ? AND created_at <= ?", m.Principal.WorkspaceID, m.ResourceID, m.Principal.UserID, resourceaccess.RequestPending, m.Now.Add(-resourceaccess.RequestLifetime)).Update("status", resourceaccess.RequestExpired).Error; err != nil {
			return err
		}
		if err := tx.Where("workspace_id = ? AND resource_id = ? AND applicant_user_id = ? AND status = ?", m.Principal.WorkspaceID, m.ResourceID, m.Principal.UserID, "pending").Find(&rows).Error; err != nil {
			return err
		}
		if len(rows) > 0 {
			result = rows[0]
			return nil
		}
		result = AccessRequestRecord{WorkspaceID: m.Principal.WorkspaceID, ResourceID: string(m.ResourceID), ApplicantUserID: m.Principal.UserID, Reason: m.Reason, NotifyOwner: m.NotifyOwner, Status: "pending", CreatedAt: m.Now}
		if err := tx.Create(&result).Error; err != nil {
			return err
		}
		if err := appendAuditAndOutbox(tx, m.Principal, m.ResourceID, "access.requested", nil, requestView(result), m.Now); err != nil {
			return err
		}
		if m.NotifyOwner {
			return r.enqueueRequestNotification(tx, m, result.ID, m.OwnerUserID)
		}
		return nil
	})
	return requestView(result), err
}
func (r *Repository) UpdateAccessRequest(ctx context.Context, m resourceaccess.RequestMutation) (resourceaccess.AccessRequest, error) {
	var result AccessRequestRecord
	expiredDecision := false
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if r.requestOwnerGuard == nil {
			return resourceaccess.ErrUnavailable
		}
		if err := r.requestOwnerGuard(tx, m); err != nil {
			return err
		}
		// All request and policy writes lock the policy first, preserving one lock order.
		var policy PolicyRecord
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("workspace_id = ? AND resource_id = ?", m.Principal.WorkspaceID, m.ResourceID).First(&policy).Error; err != nil {
			return mapNotFound(err)
		}
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("workspace_id = ? AND resource_id = ? AND id = ?", m.Principal.WorkspaceID, m.ResourceID, m.RequestID).First(&result).Error; err != nil {
			return mapNotFound(err)
		}
		if r.requestClock == nil {
			return resourceaccess.ErrUnavailable
		}
		m.Now = r.requestClock.Now().UTC()
		before := requestView(result)
		if resourceaccess.RequestHasExpired(result.Status, result.CreatedAt, m.Now) {
			result.Status = resourceaccess.RequestExpired
			if m.Read != nil {
				result.Read = *m.Read
			}
			if err := tx.Save(&result).Error; err != nil {
				return err
			}
			expiredDecision = m.Decision != ""
			return appendAuditAndOutbox(tx, m.Principal, m.ResourceID, "access.request.expired", before, requestView(result), m.Now)
		}
		revoke := m.Decision == resourceaccess.RequestRevoked && result.Status == resourceaccess.RequestApproved
		if m.Decision == resourceaccess.RequestRevoked && result.Status != resourceaccess.RequestApproved && result.Status != resourceaccess.RequestRevoked {
			return resourceaccess.ErrConflict
		}
		if m.Decision != "" && result.Status != resourceaccess.RequestPending && result.Status != m.Decision && !revoke {
			return resourceaccess.ErrConflict
		}
		transition := m.Decision != "" && (result.Status == resourceaccess.RequestPending || revoke)
		if m.Decision == resourceaccess.RequestApproved && result.Status == resourceaccess.RequestApproved {
			level, duration := m.GrantLevel, m.GrantDuration
			if level == "" {
				level = resourceaccess.LevelEditor
			}
			if duration == "" {
				duration = resourceaccess.LinkDurationNever
			}
			if !level.Valid() || !duration.Valid() || result.ReviewedAt == nil {
				return resourceaccess.ErrInvalidInput
			}
			expected := resourceaccess.GrantExpiry(*result.ReviewedAt, duration)
			if level != result.GrantLevel || (expected == nil) != (result.GrantExpiresAt == nil) || expected != nil && !expected.Equal(*result.GrantExpiresAt) {
				return resourceaccess.ErrConflict
			}
		}
		if transition {
			if m.Decision == resourceaccess.RequestApproved {
				if m.GrantLevel == "" {
					m.GrantLevel = resourceaccess.LevelEditor
				}
				if m.GrantDuration == "" {
					m.GrantDuration = resourceaccess.LinkDurationNever
				}
				if !m.GrantLevel.Valid() || !m.GrantDuration.Valid() {
					return resourceaccess.ErrInvalidInput
				}
				result.GrantLevel = m.GrantLevel
				result.GrantExpiresAt = resourceaccess.GrantExpiry(m.Now, m.GrantDuration)
			}
			result.Status = m.Decision
			result.ReviewedAt = &m.Now
			result.ReviewedBy = &m.Principal.UserID
		}
		if m.Read != nil {
			result.Read = *m.Read
		}
		if err := tx.Save(&result).Error; err != nil {
			return err
		}
		if transition && (m.Decision == resourceaccess.RequestApproved || m.Decision == resourceaccess.RequestRevoked) {
			previousRevision := policy.Revision
			if err := tx.Model(&policy).Updates(map[string]any{"revision": policy.Revision + 1, "updated_by": m.Principal.UserID, "updated_at": m.Now}).Error; err != nil {
				return err
			}
			if r.policyEventAppenderFactory == nil {
				return resourceaccess.ErrUnavailable
			}
			appender, err := r.policyEventAppenderFactory(tx)
			if err != nil {
				return err
			}
			if appender == nil {
				return resourceaccess.ErrUnavailable
			}
			if err := appender.AppendPolicyRevisionEvent(ctx, resourceaccess.PolicyRevisionEvent{WorkspaceID: m.Principal.WorkspaceID, ResourceID: m.ResourceID, ActorUserID: m.Principal.UserID, IdempotencyKey: "access-request-" + strconv.FormatUint(uint64(result.ID), 10) + "-" + string(m.Decision), PreviousRevision: previousRevision, PolicyRevision: previousRevision + 1, Mode: resourceaccess.Mode(policy.Mode)}); err != nil {
				return err
			}
		}
		if err := appendAuditAndOutbox(tx, m.Principal, m.ResourceID, "access.request.updated", before, requestView(result), m.Now); err != nil {
			return err
		}
		if transition {
			return r.enqueueRequestNotification(tx, m, result.ID, result.ApplicantUserID)
		}
		return nil
	})
	if err == nil && expiredDecision {
		return requestView(result), resourceaccess.ErrConflict
	}
	return requestView(result), err
}
func (r *Repository) ApprovedRequestResources(ctx context.Context, p resourceaccess.Principal, ids []documentlibrary.ResourceID) (map[documentlibrary.ResourceID][]resourceaccess.RequestGrant, error) {
	var rows []AccessRequestRecord
	err := r.db.WithContext(ctx).Select("resource_id", "grant_level", "grant_expires_at").Where("workspace_id = ? AND resource_id IN ? AND applicant_user_id = ? AND status = ?", p.WorkspaceID, stringResourceIDs(ids), p.UserID, resourceaccess.RequestApproved).Find(&rows).Error
	result := map[documentlibrary.ResourceID][]resourceaccess.RequestGrant{}
	for _, row := range rows {
		id := documentlibrary.ResourceID(row.ResourceID)
		result[id] = append(result[id], resourceaccess.RequestGrant{Level: row.GrantLevel, ExpiresAt: row.GrantExpiresAt})
	}
	return result, err
}

func (r *Repository) CountPendingAccessRequests(ctx context.Context, p resourceaccess.Principal, id documentlibrary.ResourceID, owner bool, now time.Time) (int, error) {
	query := r.db.WithContext(ctx).Model(&AccessRequestRecord{}).Where("workspace_id = ? AND resource_id = ? AND status = ?", p.WorkspaceID, id, resourceaccess.RequestPending)
	query = query.Where("created_at > ?", now.Add(-resourceaccess.RequestLifetime))
	if !owner {
		query = query.Where("applicant_user_id = ?", p.UserID)
	}
	var count int64
	err := query.Count(&count).Error
	return int(count), err
}
func (r *Repository) enqueueRequestNotification(tx *gorm.DB, m resourceaccess.RequestMutation, id uint, recipient uint) error {
	if r.requestNotificationAppender == nil {
		return resourceaccess.ErrUnavailable
	}
	return r.requestNotificationAppender(tx, m, id, recipient)
}

// NotificationRequests reads status only for receipt-owned request IDs supplied by the inbox.
func (r *Repository) NotificationRequests(ctx context.Context, workspaceID uint, ids []uint) (map[uint]resourceaccess.AccessRequest, error) {
	out := map[uint]resourceaccess.AccessRequest{}
	if workspaceID == 0 || len(ids) == 0 {
		return out, nil
	}
	var rows []AccessRequestRecord
	if err := r.db.WithContext(ctx).Where("workspace_id = ? AND id IN ?", workspaceID, ids).Find(&rows).Error; err != nil {
		return nil, err
	}
	for _, row := range rows {
		item := requestView(row)
		if resourceaccess.RequestHasExpired(item.Status, item.CreatedAt, time.Now().UTC()) {
			item.Status = resourceaccess.RequestExpired
		}
		out[row.ID] = item
	}
	return out, nil
}
