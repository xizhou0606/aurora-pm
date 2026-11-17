package collaborationhttp

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/collaboration"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/collaboration/securetoken"
)

type serviceTokenVerifierFake struct{ calls int }

func (fake *serviceTokenVerifierFake) VerifyCollaborationServiceToken(_ context.Context, token string) error {
	fake.calls++
	if token != "service-token" {
		return collaboration.ErrForbidden
	}
	return nil
}

type documentAuthorityFake struct {
	calls      []string
	room       collaboration.Room
	commit     AuthorityCommitInput
	returnErr  error
	projection json.RawMessage
}

func (fake *documentAuthorityFake) AuthorizeDocumentConnection(
	_ context.Context,
	room collaboration.Room,
	_, _ string,
) (AuthorityGrant, error) {
	fake.calls = append(fake.calls, "authorize")
	fake.room = room
	return authorityGrant(), fake.returnErr
}

func (fake *documentAuthorityFake) RevalidateDocumentGrant(
	_ context.Context,
	room collaboration.Room,
	_ AuthorityGrantProof,
) (AuthorityGrant, error) {
	fake.calls = append(fake.calls, "revalidate")
	fake.room = room
	return authorityGrant(), fake.returnErr
}

func (fake *documentAuthorityFake) LoadDocumentForGrant(
	_ context.Context,
	room collaboration.Room,
	_ AuthorityGrantProof,
) (collaboration.DocumentState, error) {
	fake.calls = append(fake.calls, "load")
	fake.room = room
	projection := []byte(`{"type":"doc"}`)
	projectionHash, _ := collaboration.CanonicalJSONHash(projection)
	return collaboration.DocumentState{
		Room: room, DurableRevision: 2,
		Snapshot: []byte("snapshot"), SnapshotHash: collaboration.SHA256Hex([]byte("snapshot")),
		StateVector: []byte("vector"), StateVectorHash: collaboration.SHA256Hex([]byte("vector")),
		Projection: projection, ProjectionHash: projectionHash,
	}, fake.returnErr
}

func (fake *documentAuthorityFake) ValidateDocumentMutation(
	_ context.Context,
	input AuthorityValidateInput,
) (AuthorityValidationReceipt, error) {
	fake.calls = append(fake.calls, "validate")
	fake.room, fake.projection = input.Room, input.Projection
	return AuthorityValidationReceipt{
		Token: "receipt", CandidateSnapshotHash: input.CandidateSnapshotHash,
		PolicyRevision: 9, ExpiresAt: time.Unix(2_000_000_000, 0).UTC(),
	}, fake.returnErr
}

func (fake *documentAuthorityFake) CommitDocumentMutation(
	_ context.Context,
	input AuthorityCommitInput,
) (collaboration.DocumentCommit, error) {
	fake.calls = append(fake.calls, "commit")
	fake.room, fake.commit = input.Update.Room, input
	return collaboration.DocumentCommit{
		Room: input.Update.Room, OperationID: input.Update.OperationID, DurableRevision: 3,
		SnapshotHash: input.Update.SnapshotHash, StateVectorHash: input.Update.StateVectorHash,
		ProjectionHash: input.Update.ProjectionHash, PolicyRevision: 9,
		CommittedAt: time.Unix(2_000_000_000, 0).UTC(),
	}, fake.returnErr
}

func authorityGrant() AuthorityGrant {
	return AuthorityGrant{
		GrantID: "grant", Actor: AuthorityActor{ID: "7", DisplayName: "Alice"}, Level: "editor",
		PolicyRevision: 9, ExpiresAt: time.Unix(2_000_000_000, 0).UTC(), RevalidateAfterM: 1000,
	}
}

func authorityProofJSON() string {
	return `{"grant_id":"grant","actor_id":"7","level":"editor","policy_revision":9}`
}

func authorityRequest(engine http.Handler, method, path, body, token string) *httptest.ResponseRecorder {
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(method, path, strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set(collaborationContractHeader, CollaborationContractVersion)
	if token != "" {
		request.Header.Set("Authorization", "Bearer "+token)
	}
	engine.ServeHTTP(recorder, request)
	return recorder
}

func TestAuthorityRoutesRequireExactContractVersionAfterServiceIdentity(t *testing.T) {
	gin.SetMode(gin.TestMode)
	authority := &documentAuthorityFake{}
	handler, _ := NewAuthorityHandler(&serviceTokenVerifierFake{}, authority)
	engine := gin.New()
	handler.RegisterRoutes(engine.Group(""))
	for _, version := range []string{"", "2", " 1"} {
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodGet, "/internal/collaboration/v1/health", nil)
		request.Header.Set("Authorization", "Bearer service-token")
		if version != "" {
			request.Header.Set(collaborationContractHeader, version)
		}
		engine.ServeHTTP(recorder, request)
		if recorder.Code != http.StatusPreconditionFailed || recorder.Header().Get("X-PM-Error-Code") != "unsupported-contract-version" {
			t.Fatalf("version=%q status=%d headers=%v", version, recorder.Code, recorder.Header())
		}
	}
}

func TestAuthorityRoutesVerifyServiceIdentityAndMapContract(t *testing.T) {
	gin.SetMode(gin.TestMode)
	verifier := &serviceTokenVerifierFake{}
	authority := &documentAuthorityFake{}
	handler, err := NewAuthorityHandler(verifier, authority)
	if err != nil {
		t.Fatal(err)
	}
	engine := gin.New()
	handler.RegisterRoutes(engine.Group(""))
	room := `{"workspace_id":"11","resource_id":"resource-1"}`
	grant := authorityProofJSON()
	projection := `{"type":"doc"}`
	projectionHash, _ := collaboration.CanonicalJSONHash([]byte(projection))
	snapshot, update, vector := []byte("snapshot"), []byte("update"), []byte("vector")
	cases := []struct {
		method string
		path   string
		body   string
		status int
	}{
		{http.MethodGet, "/internal/collaboration/v1/health", "", http.StatusNoContent},
		{http.MethodPost, "/internal/collaboration/v1/authorize", `{"room":` + room + `,"connection_ticket":"ticket","connection_id":"connection"}`, http.StatusOK},
		{http.MethodPost, "/internal/collaboration/v1/revalidate", `{"room":` + room + `,"grant":` + grant + `}`, http.StatusOK},
		{http.MethodPost, "/internal/collaboration/v1/load", `{"room":` + room + `,"grant":` + grant + `}`, http.StatusOK},
		{http.MethodPost, "/internal/collaboration/v1/validate-mutation", `{"room":` + room + `,"grant":` + grant + `,"candidate_snapshot_hash":"` + collaboration.SHA256Hex(snapshot) + `","projection":` + projection + `,"references":{"asset_ids":[],"bitable_resource_ids":[]}}`, http.StatusOK},
		{http.MethodPost, "/internal/collaboration/v1/commit", `{"room":` + room + `,"grant":` + grant + `,"validation_token":"receipt","operation_id":"operation-1","expected_durable_revision":2,"base_state_vector_hash":"` + collaboration.SHA256Hex([]byte("base-vector")) + `","update_base64":"` + base64.StdEncoding.EncodeToString(update) + `","update_hash":"` + collaboration.SHA256Hex(update) + `","candidate_snapshot_hash":"` + collaboration.SHA256Hex(snapshot) + `","snapshot_base64":"` + base64.StdEncoding.EncodeToString(snapshot) + `","state_vector_base64":"` + base64.StdEncoding.EncodeToString(vector) + `","state_vector_hash":"` + collaboration.SHA256Hex(vector) + `","projection":` + projection + `,"references":{"asset_ids":[],"bitable_resource_ids":[]}}`, http.StatusOK},
	}
	for _, test := range cases {
		response := authorityRequest(engine, test.method, test.path, test.body, "service-token")
		if response.Code != test.status {
			t.Fatalf("%s status=%d body=%s", test.path, response.Code, response.Body.String())
		}
	}
	if verifier.calls != len(cases) || strings.Join(authority.calls, ",") != "authorize,revalidate,load,validate,commit" {
		t.Fatalf("verifier=%d authority=%v", verifier.calls, authority.calls)
	}
	if authority.room != (collaboration.Room{WorkspaceID: 11, ResourceID: "resource-1"}) ||
		string(authority.commit.Update.Update) != string(update) || authority.commit.Update.ProjectionHash != projectionHash {
		t.Fatalf("decoded authority input = room:%#v commit:%#v", authority.room, authority.commit)
	}
	if response := authorityRequest(engine, http.MethodPost, "/internal/collaboration/v1/load", `{}`, ""); response.Code != http.StatusUnauthorized || len(authority.calls) != 5 {
		t.Fatalf("unauthenticated request = %d calls=%v", response.Code, authority.calls)
	}
	if response := authorityRequest(engine, http.MethodPost, "/internal/collaboration/v1/validate", `{}`, "service-token"); response.Code != http.StatusNotFound {
		t.Fatalf("obsolete validate route status=%d", response.Code)
	}
}

func TestAuthorityHandlerRejectsUnknownAndOversizedPayloadBeforeAuthority(t *testing.T) {
	gin.SetMode(gin.TestMode)
	authority := &documentAuthorityFake{}
	handler, _ := NewAuthorityHandler(&serviceTokenVerifierFake{}, authority)
	engine := gin.New()
	handler.RegisterRoutes(engine.Group(""))
	unknown := `{"room":{"workspace_id":"11","resource_id":"resource"},"connection_ticket":"ticket","connection_id":"connection","actor_id":"forged"}`
	if response := authorityRequest(engine, http.MethodPost, "/internal/collaboration/v1/authorize", unknown, "service-token"); response.Code != http.StatusBadRequest {
		t.Fatalf("unknown DTO status=%d body=%s", response.Code, response.Body.String())
	}
	oversized := strings.Repeat("A", maxAuthorityRequestBytes+1)
	if response := authorityRequest(engine, http.MethodPost, "/internal/collaboration/v1/commit", oversized, "service-token"); response.Code != http.StatusBadRequest {
		t.Fatalf("oversized DTO status=%d", response.Code)
	}
	if len(authority.calls) != 0 {
		t.Fatalf("authority called for rejected payload: %v", authority.calls)
	}
}

func TestAuthorityHandlerMapsConflictForNodeBridge(t *testing.T) {
	gin.SetMode(gin.TestMode)
	authority := &documentAuthorityFake{returnErr: collaboration.ErrConflict}
	handler, _ := NewAuthorityHandler(&serviceTokenVerifierFake{}, authority)
	engine := gin.New()
	handler.RegisterRoutes(engine.Group(""))
	body := `{"room":{"workspace_id":"11","resource_id":"resource"},"connection_ticket":"ticket","connection_id":"connection"}`
	response := authorityRequest(engine, http.MethodPost, "/internal/collaboration/v1/authorize", body, "service-token")
	if response.Code != http.StatusConflict || response.Header().Get("X-PM-Error-Code") != "conflict" || !errors.Is(authority.returnErr, collaboration.ErrConflict) {
		t.Fatalf("conflict response=%d headers=%v", response.Code, response.Header())
	}
}

func TestAuthorityGrantProofAcceptsOpaqueEnvelopeBounds(t *testing.T) {
	proof := AuthorityGrantProof{
		GrantID: strings.Repeat("g", 1024), ActorID: "7", Level: "editor", PolicyRevision: 9,
	}
	if !proof.valid() {
		t.Fatal("valid encrypted grant-sized proof was rejected")
	}
	proof.GrantID = strings.Repeat("g", securetoken.MaxTokenBytes+1)
	if proof.valid() {
		t.Fatal("oversized grant proof was accepted")
	}
}
