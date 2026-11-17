package workbench

import (
	"context"
	"errors"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"time"
)

const (
	MenuTypeDirectory = "directory"
	MenuTypePage      = "page"
	MenuTypeExternal  = "external"
)

var (
	ErrMenuNotFound      = errors.New("access menu not found")
	ErrMenuCodeInvalid   = errors.New("access menu code is invalid")
	ErrMenuNameInvalid   = errors.New("access menu name is invalid")
	ErrMenuTypeInvalid   = errors.New("access menu type is invalid")
	ErrMenuPathInvalid   = errors.New("access menu path is invalid")
	ErrMenuHostInvalid   = errors.New("access menu host is invalid")
	ErrMenuParentInvalid = errors.New("access menu parent is invalid")
	ErrMenuInUse         = errors.New("access menu is still in use")
	ErrMenuSystemManaged = errors.New("access system menu cannot be changed this way")
	ErrMenuCodeConflict  = errors.New("access menu code already exists")
	ErrMenuDuplicateID   = errors.New("access menu IDs contain a duplicate or invalid value")
)

var menuCodePattern = regexp.MustCompile(`^[a-z0-9][a-z0-9.-]*$`)

type Menu struct {
	ID            uint      `json:"id"`
	Code          string    `json:"code"`
	ParentID      *uint     `json:"parent_id,omitempty"`
	Name          string    `json:"name"`
	MenuType      string    `json:"menu_type"`
	Path          string    `json:"path,omitempty"`
	Icon          string    `json:"icon,omitempty"`
	HostKey       string    `json:"host_key"`
	SortOrder     int       `json:"sort_order"`
	Enabled       bool      `json:"enabled"`
	SystemManaged bool      `json:"system_managed"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

type MenuInput struct {
	Code      string
	ParentID  *uint
	Name      string
	MenuType  string
	Path      string
	Icon      string
	HostKey   string
	SortOrder int
	Enabled   bool
}

type MenuUpdate struct {
	ParentID  *uint
	ParentSet bool
	Name      *string
	MenuType  *string
	Path      *string
	Icon      *string
	HostKey   *string
	SortOrder *int
	Enabled   *bool
}

type MenuNode struct {
	Menu
	DirectGrant bool       `json:"direct_grant"`
	Structural  bool       `json:"structural"`
	Children    []MenuNode `json:"children"`
}

type MenuService struct {
	repository MenuRepository
	grants     MenuGrantReader
}

func NewMenuService(repository MenuRepository, grants MenuGrantReader) *MenuService {
	return &MenuService{repository: repository, grants: grants}
}

func (service *MenuService) List(ctx context.Context, includeDisabled bool) ([]Menu, error) {
	return service.repository.ListMenus(ctx, includeDisabled)
}

// Bootstrap makes system-owned menu registrations idempotent. These entries
// are part of the backend access catalog, not a workspace-local preference.
func (service *MenuService) Bootstrap(ctx context.Context, inputs []MenuInput) ([]Menu, error) {
	if len(inputs) == 0 {
		return []Menu{}, nil
	}
	normalized := make([]MenuInput, 0, len(inputs))
	seen := make(map[string]struct{}, len(inputs))
	for _, input := range inputs {
		value, err := normalizeMenuInput(input)
		if err != nil {
			return nil, err
		}
		if _, exists := seen[value.Code]; exists {
			return nil, ErrMenuCodeConflict
		}
		if err := service.validateParent(ctx, value.ParentID, value.HostKey, 0); err != nil {
			return nil, err
		}
		seen[value.Code] = struct{}{}
		normalized = append(normalized, value)
	}
	return service.repository.EnsureSystemMenus(ctx, normalized)
}

func (service *MenuService) Create(ctx context.Context, input MenuInput) (Menu, error) {
	normalized, err := normalizeMenuInput(input)
	if err != nil {
		return Menu{}, err
	}
	if err := service.validateParent(ctx, normalized.ParentID, normalized.HostKey, 0); err != nil {
		return Menu{}, err
	}
	return service.repository.CreateMenu(ctx, normalized)
}

func (service *MenuService) Update(ctx context.Context, id uint, input MenuUpdate) (Menu, error) {
	if id == 0 {
		return Menu{}, ErrMenuNotFound
	}
	before, err := service.repository.FindMenu(ctx, id)
	if err != nil {
		return Menu{}, err
	}
	if before.SystemManaged && (input.ParentSet || input.Name != nil || input.MenuType != nil || input.Path != nil || input.Icon != nil || input.HostKey != nil || input.SortOrder != nil || input.Enabled != nil) {
		return Menu{}, ErrMenuSystemManaged
	}
	after, err := mergeMenuUpdate(before, input)
	if err != nil {
		return Menu{}, err
	}
	if err := service.validateParent(ctx, after.ParentID, after.HostKey, id); err != nil {
		return Menu{}, err
	}
	if input.Enabled != nil && !*input.Enabled {
		hasChildren, err := service.repository.HasChildren(ctx, id)
		if err != nil {
			return Menu{}, err
		}
		if hasChildren {
			return Menu{}, ErrMenuInUse
		}
	}
	return service.repository.UpdateMenu(ctx, id, normalizedMenuUpdate(input, after))
}

func (service *MenuService) Delete(ctx context.Context, id uint) error {
	if id == 0 {
		return ErrMenuNotFound
	}
	menu, err := service.repository.FindMenu(ctx, id)
	if err != nil {
		return err
	}
	if menu.SystemManaged {
		return ErrMenuSystemManaged
	}
	hasChildren, err := service.repository.HasChildren(ctx, id)
	if err != nil {
		return err
	}
	if hasChildren {
		return ErrMenuInUse
	}
	if service.grants != nil {
		count, err := service.grants.CountMenuGrants(ctx, id)
		if err != nil {
			return err
		}
		if count > 0 {
			return ErrMenuInUse
		}
	}
	return service.repository.DeleteMenu(ctx, id)
}

// ValidateEnabledMenuIDs is the identity-facing menu grant validator.
func (service *MenuService) ValidateEnabledMenuIDs(ctx context.Context, ids []uint) error {
	if _, err := normalizedMenuIDs(ids); err != nil {
		return err
	}
	if len(ids) == 0 {
		return nil
	}
	rows, err := service.repository.EnabledMenuIDs(ctx, ids)
	if err != nil {
		return err
	}
	if len(rows) != len(ids) {
		return ErrMenuNotFound
	}
	return nil
}

func (service *MenuService) MenusForUser(ctx context.Context, userID uint, hostKey string) ([]MenuNode, error) {
	hostKey = strings.ToLower(strings.TrimSpace(hostKey))
	if hostKey == "" {
		hostKey = "pm"
	}
	if hostKey != "pm" && hostKey != "one2all" {
		return nil, ErrMenuHostInvalid
	}
	if userID == 0 || service.grants == nil {
		return []MenuNode{}, nil
	}
	directIDs, err := service.grants.DirectMenuIDsForUser(ctx, userID)
	if err != nil {
		return nil, err
	}
	if len(directIDs) == 0 {
		return []MenuNode{}, nil
	}
	menus, err := service.repository.ListEnabledMenus(ctx)
	if err != nil {
		return nil, err
	}
	byID := make(map[uint]Menu, len(menus))
	for _, menu := range menus {
		byID[menu.ID] = menu
	}
	direct := make(map[uint]struct{}, len(directIDs))
	for _, id := range directIDs {
		menu, exists := byID[id]
		if exists && (menu.HostKey == hostKey || menu.HostKey == "all") {
			direct[id] = struct{}{}
		}
	}
	included := make(map[uint]struct{}, len(direct)*2)
	for id := range direct {
		chain := make([]uint, 0, 4)
		valid := true
		for currentID := id; currentID != 0; {
			menu, exists := byID[currentID]
			if !exists {
				valid = false
				break
			}
			chain = append(chain, currentID)
			if menu.ParentID == nil {
				break
			}
			currentID = *menu.ParentID
		}
		if !valid {
			delete(direct, id)
			continue
		}
		for _, chainID := range chain {
			included[chainID] = struct{}{}
		}
	}
	return menuTree(byID, included, direct), nil
}

func (service *MenuService) validateParent(ctx context.Context, parentID *uint, hostKey string, menuID uint) error {
	if parentID == nil {
		return nil
	}
	if *parentID == 0 || *parentID == menuID {
		return ErrMenuParentInvalid
	}
	parent, err := service.repository.FindMenu(ctx, *parentID)
	if err != nil || !parent.Enabled || parent.MenuType != MenuTypeDirectory ||
		(parent.HostKey != "all" && parent.HostKey != hostKey) {
		return ErrMenuParentInvalid
	}
	visited := map[uint]struct{}{menuID: {}}
	for parent.ParentID != nil {
		if _, exists := visited[parent.ID]; exists {
			return ErrMenuParentInvalid
		}
		visited[parent.ID] = struct{}{}
		parent, err = service.repository.FindMenu(ctx, *parent.ParentID)
		if err != nil {
			return ErrMenuParentInvalid
		}
	}
	return nil
}

func normalizeMenuInput(input MenuInput) (MenuInput, error) {
	code := strings.TrimSpace(input.Code)
	if len(code) == 0 || len(code) > 96 || !menuCodePattern.MatchString(code) {
		return MenuInput{}, ErrMenuCodeInvalid
	}
	name := strings.TrimSpace(input.Name)
	if name == "" || len([]rune(name)) > 128 {
		return MenuInput{}, ErrMenuNameInvalid
	}
	menuType := strings.ToLower(strings.TrimSpace(input.MenuType))
	path := strings.TrimSpace(input.Path)
	if !validMenuPath(menuType, path) {
		return MenuInput{}, ErrMenuPathInvalid
	}
	hostKey := strings.ToLower(strings.TrimSpace(input.HostKey))
	if hostKey != "pm" && hostKey != "one2all" && hostKey != "all" {
		return MenuInput{}, ErrMenuHostInvalid
	}
	icon := strings.TrimSpace(input.Icon)
	if len([]rune(icon)) > 128 {
		return MenuInput{}, ErrMenuNameInvalid
	}
	return MenuInput{
		Code: code, ParentID: input.ParentID, Name: name, MenuType: menuType,
		Path: path, Icon: icon, HostKey: hostKey, SortOrder: input.SortOrder, Enabled: input.Enabled,
	}, nil
}

func mergeMenuUpdate(before Menu, input MenuUpdate) (Menu, error) {
	after := before
	if input.ParentSet {
		after.ParentID = input.ParentID
	}
	if input.Name != nil {
		after.Name = *input.Name
	}
	if input.MenuType != nil {
		after.MenuType = *input.MenuType
	}
	if input.Path != nil {
		after.Path = *input.Path
	}
	if input.Icon != nil {
		after.Icon = *input.Icon
	}
	if input.HostKey != nil {
		after.HostKey = *input.HostKey
	}
	if input.SortOrder != nil {
		after.SortOrder = *input.SortOrder
	}
	if input.Enabled != nil {
		after.Enabled = *input.Enabled
	}
	normalized, err := normalizeMenuInput(MenuInput{
		Code: before.Code, ParentID: after.ParentID, Name: after.Name, MenuType: after.MenuType,
		Path: after.Path, Icon: after.Icon, HostKey: after.HostKey, SortOrder: after.SortOrder, Enabled: after.Enabled,
	})
	if err == nil {
		after.ParentID = normalized.ParentID
		after.Name = normalized.Name
		after.MenuType = normalized.MenuType
		after.Path = normalized.Path
		after.Icon = normalized.Icon
		after.HostKey = normalized.HostKey
		after.SortOrder = normalized.SortOrder
		after.Enabled = normalized.Enabled
	}
	return after, err
}

func normalizedMenuUpdate(input MenuUpdate, after Menu) MenuUpdate {
	result := MenuUpdate{ParentSet: input.ParentSet}
	if input.ParentSet {
		result.ParentID = after.ParentID
	}
	if input.Name != nil {
		result.Name = &after.Name
	}
	if input.MenuType != nil {
		result.MenuType = &after.MenuType
	}
	if input.Path != nil {
		result.Path = &after.Path
	}
	if input.Icon != nil {
		result.Icon = &after.Icon
	}
	if input.HostKey != nil {
		result.HostKey = &after.HostKey
	}
	if input.SortOrder != nil {
		result.SortOrder = &after.SortOrder
	}
	if input.Enabled != nil {
		result.Enabled = &after.Enabled
	}
	return result
}

func validMenuPath(menuType, path string) bool {
	switch menuType {
	case MenuTypeDirectory:
		return path == ""
	case MenuTypePage:
		return strings.HasPrefix(path, "/") && !strings.HasPrefix(path, "//") && !strings.ContainsAny(path, "?#")
	case MenuTypeExternal:
		parsed, err := url.Parse(path)
		return err == nil && (parsed.Scheme == "http" || parsed.Scheme == "https") && parsed.Host != "" && parsed.User == nil
	default:
		return false
	}
}

func normalizedMenuIDs(ids []uint) ([]uint, error) {
	result := make([]uint, len(ids))
	seen := make(map[uint]struct{}, len(ids))
	for index, id := range ids {
		if id == 0 {
			return nil, ErrMenuDuplicateID
		}
		if _, exists := seen[id]; exists {
			return nil, ErrMenuDuplicateID
		}
		seen[id] = struct{}{}
		result[index] = id
	}
	return result, nil
}

func menuTree(byID map[uint]Menu, included, direct map[uint]struct{}) []MenuNode {
	children := make(map[uint][]uint, len(included))
	roots := make([]uint, 0, len(included))
	for id := range included {
		menu := byID[id]
		if menu.ParentID == nil {
			roots = append(roots, id)
			continue
		}
		if _, exists := included[*menu.ParentID]; exists {
			children[*menu.ParentID] = append(children[*menu.ParentID], id)
		}
	}
	sortMenuIDs(byID, roots)
	for parentID := range children {
		sortMenuIDs(byID, children[parentID])
	}
	var build func(uint) MenuNode
	build = func(id uint) MenuNode {
		menu := byID[id]
		_, directlyGranted := direct[id]
		node := MenuNode{Menu: menu, DirectGrant: directlyGranted, Structural: !directlyGranted, Children: []MenuNode{}}
		for _, childID := range children[id] {
			node.Children = append(node.Children, build(childID))
		}
		return node
	}
	result := make([]MenuNode, 0, len(roots))
	for _, id := range roots {
		result = append(result, build(id))
	}
	return result
}

func sortMenuIDs(byID map[uint]Menu, ids []uint) {
	sort.Slice(ids, func(left, right int) bool {
		a, b := byID[ids[left]], byID[ids[right]]
		if a.SortOrder == b.SortOrder {
			return a.ID < b.ID
		}
		return a.SortOrder < b.SortOrder
	})
}
