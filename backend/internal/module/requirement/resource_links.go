package requirement

import (
	"context"
	"errors"
	"net/url"
	"strings"
)

var ErrResourceLinkInvalid = errors.New("requirement resource link is invalid")
var ErrResourceLinkConflict = errors.New("requirement resource link changed")
var ErrResourceLinkNotFound = errors.New("requirement resource link not found")

type ResourceLink struct {
	ResourceID     string `json:"resource_id"`
	Name           string `json:"name"`
	DocumentID     uint   `json:"document_id"`
	PolicyRevision uint64 `json:"policy_revision"`
	Private        bool   `json:"private"`
	CanManage      bool   `json:"can_manage"`
}
type ResourceLinkPage struct {
	Items              []ResourceLink `json:"items"`
	RequirementVersion uint           `json:"requirement_version"`
	ProjectSetID       uint           `json:"project_set_id"`
	ProductDocURL      string         `json:"product_doc_url"`
}
type ResourceLinkCommand struct {
	WorkspaceID, UserID, RequirementID uint
	ResourceID                         string `json:"resource_id"`
	ExpectedVersion                    uint   `json:"expected_version"`
	ExpectedPolicyRevision             uint64 `json:"expected_policy_revision"`
	OperationID                        string `json:"operation_id"`
	Remove                             bool
	ProductDocURL                      *string `json:"product_doc_url,omitempty"`
}

// ResourceLinkAccess is supplied by the composition root, not by the browser.
type ResourceLinkAccess interface {
	LockResource(context.Context, uint, string) error
	RequireRequirement(context.Context, uint, uint, uint, bool) error
	Resource(context.Context, uint, uint, string, bool) (ResourceLink, error)
	Candidates(context.Context, uint, uint) ([]ResourceLink, error)
	ApplyResourcePolicy(context.Context, ResourceLinkCommand) error
}
type ResourceLinkStore interface {
	ReadResourceLinks(context.Context, uint, uint, uint, bool) (ResourceLinkPage, error)
	ApplyResourceLink(context.Context, ResourceLinkCommand) (ResourceLinkPage, error)
	ResourceLinkProjectSets(context.Context, uint, []string) (map[string][]uint, error)
}
type ResourceLinkService struct{ store ResourceLinkStore }

func NewResourceLinkService(store ResourceLinkStore) *ResourceLinkService {
	return &ResourceLinkService{store: store}
}
func (s *ResourceLinkService) Read(ctx context.Context, w, u, r uint, candidates bool) (ResourceLinkPage, error) {
	if w == 0 || u == 0 || r == 0 {
		return ResourceLinkPage{}, ErrResourceLinkInvalid
	}
	return s.store.ReadResourceLinks(ctx, w, u, r, candidates)
}
func (s *ResourceLinkService) Apply(ctx context.Context, c ResourceLinkCommand) (ResourceLinkPage, error) {
	c.ResourceID = strings.TrimSpace(c.ResourceID)
	c.OperationID = strings.TrimSpace(c.OperationID)
	if c.ProductDocURL != nil {
		value := strings.TrimSpace(*c.ProductDocURL)
		if !c.Remove || len(value) > 1000 {
			return ResourceLinkPage{}, ErrResourceLinkInvalid
		}
		if value != "" {
			parsed, err := url.Parse(value)
			if err != nil || (parsed.Scheme != "https" && parsed.Scheme != "http") || parsed.Hostname() == "" || parsed.User != nil {
				return ResourceLinkPage{}, ErrResourceLinkInvalid
			}
		}
		c.ProductDocURL = &value
	}
	if c.WorkspaceID == 0 || c.UserID == 0 || c.RequirementID == 0 || c.ResourceID == "" || len(c.ResourceID) > 128 || c.ExpectedVersion == 0 || c.ExpectedPolicyRevision == 0 || c.OperationID == "" || len(c.OperationID) > 100 {
		return ResourceLinkPage{}, ErrResourceLinkInvalid
	}
	return s.store.ApplyResourceLink(ctx, c)
}
