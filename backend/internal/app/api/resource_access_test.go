package api

import (
	"context"
	"errors"
	documentread "github.com/xizhou0606/aurora-pm/backend/internal/app/documentread"
	"strconv"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	accesscatalog "github.com/xizhou0606/aurora-pm/backend/internal/generated/accesscatalog"
	auditgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/audit/gorm"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/document"
	documentgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/document/gorm"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/documentlibrary"
	documentlibrarygorm "github.com/xizhou0606/aurora-pm/backend/internal/module/documentlibrary/gorm"
	requirementgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/requirement/gorm"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/resourceaccess"
	resourceaccessgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/resourceaccess/gorm"
)

type stubEffectiveActions struct {
	codes []string
	err   error
}

func (stub stubEffectiveActions) EffectiveActionCodes(context.Context, uint) ([]string, error) {
	return append([]string(nil), stub.codes...), stub.err
}

type stubMembershipVerifier struct{ err error }

func (stub stubMembershipVerifier) VerifyWorkspaceMembership(context.Context, uint, uint) error {
	return stub.err
}

func TestDocumentResourceGlobalActionsStayTypeBounded(t *testing.T) {
	authorizer := documentread.GlobalAuthorizer{
		Actions:     stubEffectiveActions{codes: []string{string(accesscatalog.ActionBitablesRecordsWrite)}},
		Memberships: stubMembershipVerifier{},
	}
	principal := resourceaccess.Principal{WorkspaceID: 7, UserID: 5}
	bitable := documentlibrary.Resource{ID: "br", WorkspaceID: 7, TypeKey: documentlibrary.TypeBitable}
	richText := documentlibrary.Resource{ID: "dr", WorkspaceID: 7, TypeKey: documentlibrary.TypeRichText}

	allowed, err := authorizer.AuthorizeDocumentResourceOperation(
		context.Background(), principal, bitable, documentlibrary.OperationEdit,
	)
	if err != nil || !allowed {
		t.Fatalf("bitable edit allowed=%v err=%v", allowed, err)
	}
	allowed, err = authorizer.AuthorizeDocumentResourceOperation(
		context.Background(), principal, richText, documentlibrary.OperationEdit,
	)
	if err != nil || allowed {
		t.Fatalf("bitable Action expanded to rich text: allowed=%v err=%v", allowed, err)
	}
}

func TestDocumentResourceGlobalAuthorizerRequiresLiveMembershipAndExactSharedAction(t *testing.T) {
	principal := resourceaccess.Principal{WorkspaceID: 7, UserID: 5}
	resource := documentlibrary.Resource{ID: "dr", WorkspaceID: 7, TypeKey: documentlibrary.TypeRichText}
	denied := documentread.GlobalAuthorizer{
		Actions:     stubEffectiveActions{codes: []string{string(accesscatalog.ActionDocumentsAccessManage)}},
		Memberships: stubMembershipVerifier{err: errors.New("inactive")},
	}
	allowed, err := denied.AuthorizeDocumentResourceOperation(
		context.Background(), principal, resource, documentlibrary.OperationManageAccess,
	)
	if err != nil || allowed {
		t.Fatalf("inactive membership allowed=%v err=%v", allowed, err)
	}

	active := denied
	active.Memberships = stubMembershipVerifier{}
	allowed, err = active.AuthorizeDocumentResourceOperation(
		context.Background(), principal, resource, documentlibrary.OperationManageAccess,
	)
	if err != nil || !allowed {
		t.Fatalf("exact shared action allowed=%v err=%v", allowed, err)
	}
	allowed, err = active.AuthorizeDocumentResourceOperation(
		context.Background(), principal, resource, documentlibrary.OperationViewAccess,
	)
	if err != nil || allowed {
		t.Fatalf("manage action implied view action: allowed=%v err=%v", allowed, err)
	}
}

func TestUnknownDocumentResourceTypeFailsClosed(t *testing.T) {
	authorizer := documentread.GlobalAuthorizer{
		Actions:     stubEffectiveActions{codes: []string{string(accesscatalog.ActionDocumentsContentView)}},
		Memberships: stubMembershipVerifier{},
	}
	allowed, err := authorizer.AuthorizeDocumentResourceOperation(
		context.Background(),
		resourceaccess.Principal{WorkspaceID: 7, UserID: 5},
		documentlibrary.Resource{ID: "future", WorkspaceID: 7, TypeKey: "future"},
		documentlibrary.OperationView,
	)
	if err != nil || allowed {
		t.Fatalf("unknown type allowed=%v err=%v", allowed, err)
	}
}

func TestDocumentResourceRealtimeActionsRequireTransportAndTypeContentCapabilities(t *testing.T) {
	principal := resourceaccess.Principal{WorkspaceID: 7, UserID: 5}
	resource := documentlibrary.Resource{ID: "br", WorkspaceID: 7, TypeKey: documentlibrary.TypeBitable}
	for _, test := range []struct {
		name      string
		operation documentlibrary.Operation
		codes     []string
		allowed   bool
	}{
		{name: "legacy view alone cannot collaborate", operation: documentlibrary.OperationCollaborate,
			codes: []string{string(accesscatalog.ActionBitablesResourcesView)}},
		{name: "transport view alone cannot read content", operation: documentlibrary.OperationCollaborate,
			codes: []string{string(accesscatalog.ActionDocumentsCollaborationView)}},
		{name: "transport and type view can collaborate", operation: documentlibrary.OperationCollaborate,
			codes: []string{string(accesscatalog.ActionDocumentsCollaborationView), string(accesscatalog.ActionBitablesResourcesView)}, allowed: true},
		{name: "write only cannot acquire lease", operation: documentlibrary.OperationCellLease,
			codes: []string{string(accesscatalog.ActionBitablesRecordsWrite)}},
		{name: "transport edit alone cannot mutate content", operation: documentlibrary.OperationCellLease,
			codes: []string{string(accesscatalog.ActionDocumentsCollaborationEdit)}},
		{name: "transport edit and type write can acquire lease", operation: documentlibrary.OperationCellLease,
			codes: []string{string(accesscatalog.ActionDocumentsCollaborationEdit), string(accesscatalog.ActionBitablesRecordsWrite)}, allowed: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			authorizer := documentread.GlobalAuthorizer{
				Actions: stubEffectiveActions{codes: test.codes}, Memberships: stubMembershipVerifier{},
			}
			allowed, err := authorizer.AuthorizeDocumentResourceOperation(
				context.Background(), principal, resource, test.operation,
			)
			if err != nil || allowed != test.allowed {
				t.Fatalf("allowed=%v want=%v err=%v", allowed, test.allowed, err)
			}
		})
	}
}

func TestDocumentCreationLifecycleCreatesPrivatePolicyInOwningTransaction(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(append(documentlibrarygorm.MigrationRecords(), resourceaccessgorm.MigrationRecords()...)...); err != nil {
		t.Fatal(err)
	}
	var resourceID string
	err = db.Transaction(func(tx *gorm.DB) error {
		lifecycle, lifecycleErr := newDocumentResourceLifecycle(tx)
		if lifecycleErr != nil {
			return lifecycleErr
		}
		resourceID, lifecycleErr = lifecycle.CreatePrivateDocumentResource(
			context.Background(),
			document.Principal{WorkspaceID: 7, UserID: 5},
			document.Node{ID: 1, WorkspaceID: 7, Title: "ADR", CreatedBy: 5},
			document.Document{ID: 11, NodeID: 1, WorkspaceID: 7, CreatedBy: 5, OwnerUserID: 5},
		)
		return lifecycleErr
	})
	if err != nil || resourceID == "" {
		t.Fatalf("resourceID=%q err=%v", resourceID, err)
	}
	resource, binding, err := documentlibrarygorm.NewRepository(db).ResolveContent(
		context.Background(), 7, documentlibrary.TypeRichText, "11",
	)
	if err != nil || string(resource.ID) != resourceID || binding.ResourceID != resource.ID {
		t.Fatalf("resource=%+v binding=%+v err=%v", resource, binding, err)
	}
	policy, err := resourceaccessgorm.NewRepository(db).GetPolicy(context.Background(), 7, resource.ID)
	if err != nil || policy.Mode != resourceaccess.ModePrivate || policy.Revision != 1 {
		t.Fatalf("policy=%+v err=%v", policy, err)
	}
}

func TestDocumentRepositoryInvokesPrivateResourceLifecycle(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	records := append(documentgorm.MigrationRecords(), documentlibrarygorm.MigrationRecords()...)
	records = append(records, &requirementgorm.ResourceLinkRecord{})
	records = append(records, resourceaccessgorm.MigrationRecords()...)
	records = append(records, &auditgorm.AuditLog{}, &auditgorm.OutboxEvent{})
	if err := db.AutoMigrate(records...); err != nil {
		t.Fatal(err)
	}
	repository := documentgorm.NewRepository(db)
	repository.ConfigureResourceLifecycleFactory(newDocumentResourceLifecycle)
	repository.ConfigureMutationRecorderFactory(newDocumentMutationRecorder)
	space, err := repository.EnsureDefaultSpace(context.Background(), 7)
	if err != nil {
		t.Fatal(err)
	}
	content, hash, plain, err := document.NormalizeContent(document.DefaultContent())
	if err != nil {
		t.Fatal(err)
	}
	node, body, err := repository.CreateNode(context.Background(), document.Node{
		WorkspaceID: 7, SpaceID: space.ID, Type: document.NodeDocument, Title: "Private ADR",
		SortOrder: 1024, CreatedBy: 5, UpdatedBy: 5,
	}, &document.Document{
		WorkspaceID: 7, Content: content, SchemaVersion: 1, Revision: 1, ContentHash: hash,
		PlainText: plain, OwnerUserID: 5, CreatedBy: 5, UpdatedBy: 5,
	})
	if err != nil || body == nil || node.ResourceID == "" || body.ResourceID != node.ResourceID {
		t.Fatalf("node=%+v body=%+v err=%v", node, body, err)
	}
	resource, _, err := documentlibrarygorm.NewRepository(db).ResolveContent(
		context.Background(), 7, documentlibrary.TypeRichText, strconv.FormatUint(uint64(body.ID), 10),
	)
	if err != nil || string(resource.ID) != node.ResourceID {
		t.Fatalf("resource=%+v err=%v", resource, err)
	}
	policy, err := resourceaccessgorm.NewRepository(db).GetPolicy(context.Background(), 7, resource.ID)
	if err != nil || policy.Mode != resourceaccess.ModePrivate || policy.Revision != 1 {
		t.Fatalf("policy=%+v err=%v", policy, err)
	}
	node.Title = "Renamed private ADR"
	node.UpdatedBy = 5
	node, err = repository.UpdateNode(context.Background(), node)
	if err != nil || node.ResourceID != string(resource.ID) {
		t.Fatalf("renamed node=%+v err=%v", node, err)
	}
	resource, _, err = documentlibrarygorm.NewRepository(db).ResolveContent(
		context.Background(), 7, documentlibrary.TypeRichText, strconv.FormatUint(uint64(body.ID), 10),
	)
	if err != nil || resource.Name != node.Title || resource.Revision != 2 {
		t.Fatalf("renamed resource=%+v err=%v", resource, err)
	}
	transferred, err := repository.TransferDocumentOwner(context.Background(), document.TransferDocumentOwnerCommand{
		WorkspaceID: 7, ActorUserID: 5, DocumentID: body.ID, OwnerUserID: 6,
		Effect: document.MutationEffect{
			WorkspaceID: 7, PrincipalID: 5, Resource: "document", Action: "owner.transferred",
			Payload: map[string]any{"owner_user_id": 6},
		},
	})
	if err != nil || transferred.OwnerUserID != 6 || transferred.ResourceID != string(resource.ID) {
		t.Fatalf("transferred=%+v err=%v", transferred, err)
	}
	resource, _, err = documentlibrarygorm.NewRepository(db).ResolveContent(
		context.Background(), 7, documentlibrary.TypeRichText, strconv.FormatUint(uint64(body.ID), 10),
	)
	if err != nil || resource.OwnerUserID != 6 || resource.Revision != 3 {
		t.Fatalf("owner resource=%+v err=%v", resource, err)
	}
	if err := repository.ArchiveNodes(context.Background(), 7, []uint{node.ID}, 5); err != nil {
		t.Fatal(err)
	}
	resource, _, err = documentlibrarygorm.NewRepository(db).ResolveContent(
		context.Background(), 7, documentlibrary.TypeRichText, strconv.FormatUint(uint64(body.ID), 10),
	)
	if err != nil || resource.Lifecycle != "archived" || resource.ArchivedAt == nil || resource.Revision != 4 {
		t.Fatalf("archived resource=%+v err=%v", resource, err)
	}
}

func TestDocumentCreationLifecycleRollsBackResourceAndPolicyTogether(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(append(documentlibrarygorm.MigrationRecords(), resourceaccessgorm.MigrationRecords()...)...); err != nil {
		t.Fatal(err)
	}
	sentinel := errors.New("document insert failed")
	err = db.Transaction(func(tx *gorm.DB) error {
		lifecycle, lifecycleErr := newDocumentResourceLifecycle(tx)
		if lifecycleErr != nil {
			return lifecycleErr
		}
		if _, lifecycleErr = lifecycle.CreatePrivateDocumentResource(
			context.Background(),
			document.Principal{WorkspaceID: 7, UserID: 5},
			document.Node{ID: 1, WorkspaceID: 7, Title: "ADR", CreatedBy: 5},
			document.Document{ID: 11, NodeID: 1, WorkspaceID: 7, CreatedBy: 5, OwnerUserID: 5},
		); lifecycleErr != nil {
			return lifecycleErr
		}
		return sentinel
	})
	if !errors.Is(err, sentinel) {
		t.Fatalf("rollback err=%v", err)
	}
	var resources, policies int64
	if err := db.Model(&documentlibrarygorm.ResourceRecord{}).Count(&resources).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&resourceaccessgorm.PolicyRecord{}).Count(&policies).Error; err != nil {
		t.Fatal(err)
	}
	if resources != 0 || policies != 0 {
		t.Fatalf("partial state resources=%d policies=%d", resources, policies)
	}
}
