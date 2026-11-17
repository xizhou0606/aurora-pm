package api

import (
	"context"
	"encoding/json"
	documentread "github.com/xizhou0606/aurora-pm/backend/internal/app/documentread"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/xizhou0606/aurora-pm/backend/internal/module/collaboration"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/document"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/documentlibrary"
	documentlibrarygorm "github.com/xizhou0606/aurora-pm/backend/internal/module/documentlibrary/gorm"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/resourceaccess"
	pmauth "github.com/xizhou0606/aurora-pm/backend/internal/platform/auth"
)

type documentStateReaderStub struct {
	state collaboration.DocumentState
	err   error
}

type collaborationResourceLockerStub struct {
	resources map[documentlibrary.ResourceID]documentlibrary.Resource
	bindings  map[documentlibrary.ResourceID]documentlibrary.ContentBinding
	locked    []documentlibrary.ResourceID
}

func (stub *collaborationResourceLockerStub) LockActiveResource(
	_ context.Context, workspaceID uint, resourceID documentlibrary.ResourceID,
) (documentlibrary.Resource, documentlibrary.ContentBinding, error) {
	stub.locked = append(stub.locked, resourceID)
	resource, ok := stub.resources[resourceID]
	if !ok || resource.WorkspaceID != workspaceID {
		return documentlibrary.Resource{}, documentlibrary.ContentBinding{}, documentlibrary.ErrNotFound
	}
	return resource, stub.bindings[resourceID], nil
}

type collaborationAssetLockerStub struct{ ids []uint }

func (stub *collaborationAssetLockerStub) LockCollaborationAssetReferences(
	_ context.Context, _, _ uint, ids []uint,
) error {
	stub.ids = append([]uint(nil), ids...)
	return nil
}

type collaborationPolicyStub struct {
	revision uint64
	locked   []documentlibrary.ResourceID
}

func (stub *collaborationPolicyStub) LockCurrentPolicy(
	_ context.Context, _ uint, resourceID documentlibrary.ResourceID,
) (resourceaccess.Policy, error) {
	stub.locked = append(stub.locked, resourceID)
	return resourceaccess.Policy{ResourceID: resourceID, Revision: stub.revision}, nil
}

func (stub *collaborationPolicyStub) Evaluate(
	_ context.Context, _ resourceaccess.Principal, request resourceaccess.EvaluationRequest,
) (resourceaccess.Evaluation, error) {
	return resourceaccess.Evaluation{ResourceID: request.ResourceID, Operation: request.Operation, Allowed: true, PolicyRevision: stub.revision}, nil
}

func (stub *collaborationPolicyStub) BatchEvaluate(
	_ context.Context, _ resourceaccess.Principal, requests []resourceaccess.EvaluationRequest,
) ([]resourceaccess.Evaluation, error) {
	result := make([]resourceaccess.Evaluation, 0, len(requests))
	for _, request := range requests {
		result = append(result, resourceaccess.Evaluation{ResourceID: request.ResourceID, Operation: request.Operation, Allowed: true, PolicyRevision: stub.revision})
	}
	return result, nil
}

func TestDocumentCommitGuardLocksCanonicalResourceReferencesAndPolicies(t *testing.T) {
	const documentResourceID = documentlibrary.ResourceID("00000000-0000-4000-8000-000000000010")
	const bitableResourceID = documentlibrary.ResourceID("00000000-0000-4000-8000-000000000020")
	resources := &collaborationResourceLockerStub{
		resources: map[documentlibrary.ResourceID]documentlibrary.Resource{
			documentResourceID: {ID: documentResourceID, WorkspaceID: 11, TypeKey: documentlibrary.TypeRichText, Lifecycle: "active"},
			bitableResourceID:  {ID: bitableResourceID, WorkspaceID: 11, TypeKey: documentlibrary.TypeBitable, Lifecycle: "active"},
		},
		bindings: map[documentlibrary.ResourceID]documentlibrary.ContentBinding{
			documentResourceID: {WorkspaceID: 11, ResourceID: documentResourceID, TypeKey: documentlibrary.TypeRichText, PayloadRef: "31"},
			bitableResourceID:  {WorkspaceID: 11, ResourceID: bitableResourceID, TypeKey: documentlibrary.TypeBitable, PayloadRef: "table"},
		},
	}
	assets := &collaborationAssetLockerStub{}
	policies := &collaborationPolicyStub{revision: 7}
	guard := collaborationDocumentCommitGuard{resources: resources, assets: assets, policies: policies, policyDB: policies}
	projection := json.RawMessage(`{"type":"doc","content":[{"type":"image","attrs":{"assetId":19,"src":"/protected"}},{"type":"bitable","attrs":{"schemaVersion":1,"resourceId":"00000000-0000-4000-8000-000000000020","tableId":"00000000-0000-4000-8000-000000000021","viewId":"00000000-0000-4000-8000-000000000022","height":420}}]}`)
	err := guard.VerifyDocumentCommit(context.Background(), collaboration.Principal{WorkspaceID: 11, UserID: 5}, collaboration.DocumentUpdate{
		Room: collaboration.Room{WorkspaceID: 11, ResourceID: string(documentResourceID)}, Projection: projection, PolicyRevision: 7,
	})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join([]string{string(resources.locked[0]), string(resources.locked[1])}, ",") != string(documentResourceID)+","+string(bitableResourceID) {
		t.Fatalf("resource locks=%v", resources.locked)
	}
	if len(assets.ids) != 1 || assets.ids[0] != 19 {
		t.Fatalf("asset locks=%v", assets.ids)
	}
	if len(policies.locked) != 2 || policies.locked[0] != documentResourceID || policies.locked[1] != bitableResourceID {
		t.Fatalf("policy locks=%v", policies.locked)
	}

	policies.revision = 8
	if err := guard.VerifyDocumentCommit(context.Background(), collaboration.Principal{WorkspaceID: 11, UserID: 5}, collaboration.DocumentUpdate{
		Room: collaboration.Room{WorkspaceID: 11, ResourceID: string(documentResourceID)}, Projection: projection, PolicyRevision: 7,
	}); err != collaboration.ErrStalePolicy {
		t.Fatalf("stale policy error=%v", err)
	}
}

func (stub documentStateReaderStub) LoadDocumentState(
	context.Context, collaboration.Room,
) (collaboration.DocumentState, error) {
	return stub.state, stub.err
}

func TestCollaborationRequestResolverUsesAuthenticatedContextAndOpaqueHeaders(t *testing.T) {
	gin.SetMode(gin.TestMode)
	context, _ := gin.CreateTestContext(httptest.NewRecorder())
	context.Request = httptest.NewRequest("GET", "/", nil)
	connectionID := uuid.NewString()
	context.Request.Header.Set(collaborationConnectionHeader, connectionID)
	context.Request.Header.Set("X-PM-Resource-Access", "  short-lived-secret  ")
	context.Set(pmauth.PrincipalContextKey, pmauth.Principal{UserID: 7, Username: "alice", DisplayName: "Alice"})
	context.Set(pmauth.WorkspaceIDContextKey, uint(11))

	principal, err := (collaborationRequestResolver{}).ResolveCollaborationPrincipal(context)
	if err != nil || principal.WorkspaceID != 11 || principal.UserID != 7 || principal.DisplayName != "Alice" ||
		principal.ConnectionID != connectionID || principal.LinkAccessSecret != "short-lived-secret" {
		t.Fatalf("principal=%+v err=%v", principal, err)
	}

	context.Request.Header.Set(collaborationConnectionHeader, "browser-tab")
	if _, err := (collaborationRequestResolver{}).ResolveCollaborationPrincipal(context); err == nil {
		t.Fatal("non-canonical connection identity accepted")
	}
	context.Request.Header.Set(collaborationConnectionHeader, connectionID)
	context.Request.Header.Set("X-PM-Resource-Access", string(make([]byte, 257)))
	if _, err := (collaborationRequestResolver{}).ResolveCollaborationPrincipal(context); err == nil {
		t.Fatal("oversized link access secret accepted")
	}
}

func TestCollaborationRoomResolverAcceptsOnlyLiveUnifiedResource(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(documentlibrarygorm.MigrationRecords()...); err != nil {
		t.Fatal(err)
	}
	resources, err := documentlibrary.NewService(documentlibrarygorm.NewRepository(db), documentlibrary.DefaultRegistry())
	if err != nil {
		t.Fatal(err)
	}
	resource, _, err := resources.CreateResource(
		context.Background(), documentlibrary.Principal{WorkspaceID: 11, UserID: 7},
		documentlibrary.TypeBitable, uuid.NewString(), "Roadmap", nil,
	)
	if err != nil {
		t.Fatal(err)
	}
	gin.SetMode(gin.TestMode)
	requestContext, _ := gin.CreateTestContext(httptest.NewRecorder())
	requestContext.Request = httptest.NewRequest("GET", "/", nil)
	requestContext.Params = gin.Params{{Key: "resourceId", Value: string(resource.ID)}}
	resolver := collaborationRequestResolver{resources: resources}
	room, err := resolver.ResolveCollaborationRoom(requestContext, collaboration.Principal{WorkspaceID: 11, UserID: 7})
	if err != nil || room != (collaboration.Room{WorkspaceID: 11, ResourceID: string(resource.ID)}) {
		t.Fatalf("room=%+v err=%v", room, err)
	}
	if _, err := resolver.ResolveCollaborationRoom(requestContext, collaboration.Principal{WorkspaceID: 12, UserID: 7}); err == nil {
		t.Fatal("foreign workspace resolved the room")
	}
}

func TestDocumentCollaborationProjectionReaderUsesCanonicalDurableProjection(t *testing.T) {
	projection := json.RawMessage(`{"content":[{"content":[{"text":"durable","type":"text"}],"type":"paragraph"}],"type":"doc"}`)
	reader := documentread.CollaborationProjectionReader{States: documentStateReaderStub{state: collaboration.DocumentState{
		Room: collaboration.Room{WorkspaceID: 11, ResourceID: "resource-1"}, DurableRevision: 8,
		Projection: projection, ProjectionHash: collaboration.SHA256Hex(projection),
	}}}
	value, err := reader.LoadDocumentProjection(
		context.Background(), document.Principal{WorkspaceID: 11, UserID: 7}, "resource-1",
	)
	if err != nil {
		t.Fatal(err)
	}
	if value.Revision != 8 || value.SchemaVersion != document.SchemaVersionV3 || value.PlainText != "durable" ||
		value.ContentHash != collaboration.SHA256Hex(value.Content) {
		t.Fatalf("projection=%+v", value)
	}
}

func TestDocumentCollaborationProjectionReaderRejectsHashMismatch(t *testing.T) {
	reader := documentread.CollaborationProjectionReader{States: documentStateReaderStub{state: collaboration.DocumentState{
		Room: collaboration.Room{WorkspaceID: 11, ResourceID: "resource-1"}, DurableRevision: 8,
		Projection: json.RawMessage(`{"type":"doc"}`), ProjectionHash: "not-canonical",
	}}}
	if _, err := reader.LoadDocumentProjection(
		context.Background(), document.Principal{WorkspaceID: 11, UserID: 7}, "resource-1",
	); err == nil {
		t.Fatal("corrupt durable projection accepted")
	}
}

func TestCollectCollaborationProjectionReferencesUsesProductionDocumentNodes(t *testing.T) {
	var root map[string]any
	decoder := json.NewDecoder(strings.NewReader(`{
		"type":"doc","content":[
			{"type":"image","attrs":{"assetId":17,"src":"/protected"}},
			{"type":"image","attrs":{"assetId":17,"src":"/protected"}},
			{"type":"bitable","attrs":{"schemaVersion":1,"resourceId":"00000000-0000-4000-8000-000000000001","tableId":"00000000-0000-4000-8000-000000000002","viewId":"00000000-0000-4000-8000-000000000003","height":420}}
		]
	}`))
	decoder.UseNumber()
	if err := decoder.Decode(&root); err != nil {
		t.Fatal(err)
	}
	references := collaborationProjectionReferences{}
	if err := collectCollaborationProjectionReferences(root, &references); err != nil {
		t.Fatal(err)
	}
	if assets := canonicalReferenceIDs(references.AssetIDs); len(assets) != 1 || assets[0] != "17" {
		t.Fatalf("assets=%v", assets)
	}
	if bitables := canonicalReferenceIDs(references.BitableResourceIDs); len(bitables) != 1 || bitables[0] != "00000000-0000-4000-8000-000000000001" {
		t.Fatalf("bitables=%v", bitables)
	}
}
