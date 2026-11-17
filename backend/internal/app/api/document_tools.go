package api

import (
	"github.com/xizhou0606/aurora-pm/backend/internal/app/documentread"
	"github.com/xizhou0606/aurora-pm/backend/internal/app/documentwrite"
	collaborationclient "github.com/xizhou0606/aurora-pm/backend/internal/integration/collaboration"
	collaborationgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/collaboration/gorm"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/identity"
	identitygorm "github.com/xizhou0606/aurora-pm/backend/internal/module/identity/gorm"
	projectgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/project/gorm"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/resourceaccess"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/toolruntime"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/toolruntime/pmtools"
	"github.com/xizhou0606/aurora-pm/backend/internal/platform/config"
	"gorm.io/gorm"
)

func assembleDocumentTools(db *gorm.DB, actions *identity.AccessService, cfg config.Config) (map[string]toolruntime.ToolHandler, toolruntime.ObjectActionAuthorizer, error) {
	// Reuse the HTTP composition, including resource lifecycle, comments, assets,
	// authoritative user actions and membership. No reduced tool-only service.
	_, documents, err := assembleDocumentRoutes(db, actions, cfg.ObjectStorage, cfg.Collaboration.Enabled, cfg.Notification)
	if err != nil {
		return nil, nil, err
	}
	memberships, err := identity.NewWorkspaceMembershipService(identitygorm.NewWorkspaceMembershipRepository(db))
	if err != nil {
		return nil, nil, err
	}
	resources, err := assembleResourceAccess(db, actions, memberships, documents, cfg.Notification)
	if err != nil {
		return nil, nil, err
	}
	states, err := collaborationgorm.NewRepository(db)
	if err != nil {
		return nil, nil, err
	}
	documents.ConfigureCollaborationProjection(documentread.CollaborationProjectionReader{States: states})
	if cfg.Collaboration.Enabled {
		runtime, err := assembleCollaborationRoutes(db, resources, documents, cfg.Collaboration)
		if err != nil {
			return nil, nil, err
		}
		documents.ConfigureContentReplacer(documentwrite.CollaborationReplacer{Tickets: runtime.Tickets, Operations: states, Client: collaborationclient.ReplacementClient{URL: cfg.Collaboration.InternalURL, ServiceToken: cfg.Collaboration.ServiceToken}})
	}
	users := projectgorm.NewFactUserResolver(db)
	reads, err := pmtools.NewDocumentReadHandlers(documents, users)
	if err != nil {
		return nil, nil, err
	}
	writes, err := pmtools.NewDocumentWriteHandlers(documents, users)
	if err != nil {
		return nil, nil, err
	}
	for name, handler := range writes {
		reads[name] = handler
	}
	accessTools := &pmtools.DocumentAccessTools{Users: users, Policies: resources.Policies, Requests: resources.Requests, Links: resources.Links, Subjects: resources.Subjects, Mutations: &resourceaccess.ToolMutationService{Policies: resources.Policies, Requests: resources.Requests, Links: resources.Links}}
	accessHandlers, err := accessTools.Handlers()
	if err != nil {
		return nil, nil, err
	}
	for name, handler := range accessHandlers {
		reads[name] = handler
	}
	return reads, accessTools, nil
}
