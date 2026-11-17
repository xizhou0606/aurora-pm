package toolruntimegorm

import (
	"encoding/json"
	"time"

	"github.com/xizhou0606/aurora-pm/backend/internal/module/toolruntime"
)

type historyFacts struct {
	device   *ExternalDeviceAuthorization
	request  *ExternalAuthorizationRequest
	grant    *ExternalAuthorizationGrant
	codes    []ExternalAuthorizationCode
	families []ExternalCredentialFamily
}

func projectHistory(id string, f historyFacts, now time.Time) (toolruntime.AuthorizationHistoryRecord, error) {
	item := toolruntime.AuthorizationHistoryRecord{ID: id, GrantTools: []string{}}
	if d := f.device; d != nil {
		item.UserCode = d.UserCode
		item.Channel = d.Channel
		item.ClientName = "PM CLI"
		item.CreatedAt = d.CreatedAt.UTC()
		item.DecidedAt = utcPointer(d.DecidedAt)
		item.ExpiresAt = d.ExpiresAt.UTC()
		item.Status = d.Status
		item.CodeValid = (d.Status == "pending" || d.Status == "approved") && now.Before(d.ExpiresAt)
		if d.Status == "approved" || d.Status == "consumed" {
			item.PrincipalID = d.PrincipalID
			item.Status = "approved"
		} else if d.Status == "approving" {
			item.Status = "pending"
		}
	} else if r := f.request; r != nil {
		item.Channel = r.Channel
		item.ClientName = r.ClientName
		item.CreatedAt = r.CreatedAt.UTC()
		item.DecidedAt = utcPointer(r.DecidedAt)
		item.ExpiresAt = r.RequestExpiresAt.UTC()
		item.Status = r.Decision
		if r.Decision == string(toolruntime.AuthorizationAuthorize) {
			item.Status = "approved"
			item.PrincipalID = &r.PrincipalID
		}
		if r.Decision == string(toolruntime.AuthorizationDeny) {
			item.Status = "denied"
		}
		if err := json.Unmarshal([]byte(r.ToolNames), &item.GrantTools); err != nil {
			return item, err
		}
		// OAuth requests are not authorization codes. A code only exists once
		// explicitly issued, and its secret is never projected into this API.
		if item.Status == "approved" {
			for _, c := range f.codes {
				if c.ConsumedAt == nil && now.Before(c.ExpiresAt) {
					item.CodeValid = true
				}
			}
		}
	} else {
		return item, toolruntime.ErrAuthorizationRecordNotFound
	}
	if g := f.grant; g != nil {
		item.ExpiresAt = g.ExpiresAt.UTC()
		if err := json.Unmarshal([]byte(g.AllowedTools), &item.GrantTools); err != nil {
			return item, err
		}
		switch {
		case g.RevokedAt != nil:
			item.Status = "revoked"
			item.CodeValid = false
		case !now.Before(g.ExpiresAt):
			item.Status = "expired"
			item.CodeValid = false
		default:
			item.CanRevoke = true
		}

		// Family validity is independent of one-use code validity and short-lived
		// access token refresh. Business permissions and workspace gates remain
		// enforced at execution time; this flag does not promise tool access.
		if item.Status == "approved" && g.RevokedAt == nil && !now.Before(g.EffectiveFrom) && now.Before(g.ExpiresAt) {
			for _, family := range f.families {
				if family.RevokedAt == nil && !now.Before(family.CreatedAt) && now.Before(family.AbsoluteExpiresAt) {
					item.CredentialValid = true
					break
				}
			}
		}
		if item.Status == "approved" && !item.CodeValid && len(f.families) > 0 {
			active, allRevoked := false, true
			for _, family := range f.families {
				if family.RevokedAt == nil {
					allRevoked = false
					if now.Before(family.AbsoluteExpiresAt) {
						active = true
					}
				}
			}
			if !active {
				item.CanRevoke = false
				if allRevoked {
					item.Status = "revoked"
				} else {
					item.Status = "expired"
				}
			}
		}
	} else if item.Status == "pending" && !now.Before(item.ExpiresAt) {
		item.Status = "expired"
		item.CodeValid = false
	}
	return item, nil
}
