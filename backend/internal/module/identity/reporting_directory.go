package identity

import (
	"context"
	"errors"
	"sort"
)

// ErrReportingDirectoryQueryInvalid guards the workspace reporting directory
// read against malformed workspace or user references.
var ErrReportingDirectoryQueryInvalid = errors.New("identity reporting directory query is invalid")

// ErrReportingDirectoryMemberNotFound reports that a requested user is not an
// active member of the workspace directory.
var ErrReportingDirectoryMemberNotFound = errors.New("identity reporting directory member not found")

// DirectoryMember is the read-only personnel projection consumed by reporting
// surfaces: an active workspace member together with the single user group
// (角色组) it belongs to and its effective role names. RoleNames resolves the
// granted permission roles first and falls back to the synced Feishu job
// title when the member holds none; JobTitle always carries that fallback
// source so surfaces can tell 职务兜底 apart from granted roles. ID is the
// PM-internal users.id used by membership and sessions; FactID is its
// compatibility alias, also used by project data. It
// carries no permission facts; authorization stays with the access catalog.
// The department fields mirror the synced Feishu org projection and are
// display facts only.
type DirectoryMember struct {
	ID          uint     `json:"id"`
	FactID      uint     `json:"fact_id"`
	Username    string   `json:"username"`
	DisplayName string   `json:"display_name"`
	AvatarURL   string   `json:"avatar_url,omitempty"`
	GroupID     uint     `json:"group_id"`
	GroupCode   string   `json:"group_code,omitempty"`
	GroupName   string   `json:"group_name,omitempty"`
	RoleNames   []string `json:"role_names"`
	// JobTitle mirrors the synced Feishu job title (职务). Display fact only —
	// it never grants permission and is the documented fallback source of
	// RoleNames for members without granted roles.
	JobTitle       string `json:"job_title,omitempty"`
	DepartmentID   string `json:"department_id,omitempty"`
	DepartmentName string `json:"department_name,omitempty"`
}

// DirectoryDepartment is one synced Feishu department node of the org tree.
// ID is the PM-internal row id (used as the reporting subject reference);
// OpenID is the Feishu open_department_id.
type DirectoryDepartment struct {
	ID          uint   `json:"id"`
	OpenID      string `json:"open_department_id"`
	ParentID    string `json:"parent_department_id"`
	Name        string `json:"name"`
	Order       int64  `json:"order"`
	MemberCount int    `json:"member_count"`
}

// WorkspaceDepartment is one Feishu department with its subtree members.
// Members include every person in the department subtree so intermediate
// nodes can be reported as a whole.
type WorkspaceDepartment struct {
	ID      uint
	OpenID  string
	Name    string
	Members []DirectoryMember
}

// DirectoryStore owns the workspace-scoped member projection. The repository
// lives in identitygorm because users, workspace membership, user groups, and
// roles are identity-owned records.
type DirectoryStore interface {
	ListWorkspaceDirectoryMembers(context.Context, uint) ([]DirectoryMember, error)
	ListEnabledWorkspaceIDs(context.Context) ([]uint, error)
	ListEnabledGroups(context.Context) ([]UserGroup, error)
	// ListFeishuDepartments reads the synced Feishu org tree (empty before the
	// first successful sync).
	ListFeishuDepartments(context.Context) ([]DirectoryDepartment, error)
}

// DirectoryService exposes the reporting directory read over the store.
type DirectoryService struct {
	store DirectoryStore
	org   *FeishuOrgSyncService
}

func NewDirectoryService(store DirectoryStore) (*DirectoryService, error) {
	if store == nil {
		return nil, ErrReportingDirectoryQueryInvalid
	}
	return &DirectoryService{store: store}, nil
}

// WithFeishuOrg attaches the background org refresher; a nil syncer keeps the
// directory on the 用户组 grouping only.
func (service *DirectoryService) WithFeishuOrg(org *FeishuOrgSyncService) *DirectoryService {
	service.org = org
	return service
}

// ListWorkspaceMembers returns the active members of a workspace ordered by
// group name and display name. Members without an enabled group keep a zero
// GroupID so callers can render an explicit ungrouped bucket. When the Feishu
// org sync is attached, a stale projection triggers a background refresh —
// the read itself never waits for Feishu.
func (service *DirectoryService) ListWorkspaceMembers(
	ctx context.Context,
	workspaceID uint,
) ([]DirectoryMember, error) {
	if workspaceID == 0 {
		return nil, ErrReportingDirectoryQueryInvalid
	}
	if service.org != nil {
		service.org.EnsureFresh(ctx)
	}
	members, err := service.store.ListWorkspaceDirectoryMembers(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	sort.SliceStable(members, func(left, right int) bool {
		if members[left].GroupName != members[right].GroupName {
			return members[left].GroupName < members[right].GroupName
		}
		if members[left].DisplayName != members[right].DisplayName {
			return members[left].DisplayName < members[right].DisplayName
		}
		return members[left].ID < members[right].ID
	})
	return members, nil
}

// ListFeishuDepartments returns the synced Feishu org tree for directory
// rendering; empty until the first successful sync.
func (service *DirectoryService) ListFeishuDepartments(
	ctx context.Context,
) ([]DirectoryDepartment, error) {
	if service == nil || service.store == nil {
		return nil, ErrReportingDirectoryQueryInvalid
	}
	return service.store.ListFeishuDepartments(ctx)
}

// FindWorkspaceMember resolves one member of the workspace directory by its
// platform fact ID. It is used to fail closed when a report is requested for
// a non-member fact ID.
func (service *DirectoryService) FindWorkspaceMember(
	ctx context.Context,
	workspaceID uint,
	factID uint,
) (DirectoryMember, error) {
	members, err := service.ListWorkspaceMembers(ctx, workspaceID)
	if err != nil {
		return DirectoryMember{}, err
	}
	for _, member := range members {
		if member.FactID == factID {
			return member, nil
		}
	}
	return DirectoryMember{}, ErrReportingDirectoryMemberNotFound
}

// WorkspaceGroup is one role-group bucket with its active members.
type WorkspaceGroup struct {
	ID      uint
	Code    string
	Name    string
	Members []DirectoryMember
}

// ListEnabledGroups enumerates every enabled role group (用户组). Groups are
// global; reporting surfaces render them even when a workspace has zero
// members bound so the organisational hierarchy stays complete.
func (service *DirectoryService) ListEnabledGroups(ctx context.Context) ([]UserGroup, error) {
	if service == nil || service.store == nil {
		return nil, ErrReportingDirectoryQueryInvalid
	}
	return service.store.ListEnabledGroups(ctx)
}

// ListEnabledWorkspaceIDs enumerates every enabled workspace for scheduled,
// workspace-by-workspace report generation.
func (service *DirectoryService) ListEnabledWorkspaceIDs(ctx context.Context) ([]uint, error) {
	if service == nil || service.store == nil {
		return nil, ErrReportingDirectoryQueryInvalid
	}
	return service.store.ListEnabledWorkspaceIDs(ctx)
}

// FindWorkspaceDepartment resolves one synced Feishu department together
// with every active workspace member in its subtree. Reports use it to
// aggregate a whole department while staying inside the workspace boundary.
func (service *DirectoryService) FindWorkspaceDepartment(
	ctx context.Context,
	workspaceID uint,
	departmentID uint,
) (WorkspaceDepartment, error) {
	if workspaceID == 0 || departmentID == 0 {
		return WorkspaceDepartment{}, ErrReportingDirectoryQueryInvalid
	}
	departments, err := service.store.ListFeishuDepartments(ctx)
	if err != nil {
		return WorkspaceDepartment{}, err
	}
	var department DirectoryDepartment
	children := map[string][]string{}
	openByID := map[uint]string{}
	for _, node := range departments {
		openByID[node.ID] = node.OpenID
		if node.ID == departmentID {
			department = node
		}
		children[node.ParentID] = append(children[node.ParentID], node.OpenID)
	}
	if department.ID == 0 {
		return WorkspaceDepartment{}, ErrReportingDirectoryMemberNotFound
	}
	// Subtree = the department's own open id plus every descendant.
	subtree := map[string]bool{department.OpenID: true}
	queue := []string{department.OpenID}
	for len(queue) > 0 {
		parent := queue[0]
		queue = queue[1:]
		for _, childOpenID := range children[parent] {
			if !subtree[childOpenID] {
				subtree[childOpenID] = true
				queue = append(queue, childOpenID)
			}
		}
	}
	members, err := service.ListWorkspaceMembers(ctx, workspaceID)
	if err != nil {
		return WorkspaceDepartment{}, err
	}
	result := WorkspaceDepartment{
		ID: department.ID, OpenID: department.OpenID, Name: department.Name,
		Members: []DirectoryMember{},
	}
	for _, member := range members {
		if member.DepartmentID != "" && subtree[member.DepartmentID] {
			result.Members = append(result.Members, member)
		}
	}
	return result, nil
}

// FindWorkspaceGroup resolves one enabled role group of the workspace
// directory together with its members. Reports use it to aggregate a whole
// group while staying inside the workspace visibility boundary.
func (service *DirectoryService) FindWorkspaceGroup(
	ctx context.Context,
	workspaceID uint,
	groupID uint,
) (WorkspaceGroup, error) {
	if groupID == 0 {
		return WorkspaceGroup{}, ErrReportingDirectoryQueryInvalid
	}
	// Resolve against the enabled group catalog first: a group with zero
	// members still exists as a hierarchy node.
	groups, err := service.ListEnabledGroups(ctx)
	if err != nil {
		return WorkspaceGroup{}, err
	}
	group := WorkspaceGroup{}
	for _, enabled := range groups {
		if enabled.ID == groupID {
			group = WorkspaceGroup{ID: enabled.ID, Code: enabled.Code, Name: enabled.Name, Members: []DirectoryMember{}}
			break
		}
	}
	if group.ID == 0 {
		return WorkspaceGroup{}, ErrReportingDirectoryMemberNotFound
	}
	members, err := service.ListWorkspaceMembers(ctx, workspaceID)
	if err != nil {
		return WorkspaceGroup{}, err
	}
	for _, member := range members {
		if member.GroupID == groupID {
			group.Members = append(group.Members, member)
		}
	}
	return group, nil
}
