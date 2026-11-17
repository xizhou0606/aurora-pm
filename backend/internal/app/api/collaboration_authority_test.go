package api

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/xizhou0606/aurora-pm/backend/internal/module/collaboration"
	collaborationhttp "github.com/xizhou0606/aurora-pm/backend/internal/module/collaboration/http"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/collaboration/securetoken"
)

const (
	authorityConnectionID = "3f47ac10-b42c-4d44-9551-391ab5aab956"
	clientConnectionID    = "5730ba85-6a1b-4230-9b0d-36a9d97be269"
)

type authorityClock struct{ milliseconds atomic.Int64 }

func newAuthorityClock(now time.Time) *authorityClock {
	clock := &authorityClock{}
	clock.milliseconds.Store(now.UnixMilli())
	return clock
}

func (clock *authorityClock) Now() time.Time {
	return time.UnixMilli(clock.milliseconds.Load()).UTC()
}

func (clock *authorityClock) Advance(duration time.Duration) {
	clock.milliseconds.Add(duration.Milliseconds())
}

type authorityAccessFake struct {
	decision   collaboration.AccessDecision
	err        error
	calls      int
	principals []collaboration.Principal
}

func (fake *authorityAccessFake) EvaluateDocumentCollaborationAccess(
	_ context.Context,
	principal collaboration.Principal,
	_ collaboration.Room,
) (collaboration.AccessDecision, error) {
	fake.calls++
	fake.principals = append(fake.principals, principal)
	return fake.decision, fake.err
}

type authorityDocumentsFake struct {
	clock       *authorityClock
	loadCalls   int
	commitCalls int
	principal   collaboration.Principal
	update      collaboration.DocumentUpdate
	commit      collaboration.DocumentCommit
	err         error
}

func (fake *authorityDocumentsFake) Load(
	_ context.Context,
	principal collaboration.Principal,
	room collaboration.Room,
) (collaboration.DocumentState, error) {
	fake.loadCalls++
	fake.principal = principal
	return collaboration.DocumentState{Room: room, DurableRevision: 1}, fake.err
}

func (fake *authorityDocumentsFake) Commit(
	_ context.Context,
	principal collaboration.Principal,
	input collaboration.DocumentUpdate,
) (collaboration.DocumentCommit, error) {
	fake.commitCalls++
	fake.principal, fake.update = principal, input
	if fake.err != nil {
		return collaboration.DocumentCommit{}, fake.err
	}
	if !fake.commit.CommittedAt.IsZero() {
		return fake.commit, nil
	}
	return collaboration.DocumentCommit{
		Room: input.Room, OperationID: input.OperationID,
		DurableRevision: input.ExpectedDurableRevision + 1,
		SnapshotHash:    input.SnapshotHash, StateVectorHash: input.StateVectorHash,
		ProjectionHash: input.ProjectionHash, PolicyRevision: input.PolicyRevision,
		CommittedAt: fake.clock.Now(),
	}, nil
}

type authorityProjectionFake struct {
	calls      int
	principal  collaboration.Principal
	projection json.RawMessage
	references collaborationhttp.AuthorityReferenceSet
	err        error
}

func (fake *authorityProjectionFake) ValidateCollaborationProjection(
	_ context.Context,
	principal collaboration.Principal,
	_ collaboration.Room,
	projection json.RawMessage,
	references collaborationhttp.AuthorityReferenceSet,
) error {
	fake.calls++
	fake.principal = principal
	fake.projection = append(json.RawMessage(nil), projection...)
	fake.references = collaborationhttp.AuthorityReferenceSet{
		AssetIDs:           append([]string(nil), references.AssetIDs...),
		BitableResourceIDs: append([]string(nil), references.BitableResourceIDs...),
	}
	return fake.err
}

type authorityHarness struct {
	authority  *collaborationDocumentAuthority
	clock      *authorityClock
	access     *authorityAccessFake
	documents  *authorityDocumentsFake
	projection *authorityProjectionFake
	room       collaboration.Room
	principal  collaboration.Principal
}

func newAuthorityHarness(t *testing.T) *authorityHarness {
	t.Helper()
	clock := newAuthorityClock(time.Date(2026, 9, 8, 12, 0, 0, 0, time.UTC))
	tokens, err := securetoken.New(
		[]byte("authority-proof-key-32-bytes!!!!"),
		securetoken.WithClock(clock.Now),
	)
	if err != nil {
		t.Fatal(err)
	}
	access := &authorityAccessFake{decision: collaboration.AccessDecision{
		CanView: true, CanEdit: true, PolicyRevision: 9, ExpiresAt: clock.Now().Add(time.Minute),
	}}
	documents := &authorityDocumentsFake{clock: clock}
	projection := &authorityProjectionFake{}
	authority, err := newCollaborationDocumentAuthority(
		tokens, access, documents, projection,
		collaborationTicketTTL, collaborationGrantTTL, collaborationReceiptTTL, clock.Now,
	)
	if err != nil {
		t.Fatal(err)
	}
	return &authorityHarness{
		authority: authority, clock: clock, access: access, documents: documents, projection: projection,
		room: collaboration.Room{WorkspaceID: 11, ResourceID: "resource-1"},
		principal: collaboration.Principal{
			WorkspaceID: 11, UserID: 7, DisplayName: "Alice", ConnectionID: clientConnectionID,
			LinkAccessSecret: "resource-link-secret",
		},
	}
}

func (harness *authorityHarness) grant(t *testing.T) collaborationhttp.AuthorityGrant {
	t.Helper()
	ticket, _, err := harness.authority.IssueDocumentConnectionTicket(
		context.Background(), harness.principal, harness.room,
	)
	if err != nil {
		t.Fatal(err)
	}
	grant, err := harness.authority.AuthorizeDocumentConnection(
		context.Background(), harness.room, ticket, authorityConnectionID,
	)
	if err != nil {
		t.Fatal(err)
	}
	return grant
}

func proofFromGrant(grant collaborationhttp.AuthorityGrant) collaborationhttp.AuthorityGrantProof {
	return collaborationhttp.AuthorityGrantProof{
		GrantID: grant.GrantID, ActorID: grant.Actor.ID,
		Level: grant.Level, PolicyRevision: grant.PolicyRevision,
	}
}

func TestCollaborationAuthorityEncryptedLifecycle(t *testing.T) {
	harness := newAuthorityHarness(t)
	ticket, expiresAt, err := harness.authority.IssueDocumentConnectionTicket(
		context.Background(), harness.principal, harness.room,
	)
	if err != nil {
		t.Fatal(err)
	}
	if !expiresAt.Equal(harness.clock.Now().Add(collaborationTicketTTL)) {
		t.Fatalf("ticket expiry = %v", expiresAt)
	}
	assertOpaqueProof(t, ticket, "resource-link-secret", "resource-1", "Alice", clientConnectionID)
	grant, err := harness.authority.AuthorizeDocumentConnection(
		context.Background(), harness.room, ticket, authorityConnectionID,
	)
	if err != nil {
		t.Fatal(err)
	}
	assertOpaqueProof(t, grant.GrantID, "resource-link-secret", authorityConnectionID)
	if grant.Actor.ID != "7" || grant.Actor.DisplayName != "Alice" || grant.Level != "editor" ||
		grant.PolicyRevision != 9 || grant.RevalidateAfterM <= 0 || !grant.ExpiresAt.After(harness.clock.Now()) {
		t.Fatalf("grant = %#v", grant)
	}
	proof := proofFromGrant(grant)
	if _, err = harness.authority.LoadDocumentForGrant(context.Background(), harness.room, proof); err != nil {
		t.Fatal(err)
	}
	if harness.documents.principal.ConnectionID != clientConnectionID ||
		harness.documents.principal.LinkAccessSecret != "resource-link-secret" {
		t.Fatalf("load principal = %#v", harness.documents.principal)
	}

	update := validAuthorityUpdate(harness.room)
	references := collaborationhttp.AuthorityReferenceSet{
		AssetIDs: []string{"asset-b", "asset-a"}, BitableResourceIDs: []string{"bitable-1"},
	}
	receipt, err := harness.authority.ValidateDocumentMutation(context.Background(), collaborationhttp.AuthorityValidateInput{
		Room: harness.room, Grant: proof, CandidateSnapshotHash: update.SnapshotHash,
		Projection: update.Projection, References: references,
	})
	if err != nil {
		t.Fatal(err)
	}
	assertOpaqueProof(t, receipt.Token, update.SnapshotHash, update.ProjectionHash, "asset-a")
	// Reference order is not authority: the receipt binds the normalized set.
	commit, err := harness.authority.CommitDocumentMutation(context.Background(), collaborationhttp.AuthorityCommitInput{
		Grant: proof, ValidationToken: receipt.Token,
		References: collaborationhttp.AuthorityReferenceSet{
			AssetIDs: []string{"asset-a", "asset-b"}, BitableResourceIDs: []string{"bitable-1"},
		},
		Update: update,
	})
	if err != nil {
		t.Fatal(err)
	}
	if commit.PolicyRevision != 9 || harness.documents.commitCalls != 1 || harness.projection.calls != 2 ||
		harness.documents.update.ActorUserID != 7 || harness.documents.update.PolicyRevision != 9 {
		t.Fatalf("commit=%#v documents=%#v projection calls=%d", commit, harness.documents.update, harness.projection.calls)
	}
	if harness.projection.principal.LinkAccessSecret != "resource-link-secret" {
		t.Fatalf("projection principal = %#v", harness.projection.principal)
	}
}

func TestActiveCollaborationTicketIssuerRequiresDurableState(t *testing.T) {
	harness := newAuthorityHarness(t)
	issuer := activeCollaborationTicketIssuer{
		authority: harness.authority, documents: harness.documents,
		publicURL: "wss://collaboration.example.test/socket",
	}
	ticket, err := issuer.IssueDocumentConnectionTicket(
		context.Background(), harness.principal, harness.room,
	)
	if err != nil {
		t.Fatal(err)
	}
	if ticket.Ticket == "" || ticket.WebSocketURL != "wss://collaboration.example.test/socket" ||
		!ticket.ExpiresAt.Equal(harness.clock.Now().Add(collaborationTicketTTL)) || harness.documents.loadCalls != 1 {
		t.Fatalf("ticket=%+v load calls=%d", ticket, harness.documents.loadCalls)
	}
	harness.documents.err = collaboration.ErrNotFound
	if _, err = issuer.IssueDocumentConnectionTicket(
		context.Background(), harness.principal, harness.room,
	); !errors.Is(err, collaboration.ErrNotFound) {
		t.Fatalf("unseeded ticket error=%v", err)
	}
}

func TestCollaborationAuthorityRejectsInvalidTicketsAndConnection(t *testing.T) {
	harness := newAuthorityHarness(t)
	ticket, _, err := harness.authority.IssueDocumentConnectionTicket(context.Background(), harness.principal, harness.room)
	if err != nil {
		t.Fatal(err)
	}
	tampered := mutateToken(ticket)
	wrongRoom := collaboration.Room{WorkspaceID: 11, ResourceID: "resource-2"}
	cases := []struct {
		name       string
		room       collaboration.Room
		ticket     string
		connection string
	}{
		{name: "tampered", room: harness.room, ticket: tampered, connection: authorityConnectionID},
		{name: "wrong room", room: wrongRoom, ticket: ticket, connection: authorityConnectionID},
		{name: "non UUID", room: harness.room, ticket: ticket, connection: "connection"},
		{name: "non canonical UUID", room: harness.room, ticket: ticket, connection: strings.ToUpper(authorityConnectionID)},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			if _, err := harness.authority.AuthorizeDocumentConnection(
				context.Background(), test.room, test.ticket, test.connection,
			); !errors.Is(err, collaboration.ErrForbidden) {
				t.Fatalf("error = %v", err)
			}
		})
	}
	harness.clock.Advance(collaborationTicketTTL + time.Millisecond)
	if _, err = harness.authority.AuthorizeDocumentConnection(
		context.Background(), harness.room, ticket, authorityConnectionID,
	); !errors.Is(err, collaboration.ErrForbidden) {
		t.Fatalf("expired ticket error = %v", err)
	}
}

func TestCollaborationAuthorityValidatesConfiguredTTLAndCapsPolicyWindow(t *testing.T) {
	harness := newAuthorityHarness(t)
	if _, err := newCollaborationDocumentAuthority(
		harness.authority.tokens, harness.access, harness.documents, harness.projection,
		0, collaborationGrantTTL, collaborationReceiptTTL, harness.clock.Now,
	); !errors.Is(err, collaboration.ErrInvalidInput) {
		t.Fatalf("zero ttl error = %v", err)
	}
	if _, err := newCollaborationDocumentAuthority(
		harness.authority.tokens, harness.access, harness.documents, harness.projection,
		securetoken.MaxTTL+time.Millisecond, collaborationGrantTTL, collaborationReceiptTTL, harness.clock.Now,
	); !errors.Is(err, collaboration.ErrInvalidInput) {
		t.Fatalf("oversized ttl error = %v", err)
	}

	harness.access.decision.ExpiresAt = harness.clock.Now().Add(2 * time.Second)
	ticket, expiresAt, err := harness.authority.IssueDocumentConnectionTicket(context.Background(), harness.principal, harness.room)
	if err != nil {
		t.Fatal(err)
	}
	if !expiresAt.Equal(harness.access.decision.ExpiresAt) {
		t.Fatalf("policy-capped expiry = %v", expiresAt)
	}
	harness.clock.Advance(2*time.Second + time.Millisecond)
	if _, err = harness.authority.AuthorizeDocumentConnection(
		context.Background(), harness.room, ticket, authorityConnectionID,
	); !errors.Is(err, collaboration.ErrForbidden) {
		t.Fatalf("policy-capped ticket error = %v", err)
	}
}

func TestCollaborationAuthorityRevalidatesCurrentPolicyAndDowngrades(t *testing.T) {
	harness := newAuthorityHarness(t)
	grant := harness.grant(t)
	oldProof := proofFromGrant(grant)
	harness.access.decision.PolicyRevision = 10
	harness.access.decision.CanEdit = false

	if _, err := harness.authority.LoadDocumentForGrant(
		context.Background(), harness.room, oldProof,
	); !errors.Is(err, collaboration.ErrForbidden) {
		t.Fatalf("stale load error = %v", err)
	}
	refreshed, err := harness.authority.RevalidateDocumentGrant(context.Background(), harness.room, oldProof)
	if err != nil {
		t.Fatal(err)
	}
	if refreshed.Level != "viewer" || refreshed.PolicyRevision != 10 || refreshed.GrantID == grant.GrantID {
		t.Fatalf("refreshed grant = %#v", refreshed)
	}
	viewerProof := proofFromGrant(refreshed)
	if _, err = harness.authority.LoadDocumentForGrant(context.Background(), harness.room, viewerProof); err != nil {
		t.Fatal(err)
	}
	update := validAuthorityUpdate(harness.room)
	if _, err = harness.authority.ValidateDocumentMutation(context.Background(), collaborationhttp.AuthorityValidateInput{
		Room: harness.room, Grant: viewerProof, CandidateSnapshotHash: update.SnapshotHash,
		Projection: update.Projection,
	}); !errors.Is(err, collaboration.ErrForbidden) {
		t.Fatalf("viewer validation error = %v", err)
	}
}

func TestCollaborationAuthorityChecksCurrentPolicyAndExactGrantProof(t *testing.T) {
	harness := newAuthorityHarness(t)
	ticket, _, err := harness.authority.IssueDocumentConnectionTicket(context.Background(), harness.principal, harness.room)
	if err != nil {
		t.Fatal(err)
	}
	harness.access.decision.CanView = false
	if _, err = harness.authority.AuthorizeDocumentConnection(
		context.Background(), harness.room, ticket, authorityConnectionID,
	); !errors.Is(err, collaboration.ErrForbidden) {
		t.Fatalf("revoked ticket error = %v", err)
	}

	for _, mutate := range []func(*collaborationhttp.AuthorityGrantProof){
		func(proof *collaborationhttp.AuthorityGrantProof) { proof.ActorID = "8" },
		func(proof *collaborationhttp.AuthorityGrantProof) { proof.Level = "viewer" },
		func(proof *collaborationhttp.AuthorityGrantProof) { proof.PolicyRevision++ },
	} {
		candidate := newAuthorityHarness(t)
		proof := proofFromGrant(candidate.grant(t))
		mutate(&proof)
		if _, err = candidate.authority.LoadDocumentForGrant(
			context.Background(), candidate.room, proof,
		); !errors.Is(err, collaboration.ErrForbidden) {
			t.Fatalf("forged proof error = %v", err)
		}
		if candidate.documents.loadCalls != 0 {
			t.Fatal("forged proof reached document service")
		}
	}

	expired := newAuthorityHarness(t)
	proof := proofFromGrant(expired.grant(t))
	expired.clock.Advance(collaborationGrantTTL + time.Millisecond)
	if _, err = expired.authority.LoadDocumentForGrant(
		context.Background(), expired.room, proof,
	); !errors.Is(err, collaboration.ErrForbidden) {
		t.Fatalf("expired grant error = %v", err)
	}
}

func TestCollaborationAuthorityReceiptRejectsEveryChangedBinding(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(*authorityHarness, *collaborationhttp.AuthorityCommitInput)
	}{
		{name: "tampered receipt", mutate: func(_ *authorityHarness, input *collaborationhttp.AuthorityCommitInput) {
			input.ValidationToken = mutateToken(input.ValidationToken)
		}},
		{name: "snapshot", mutate: func(_ *authorityHarness, input *collaborationhttp.AuthorityCommitInput) {
			input.Update.Snapshot = []byte("different-snapshot")
			input.Update.SnapshotHash = collaboration.SHA256Hex(input.Update.Snapshot)
		}},
		{name: "projection", mutate: func(_ *authorityHarness, input *collaborationhttp.AuthorityCommitInput) {
			input.Update.Projection = json.RawMessage(`{"type":"doc","changed":true}`)
			input.Update.ProjectionHash, _ = collaboration.CanonicalJSONHash(input.Update.Projection)
		}},
		{name: "references", mutate: func(_ *authorityHarness, input *collaborationhttp.AuthorityCommitInput) {
			input.References.AssetIDs = append(input.References.AssetIDs, "asset-2")
		}},
		{name: "grant", mutate: func(harness *authorityHarness, input *collaborationhttp.AuthorityCommitInput) {
			input.Grant = proofFromGrant(harness.grant(t))
		}},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			harness := newAuthorityHarness(t)
			grant := harness.grant(t)
			proof := proofFromGrant(grant)
			update := validAuthorityUpdate(harness.room)
			references := collaborationhttp.AuthorityReferenceSet{AssetIDs: []string{"asset-1"}}
			receipt, err := harness.authority.ValidateDocumentMutation(context.Background(), collaborationhttp.AuthorityValidateInput{
				Room: harness.room, Grant: proof, CandidateSnapshotHash: update.SnapshotHash,
				Projection: update.Projection, References: references,
			})
			if err != nil {
				t.Fatal(err)
			}
			input := collaborationhttp.AuthorityCommitInput{
				Grant: proof, ValidationToken: receipt.Token, References: references, Update: update,
			}
			test.mutate(harness, &input)
			if _, err = harness.authority.CommitDocumentMutation(
				context.Background(), input,
			); !errors.Is(err, collaboration.ErrForbidden) {
				t.Fatalf("error = %v", err)
			}
			if harness.documents.commitCalls != 0 {
				t.Fatalf("document commit called = %d", harness.documents.commitCalls)
			}
		})
	}
}

func TestCollaborationAuthorityRejectsExpiredReceiptAndProjectionFailure(t *testing.T) {
	harness := newAuthorityHarness(t)
	grant := harness.grant(t)
	proof := proofFromGrant(grant)
	update := validAuthorityUpdate(harness.room)
	receipt, err := harness.authority.ValidateDocumentMutation(context.Background(), collaborationhttp.AuthorityValidateInput{
		Room: harness.room, Grant: proof, CandidateSnapshotHash: update.SnapshotHash, Projection: update.Projection,
	})
	if err != nil {
		t.Fatal(err)
	}
	harness.clock.Advance(collaborationReceiptTTL + time.Millisecond)
	if _, err = harness.authority.CommitDocumentMutation(context.Background(), collaborationhttp.AuthorityCommitInput{
		Grant: proof, ValidationToken: receipt.Token, Update: update,
	}); !errors.Is(err, collaboration.ErrForbidden) {
		t.Fatalf("expired receipt error = %v", err)
	}

	failing := newAuthorityHarness(t)
	failing.projection.err = collaboration.ErrForbidden
	grant = failing.grant(t)
	if _, err = failing.authority.ValidateDocumentMutation(context.Background(), collaborationhttp.AuthorityValidateInput{
		Room: failing.room, Grant: proofFromGrant(grant), CandidateSnapshotHash: update.SnapshotHash,
		Projection: update.Projection,
	}); !errors.Is(err, collaboration.ErrForbidden) {
		t.Fatalf("projection error = %v", err)
	}
}

func TestCollaborationAuthorityRejectsCorruptCommitResponse(t *testing.T) {
	harness := newAuthorityHarness(t)
	grant := harness.grant(t)
	proof := proofFromGrant(grant)
	update := validAuthorityUpdate(harness.room)
	receipt, err := harness.authority.ValidateDocumentMutation(context.Background(), collaborationhttp.AuthorityValidateInput{
		Room: harness.room, Grant: proof, CandidateSnapshotHash: update.SnapshotHash, Projection: update.Projection,
	})
	if err != nil {
		t.Fatal(err)
	}
	harness.documents.commit = collaboration.DocumentCommit{
		Room: harness.room, OperationID: update.OperationID, DurableRevision: 999,
		SnapshotHash: update.SnapshotHash, StateVectorHash: update.StateVectorHash,
		ProjectionHash: update.ProjectionHash, PolicyRevision: 9, CommittedAt: harness.clock.Now(),
	}
	if _, err = harness.authority.CommitDocumentMutation(context.Background(), collaborationhttp.AuthorityCommitInput{
		Grant: proof, ValidationToken: receipt.Token, Update: update,
	}); !errors.Is(err, collaboration.ErrDataCorrupt) {
		t.Fatalf("corrupt commit error = %v", err)
	}
}

func TestCollaborationServiceTokenVerifier(t *testing.T) {
	secret := "service-token-at-least-thirty-two-bytes"
	verifier, err := newCollaborationServiceTokenVerifier(secret)
	if err != nil {
		t.Fatal(err)
	}
	if err = verifier.VerifyCollaborationServiceToken(context.Background(), secret); err != nil {
		t.Fatal(err)
	}
	for _, candidate := range []string{
		"service-token-at-least-thirty-two-bytf",
		" " + secret,
		strings.Repeat("x", maxCollaborationSecretBytes+1),
	} {
		if err = verifier.VerifyCollaborationServiceToken(
			context.Background(), candidate,
		); !errors.Is(err, collaboration.ErrForbidden) {
			t.Fatalf("candidate length %d error = %v", len(candidate), err)
		}
	}
	if _, err = newCollaborationServiceTokenVerifier("short"); !errors.Is(err, collaboration.ErrInvalidInput) {
		t.Fatalf("short configured secret error = %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err = verifier.VerifyCollaborationServiceToken(ctx, secret); !errors.Is(err, collaboration.ErrForbidden) {
		t.Fatalf("canceled verification error = %v", err)
	}

	var group sync.WaitGroup
	var failures atomic.Int64
	for worker := 0; worker < 32; worker++ {
		group.Add(1)
		go func() {
			defer group.Done()
			for index := 0; index < 100; index++ {
				if verifier.VerifyCollaborationServiceToken(context.Background(), secret) != nil {
					failures.Add(1)
				}
			}
		}()
	}
	group.Wait()
	if failures.Load() != 0 {
		t.Fatalf("concurrent failures = %d", failures.Load())
	}
}

func validAuthorityUpdate(room collaboration.Room) collaboration.DocumentUpdate {
	projection := json.RawMessage(`{"type":"doc","content":[]}`)
	projectionHash, _ := collaboration.CanonicalJSONHash(projection)
	update := []byte("incremental-update")
	snapshot := []byte("candidate-snapshot")
	vector := []byte("candidate-vector")
	return collaboration.DocumentUpdate{
		Room: room, OperationID: "operation-1", ExpectedDurableRevision: 2,
		BaseStateVectorHash: collaboration.SHA256Hex([]byte("base-vector")),
		Update:              update, UpdateHash: collaboration.SHA256Hex(update),
		Snapshot: snapshot, SnapshotHash: collaboration.SHA256Hex(snapshot),
		StateVector: vector, StateVectorHash: collaboration.SHA256Hex(vector),
		Projection: projection, ProjectionHash: projectionHash,
	}
}

func mutateToken(token string) string {
	decoded, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil || len(decoded) == 0 {
		return token + "x"
	}
	decoded[len(decoded)-1] ^= 1
	return base64.RawURLEncoding.EncodeToString(decoded)
}

func assertOpaqueProof(t *testing.T, token string, values ...string) {
	t.Helper()
	decoded, err := base64.RawURLEncoding.Strict().DecodeString(token)
	if err != nil {
		t.Fatalf("proof is not raw base64url: %v", err)
	}
	for _, value := range values {
		if strings.Contains(token, value) || bytes.Contains(decoded, []byte(value)) {
			t.Fatalf("proof disclosed %q", value)
		}
	}
}
