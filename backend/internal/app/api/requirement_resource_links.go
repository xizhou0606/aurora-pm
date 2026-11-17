package api

import (
	"context"
	"errors"
	"strconv"

	"github.com/gin-gonic/gin"
	accesscatalog "github.com/xizhou0606/aurora-pm/backend/internal/generated/accesscatalog"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/document"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/documentlibrary"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/identity"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/project"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/requirement"
	requirementgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/requirement/gorm"
	requirementhttp "github.com/xizhou0606/aurora-pm/backend/internal/module/requirement/http"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/resourceaccess"
	"gorm.io/gorm"
)

type requirementResourceAccess struct {
	actions     *identity.AccessService
	memberships *identity.WorkspaceMembershipService
	projects    project.AccessChecker
	resources   *documentlibrary.Service
	policies    *resourceaccess.PolicyService
}

func (a requirementResourceAccess) RequireRequirement(ctx context.Context, w, u, p uint, write bool) error {
	if err := a.memberships.VerifyWorkspaceMembership(ctx, u, w); err != nil {
		return resourceaccess.ErrForbidden
	}
	if err := a.projects.RequireProjectAccess(ctx, w, p, u); err != nil {
		return resourceaccess.ErrForbidden
	}
	codes, err := a.actions.EffectiveActionCodes(ctx, u)
	if err != nil {
		return err
	}
	wanted := accesscatalog.ActionRequirementsPoolView
	if write {
		wanted = accesscatalog.ActionRequirementsPoolUpdate
	}
	for _, code := range codes {
		if code == string(wanted) {
			return nil
		}
	}
	return resourceaccess.ErrForbidden
}
func (a requirementResourceAccess) Resource(ctx context.Context, w, u uint, id string, manage bool) (requirement.ResourceLink, error) {
	p := resourceaccess.Principal{WorkspaceID: w, UserID: u}
	resourceID := documentlibrary.ResourceID(id)
	operation := documentlibrary.OperationView
	if manage {
		operation = documentlibrary.OperationManageAccess
	}
	if err := a.policies.AuthorizeOperation(ctx, p, resourceID, operation); err != nil {
		if errors.Is(err, resourceaccess.ErrForbidden) || errors.Is(err, resourceaccess.ErrNotFound) {
			return requirement.ResourceLink{}, requirement.ErrResourceLinkNotFound
		}
		return requirement.ResourceLink{}, err
	}
	resource, binding, err := a.resources.ResolveResource(ctx, w, resourceID)
	if err != nil {
		return requirement.ResourceLink{}, err
	}
	if resource.TypeKey != documentlibrary.TypeRichText {
		return requirement.ResourceLink{}, resourceaccess.ErrInvalidInput
	}
	docID, err := strconv.ParseUint(binding.PayloadRef, 10, 64)
	if err != nil {
		return requirement.ResourceLink{}, err
	}
	value := requirement.ResourceLink{ResourceID: id, Name: resource.Name, DocumentID: uint(docID)}
	policy, policyErr := a.policies.GetPolicy(ctx, p, resourceID)
	if policyErr == nil {
		value.PolicyRevision = policy.Revision
		value.Private = policy.Mode == resourceaccess.ModePrivate || policy.Mode == resourceaccess.ModeLink
		value.CanManage = policy.CanManageAccess && policy.Mode != resourceaccess.ModeLegacy
	}
	if manage && !value.CanManage {
		return requirement.ResourceLink{}, resourceaccess.ErrForbidden
	}
	return value, nil
}
func (a requirementResourceAccess) Candidates(ctx context.Context, w, u uint) ([]requirement.ResourceLink, error) {
	resources, err := a.resources.ListResources(ctx, w)
	if err != nil {
		return nil, err
	}
	result := []requirement.ResourceLink{}
	for _, resource := range resources {
		if resource.TypeKey != documentlibrary.TypeRichText || resource.OwnerUserID != u {
			continue
		}
		value, err := a.Resource(ctx, w, u, string(resource.ID), true)
		if err == nil {
			result = append(result, value)
		} else if !errors.Is(err, requirement.ErrResourceLinkNotFound) && !errors.Is(err, resourceaccess.ErrForbidden) {
			return nil, err
		}
	}
	return result, nil
}
func (a requirementResourceAccess) LockResource(ctx context.Context, w uint, id string) error {
	return a.resources.LockActiveResource(ctx, w, documentlibrary.ResourceID(id))
}
func (a requirementResourceAccess) ApplyResourcePolicy(ctx context.Context, c requirement.ResourceLinkCommand) error {
	return a.policies.TouchAssociation(ctx, resourceaccess.Principal{WorkspaceID: c.WorkspaceID, UserID: c.UserID}, documentlibrary.ResourceID(c.ResourceID), c.ExpectedPolicyRevision, "requirement-link:"+c.OperationID, !c.Remove)
}
func assembleRequirementResourceLinks(db *gorm.DB, actions *identity.AccessService, memberships *identity.WorkspaceMembershipService, projects project.AccessChecker, documents *document.Service) (func(*gin.RouterGroup), error) {
	service := newRequirementResourceLinkService(db, actions, memberships, projects, documents)
	handler := requirementhttp.NewResourceLinksHandler(service)
	return handler.RegisterRoutes, nil
}

func newRequirementResourceLinkService(db *gorm.DB, actions *identity.AccessService, memberships *identity.WorkspaceMembershipService, projects project.AccessChecker, documents *document.Service) *requirement.ResourceLinkService {
	repository := requirementgorm.NewResourceLinkRepository(db, func(tx *gorm.DB) (requirement.ResourceLinkAccess, error) {
		policies, resources, _, err := newResourcePolicyService(tx, actions, memberships, documents)
		if err != nil {
			return nil, err
		}
		return requirementResourceAccess{actions: actions, memberships: memberships, projects: projects, resources: resources, policies: policies}, nil
	})
	return requirement.NewResourceLinkService(repository)
}
