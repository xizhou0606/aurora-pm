package workbench_test

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	workbench "github.com/xizhou0606/aurora-pm/backend/internal/module/workbench"
	workbenchgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/workbench/gorm"
)

func TestMenuServiceRejectsInvalidHierarchyAndHostBoundary(t *testing.T) {
	menus := newMenuFixture(t)
	parent, err := menus.Create(context.Background(), workbench.MenuInput{Code: "pm-root", Name: "PM", MenuType: workbench.MenuTypeDirectory, HostKey: "pm", Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := menus.Create(context.Background(), workbench.MenuInput{Code: "wrong-host", ParentID: &parent.ID, Name: "Other", MenuType: workbench.MenuTypePage, Path: "/other", HostKey: "one2all", Enabled: true}); !errors.Is(err, workbench.ErrMenuParentInvalid) {
		t.Fatalf("cross-host parent err=%v", err)
	}
	if _, err := menus.Create(context.Background(), workbench.MenuInput{Code: "non-directory", Name: "Page", MenuType: workbench.MenuTypePage, Path: "/page", HostKey: "pm", Enabled: true}); err != nil {
		t.Fatal(err)
	}
}

func TestMenuServiceBootstrapIsIdempotentAndProtectsRegularCode(t *testing.T) {
	menus := newMenuFixture(t)
	input := workbench.MenuInput{Code: "access-administration", Name: "Access administration", MenuType: workbench.MenuTypePage, Path: "/access/admin", HostKey: "pm", Enabled: true}
	first, err := menus.Bootstrap(context.Background(), []workbench.MenuInput{input})
	if err != nil || len(first) != 1 || !first[0].SystemManaged {
		t.Fatalf("first bootstrap=%+v err=%v", first, err)
	}
	second, err := menus.Bootstrap(context.Background(), []workbench.MenuInput{input})
	if err != nil || len(second) != 1 || second[0].ID != first[0].ID || !second[0].SystemManaged {
		t.Fatalf("second bootstrap=%+v err=%v", second, err)
	}

	regular := workbench.MenuInput{Code: "regular-menu", Name: "Regular", MenuType: workbench.MenuTypePage, Path: "/regular", HostKey: "pm", Enabled: true}
	if _, err := menus.Create(context.Background(), regular); err != nil {
		t.Fatal(err)
	}
	if _, err := menus.Bootstrap(context.Background(), []workbench.MenuInput{regular}); !errors.Is(err, workbench.ErrMenuCodeConflict) {
		t.Fatalf("regular code bootstrap err=%v", err)
	}
}

func newMenuFixture(t *testing.T) *workbench.MenuService {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&workbenchgorm.AccessMenu{}); err != nil {
		t.Fatal(err)
	}
	return workbench.NewMenuService(workbenchgorm.NewAccessMenuRepository(db), nil)
}
