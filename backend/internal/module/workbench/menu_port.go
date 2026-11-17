package workbench

import "context"

// MenuRepository is workbench-owned persistence. Identity never reads this
// storage directly; it validates menu grants through MenuService instead.
type MenuRepository interface {
	ListMenus(context.Context, bool) ([]Menu, error)
	ListEnabledMenus(context.Context) ([]Menu, error)
	FindMenu(context.Context, uint) (Menu, error)
	EnsureSystemMenus(context.Context, []MenuInput) ([]Menu, error)
	CreateMenu(context.Context, MenuInput) (Menu, error)
	UpdateMenu(context.Context, uint, MenuUpdate) (Menu, error)
	DeleteMenu(context.Context, uint) error
	HasChildren(context.Context, uint) (bool, error)
	EnabledMenuIDs(context.Context, []uint) ([]uint, error)
}

// MenuGrantReader is fulfilled by identity's AccessService. It conveys grant
// facts only, leaving menu records and hierarchy rules to workbench.
type MenuGrantReader interface {
	DirectMenuIDsForUser(context.Context, uint) ([]uint, error)
	CountMenuGrants(context.Context, uint) (int64, error)
}
