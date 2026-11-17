package projectgorm

import (
	"context"
	"errors"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/project"
	"gorm.io/gorm"
)

func TestListRepositoryScopesFiltersAndPaginatesProjectSets(t *testing.T) {
	db := openProjectListTestDB(t)
	rows := []ProjectSet{
		{WorkspaceID: 7, TeamID: 70, Name: "Alpha", Code: "PM-A", Status: "active"},
		{WorkspaceID: 7, TeamID: 71, Name: "Beta", Code: "PM-B", Status: "active"},
		{WorkspaceID: 8, TeamID: 70, Name: "Alpha other", Code: "PM-X", Status: "active"},
	}
	if err := db.Create(&rows).Error; err != nil {
		t.Fatal(err)
	}
	repository := NewListRepository(db)
	page, err := repository.ListProjectSets(context.Background(), 7, project.ProjectSetListFilter{
		TeamID: 70, Query: "Alpha", Limit: 1, Offset: 0,
	})
	if err != nil {
		t.Fatal(err)
	}
	if page.Total != 1 || len(page.Items) != 1 || page.Items[0].WorkspaceID != 7 || page.Items[0].TeamID != 70 {
		t.Fatalf("page=%+v", page)
	}
	empty, err := repository.ListProjectSets(context.Background(), 7, project.ProjectSetListFilter{
		Query: "Alpha", Limit: 1, Offset: 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	if empty.Total != 1 || len(empty.Items) != 0 || empty.Items == nil {
		t.Fatalf("empty=%+v", empty)
	}
}

func TestListRepositoryScopesPlanUnitsThroughWorkspaceParent(t *testing.T) {
	db := openProjectListTestDB(t)
	projectRow := ProjectSet{WorkspaceID: 7, TeamID: 70, Name: "Workspace seven", Status: "active"}
	if err := db.Create(&projectRow).Error; err != nil {
		t.Fatal(err)
	}
	units := []PlanUnit{
		{WorkspaceID: 7, TeamID: 70, ProjectSetID: projectRow.ID, Title: "V1", Type: "version", Status: "planning"},
		{WorkspaceID: 8, TeamID: 70, ProjectSetID: projectRow.ID, Title: "Other", Type: "version", Status: "planning"},
	}
	if err := db.Create(&units).Error; err != nil {
		t.Fatal(err)
	}
	repository := NewListRepository(db)
	listed, err := repository.ListPlanUnits(context.Background(), 7, projectRow.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(listed) != 1 || listed[0].WorkspaceID != 7 || listed[0].ProjectSetID != projectRow.ID {
		t.Fatalf("listed=%+v", listed)
	}
	if _, err := repository.ListPlanUnits(context.Background(), 8, projectRow.ID); !errors.Is(err, project.ErrProjectSetNotFound) {
		t.Fatalf("cross-workspace err=%v", err)
	}
}

func TestListServiceNormalizesSnapshotPagination(t *testing.T) {
	store := &listStoreCapture{}
	service, err := project.NewListService(store)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.ListProjectSets(context.Background(), 7, project.ProjectSetListFilter{Limit: 900}); err != nil {
		t.Fatal(err)
	}
	if store.filter.Limit != project.MaximumProjectSetListLimit {
		t.Fatalf("limit=%d", store.filter.Limit)
	}
	if _, err := service.ListProjectSets(context.Background(), 7, project.ProjectSetListFilter{Offset: -1}); !errors.Is(err, project.ErrReadQueryInvalid) {
		t.Fatalf("negative offset err=%v", err)
	}
}

type listStoreCapture struct{ filter project.ProjectSetListFilter }

func (store *listStoreCapture) ListProjectSets(
	_ context.Context,
	_ uint,
	filter project.ProjectSetListFilter,
) (project.ProjectSetListPage, error) {
	store.filter = filter
	return project.ProjectSetListPage{Items: []project.ProjectSetSnapshot{}}, nil
}

func (*listStoreCapture) ListPlanUnits(context.Context, uint, uint) ([]project.PlanUnitSnapshot, error) {
	return []project.PlanUnitSnapshot{}, nil
}

func openProjectListTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&ProjectSet{}, &PlanUnit{}); err != nil {
		t.Fatal(err)
	}
	return db
}
