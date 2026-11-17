package projectgorm

import (
	"context"
	"errors"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/project"
	"gorm.io/gorm"
)

func TestReadRepositoryScopesProjectAndPlanUnitByWorkspace(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&ProjectSet{}, &PlanUnit{}); err != nil {
		t.Fatal(err)
	}
	projectRow := ProjectSet{WorkspaceID: 7, TeamID: 70, Name: "Workspace seven", Status: "active"}
	if err := db.Create(&projectRow).Error; err != nil {
		t.Fatal(err)
	}
	unit := PlanUnit{WorkspaceID: 7, TeamID: 70, ProjectSetID: projectRow.ID, Title: "V1", Type: "version", Status: "planning"}
	if err := db.Create(&unit).Error; err != nil {
		t.Fatal(err)
	}
	repository := NewReadRepository(db)
	if got, err := repository.GetProjectSet(context.Background(), 7, projectRow.ID); err != nil || got.WorkspaceID != 7 {
		t.Fatalf("workspace project: got=%+v err=%v", got, err)
	}
	if _, err := repository.GetProjectSet(context.Background(), 8, projectRow.ID); !errors.Is(err, project.ErrProjectSetNotFound) {
		t.Fatalf("cross-workspace project err=%v", err)
	}
	if _, err := repository.GetPlanUnit(context.Background(), 8, unit.ID); !errors.Is(err, project.ErrPlanUnitNotFound) {
		t.Fatalf("cross-workspace plan unit err=%v", err)
	}
}
