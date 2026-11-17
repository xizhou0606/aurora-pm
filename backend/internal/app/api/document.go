package api

import (
	"context"
	documentread "github.com/xizhou0606/aurora-pm/backend/internal/app/documentread"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/xizhou0606/aurora-pm/backend/internal/integration/objectstore"
	audit "github.com/xizhou0606/aurora-pm/backend/internal/module/audit"
	auditgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/audit/gorm"
	document "github.com/xizhou0606/aurora-pm/backend/internal/module/document"
	documentgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/document/gorm"
	documenthttp "github.com/xizhou0606/aurora-pm/backend/internal/module/document/http"
	identity "github.com/xizhou0606/aurora-pm/backend/internal/module/identity"
	identitygorm "github.com/xizhou0606/aurora-pm/backend/internal/module/identity/gorm"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/notification"
	notificationgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/notification/gorm"
	notificationhttp "github.com/xizhou0606/aurora-pm/backend/internal/module/notification/http"
	requirement "github.com/xizhou0606/aurora-pm/backend/internal/module/requirement"
	requirementgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/requirement/gorm"
	"github.com/xizhou0606/aurora-pm/backend/internal/platform/config"
)

type documentCommentMentionResolver struct {
	memberships *identity.WorkspaceMembershipService
}

type documentWorkspaceUserResolver struct {
	memberships *identity.WorkspaceMembershipService
}

func (resolver documentWorkspaceUserResolver) ValidateActiveWorkspaceUser(
	ctx context.Context,
	workspaceID, userID uint,
) error {
	return resolver.memberships.VerifyWorkspaceMembership(ctx, userID, workspaceID)
}

func (resolver documentCommentMentionResolver) ValidateCommentMentionUserIDs(
	ctx context.Context,
	workspaceID uint,
	userIDs []uint,
) error {
	for _, userID := range userIDs {
		if err := resolver.memberships.VerifyWorkspaceMembership(ctx, userID, workspaceID); err != nil {
			return document.ErrCommentMentionInvalid
		}
	}
	return nil
}

type documentMutationRecorder struct {
	service            *audit.MutationEffectService
	tx                 *gorm.DB
	notificationConfig config.Notification
}

func (recorder documentMutationRecorder) RecordDocumentMutation(
	ctx context.Context,
	effect document.MutationEffect,
) error {
	if err := recorder.service.RecordMutationEffect(ctx, audit.MutationEffect{
		WorkspaceID: effect.WorkspaceID, PrincipalID: effect.PrincipalID,
		Resource: effect.Resource, ResourceID: effect.ResourceID, Action: effect.Action,
		EventType: "pm.document." + effect.Action, Payload: effect.Payload,
	}); err != nil {
		return err
	}
	// The site inbox is independent of external notification delivery switches.
	if effect.CommentMention == nil {
		return nil
	}
	mention := effect.CommentMention
	intent := notification.DocumentCommentMention(effect.WorkspaceID, effect.PrincipalID, mention.DocumentID, mention.ThreadID, mention.RecipientUserIDs, recorder.notificationConfig.PublicBaseURL)
	intent.SkipExternal = !recorder.notificationConfig.Enabled
	intent.CommentID = mention.CommentID
	if len(intent.RecipientUserIDs) == 0 {
		return nil
	}
	return notification.Enqueue(recorder.tx.WithContext(ctx), "document_comment", mention.CommentID, intent)
}

func newDocumentMutationRecorder(tx *gorm.DB) (document.MutationRecorder, error) {
	service, err := audit.NewMutationEffectService(auditgorm.NewMutationEffectRepository(tx))
	if err != nil {
		return nil, err
	}
	return documentMutationRecorder{service: service, tx: tx}, nil
}

func assembleDocumentRoutes(
	db *gorm.DB,
	access *identity.AccessService,
	assetConfig config.ObjectStorage,
	newDocumentCollaborationEnabled bool,
	notificationConfigs ...config.Notification,
) (func(*gin.RouterGroup), *document.Service, error) {
	memberships, err := identity.NewWorkspaceMembershipService(identitygorm.NewWorkspaceMembershipRepository(db))
	if err != nil {
		return nil, nil, err
	}
	participants, err := requirement.NewDocumentParticipantService(requirementgorm.NewReadRepository(db))
	if err != nil {
		return nil, nil, err
	}
	effects, err := audit.NewMutationEffectService(auditgorm.NewMutationEffectRepository(db))
	if err != nil {
		return nil, nil, err
	}
	repository := documentgorm.NewRepository(db)
	repository.ConfigureResourceLifecycleFactory(newDocumentResourceLifecycle)
	if newDocumentCollaborationEnabled {
		repository.ConfigureNewDocumentCollaboration(newDocumentCollaborationFactory)
	}
	repository.ConfigureMutationRecorderFactory(func(tx *gorm.DB) (document.MutationRecorder, error) {
		recorder, err := newDocumentMutationRecorder(tx)
		if err != nil {
			return nil, err
		}
		configured := recorder.(documentMutationRecorder)
		configured.tx = tx
		if len(notificationConfigs) > 0 {
			configured.notificationConfig = notificationConfigs[0]
		}
		return configured, nil
	})
	service, err := document.NewService(
		repository,
		documentread.ActionResolver{Access: access, Memberships: memberships},
		documentread.RequirementResolver{Service: participants},
		documentMutationRecorder{service: effects},
	)
	if err != nil {
		return nil, nil, err
	}
	service.ConfigureComments(repository, documentCommentMentionResolver{memberships: memberships})
	service.ConfigureCollaborationCreation(newDocumentCollaborationEnabled)
	service.ConfigureWorkspaceUsers(documentWorkspaceUserResolver{memberships: memberships})
	assetStorage, err := objectstore.New(assetConfig)
	if err != nil {
		return nil, nil, err
	}
	service.ConfigureAssetStorage(assetStorage, assetConfig.GlobalPrefix)
	handler, err := documenthttp.NewHandler(service)
	if err != nil {
		return nil, nil, err
	}
	handler.ConfigureUserDisplays(identitygorm.NewUserDisplayRepository(db))
	mentions := notificationhttp.NewMentionHandler(notificationgorm.NewMentionRepository(db), memberships)
	return func(group *gin.RouterGroup) {
		handler.RegisterRoutes(group)
		mentions.RegisterRoutes(group)
		notificationhttp.NewInboxHandler(inboxRepository(db), memberships).RegisterRoutes(group)
	}, service, nil
}
