package api

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	documentread "github.com/xizhou0606/aurora-pm/backend/internal/app/documentread"
	notificationgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/notification/gorm"
	"github.com/xizhou0606/aurora-pm/backend/internal/platform/config"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	accesscatalog "github.com/xizhou0606/aurora-pm/backend/internal/generated/accesscatalog"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/collaboration"
	collaborationgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/collaboration/gorm"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/document"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/documentlibrary"
	documentlibrarygorm "github.com/xizhou0606/aurora-pm/backend/internal/module/documentlibrary/gorm"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/identity"
	identitygorm "github.com/xizhou0606/aurora-pm/backend/internal/module/identity/gorm"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/notification"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/project"
	projectgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/project/gorm"
	requirementgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/requirement/gorm"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/resourceaccess"
	resourceaccessgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/resourceaccess/gorm"
	resourceaccesshttp "github.com/xizhou0606/aurora-pm/backend/internal/module/resourceaccess/http"
)

type resourceAccessRuntime struct {
	MountRoutes func(*gin.RouterGroup)
	Requests    *resourceaccess.RequestService
	Links       *resourceaccess.LinkService
	Subjects    identity.SubjectDirectory
	Policies    *resourceaccess.PolicyService
	Resources   *documentlibrary.Service
	Actions     *identity.AccessService
	Memberships *identity.WorkspaceMembershipService
}

type privatePolicyCreator interface {
	CreatePrivatePolicy(
		context.Context, resourceaccess.Principal, documentlibrary.ResourceID, time.Time,
	) (resourceaccess.Policy, error)
}

type documentResourceLifecycle struct {
	resources *documentlibrary.Service
	policies  privatePolicyCreator
}

func newDocumentResourceLifecycle(tx *gorm.DB) (document.ResourceLifecycle, error) {
	repository := documentlibrarygorm.NewRepository(tx)
	repository.ConfigureReferenceChecker(func(db *gorm.DB) documentlibrary.ReferenceChecker {
		return requirementgorm.NewResourceLinkRepository(db, nil)
	})
	resources, err := documentlibrary.NewService(repository, documentlibrary.DefaultRegistry())
	if err != nil {
		return nil, err
	}
	return documentResourceLifecycle{resources: resources, policies: resourceaccessgorm.NewRepository(tx)}, nil
}

func (lifecycle documentResourceLifecycle) CreatePrivateDocumentResource(
	ctx context.Context,
	principal document.Principal,
	node document.Node,
	body document.Document,
) (string, error) {
	if lifecycle.resources == nil || lifecycle.policies == nil || principal.UserID == 0 ||
		principal.WorkspaceID == 0 || node.ID == 0 || body.ID == 0 || body.NodeID != node.ID {
		return "", document.ErrContextActionsUnavailable
	}
	var parentRef *string
	if node.ParentID != nil {
		value := fmt.Sprintf("document-node:%d", *node.ParentID)
		parentRef = &value
	}
	resource, binding, err := lifecycle.resources.CreateResource(
		ctx,
		documentlibrary.Principal{WorkspaceID: principal.WorkspaceID, UserID: principal.UserID},
		documentlibrary.TypeRichText,
		strconv.FormatUint(uint64(body.ID), 10),
		node.Title,
		parentRef,
	)
	if err != nil || binding.ResourceID != resource.ID {
		return "", document.ErrContextActionsUnavailable
	}
	if _, err := lifecycle.policies.CreatePrivatePolicy(
		ctx,
		resourceaccess.Principal{WorkspaceID: principal.WorkspaceID, UserID: principal.UserID},
		resource.ID,
		time.Now().UTC(),
	); err != nil {
		return "", document.ErrContextActionsUnavailable
	}
	return string(resource.ID), nil
}

func (lifecycle documentResourceLifecycle) SynchronizeDocumentResource(
	ctx context.Context,
	principal document.Principal,
	node document.Node,
	body document.Document,
) (string, error) {
	if lifecycle.resources == nil || principal.UserID == 0 || principal.WorkspaceID == 0 ||
		node.ID == 0 || body.ID == 0 || body.NodeID != node.ID || body.OwnerUserID == 0 {
		return "", document.ErrContextActionsUnavailable
	}
	resource, _, err := lifecycle.resources.ResolveContentResource(
		ctx, principal.WorkspaceID, documentlibrary.TypeRichText, strconv.FormatUint(uint64(body.ID), 10),
	)
	if err != nil {
		return "", document.ErrContextActionsUnavailable
	}
	var parentRef *string
	if node.ParentID != nil {
		value := fmt.Sprintf("document-node:%d", *node.ParentID)
		parentRef = &value
	}
	resource, err = lifecycle.resources.ReplaceResourceMetadata(
		ctx,
		documentlibrary.Principal{WorkspaceID: principal.WorkspaceID, UserID: principal.UserID},
		resource.ID,
		resource.Revision,
		node.Title,
		parentRef,
		body.OwnerUserID,
	)
	if err != nil {
		return "", document.ErrContextActionsUnavailable
	}
	return string(resource.ID), nil
}

func (lifecycle documentResourceLifecycle) ArchiveDocumentResources(
	ctx context.Context,
	principal document.Principal,
	documents []document.Document,
) error {
	if lifecycle.resources == nil || principal.UserID == 0 || principal.WorkspaceID == 0 {
		return document.ErrContextActionsUnavailable
	}
	for _, body := range documents {
		resource, _, err := lifecycle.resources.ResolveContentResource(
			ctx, principal.WorkspaceID, documentlibrary.TypeRichText, strconv.FormatUint(uint64(body.ID), 10),
		)
		if err != nil {
			return document.ErrContextActionsUnavailable
		}
		if err := lifecycle.resources.ArchiveResource(
			ctx,
			documentlibrary.Principal{WorkspaceID: principal.WorkspaceID, UserID: principal.UserID},
			resource.ID,
			resource.Revision,
		); err != nil {
			return document.ErrContextActionsUnavailable
		}
	}
	return nil
}

func assembleResourceAccess(
	db *gorm.DB,
	actions *identity.AccessService,
	memberships *identity.WorkspaceMembershipService,
	documents *document.Service,
	notificationConfigs ...config.Notification,
) (resourceAccessRuntime, error) {
	policies, resources, subjects, err := newResourcePolicyService(db, actions, memberships, documents, notificationConfigs...)
	if err != nil {
		return resourceAccessRuntime{}, err
	}
	clock := resourceaccess.SystemClock{}
	store := resourceaccessgorm.NewRepository(db)
	limiter, err := resourceaccessgorm.NewLinkExchangeLimiter(db, clock)
	if err != nil {
		return resourceAccessRuntime{}, err
	}
	links, err := resourceaccess.NewLinkService(policies, store, limiter)
	if err != nil {
		return resourceAccessRuntime{}, err
	}
	handler, err := resourceaccesshttp.NewHandler(policies, links, subjects)
	if err != nil {
		return resourceAccessRuntime{}, err
	}
	requests := resourceaccess.NewRequestService(policies, policies.RequestStore())
	requests.ConfigureParticipantNames(func(ctx context.Context, ids []uint) (map[uint]string, error) {
		users, err := actions.ListActiveUsers(ctx)
		if err != nil {
			return nil, err
		}
		names := make(map[uint]string, len(ids))
		for _, id := range ids {
			names[id] = strconv.FormatUint(uint64(id), 10)
		}
		for _, user := range users {
			if _, requested := names[user.ID]; requested {
				name := user.DisplayName
				if name == "" {
					name = user.Username
				}
				names[user.ID] = name
			}
		}
		return names, nil
	})
	requests.ConfigureRequestAuthorizer(func(ctx context.Context, principal resourceaccess.Principal) error {
		codes, err := actions.EffectiveActionCodes(ctx, principal.UserID)
		if err != nil {
			return resourceaccess.ErrUnavailable
		}
		for _, code := range codes {
			if accesscatalog.ActionImplies(accesscatalog.ActionCode(code), accesscatalog.ActionDocumentsAccessRequest) {
				return nil
			}
		}
		return resourceaccess.ErrForbidden
	})
	handler.ConfigureRequests(requests)
	if documents != nil {
		documents.ConfigureResourceAccess(documentread.ResourceAccessResolver{Resources: resources, Policies: policies})
	}
	return resourceAccessRuntime{
		MountRoutes: handler.RegisterRoutes,
		Requests:    requests, Links: links, Subjects: subjects,
		Policies: policies, Resources: resources,
		Actions: actions, Memberships: memberships,
	}, nil
}

func newResourcePolicyService(
	db *gorm.DB,
	actions *identity.AccessService,
	memberships *identity.WorkspaceMembershipService,
	documents *document.Service,
	notificationConfigs ...config.Notification,
) (*resourceaccess.PolicyService, *documentlibrary.Service, identity.SubjectDirectory, error) {
	resources, err := documentlibrary.NewService(documentlibrarygorm.NewRepository(db), documentlibrary.DefaultRegistry())
	if err != nil {
		return nil, nil, nil, err
	}
	projectSets, err := project.NewProjectSetSubjectService(projectgorm.NewProjectSetSubjectRepository(db))
	if err != nil {
		return nil, nil, nil, err
	}
	subjects, err := identity.NewSubjectDirectoryService(identitygorm.NewSubjectRepository(db), projectSets, nil)
	if err != nil {
		return nil, nil, nil, err
	}
	clock := resourceaccess.SystemClock{}
	store := resourceaccessgorm.NewRepository(db)
	store.ConfigurePolicyRevisionEventAppenderFactory(func(tx *gorm.DB) (resourceaccess.PolicyRevisionEventAppender, error) {
		delegate, err := collaborationgorm.NewAppender(tx)
		if err != nil {
			return nil, err
		}
		return resourcePolicyRevisionEventAppender{delegate: delegate}, nil
	})
	policies, err := resourceaccess.NewPolicyService(
		resources,
		subjects,
		documentread.GlobalAuthorizer{Actions: actions, Memberships: memberships},
		documentread.LegacyEvaluator{Documents: documents, Resources: resources},
		store,
		clock,
	)
	if err != nil {
		return nil, nil, nil, err
	}
	policies.ConfigureRequirementSource(requirementgorm.NewResourceLinkRepository(db, nil))
	policies.ConfigureApprovedRequests(store)
	store.ConfigureRequestOwnerGuard(func(tx *gorm.DB, mutation resourceaccess.RequestMutation) error {
		resource, _, err := documentlibrarygorm.NewRepository(tx).LockActiveResource(tx.Statement.Context, mutation.Principal.WorkspaceID, mutation.ResourceID)
		if err != nil || resource.OwnerUserID != mutation.OwnerUserID {
			return resourceaccess.ErrNotFound
		}
		if mutation.RequestID != 0 && resource.OwnerUserID != mutation.Principal.UserID {
			return resourceaccess.ErrNotFound
		}
		return nil
	})
	store.ConfigureRequestNotificationAppender(func(tx *gorm.DB, mutation resourceaccess.RequestMutation, id, recipient uint) error {
		scene := notification.SceneDocumentAccessRequested
		if mutation.Decision != "" {
			scene = notification.SceneDocumentAccessReviewed
			if err := notificationgorm.NewMentionRepository(tx).MarkRequestReviewed(tx.Statement.Context, mutation.Principal.WorkspaceID, mutation.Principal.UserID, id, "document_access_request"); err != nil {
				return err
			}
		}
		vars := map[string]string{"resourceId": string(mutation.ResourceID), "status": string(mutation.Decision), "grantLevel": string(mutation.GrantLevel), "grantDuration": string(mutation.GrantDuration)}
		_, binding, err := documentlibrarygorm.NewRepository(tx).Get(tx.Statement.Context, mutation.Principal.WorkspaceID, mutation.ResourceID)
		if err != nil {
			return err
		}
		baseURL := ""
		if len(notificationConfigs) > 0 {
			baseURL = notificationConfigs[0].PublicBaseURL
		}
		var documentID uint64
		switch binding.TypeKey {
		case documentlibrary.TypeRichText:
			documentID, err = strconv.ParseUint(binding.PayloadRef, 10, 64)
			if err != nil || documentID == 0 {
				return resourceaccess.ErrInvalidInput
			}
			vars["url"] = notification.PublicURL(baseURL, "workspace", "documents", binding.PayloadRef) + "#access-requests"
		case documentlibrary.TypeBitable:
			vars["url"] = notification.PublicURL(baseURL, "workspace", "bitables", string(mutation.ResourceID)) + "#access-requests"
		default:
			return resourceaccess.ErrInvalidInput
		}
		intent := notification.Intent{WorkspaceID: mutation.Principal.WorkspaceID, Scene: scene, ActorUserID: mutation.Principal.UserID, RecipientUserIDs: []uint{recipient}, Vars: vars, FallbackPlainText: notification.PlainText(scene, vars)}
		intent.SkipExternal = len(notificationConfigs) == 0 || !notificationConfigs[0].Enabled
		intent.Vars["notification_event_key"] = fmt.Sprintf("request:%d:%s", id, mutation.Decision)
		intent.DocumentID, intent.RequestID = uint(documentID), id
		intent.ResourceType, intent.ResourceID = string(binding.TypeKey), string(mutation.ResourceID)
		return notification.Enqueue(tx, "document_access_request", id, intent)
	})
	return policies, resources, subjects, nil
}

type resourcePolicyRevisionEventAppender struct {
	delegate collaboration.EventAppender
}

func (appender resourcePolicyRevisionEventAppender) AppendPolicyRevisionEvent(
	ctx context.Context,
	event resourceaccess.PolicyRevisionEvent,
) error {
	if appender.delegate == nil {
		return resourceaccess.ErrUnavailable
	}
	payload, err := json.Marshal(map[string]any{
		"previous_policy_revision": event.PreviousRevision,
		"policy_revision":          event.PolicyRevision,
		"mode":                     event.Mode,
	})
	if err != nil {
		return err
	}
	digest := sha256.Sum256([]byte(fmt.Sprintf(
		"%d\x00%s\x00%s", event.WorkspaceID, event.ResourceID, event.IdempotencyKey,
	)))
	_, err = appender.delegate.Append(ctx, collaboration.AppendInput{
		Room:        collaboration.Room{WorkspaceID: event.WorkspaceID, ResourceID: string(event.ResourceID)},
		OperationID: "policy:" + hex.EncodeToString(digest[:]), ActorUserID: event.ActorUserID,
		Type: "resource.policy.committed", Payload: payload, PolicyRevision: event.PolicyRevision,
	})
	return err
}

var _ document.ResourceLifecycle = documentResourceLifecycle{}
