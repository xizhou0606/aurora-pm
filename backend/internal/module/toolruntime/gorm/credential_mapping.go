package toolruntimegorm

import (
	"strings"
	"time"

	"github.com/xizhou0606/aurora-pm/backend/internal/module/toolruntime"
)

func bindingRecord(value toolruntime.CredentialBinding) (credentialBindingRecord, bool) {
	expected, err := toolruntime.AudienceForChannel(value.Channel)
	if value.PrincipalID == 0 || value.WorkspaceID == 0 || value.GrantID == 0 ||
		strings.TrimSpace(value.ClientID) == "" || len(value.ClientID) > 255 || err != nil || value.Audience != expected {
		return credentialBindingRecord{}, false
	}
	return credentialBindingRecord{
		PrincipalID: value.PrincipalID, WorkspaceID: value.WorkspaceID, GrantID: value.GrantID,
		ClientID: strings.TrimSpace(value.ClientID), Channel: string(value.Channel), Audience: string(value.Audience),
	}, true
}

func bindingDomain(value credentialBindingRecord) (toolruntime.CredentialBinding, bool) {
	result := toolruntime.CredentialBinding{
		PrincipalID: value.PrincipalID, WorkspaceID: value.WorkspaceID, GrantID: value.GrantID,
		ClientID: strings.TrimSpace(value.ClientID), Channel: toolruntime.DeliveryChannel(value.Channel),
		Audience: toolruntime.TokenAudience(value.Audience),
	}
	_, ok := bindingRecord(result)
	return result, ok
}

func authorizationCodeRecord(value toolruntime.AuthorizationCodeCredential) (ExternalAuthorizationCode, bool) {
	binding, ok := bindingRecord(value.Binding)
	if !ok || !credentialDigestPattern.MatchString(value.CodeDigest) || !authorizationIDPattern.MatchString(value.AuthorizationRequestID) ||
		!pkceChallengePattern.MatchString(value.PKCEChallenge) || value.RedirectURI == "" || value.ExpiresAt.IsZero() || value.CreatedAt.IsZero() ||
		!value.CreatedAt.Before(value.ExpiresAt) || value.ConsumedAt != nil {
		return ExternalAuthorizationCode{}, false
	}
	return ExternalAuthorizationCode{
		CodeDigest: value.CodeDigest, AuthorizationRequestID: value.AuthorizationRequestID,
		Binding: binding, PKCEChallenge: value.PKCEChallenge, RedirectURI: value.RedirectURI,
		ExpiresAt: value.ExpiresAt.UTC(), CreatedAt: value.CreatedAt.UTC(),
	}, true
}

func authorizationCodeDomain(row ExternalAuthorizationCode) (toolruntime.AuthorizationCodeCredential, bool) {
	binding, ok := bindingDomain(row.Binding)
	if !ok || !credentialDigestPattern.MatchString(row.CodeDigest) || !authorizationIDPattern.MatchString(row.AuthorizationRequestID) ||
		!pkceChallengePattern.MatchString(row.PKCEChallenge) || row.RedirectURI == "" || row.ExpiresAt.IsZero() || row.CreatedAt.IsZero() ||
		!validOptionalTime(row.ConsumedAt, row.CreatedAt) {
		return toolruntime.AuthorizationCodeCredential{}, false
	}
	return toolruntime.AuthorizationCodeCredential{
		CodeDigest: row.CodeDigest, AuthorizationRequestID: row.AuthorizationRequestID, Binding: binding,
		PKCEChallenge: row.PKCEChallenge, RedirectURI: row.RedirectURI,
		ExpiresAt: row.ExpiresAt.UTC(), ConsumedAt: utcPointer(row.ConsumedAt), CreatedAt: row.CreatedAt.UTC(),
	}, true
}

func credentialFamilyRecord(value toolruntime.CredentialFamily) (ExternalCredentialFamily, bool) {
	binding, ok := bindingRecord(value.Binding)
	if !ok || !validFamilyID(value.ID) || value.AbsoluteExpiresAt.IsZero() || value.CreatedAt.IsZero() ||
		value.UpdatedAt.IsZero() || value.UpdatedAt.Before(value.CreatedAt) || !validOptionalTime(value.RevokedAt, value.CreatedAt) {
		return ExternalCredentialFamily{}, false
	}
	if !value.CreatedAt.Before(value.AbsoluteExpiresAt) {
		return ExternalCredentialFamily{}, false
	}
	return ExternalCredentialFamily{
		ID: value.ID, Binding: binding, AbsoluteExpiresAt: value.AbsoluteExpiresAt.UTC(),
		RevokedAt: utcPointer(value.RevokedAt), CreatedAt: value.CreatedAt.UTC(), UpdatedAt: value.UpdatedAt.UTC(),
	}, true
}

func credentialFamilyDomain(row ExternalCredentialFamily) (toolruntime.CredentialFamily, bool) {
	binding, ok := bindingDomain(row.Binding)
	if !ok || !validFamilyID(row.ID) || row.AbsoluteExpiresAt.IsZero() || row.CreatedAt.IsZero() || row.UpdatedAt.IsZero() ||
		!row.CreatedAt.Before(row.AbsoluteExpiresAt) || row.UpdatedAt.Before(row.CreatedAt) || !validOptionalTime(row.RevokedAt, row.CreatedAt) {
		return toolruntime.CredentialFamily{}, false
	}
	return toolruntime.CredentialFamily{
		ID: row.ID, Binding: binding, AbsoluteExpiresAt: row.AbsoluteExpiresAt.UTC(), RevokedAt: utcPointer(row.RevokedAt),
		CreatedAt: row.CreatedAt.UTC(), UpdatedAt: row.UpdatedAt.UTC(),
	}, true
}

func accessCredentialRecord(value toolruntime.AccessCredential) (ExternalAccessCredential, bool) {
	binding, ok := bindingRecord(value.Binding)
	if !ok || !credentialDigestPattern.MatchString(value.TokenDigest) || !validFamilyID(value.FamilyID) ||
		value.ExpiresAt.IsZero() || value.CreatedAt.IsZero() || !value.CreatedAt.Before(value.ExpiresAt) {
		return ExternalAccessCredential{}, false
	}
	return ExternalAccessCredential{
		TokenDigest: value.TokenDigest, FamilyID: value.FamilyID, Binding: binding,
		ExpiresAt: value.ExpiresAt.UTC(), CreatedAt: value.CreatedAt.UTC(),
	}, true
}

func accessCredentialDomain(row ExternalAccessCredential) (toolruntime.AccessCredential, bool) {
	binding, ok := bindingDomain(row.Binding)
	if !ok || !credentialDigestPattern.MatchString(row.TokenDigest) || !validFamilyID(row.FamilyID) ||
		row.ExpiresAt.IsZero() || row.CreatedAt.IsZero() || !row.CreatedAt.Before(row.ExpiresAt) {
		return toolruntime.AccessCredential{}, false
	}
	return toolruntime.AccessCredential{
		TokenDigest: row.TokenDigest, FamilyID: row.FamilyID, Binding: binding,
		ExpiresAt: row.ExpiresAt.UTC(), CreatedAt: row.CreatedAt.UTC(),
	}, true
}

func refreshCredentialRecord(value toolruntime.RefreshCredential) (ExternalRefreshCredential, bool) {
	binding, ok := bindingRecord(value.Binding)
	if !ok || !credentialDigestPattern.MatchString(value.TokenDigest) || !validFamilyID(value.FamilyID) ||
		value.ExpiresAt.IsZero() || value.CreatedAt.IsZero() || !value.CreatedAt.Before(value.ExpiresAt) ||
		!validOptionalTime(value.ConsumedAt, value.CreatedAt) || !validOptionalTime(value.RevokedAt, value.CreatedAt) {
		return ExternalRefreshCredential{}, false
	}
	return ExternalRefreshCredential{
		TokenDigest: value.TokenDigest, FamilyID: value.FamilyID, Binding: binding,
		ExpiresAt: value.ExpiresAt.UTC(), ConsumedAt: utcPointer(value.ConsumedAt),
		RevokedAt: utcPointer(value.RevokedAt), CreatedAt: value.CreatedAt.UTC(),
	}, true
}

func refreshCredentialDomain(row ExternalRefreshCredential) (toolruntime.RefreshCredential, bool) {
	binding, ok := bindingDomain(row.Binding)
	if !ok || !credentialDigestPattern.MatchString(row.TokenDigest) || !validFamilyID(row.FamilyID) ||
		row.ExpiresAt.IsZero() || row.CreatedAt.IsZero() || !row.CreatedAt.Before(row.ExpiresAt) ||
		!validOptionalTime(row.ConsumedAt, row.CreatedAt) || !validOptionalTime(row.RevokedAt, row.CreatedAt) {
		return toolruntime.RefreshCredential{}, false
	}
	return toolruntime.RefreshCredential{
		TokenDigest: row.TokenDigest, FamilyID: row.FamilyID, Binding: binding, ExpiresAt: row.ExpiresAt.UTC(),
		ConsumedAt: utcPointer(row.ConsumedAt), RevokedAt: utcPointer(row.RevokedAt), CreatedAt: row.CreatedAt.UTC(),
	}, true
}

func utcPointer(value *time.Time) *time.Time {
	if value == nil {
		return nil
	}
	result := value.UTC()
	return &result
}

func validOptionalTime(value *time.Time, earliest time.Time) bool {
	return value == nil || !value.Before(earliest)
}
