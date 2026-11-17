package documentread

import (
	"github.com/xizhou0606/aurora-pm/backend/internal/integration/feishu"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/ai/docsource"
	collaborationgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/collaboration/gorm"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/document"
	documentgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/document/gorm"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/documentlibrary"
	documentlibrarygorm "github.com/xizhou0606/aurora-pm/backend/internal/module/documentlibrary/gorm"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/identity"
	identitygorm "github.com/xizhou0606/aurora-pm/backend/internal/module/identity/gorm"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/project"
	projectgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/project/gorm"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/requirement"
	requirementgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/requirement/gorm"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/resourceaccess"
	resourceaccessgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/resourceaccess/gorm"
	"gorm.io/gorm"
)

// NewSources uses the same authorization adapters as interactive document reads.
// Binding every dependency to db also supports an atomic completion transaction.
func NewSources(db *gorm.DB, reader feishu.Reader) (*docsource.Service, error) {
	actions := identity.NewAccessService(identitygorm.NewAccessRepository(db), nil)
	memberships, err := identity.NewWorkspaceMembershipService(identitygorm.NewWorkspaceMembershipRepository(db))
	if err != nil {
		return nil, err
	}
	participants, err := requirement.NewDocumentParticipantService(requirementgorm.NewReadRepository(db))
	if err != nil {
		return nil, err
	}
	documents, err := document.NewService(documentgorm.NewRepository(db), ActionResolver{Access: actions, Memberships: memberships}, RequirementResolver{Service: participants}, nil)
	if err != nil {
		return nil, err
	}
	resources, err := documentlibrary.NewService(documentlibrarygorm.NewRepository(db), documentlibrary.DefaultRegistry())
	if err != nil {
		return nil, err
	}
	projectSets, err := project.NewProjectSetSubjectService(projectgorm.NewProjectSetSubjectRepository(db))
	if err != nil {
		return nil, err
	}
	subjects, err := identity.NewSubjectDirectoryService(identitygorm.NewSubjectRepository(db), projectSets, nil)
	if err != nil {
		return nil, err
	}
	store := resourceaccessgorm.NewRepository(db)
	policies, err := resourceaccess.NewPolicyService(resources, subjects, GlobalAuthorizer{Actions: actions, Memberships: memberships}, LegacyEvaluator{Documents: documents, Resources: resources}, store, resourceaccess.SystemClock{})
	if err != nil {
		return nil, err
	}
	links := requirementgorm.NewResourceLinkRepository(db, nil)
	policies.ConfigureRequirementSource(links)
	policies.ConfigureApprovedRequests(store)
	documents.ConfigureResourceAccess(ResourceAccessResolver{Resources: resources, Policies: policies})
	states, err := collaborationgorm.NewRepository(db)
	if err != nil {
		return nil, err
	}
	documents.ConfigureCollaborationProjection(CollaborationProjectionReader{States: states})
	registry, err := docsource.NewRegistry(feishu.DocumentSourceProvider{Reader: reader}, docsource.InternalProvider{Documents: documents, Resources: resources})
	if err != nil {
		return nil, err
	}
	admins := projectgorm.NewAdminRoleChecker(db)
	units := projectgorm.NewPlanUnitAccessRepository(db)
	projects, err := project.NewAccessService(admins, admins, units, units, units)
	if err != nil {
		return nil, err
	}
	return &docsource.Service{Users: identitygorm.NewLoginRepository(db), Registry: registry, Requirements: links, Actions: actions, Memberships: memberships, Projects: projects}, nil
}
