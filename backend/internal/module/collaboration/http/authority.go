package collaborationhttp

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/collaboration"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/collaboration/securetoken"
)

const (
	maxAuthorityRequestBytes     = 12 * 1024 * 1024
	CollaborationContractVersion = "1"
	collaborationContractHeader  = "X-PM-Collaboration-Contract"
)

var authorityReferencePattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._:-]{0,127}$`)

type ServiceTokenVerifier interface {
	VerifyCollaborationServiceToken(context.Context, string) error
}

type AuthorityActor struct {
	ID          string `json:"id"`
	DisplayName string `json:"display_name"`
}

type AuthorityGrant struct {
	GrantID          string         `json:"grant_id"`
	Actor            AuthorityActor `json:"actor"`
	Level            string         `json:"level"`
	PolicyRevision   uint64         `json:"policy_revision"`
	ExpiresAt        time.Time      `json:"expires_at"`
	RevalidateAfterM int64          `json:"revalidate_after_ms"`
}

type AuthorityGrantProof struct {
	GrantID        string `json:"grant_id"`
	ActorID        string `json:"actor_id"`
	Level          string `json:"level"`
	PolicyRevision uint64 `json:"policy_revision"`
}

type AuthorityReferenceSet struct {
	AssetIDs           []string `json:"asset_ids"`
	BitableResourceIDs []string `json:"bitable_resource_ids"`
}

type AuthorityValidationReceipt struct {
	Token                 string    `json:"validation_token"`
	CandidateSnapshotHash string    `json:"candidate_snapshot_hash"`
	PolicyRevision        uint64    `json:"policy_revision"`
	ExpiresAt             time.Time `json:"expires_at"`
}

type AuthorityValidateInput struct {
	Room                  collaboration.Room
	Grant                 AuthorityGrantProof
	CandidateSnapshotHash string
	Projection            json.RawMessage
	References            AuthorityReferenceSet
}

type AuthorityCommitInput struct {
	Grant           AuthorityGrantProof
	ValidationToken string
	References      AuthorityReferenceSet
	Update          collaboration.DocumentUpdate
}

// DocumentAuthority is implemented by the Go composition root from the
// canonical ticket/grant authority, DocumentCollaborationService, and the
// schema/reference receipt authority. It is deliberately narrower than a DB
// or resourceaccess repository and never gives Node a seed operation.
type DocumentAuthority interface {
	AuthorizeDocumentConnection(context.Context, collaboration.Room, string, string) (AuthorityGrant, error)
	RevalidateDocumentGrant(context.Context, collaboration.Room, AuthorityGrantProof) (AuthorityGrant, error)
	LoadDocumentForGrant(context.Context, collaboration.Room, AuthorityGrantProof) (collaboration.DocumentState, error)
	ValidateDocumentMutation(context.Context, AuthorityValidateInput) (AuthorityValidationReceipt, error)
	CommitDocumentMutation(context.Context, AuthorityCommitInput) (collaboration.DocumentCommit, error)
}

type AuthorityHandler struct {
	serviceTokens ServiceTokenVerifier
	authority     DocumentAuthority
}

func NewAuthorityHandler(serviceTokens ServiceTokenVerifier, authority DocumentAuthority) (*AuthorityHandler, error) {
	if serviceTokens == nil || authority == nil {
		return nil, collaboration.ErrInvalidInput
	}
	return &AuthorityHandler{serviceTokens: serviceTokens, authority: authority}, nil
}

func (handler *AuthorityHandler) RegisterRoutes(group *gin.RouterGroup) {
	routes := group.Group("/internal/collaboration/v1")
	routes.GET("/health", handler.Health)
	routes.POST("/authorize", handler.Authorize)
	routes.POST("/revalidate", handler.Revalidate)
	routes.POST("/load", handler.Load)
	routes.POST("/validate-mutation", handler.ValidateMutation)
	routes.POST("/commit", handler.Commit)
}

func (handler *AuthorityHandler) Health(c *gin.Context) {
	if !requireCollaborationServiceToken(c, handler.serviceTokens) {
		return
	}
	c.Status(http.StatusNoContent)
}

func (handler *AuthorityHandler) Authorize(c *gin.Context) {
	if !requireCollaborationServiceToken(c, handler.serviceTokens) {
		return
	}
	var request authorizeRequest
	if !bindAuthorityJSON(c, &request) {
		return
	}
	room, err := request.Room.domain()
	if err != nil || strings.TrimSpace(request.ConnectionTicket) == "" ||
		len(request.ConnectionTicket) > securetoken.MaxTokenBytes || strings.TrimSpace(request.ConnectionID) == "" {
		writeAuthorityError(c, collaboration.ErrInvalidInput)
		return
	}
	grant, err := handler.authority.AuthorizeDocumentConnection(
		c.Request.Context(), room, request.ConnectionTicket, request.ConnectionID,
	)
	if err != nil {
		writeAuthorityError(c, err)
		return
	}
	if !grant.valid() {
		writeAuthorityError(c, collaboration.ErrDataCorrupt)
		return
	}
	c.JSON(http.StatusOK, gin.H{"grant": grant})
}

func (handler *AuthorityHandler) Revalidate(c *gin.Context) {
	if !requireCollaborationServiceToken(c, handler.serviceTokens) {
		return
	}
	var request grantRequest
	if !bindAuthorityJSON(c, &request) {
		return
	}
	room, err := request.Room.domain()
	if err != nil || !request.Grant.valid() {
		writeAuthorityError(c, collaboration.ErrInvalidInput)
		return
	}
	grant, err := handler.authority.RevalidateDocumentGrant(c.Request.Context(), room, request.Grant)
	if err != nil {
		writeAuthorityError(c, err)
		return
	}
	if !grant.valid() {
		writeAuthorityError(c, collaboration.ErrDataCorrupt)
		return
	}
	c.JSON(http.StatusOK, gin.H{"grant": grant})
}

func (handler *AuthorityHandler) Load(c *gin.Context) {
	if !requireCollaborationServiceToken(c, handler.serviceTokens) {
		return
	}
	var request grantRequest
	if !bindAuthorityJSON(c, &request) {
		return
	}
	room, err := request.Room.domain()
	if err != nil || !request.Grant.valid() {
		writeAuthorityError(c, collaboration.ErrInvalidInput)
		return
	}
	state, err := handler.authority.LoadDocumentForGrant(c.Request.Context(), room, request.Grant)
	if err != nil {
		writeAuthorityError(c, err)
		return
	}
	if !validAuthorityState(state, room) {
		writeAuthorityError(c, collaboration.ErrDataCorrupt)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"snapshot_base64":     base64.StdEncoding.EncodeToString(state.Snapshot),
		"durable_revision":    state.DurableRevision,
		"snapshot_hash":       state.SnapshotHash,
		"state_vector_base64": base64.StdEncoding.EncodeToString(state.StateVector),
		"state_vector_hash":   state.StateVectorHash,
		"projection_hash":     state.ProjectionHash,
	})
}

func (handler *AuthorityHandler) ValidateMutation(c *gin.Context) {
	if !requireCollaborationServiceToken(c, handler.serviceTokens) {
		return
	}
	var request validateRequest
	if !bindAuthorityJSON(c, &request) {
		return
	}
	input, err := request.domain()
	if err != nil {
		writeAuthorityError(c, err)
		return
	}
	receipt, err := handler.authority.ValidateDocumentMutation(c.Request.Context(), input)
	if err != nil {
		writeAuthorityError(c, err)
		return
	}
	if strings.TrimSpace(receipt.Token) == "" || receipt.CandidateSnapshotHash != input.CandidateSnapshotHash ||
		receipt.PolicyRevision == 0 || !receipt.ExpiresAt.After(time.Now()) {
		writeAuthorityError(c, collaboration.ErrDataCorrupt)
		return
	}
	c.JSON(http.StatusOK, receipt)
}

func (handler *AuthorityHandler) Commit(c *gin.Context) {
	if !requireCollaborationServiceToken(c, handler.serviceTokens) {
		return
	}
	var request commitRequest
	if !bindAuthorityJSON(c, &request) {
		return
	}
	input, err := request.domain()
	if err != nil {
		writeAuthorityError(c, err)
		return
	}
	commit, err := handler.authority.CommitDocumentMutation(c.Request.Context(), input)
	if err != nil {
		writeAuthorityError(c, err)
		return
	}
	if !validAuthorityCommit(commit, input.Update) {
		writeAuthorityError(c, collaboration.ErrDataCorrupt)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"commit_id": commit.OperationID, "durable_revision": commit.DurableRevision,
		"snapshot_hash": commit.SnapshotHash, "state_vector_hash": commit.StateVectorHash,
		"projection_hash": commit.ProjectionHash, "stored_at": commit.CommittedAt,
	})
}

type roomDTO struct {
	WorkspaceID string `json:"workspace_id"`
	ResourceID  string `json:"resource_id"`
}

func (room roomDTO) domain() (collaboration.Room, error) {
	workspace, err := strconv.ParseUint(room.WorkspaceID, 10, 64)
	if err != nil || workspace == 0 || uint64(uint(workspace)) != workspace || len(room.ResourceID) == 0 ||
		len(room.ResourceID) > 64 || strings.TrimSpace(room.ResourceID) != room.ResourceID {
		return collaboration.Room{}, collaboration.ErrInvalidInput
	}
	return collaboration.Room{WorkspaceID: uint(workspace), ResourceID: room.ResourceID}, nil
}

type authorizeRequest struct {
	Room             roomDTO `json:"room"`
	ConnectionTicket string  `json:"connection_ticket"`
	ConnectionID     string  `json:"connection_id"`
}

type grantRequest struct {
	Room  roomDTO             `json:"room"`
	Grant AuthorityGrantProof `json:"grant"`
}

type validateRequest struct {
	Room                  roomDTO               `json:"room"`
	Grant                 AuthorityGrantProof   `json:"grant"`
	CandidateSnapshotHash string                `json:"candidate_snapshot_hash"`
	Projection            json.RawMessage       `json:"projection"`
	References            AuthorityReferenceSet `json:"references"`
}

func (request validateRequest) domain() (AuthorityValidateInput, error) {
	room, err := request.Room.domain()
	if err != nil || !request.Grant.valid() || !collaboration.IsSHA256Hex(request.CandidateSnapshotHash) ||
		!validAuthorityReferences(request.References) {
		return AuthorityValidateInput{}, collaboration.ErrInvalidInput
	}
	if _, err = collaboration.CanonicalJSONHash(request.Projection); err != nil {
		return AuthorityValidateInput{}, err
	}
	return AuthorityValidateInput{
		Room: room, Grant: request.Grant, CandidateSnapshotHash: request.CandidateSnapshotHash,
		Projection: append(json.RawMessage(nil), request.Projection...), References: request.References,
	}, nil
}

type commitRequest struct {
	Room                    roomDTO               `json:"room"`
	Grant                   AuthorityGrantProof   `json:"grant"`
	ValidationToken         string                `json:"validation_token"`
	OperationID             string                `json:"operation_id"`
	ExpectedDurableRevision uint64                `json:"expected_durable_revision"`
	BaseStateVectorHash     string                `json:"base_state_vector_hash"`
	UpdateBase64            string                `json:"update_base64"`
	UpdateHash              string                `json:"update_hash"`
	CandidateSnapshotHash   string                `json:"candidate_snapshot_hash"`
	SnapshotBase64          string                `json:"snapshot_base64"`
	StateVectorBase64       string                `json:"state_vector_base64"`
	StateVectorHash         string                `json:"state_vector_hash"`
	Projection              json.RawMessage       `json:"projection"`
	References              AuthorityReferenceSet `json:"references"`
}

func (request commitRequest) domain() (AuthorityCommitInput, error) {
	room, err := request.Room.domain()
	update, updateErr := decodeAuthorityBytes(request.UpdateBase64, collaboration.MaxDocumentUpdateBytes)
	snapshot, snapshotErr := decodeAuthorityBytes(request.SnapshotBase64, collaboration.MaxDocumentSnapshotBytes)
	vector, vectorErr := decodeAuthorityBytes(request.StateVectorBase64, collaboration.MaxDocumentStateVectorBytes)
	projectionHash, projectionErr := collaboration.CanonicalJSONHash(request.Projection)
	input := collaboration.DocumentUpdate{
		Room: room, OperationID: request.OperationID, ExpectedDurableRevision: request.ExpectedDurableRevision,
		BaseStateVectorHash: request.BaseStateVectorHash, Update: update, UpdateHash: request.UpdateHash,
		Snapshot: snapshot, SnapshotHash: request.CandidateSnapshotHash, StateVector: vector,
		StateVectorHash: request.StateVectorHash, Projection: append(json.RawMessage(nil), request.Projection...),
		ProjectionHash: projectionHash, ActorUserID: 1, PolicyRevision: request.Grant.PolicyRevision,
	}
	if err != nil || updateErr != nil || snapshotErr != nil || vectorErr != nil || projectionErr != nil ||
		strings.TrimSpace(request.ValidationToken) == "" || len(request.ValidationToken) > securetoken.MaxTokenBytes ||
		!request.Grant.valid() ||
		!validAuthorityReferences(request.References) || collaboration.ValidateDocumentUpdate(input) != nil {
		return AuthorityCommitInput{}, collaboration.ErrInvalidInput
	}
	return AuthorityCommitInput{
		Grant: request.Grant, ValidationToken: request.ValidationToken,
		References: request.References, Update: input,
	}, nil
}

func (proof AuthorityGrantProof) valid() bool {
	return len(proof.GrantID) > 0 && len(proof.GrantID) <= securetoken.MaxTokenBytes &&
		strings.TrimSpace(proof.GrantID) == proof.GrantID &&
		len(proof.ActorID) > 0 && len(proof.ActorID) <= 128 && strings.TrimSpace(proof.ActorID) == proof.ActorID &&
		(proof.Level == "viewer" || proof.Level == "editor") && proof.PolicyRevision > 0
}

func (grant AuthorityGrant) valid() bool {
	return AuthorityGrantProof{
		GrantID: grant.GrantID, ActorID: grant.Actor.ID, Level: grant.Level, PolicyRevision: grant.PolicyRevision,
	}.valid() && len(grant.Actor.DisplayName) > 0 && len(grant.Actor.DisplayName) <= 220 &&
		grant.ExpiresAt.After(time.Now()) && grant.RevalidateAfterM > 0
}

func validAuthorityReferences(references AuthorityReferenceSet) bool {
	if len(references.AssetIDs) > 1024 || len(references.BitableResourceIDs) > 1024 {
		return false
	}
	for _, values := range [][]string{references.AssetIDs, references.BitableResourceIDs} {
		for _, value := range values {
			if !authorityReferencePattern.MatchString(value) {
				return false
			}
		}
	}
	return true
}

func validAuthorityState(state collaboration.DocumentState, room collaboration.Room) bool {
	projectionHash, err := collaboration.CanonicalJSONHash(state.Projection)
	return err == nil && state.Room == room && state.DurableRevision > 0 &&
		len(state.Snapshot) > 0 && len(state.Snapshot) <= collaboration.MaxDocumentSnapshotBytes &&
		state.SnapshotHash == collaboration.SHA256Hex(state.Snapshot) &&
		len(state.StateVector) > 0 && len(state.StateVector) <= collaboration.MaxDocumentStateVectorBytes &&
		state.StateVectorHash == collaboration.SHA256Hex(state.StateVector) && state.ProjectionHash == projectionHash
}

func validAuthorityCommit(commit collaboration.DocumentCommit, input collaboration.DocumentUpdate) bool {
	return commit.Room == input.Room && commit.OperationID == input.OperationID &&
		commit.DurableRevision == input.ExpectedDurableRevision+1 && commit.SnapshotHash == input.SnapshotHash &&
		commit.StateVectorHash == input.StateVectorHash && commit.ProjectionHash == input.ProjectionHash &&
		commit.PolicyRevision > 0 && !commit.CommittedAt.IsZero()
}

func decodeAuthorityBytes(value string, maximum int) ([]byte, error) {
	decoded, err := base64.StdEncoding.Strict().DecodeString(value)
	if err != nil || len(decoded) == 0 || len(decoded) > maximum {
		return nil, collaboration.ErrInvalidInput
	}
	return decoded, nil
}

func bindAuthorityJSON(c *gin.Context, target any) bool {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxAuthorityRequestBytes)
	decoder := json.NewDecoder(c.Request.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		writeAuthorityError(c, collaboration.ErrInvalidInput)
		return false
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		writeAuthorityError(c, collaboration.ErrInvalidInput)
		return false
	}
	return true
}

func requireCollaborationServiceToken(c *gin.Context, verifier ServiceTokenVerifier) bool {
	header := c.GetHeader("Authorization")
	const prefix = "Bearer "
	if !strings.HasPrefix(header, prefix) || strings.TrimSpace(strings.TrimPrefix(header, prefix)) == "" ||
		verifier.VerifyCollaborationServiceToken(c.Request.Context(), strings.TrimPrefix(header, prefix)) != nil {
		c.Header("X-PM-Error-Code", "invalid-service-identity")
		c.AbortWithStatus(http.StatusUnauthorized)
		return false
	}
	if c.GetHeader(collaborationContractHeader) != CollaborationContractVersion {
		c.Header("X-PM-Error-Code", "unsupported-contract-version")
		c.AbortWithStatus(http.StatusPreconditionFailed)
		return false
	}
	return true
}

func writeAuthorityError(c *gin.Context, err error) {
	status, code := collaborationError(err)
	c.Header("X-PM-Error-Code", code)
	c.AbortWithStatusJSON(status, gin.H{"error": gin.H{"code": code}})
}
