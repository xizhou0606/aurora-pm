package toolruntimegorm

import (
	"context"
	"time"

	"github.com/xizhou0606/aurora-pm/backend/internal/module/toolruntime"
	"gorm.io/gorm"
)

type AuthorizationHistoryRepository struct{ db *gorm.DB }

func NewAuthorizationHistoryRepository(db *gorm.DB) *AuthorizationHistoryRepository {
	return &AuthorizationHistoryRepository{db: db}
}

type historyRef struct {
	ID        string
	Kind      string
	CreatedAt time.Time
}

func historyUnion(scope toolruntime.AuthorizationHistoryScope) (string, []any) {
	deviceWhere, requestWhere := "d.workspace_id = ? AND d.record_id IS NOT NULL", "r.workspace_id = ?"
	deviceArgs, requestArgs := []any{scope.WorkspaceID}, []any{scope.WorkspaceID}
	if !scope.All {
		deviceWhere += " AND d.principal_id = ?"
		requestWhere += " AND r.principal_id = ?"
		deviceArgs = append(deviceArgs, scope.PrincipalID)
		requestArgs = append(requestArgs, scope.PrincipalID)
	}
	query := `SELECT d.record_id AS id, 'device' AS kind, d.created_at FROM pm_external_device_authorizations d WHERE ` + deviceWhere + ` UNION ALL SELECT 'r_' || r.id AS id, 'request' AS kind, r.created_at FROM pm_external_authorization_requests r WHERE ` + requestWhere + ` AND NOT EXISTS (SELECT 1 FROM pm_external_device_authorizations d WHERE d.authorization_request_id = r.id AND d.workspace_id = r.workspace_id)`
	return query, append(deviceArgs, requestArgs...)
}
func (r *AuthorizationHistoryRepository) ListHistory(ctx context.Context, scope toolruntime.AuthorizationHistoryScope, page, size int, now time.Time) (toolruntime.AuthorizationHistoryPage, error) {
	result := toolruntime.AuthorizationHistoryPage{Items: []toolruntime.AuthorizationHistoryRecord{}, Page: page, PageSize: size}
	query, args := historyUnion(scope)
	db := r.db.WithContext(ctx)
	if err := db.Raw("SELECT COUNT(*) FROM ("+query+") h", args...).Scan(&result.Total).Error; err != nil {
		return result, err
	}
	var refs []historyRef
	pagedArgs := append(append([]any{}, args...), size, (page-1)*size)
	if err := db.Raw("SELECT * FROM ("+query+") h ORDER BY created_at DESC, id DESC LIMIT ? OFFSET ?", pagedArgs...).Scan(&refs).Error; err != nil {
		return result, err
	}
	ids := make([]string, 0, len(refs))
	for _, ref := range refs {
		ids = append(ids, ref.ID)
	}
	facts, err := r.loadFacts(ctx, scope, ids)
	if err != nil {
		return result, err
	}
	for _, ref := range refs {
		record, err := projectHistory(ref.ID, facts[ref.ID], now)
		if err != nil {
			return result, err
		}
		result.Items = append(result.Items, record)
	}
	return result, nil
}
func (r *AuthorizationHistoryRepository) record(ctx context.Context, scope toolruntime.AuthorizationHistoryScope, id string, now time.Time) (toolruntime.AuthorizationHistoryRecord, *ExternalAuthorizationGrant, error) {
	facts, err := r.loadFacts(ctx, scope, []string{id})
	if err != nil {
		return toolruntime.AuthorizationHistoryRecord{}, nil, err
	}
	f, ok := facts[id]
	if !ok {
		return toolruntime.AuthorizationHistoryRecord{}, nil, toolruntime.ErrAuthorizationRecordNotFound
	}
	item, err := projectHistory(id, f, now)
	return item, f.grant, err
}

func (r *AuthorizationHistoryRepository) RevokeHistory(ctx context.Context, scope toolruntime.AuthorizationHistoryScope, id string, now time.Time) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		repo := NewAuthorizationHistoryRepository(tx)
		item, grant, err := repo.record(ctx, scope, id, now)
		if err != nil {
			return err
		}
		if grant == nil {
			return toolruntime.ErrAuthorizationRecordNotFound
		}
		if item.Status == "revoked" {
			return nil
		}
		if !item.CanRevoke {
			return toolruntime.ErrAuthorizationRecordNotFound
		}
		// Each approved authorization creates its own Grant. Revoking that exact
		// grant fences concurrent exchange/refresh too: token verification checks it.
		if err := tx.Model(&ExternalAuthorizationGrant{}).Where("id = ? AND workspace_id = ? AND revoked_at IS NULL", grant.ID, scope.WorkspaceID).Updates(map[string]any{"revoked_at": now, "updated_at": now}).Error; err != nil {
			return err
		}
		if err := tx.Model(&ExternalCredentialFamily{}).Where("grant_id = ? AND workspace_id = ? AND revoked_at IS NULL", grant.ID, scope.WorkspaceID).Updates(map[string]any{"revoked_at": now, "updated_at": now}).Error; err != nil {
			return err
		}
		return tx.Model(&ExternalRefreshCredential{}).Where("grant_id = ? AND workspace_id = ? AND revoked_at IS NULL", grant.ID, scope.WorkspaceID).Update("revoked_at", now).Error
	})
}
