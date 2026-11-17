package requirementgorm

import (
	"context"
	"errors"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/requirement"
	"gorm.io/gorm"
)

func TestReadRepositoryScopesRequirementByWorkspace(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&Requirement{}); err != nil {
		t.Fatal(err)
	}
	row := Requirement{WorkspaceID: 11, TeamID: 110, ProjectSetID: 111, Title: "Private requirement", Priority: "normal", Status: "backlog"}
	if err := db.Create(&row).Error; err != nil {
		t.Fatal(err)
	}
	repository := NewReadRepository(db)
	if got, err := repository.GetRequirement(context.Background(), 11, row.ID); err != nil || got.WorkspaceID != 11 {
		t.Fatalf("workspace requirement: got=%+v err=%v", got, err)
	}
	if _, err := repository.GetRequirement(context.Background(), 12, row.ID); !errors.Is(err, requirement.ErrRequirementNotFound) {
		t.Fatalf("cross-workspace requirement err=%v", err)
	}
}
