package api

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/json"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/xizhou0606/aurora-pm/backend/internal/module/collaboration"
	collaborationhttp "github.com/xizhou0606/aurora-pm/backend/internal/module/collaboration/http"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/collaboration/securetoken"
)

const (
	collaborationTicketPurpose  = "collaboration.connection-ticket"
	collaborationGrantPurpose   = "collaboration.connection-grant"
	collaborationReceiptPurpose = "collaboration.validation-receipt"
	collaborationTicketTTL      = 30 * time.Second
	collaborationGrantTTL       = 10 * time.Second
	collaborationReceiptTTL     = 5 * time.Second
	maxCollaborationSecretBytes = 512
)

type collaborationAuthorityDocumentService interface {
	Load(context.Context, collaboration.Principal, collaboration.Room) (collaboration.DocumentState, error)
	Commit(context.Context, collaboration.Principal, collaboration.DocumentUpdate) (collaboration.DocumentCommit, error)
}

type collaborationAuthorityProjectionValidator interface {
	ValidateCollaborationProjection(
		context.Context,
		collaboration.Principal,
		collaboration.Room,
		json.RawMessage,
		collaborationhttp.AuthorityReferenceSet,
	) error
}

type collaborationServiceTokenVerifier struct{ digest [sha256.Size]byte }

func newCollaborationServiceTokenVerifier(secret string) (*collaborationServiceTokenVerifier, error) {
	if len(secret) < 32 || len(secret) > maxCollaborationSecretBytes || strings.TrimSpace(secret) != secret {
		return nil, collaboration.ErrInvalidInput
	}
	return &collaborationServiceTokenVerifier{digest: sha256.Sum256([]byte(secret))}, nil
}

func (verifier *collaborationServiceTokenVerifier) VerifyCollaborationServiceToken(
	ctx context.Context,
	token string,
) error {
	if ctx == nil || ctx.Err() != nil || verifier == nil || len(token) > maxCollaborationSecretBytes {
		return collaboration.ErrForbidden
	}
	digest := sha256.Sum256([]byte(token))
	validShape := len(token) >= 32 && strings.TrimSpace(token) == token
	if subtle.ConstantTimeCompare(digest[:], verifier.digest[:]) != 1 || !validShape {
		return collaboration.ErrForbidden
	}
	return nil
}

type collaborationTicketClaims struct {
	WorkspaceID    uint   `json:"workspace_id"`
	ResourceID     string `json:"resource_id"`
	UserID         uint   `json:"user_id"`
	DisplayName    string `json:"display_name"`
	LinkSecret     string `json:"link_secret,omitempty"`
	ConnectionID   string `json:"connection_id"`
	PolicyRevision uint64 `json:"policy_revision"`
	ExpiresAt      int64  `json:"expires_at_ms"`
}

type collaborationGrantClaims struct {
	WorkspaceID    uint   `json:"workspace_id"`
	ResourceID     string `json:"resource_id"`
	UserID         uint   `json:"user_id"`
	DisplayName    string `json:"display_name"`
	LinkSecret     string `json:"link_secret,omitempty"`
	ConnectionID   string `json:"connection_id"`
	SocketID       string `json:"socket_id"`
	Level          string `json:"level"`
	PolicyRevision uint64 `json:"policy_revision"`
	ExpiresAt      int64  `json:"expires_at_ms"`
}

type collaborationReceiptClaims struct {
	WorkspaceID      uint   `json:"workspace_id"`
	ResourceID       string `json:"resource_id"`
	GrantHash        string `json:"grant_hash"`
	UserID           uint   `json:"user_id"`
	ConnectionID     string `json:"connection_id"`
	SnapshotHash     string `json:"snapshot_hash"`
	ProjectionHash   string `json:"projection_hash"`
	ReferenceSetHash string `json:"reference_set_hash"`
	PolicyRevision   uint64 `json:"policy_revision"`
	ExpiresAt        int64  `json:"expires_at_ms"`
}

type collaborationDocumentAuthority struct {
	tokens        *securetoken.Codec
	access        collaboration.DocumentCollaborationAccessEvaluator
	documents     collaborationAuthorityDocumentService
	projection    collaborationAuthorityProjectionValidator
	ticketTTL     time.Duration
	grantTTL      time.Duration
	validationTTL time.Duration
	now           func() time.Time
}

func newCollaborationDocumentAuthority(
	tokens *securetoken.Codec,
	access collaboration.DocumentCollaborationAccessEvaluator,
	documents collaborationAuthorityDocumentService,
	projection collaborationAuthorityProjectionValidator,
	ticketTTL time.Duration,
	grantTTL time.Duration,
	validationTTL time.Duration,
	now func() time.Time,
) (*collaborationDocumentAuthority, error) {
	if tokens == nil || access == nil || documents == nil || projection == nil || now == nil ||
		!validCollaborationAuthorityTTL(ticketTTL) || !validCollaborationAuthorityTTL(grantTTL) ||
		!validCollaborationAuthorityTTL(validationTTL) {
		return nil, collaboration.ErrInvalidInput
	}
	return &collaborationDocumentAuthority{
		tokens: tokens, access: access, documents: documents, projection: projection,
		ticketTTL: ticketTTL, grantTTL: grantTTL, validationTTL: validationTTL, now: now,
	}, nil
}

func (authority *collaborationDocumentAuthority) IssueDocumentConnectionTicket(
	ctx context.Context,
	principal collaboration.Principal,
	room collaboration.Room,
) (string, time.Time, error) {
	if !validCollaborationAuthorityPrincipal(principal, room) || !validCollaborationConnectionID(principal.ConnectionID) {
		return "", time.Time{}, collaboration.ErrInvalidInput
	}
	decision, err := authority.currentDecision(ctx, principal, room)
	if err != nil {
		return "", time.Time{}, err
	}
	ttl, expiresAt, err := authority.boundedTTL(decision.ExpiresAt, authority.ticketTTL)
	if err != nil {
		return "", time.Time{}, err
	}
	token, err := authority.tokens.Seal(ctx, collaborationTicketPurpose, collaborationTicketClaims{
		WorkspaceID: room.WorkspaceID, ResourceID: room.ResourceID, UserID: principal.UserID,
		DisplayName: principal.DisplayName, LinkSecret: principal.LinkAccessSecret,
		ConnectionID:   principal.ConnectionID,
		PolicyRevision: decision.PolicyRevision, ExpiresAt: expiresAt.UnixMilli(),
	}, ttl)
	if err != nil {
		return "", time.Time{}, err
	}
	return token, expiresAt, nil
}

func (authority *collaborationDocumentAuthority) AuthorizeDocumentConnection(
	ctx context.Context,
	room collaboration.Room,
	ticket, connectionID string,
) (collaborationhttp.AuthorityGrant, error) {
	var claims collaborationTicketClaims
	if !validCollaborationConnectionID(connectionID) || authority.tokens.Open(
		ctx, collaborationTicketPurpose, ticket, &claims,
	) != nil || !validTicketClaims(claims, room, authority.now()) {
		return collaborationhttp.AuthorityGrant{}, collaboration.ErrForbidden
	}
	principal := principalFromTicket(claims)
	decision, err := authority.currentDecision(ctx, principal, room)
	if err != nil {
		return collaborationhttp.AuthorityGrant{}, err
	}
	return authority.mintGrant(ctx, principal, room, connectionID, decision)
}

func (authority *collaborationDocumentAuthority) RevalidateDocumentGrant(
	ctx context.Context,
	room collaboration.Room,
	proof collaborationhttp.AuthorityGrantProof,
) (collaborationhttp.AuthorityGrant, error) {
	claims, principal, decision, err := authority.authorizeGrant(ctx, room, proof, false, true)
	if err != nil {
		return collaborationhttp.AuthorityGrant{}, err
	}
	return authority.mintGrant(ctx, principal, room, claims.SocketID, decision)
}

func (authority *collaborationDocumentAuthority) LoadDocumentForGrant(
	ctx context.Context,
	room collaboration.Room,
	proof collaborationhttp.AuthorityGrantProof,
) (collaboration.DocumentState, error) {
	_, principal, _, err := authority.authorizeGrant(ctx, room, proof, false, false)
	if err != nil {
		return collaboration.DocumentState{}, err
	}
	return authority.documents.Load(ctx, principal, room)
}

func (authority *collaborationDocumentAuthority) ValidateDocumentMutation(
	ctx context.Context,
	input collaborationhttp.AuthorityValidateInput,
) (collaborationhttp.AuthorityValidationReceipt, error) {
	claims, principal, decision, err := authority.authorizeGrant(ctx, input.Room, input.Grant, true, false)
	if err != nil || !collaboration.IsSHA256Hex(input.CandidateSnapshotHash) {
		return collaborationhttp.AuthorityValidationReceipt{}, collaboration.ErrForbidden
	}
	projectionHash, err := collaboration.CanonicalJSONHash(input.Projection)
	if err != nil {
		return collaborationhttp.AuthorityValidationReceipt{}, err
	}
	projection, references := cloneAuthorityInput(input.Projection, input.References)
	if err = authority.projection.ValidateCollaborationProjection(ctx, principal, input.Room, projection, references); err != nil {
		return collaborationhttp.AuthorityValidationReceipt{}, err
	}
	ttl, expiresAt, err := authority.boundedTTL(decision.ExpiresAt, authority.validationTTL)
	if err != nil {
		return collaborationhttp.AuthorityValidationReceipt{}, err
	}
	receiptClaims := collaborationReceiptClaims{
		WorkspaceID: input.Room.WorkspaceID, ResourceID: input.Room.ResourceID,
		GrantHash: collaboration.SHA256Hex([]byte(input.Grant.GrantID)), UserID: principal.UserID,
		ConnectionID: claims.ConnectionID, SnapshotHash: input.CandidateSnapshotHash,
		ProjectionHash: projectionHash, ReferenceSetHash: authorityReferenceSetHash(input.References),
		PolicyRevision: decision.PolicyRevision, ExpiresAt: expiresAt.UnixMilli(),
	}
	token, err := authority.tokens.Seal(ctx, collaborationReceiptPurpose, receiptClaims, ttl)
	if err != nil {
		return collaborationhttp.AuthorityValidationReceipt{}, err
	}
	return collaborationhttp.AuthorityValidationReceipt{
		Token: token, CandidateSnapshotHash: input.CandidateSnapshotHash,
		PolicyRevision: decision.PolicyRevision, ExpiresAt: expiresAt,
	}, nil
}

func (authority *collaborationDocumentAuthority) CommitDocumentMutation(
	ctx context.Context,
	input collaborationhttp.AuthorityCommitInput,
) (collaboration.DocumentCommit, error) {
	grant, principal, decision, err := authority.authorizeGrant(ctx, input.Update.Room, input.Grant, true, false)
	if err != nil {
		return collaboration.DocumentCommit{}, err
	}
	var receipt collaborationReceiptClaims
	if authority.tokens.Open(ctx, collaborationReceiptPurpose, input.ValidationToken, &receipt) != nil {
		return collaboration.DocumentCommit{}, collaboration.ErrForbidden
	}
	input.Update.ActorUserID, input.Update.PolicyRevision = principal.UserID, decision.PolicyRevision
	projectionHash, projectionErr := collaboration.CanonicalJSONHash(input.Update.Projection)
	if projectionErr != nil || collaboration.ValidateDocumentUpdate(input.Update) != nil ||
		!receiptMatchesCommit(receipt, input, grant, principal, decision, projectionHash, authority.now()) {
		return collaboration.DocumentCommit{}, collaboration.ErrForbidden
	}
	projection, references := cloneAuthorityInput(input.Update.Projection, input.References)
	if err = authority.projection.ValidateCollaborationProjection(
		ctx, principal, input.Update.Room, projection, references,
	); err != nil {
		return collaboration.DocumentCommit{}, err
	}
	commit, err := authority.documents.Commit(ctx, principal, input.Update)
	if err != nil {
		return collaboration.DocumentCommit{}, err
	}
	if !authorityCommitMatches(commit, input.Update, decision.PolicyRevision) {
		return collaboration.DocumentCommit{}, collaboration.ErrDataCorrupt
	}
	return commit, nil
}

func (authority *collaborationDocumentAuthority) authorizeGrant(
	ctx context.Context,
	room collaboration.Room,
	proof collaborationhttp.AuthorityGrantProof,
	requireEdit, allowRevisionRefresh bool,
) (collaborationGrantClaims, collaboration.Principal, collaboration.AccessDecision, error) {
	var claims collaborationGrantClaims
	if authority.tokens.Open(ctx, collaborationGrantPurpose, proof.GrantID, &claims) != nil ||
		!validGrantClaims(claims, room, proof, authority.now()) {
		return claims, collaboration.Principal{}, collaboration.AccessDecision{}, collaboration.ErrForbidden
	}
	principal := collaboration.Principal{
		WorkspaceID: claims.WorkspaceID, UserID: claims.UserID, DisplayName: claims.DisplayName,
		ConnectionID: claims.ConnectionID, LinkAccessSecret: claims.LinkSecret,
	}
	decision, err := authority.currentDecision(ctx, principal, room)
	if err != nil || requireEdit && (claims.Level != "editor" || !decision.CanEdit) ||
		!allowRevisionRefresh && decision.PolicyRevision != claims.PolicyRevision {
		if err != nil {
			return claims, principal, decision, err
		}
		return claims, principal, decision, collaboration.ErrForbidden
	}
	return claims, principal, decision, nil
}

func (authority *collaborationDocumentAuthority) mintGrant(
	ctx context.Context,
	principal collaboration.Principal,
	room collaboration.Room,
	socketID string,
	decision collaboration.AccessDecision,
) (collaborationhttp.AuthorityGrant, error) {
	if !validCollaborationConnectionID(socketID) {
		return collaborationhttp.AuthorityGrant{}, collaboration.ErrForbidden
	}
	level := "viewer"
	if decision.CanEdit {
		level = "editor"
	}
	ttl, expiresAt, err := authority.boundedTTL(decision.ExpiresAt, authority.grantTTL)
	if err != nil {
		return collaborationhttp.AuthorityGrant{}, err
	}
	claims := collaborationGrantClaims{
		WorkspaceID: room.WorkspaceID, ResourceID: room.ResourceID, UserID: principal.UserID,
		DisplayName: principal.DisplayName, LinkSecret: principal.LinkAccessSecret,
		ConnectionID: principal.ConnectionID, SocketID: socketID,
		Level: level, PolicyRevision: decision.PolicyRevision,
		ExpiresAt: expiresAt.UnixMilli(),
	}
	grantID, err := authority.tokens.Seal(ctx, collaborationGrantPurpose, claims, ttl)
	if err != nil {
		return collaborationhttp.AuthorityGrant{}, err
	}
	revalidateAfter := ttl / 2
	if revalidateAfter > 3*time.Second {
		revalidateAfter = 3 * time.Second
	}
	return collaborationhttp.AuthorityGrant{
		GrantID: grantID,
		Actor: collaborationhttp.AuthorityActor{
			ID: strconv.FormatUint(uint64(principal.UserID), 10), DisplayName: principal.DisplayName,
		},
		Level: level, PolicyRevision: decision.PolicyRevision, ExpiresAt: expiresAt,
		RevalidateAfterM: max(revalidateAfter.Milliseconds(), 1),
	}, nil
}

func (authority *collaborationDocumentAuthority) currentDecision(
	ctx context.Context,
	principal collaboration.Principal,
	room collaboration.Room,
) (collaboration.AccessDecision, error) {
	decision, err := authority.access.EvaluateDocumentCollaborationAccess(ctx, principal, room)
	if err != nil {
		return collaboration.AccessDecision{}, err
	}
	if !decision.CanView || decision.PolicyRevision == 0 || !decision.ExpiresAt.After(authority.now()) {
		return collaboration.AccessDecision{}, collaboration.ErrForbidden
	}
	return decision, nil
}

func (authority *collaborationDocumentAuthority) boundedTTL(
	accessExpiresAt time.Time,
	maximum time.Duration,
) (time.Duration, time.Time, error) {
	now := authority.now().UTC().Truncate(time.Millisecond)
	expiresAt := now.Add(maximum)
	if accessExpiresAt.Before(expiresAt) {
		expiresAt = accessExpiresAt.UTC().Truncate(time.Millisecond)
	}
	ttl := expiresAt.Sub(now)
	if ttl < securetoken.MinTTL {
		return 0, time.Time{}, collaboration.ErrForbidden
	}
	return ttl, expiresAt, nil
}

func validCollaborationAuthorityPrincipal(principal collaboration.Principal, room collaboration.Room) bool {
	return principal.WorkspaceID == room.WorkspaceID && principal.WorkspaceID > 0 && principal.UserID > 0 &&
		collaborationResourceIDPattern.MatchString(room.ResourceID) && len(principal.DisplayName) > 0 &&
		len(principal.DisplayName) <= 220 && strings.TrimSpace(principal.DisplayName) == principal.DisplayName &&
		len(principal.LinkAccessSecret) <= 256 && strings.TrimSpace(principal.LinkAccessSecret) == principal.LinkAccessSecret
}

func validCollaborationConnectionID(value string) bool {
	parsed, err := uuid.Parse(value)
	return err == nil && parsed != uuid.Nil && parsed.String() == value
}

func validCollaborationAuthorityTTL(value time.Duration) bool {
	return value >= securetoken.MinTTL && value <= securetoken.MaxTTL
}

func validTicketClaims(claims collaborationTicketClaims, room collaboration.Room, now time.Time) bool {
	return claims.WorkspaceID == room.WorkspaceID && claims.ResourceID == room.ResourceID && claims.UserID > 0 &&
		claims.PolicyRevision > 0 && claims.ExpiresAt > now.UnixMilli() && validCollaborationConnectionID(claims.ConnectionID) &&
		validCollaborationAuthorityPrincipal(principalFromTicket(claims), room)
}

func principalFromTicket(claims collaborationTicketClaims) collaboration.Principal {
	return collaboration.Principal{
		WorkspaceID: claims.WorkspaceID, UserID: claims.UserID, DisplayName: claims.DisplayName,
		ConnectionID: claims.ConnectionID, LinkAccessSecret: claims.LinkSecret,
	}
}

func validGrantClaims(
	claims collaborationGrantClaims,
	room collaboration.Room,
	proof collaborationhttp.AuthorityGrantProof,
	now time.Time,
) bool {
	principal := collaboration.Principal{
		WorkspaceID: claims.WorkspaceID, UserID: claims.UserID, DisplayName: claims.DisplayName,
		ConnectionID: claims.ConnectionID, LinkAccessSecret: claims.LinkSecret,
	}
	return validCollaborationAuthorityPrincipal(principal, room) && validCollaborationConnectionID(claims.ConnectionID) &&
		validCollaborationConnectionID(claims.SocketID) &&
		claims.ResourceID == room.ResourceID && claims.PolicyRevision > 0 && claims.ExpiresAt > now.UnixMilli() &&
		proof.ActorID == strconv.FormatUint(uint64(claims.UserID), 10) && proof.Level == claims.Level &&
		proof.PolicyRevision == claims.PolicyRevision && (claims.Level == "viewer" || claims.Level == "editor")
}

func receiptMatchesCommit(
	receipt collaborationReceiptClaims,
	input collaborationhttp.AuthorityCommitInput,
	grant collaborationGrantClaims,
	principal collaboration.Principal,
	decision collaboration.AccessDecision,
	projectionHash string,
	now time.Time,
) bool {
	return receipt.WorkspaceID == input.Update.Room.WorkspaceID && receipt.ResourceID == input.Update.Room.ResourceID &&
		receipt.GrantHash == collaboration.SHA256Hex([]byte(input.Grant.GrantID)) &&
		receipt.UserID == principal.UserID && receipt.ConnectionID == grant.ConnectionID &&
		receipt.SnapshotHash == input.Update.SnapshotHash && receipt.ProjectionHash == projectionHash &&
		receipt.ReferenceSetHash == authorityReferenceSetHash(input.References) &&
		receipt.PolicyRevision == decision.PolicyRevision && input.Update.PolicyRevision == decision.PolicyRevision &&
		receipt.ExpiresAt > now.UnixMilli()
}

func authorityReferenceSetHash(references collaborationhttp.AuthorityReferenceSet) string {
	assets := append([]string{}, references.AssetIDs...)
	bitables := append([]string{}, references.BitableResourceIDs...)
	sort.Strings(assets)
	sort.Strings(bitables)
	encoded, _ := json.Marshal(collaborationhttp.AuthorityReferenceSet{
		AssetIDs: assets, BitableResourceIDs: bitables,
	})
	return collaboration.SHA256Hex(encoded)
}

func cloneAuthorityInput(
	projection json.RawMessage,
	references collaborationhttp.AuthorityReferenceSet,
) (json.RawMessage, collaborationhttp.AuthorityReferenceSet) {
	return append(json.RawMessage(nil), projection...), collaborationhttp.AuthorityReferenceSet{
		AssetIDs:           append([]string(nil), references.AssetIDs...),
		BitableResourceIDs: append([]string(nil), references.BitableResourceIDs...),
	}
}

func authorityCommitMatches(commit collaboration.DocumentCommit, input collaboration.DocumentUpdate, revision uint64) bool {
	return commit.Room == input.Room && commit.OperationID == input.OperationID &&
		commit.DurableRevision == input.ExpectedDurableRevision+1 && commit.SnapshotHash == input.SnapshotHash &&
		commit.StateVectorHash == input.StateVectorHash && commit.ProjectionHash == input.ProjectionHash &&
		commit.PolicyRevision == revision && !commit.CommittedAt.IsZero()
}

var _ collaborationhttp.ServiceTokenVerifier = (*collaborationServiceTokenVerifier)(nil)
var _ collaborationhttp.DocumentAuthority = (*collaborationDocumentAuthority)(nil)
