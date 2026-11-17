package toolruntimegorm

import (
	"context"
	"strings"

	"github.com/xizhou0606/aurora-pm/backend/internal/module/toolruntime"
)

// Load each fact kind once per page, independent of the page size. Every
// query retains workspace scope; unowned devices are only visible to managers.
func (r *AuthorizationHistoryRepository) loadFacts(ctx context.Context, scope toolruntime.AuthorizationHistoryScope, ids []string) (map[string]historyFacts, error) {
	result := map[string]historyFacts{}
	deviceIDs, requestIDs := []string{}, []string{}
	for _, id := range ids {
		if strings.HasPrefix(id, "d_") {
			deviceIDs = append(deviceIDs, id)
		} else if strings.HasPrefix(id, "r_") {
			requestIDs = append(requestIDs, strings.TrimPrefix(id, "r_"))
		}
	}
	db := r.db.WithContext(ctx)
	grantIDs := []uint{}
	if len(deviceIDs) > 0 {
		var rows []ExternalDeviceAuthorization
		q := db.Where("record_id IN ? AND workspace_id = ?", deviceIDs, scope.WorkspaceID)
		if !scope.All {
			q = q.Where("principal_id = ?", scope.PrincipalID)
		}
		if err := q.Find(&rows).Error; err != nil {
			return nil, err
		}
		for i := range rows {
			row := &rows[i]
			result[*row.RecordID] = historyFacts{device: row}
			if row.GrantID != nil {
				grantIDs = append(grantIDs, *row.GrantID)
			}
		}
	}
	if len(requestIDs) > 0 {
		var rows []ExternalAuthorizationRequest
		q := db.Where("id IN ? AND workspace_id = ?", requestIDs, scope.WorkspaceID)
		if !scope.All {
			q = q.Where("principal_id = ?", scope.PrincipalID)
		}
		if err := q.Find(&rows).Error; err != nil {
			return nil, err
		}
		for i := range rows {
			row := &rows[i]
			result["r_"+row.ID] = historyFacts{request: row}
			if row.GrantID != nil {
				grantIDs = append(grantIDs, *row.GrantID)
			}
		}
		var codes []ExternalAuthorizationCode
		if err := db.Where("authorization_request_id IN ? AND workspace_id = ?", requestIDs, scope.WorkspaceID).Find(&codes).Error; err != nil {
			return nil, err
		}
		for _, code := range codes {
			id := "r_" + code.AuthorizationRequestID
			if f, ok := result[id]; ok {
				f.codes = append(f.codes, code)
				result[id] = f
			}
		}
	}
	if len(grantIDs) > 0 {
		var rows []ExternalAuthorizationGrant
		if err := db.Where("id IN ? AND workspace_id = ?", grantIDs, scope.WorkspaceID).Find(&rows).Error; err != nil {
			return nil, err
		}
		grants := map[uint]*ExternalAuthorizationGrant{}
		for i := range rows {
			grants[rows[i].ID] = &rows[i]
		}
		var families []ExternalCredentialFamily
		if err := db.Where("grant_id IN ? AND workspace_id = ?", grantIDs, scope.WorkspaceID).Find(&families).Error; err != nil {
			return nil, err
		}
		grouped := map[uint][]ExternalCredentialFamily{}
		for _, family := range families {
			grouped[family.Binding.GrantID] = append(grouped[family.Binding.GrantID], family)
		}
		for id, f := range result {
			var grantID *uint
			if f.device != nil {
				grantID = f.device.GrantID
			} else {
				grantID = f.request.GrantID
			}
			if grantID != nil {
				f.grant = grants[*grantID]
				if f.grant == nil {
					return nil, toolruntime.ErrAuthorizationRecordNotFound
				}
				f.families = grouped[*grantID]
				result[id] = f
			}
		}
	}
	return result, nil
}
