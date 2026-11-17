package toolruntimegorm

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	toolruntime "github.com/xizhou0606/aurora-pm/backend/internal/module/toolruntime"
	"gorm.io/gorm"
)

type AuthorizationRequestRepository struct {
	db *gorm.DB
}

func NewAuthorizationRequestRepository(db *gorm.DB) *AuthorizationRequestRepository {
	return &AuthorizationRequestRepository{db: db}
}

func (repository *AuthorizationRequestRepository) Create(ctx context.Context, request *toolruntime.AuthorizationRequest) error {
	if request == nil {
		return toolruntime.ErrAuthorizationRequestInvalid
	}
	if err := request.Validate(); err != nil {
		return err
	}
	row, err := authorizationRequestRecord(*request)
	if err != nil {
		return err
	}
	return repository.db.WithContext(ctx).Create(&row).Error
}

func (repository *AuthorizationRequestRepository) FindByID(ctx context.Context, id string) (toolruntime.AuthorizationRequest, error) {
	if id == "" {
		return toolruntime.AuthorizationRequest{}, toolruntime.ErrAuthorizationRequestNotFound
	}
	var row ExternalAuthorizationRequest
	if err := repository.db.WithContext(ctx).First(&row, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return toolruntime.AuthorizationRequest{}, toolruntime.ErrAuthorizationRequestNotFound
		}
		return toolruntime.AuthorizationRequest{}, err
	}
	return authorizationRequestDomain(row)
}

func (repository *AuthorizationRequestRepository) Decide(ctx context.Context, command toolruntime.AuthorizationDecisionCommand) (toolruntime.AuthorizationRequest, error) {
	if command.RequestID == "" || command.PrincipalID == 0 || command.WorkspaceID == 0 || command.DecidedAt.IsZero() ||
		(command.Decision != toolruntime.AuthorizationAuthorize && command.Decision != toolruntime.AuthorizationDeny) {
		return toolruntime.AuthorizationRequest{}, toolruntime.ErrAuthorizationDecisionInvalid
	}
	var decided toolruntime.AuthorizationRequest
	err := repository.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		updated := tx.Model(&ExternalAuthorizationRequest{}).
			Where("id = ? AND principal_id = ? AND workspace_id = ? AND decision = ? AND request_expires_at > ?",
				command.RequestID, command.PrincipalID, command.WorkspaceID, string(toolruntime.AuthorizationPending), command.DecidedAt.UTC()).
			Updates(map[string]any{"decision": string(command.Decision), "decided_at": command.DecidedAt.UTC(), "updated_at": command.DecidedAt.UTC()})
		if updated.Error != nil {
			return updated.Error
		}
		if updated.RowsAffected != 1 {
			return authorizationDecisionConflict(tx, command)
		}
		if command.Decision == toolruntime.AuthorizationAuthorize {
			if command.Grant == nil || command.Grant.Validate() != nil {
				return toolruntime.ErrGrantInvalid
			}
			var requestRow ExternalAuthorizationRequest
			if err := tx.First(&requestRow, "id = ?", command.RequestID).Error; err != nil {
				return err
			}
			if !authorizationGrantMatches(requestRow, *command.Grant, command.DecidedAt) {
				return toolruntime.ErrGrantInvalid
			}
			grantRow, err := grantRecord(*command.Grant)
			if err != nil {
				return err
			}
			if err := tx.Create(&grantRow).Error; err != nil {
				return err
			}
			command.Grant.ID = grantRow.ID
			if err := tx.Model(&ExternalAuthorizationRequest{}).Where("id = ?", command.RequestID).
				Update("grant_id", grantRow.ID).Error; err != nil {
				return err
			}
		} else if command.Grant != nil {
			return toolruntime.ErrAuthorizationDecisionInvalid
		}
		var row ExternalAuthorizationRequest
		if err := tx.First(&row, "id = ?", command.RequestID).Error; err != nil {
			return err
		}
		value, err := authorizationRequestDomain(row)
		if err != nil {
			return err
		}
		decided = value
		return nil
	})
	return decided, err
}

func authorizationGrantMatches(request ExternalAuthorizationRequest, grant toolruntime.Grant, decidedAt time.Time) bool {
	var tools, scopes []string
	if json.Unmarshal([]byte(request.ToolNames), &tools) != nil || json.Unmarshal([]byte(request.GrantScopes), &scopes) != nil ||
		!decidedAt.UTC().Before(grant.ExpiresAt.UTC()) {
		return false
	}
	want, err := toolruntime.NewGrant(toolruntime.GrantRequest{
		PrincipalID: request.PrincipalID, WorkspaceID: request.WorkspaceID,
		ClientID: request.ClientID, Channel: toolruntime.DeliveryChannel(request.Channel),
		AllowedTools: tools, AllowedScopes: scopes,
		EffectiveFrom: request.EffectiveFrom, ExpiresAt: request.ExpiresAt,
	}, grant.CreatedAt)
	if err != nil {
		return false
	}
	return grant.PrincipalID == want.PrincipalID && grant.WorkspaceID == want.WorkspaceID &&
		grant.ClientID == want.ClientID && grant.Channel == want.Channel &&
		grant.EffectiveFrom.Equal(want.EffectiveFrom) && grant.ExpiresAt.Equal(want.ExpiresAt) &&
		strings.Join(grant.AllowedTools, "\x00") == strings.Join(want.AllowedTools, "\x00") &&
		strings.Join(grant.AllowedScopes, "\x00") == strings.Join(want.AllowedScopes, "\x00")
}

func authorizationDecisionConflict(tx *gorm.DB, command toolruntime.AuthorizationDecisionCommand) error {
	var row ExternalAuthorizationRequest
	if err := tx.First(&row, "id = ?", command.RequestID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return toolruntime.ErrAuthorizationRequestNotFound
		}
		return err
	}
	if row.PrincipalID != command.PrincipalID || row.WorkspaceID != command.WorkspaceID {
		return toolruntime.ErrAuthorizationRequestNotFound
	}
	if row.Decision != string(toolruntime.AuthorizationPending) {
		return toolruntime.ErrAuthorizationRequestDecided
	}
	if !command.DecidedAt.UTC().Before(row.RequestExpiresAt.UTC()) {
		return toolruntime.ErrAuthorizationRequestExpired
	}
	return toolruntime.ErrAuthorizationRequestDecided
}

func authorizationRequestRecord(request toolruntime.AuthorizationRequest) (ExternalAuthorizationRequest, error) {
	if err := request.Validate(); err != nil {
		return ExternalAuthorizationRequest{}, err
	}
	toolNames, err := json.Marshal(request.ToolNames)
	if err != nil {
		return ExternalAuthorizationRequest{}, fmt.Errorf("%w: tool names cannot be encoded", toolruntime.ErrAuthorizationRequestInvalid)
	}
	grantScopes, err := json.Marshal(request.GrantScopes)
	if err != nil {
		return ExternalAuthorizationRequest{}, fmt.Errorf("%w: grant scopes cannot be encoded", toolruntime.ErrAuthorizationRequestInvalid)
	}
	row := ExternalAuthorizationRequest{
		ID: request.ID, PrincipalID: request.PrincipalID, WorkspaceID: request.WorkspaceID,
		WorkspaceName: request.WorkspaceName, ClientID: request.ClientID, ClientName: request.ClientName,
		Channel: string(request.Channel), ConfigurationMode: string(request.ConfigurationMode),
		ConfigurationVersion: request.ConfigurationVersion,
		ToolNames:            string(toolNames), GrantScopes: string(grantScopes),
		EffectiveFrom: request.EffectiveFrom.UTC(), ExpiresAt: request.ExpiresAt.UTC(),
		RequestExpiresAt: request.RequestExpiresAt.UTC(), Decision: string(request.Decision),
		DecidedAt: request.DecidedAt, GrantID: request.GrantID,
		CreatedAt: request.CreatedAt.UTC(), UpdatedAt: request.UpdatedAt.UTC(),
	}
	if request.OAuth != nil {
		row.OAuthState = stringPointer(request.OAuth.State)
		row.OAuthRedirectURI = stringPointer(request.OAuth.RedirectURI)
		row.OAuthPKCEChallenge = stringPointer(request.OAuth.PKCEChallenge)
		row.OAuthResource = stringPointer(request.OAuth.Resource)
		row.OAuthScope = stringPointer(request.OAuth.Scope)
	}
	return row, nil
}

func authorizationRequestDomain(row ExternalAuthorizationRequest) (toolruntime.AuthorizationRequest, error) {
	var toolNames, grantScopes []string
	if json.Unmarshal([]byte(row.ToolNames), &toolNames) != nil || json.Unmarshal([]byte(row.GrantScopes), &grantScopes) != nil {
		return toolruntime.AuthorizationRequest{}, toolruntime.ErrAuthorizationRequestInvalid
	}
	request := toolruntime.AuthorizationRequest{
		ID: row.ID, PrincipalID: row.PrincipalID, WorkspaceID: row.WorkspaceID,
		WorkspaceName: row.WorkspaceName, ClientID: row.ClientID, ClientName: row.ClientName,
		Channel:              toolruntime.DeliveryChannel(row.Channel),
		ConfigurationMode:    toolruntime.WorkspaceConfigurationMode(row.ConfigurationMode),
		ConfigurationVersion: row.ConfigurationVersion,
		ToolNames:            toolNames, GrantScopes: grantScopes,
		EffectiveFrom: row.EffectiveFrom, ExpiresAt: row.ExpiresAt, RequestExpiresAt: row.RequestExpiresAt,
		Decision: toolruntime.AuthorizationDecision(row.Decision), DecidedAt: utcTimePointer(row.DecidedAt), GrantID: row.GrantID,
		CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
	}
	oauthFields := []*string{row.OAuthState, row.OAuthRedirectURI, row.OAuthPKCEChallenge, row.OAuthResource, row.OAuthScope}
	oauthCount := 0
	for _, value := range oauthFields {
		if value != nil {
			oauthCount++
		}
	}
	if oauthCount != 0 {
		if oauthCount != len(oauthFields) {
			return toolruntime.AuthorizationRequest{}, toolruntime.ErrAuthorizationRequestInvalid
		}
		request.OAuth = &toolruntime.OAuthAuthorizationContext{
			State: *row.OAuthState, RedirectURI: *row.OAuthRedirectURI,
			PKCEChallenge: *row.OAuthPKCEChallenge, Resource: *row.OAuthResource, Scope: *row.OAuthScope,
		}
	}
	if err := request.Validate(); err != nil {
		return toolruntime.AuthorizationRequest{}, err
	}
	return request, nil
}

func stringPointer(value string) *string {
	return &value
}

func utcTimePointer(value *time.Time) *time.Time {
	if value == nil {
		return nil
	}
	result := value.UTC()
	return &result
}

var _ toolruntime.AuthorizationRequestRepository = (*AuthorizationRequestRepository)(nil)
