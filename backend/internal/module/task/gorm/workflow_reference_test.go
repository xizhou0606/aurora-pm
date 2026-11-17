package taskgorm

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/glebarez/sqlite"
	identitygorm "github.com/xizhou0606/aurora-pm/backend/internal/module/identity/gorm"
	projectgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/project/gorm"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/task"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func TestWorkflowScopeRepositoryEnforcesWorkspaceTeamAndProjectHierarchy(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&identitygorm.Team{}, &projectgorm.ProjectSet{}, &projectgorm.PlanUnit{}); err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&identitygorm.Team{ID: 10, WorkspaceID: 1, Name: "研发"}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&projectgorm.ProjectSet{ID: 11, WorkspaceID: 1, TeamID: 10, Name: "PM"}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&projectgorm.PlanUnit{ID: 12, WorkspaceID: 1, TeamID: 10, ProjectSetID: 11, Title: "v1"}).Error; err != nil {
		t.Fatal(err)
	}
	repository := NewWorkflowScopeRepository(db)
	if err := repository.ValidateWorkflowScope(context.Background(), 1, task.WorkflowScope{TeamID: 10, ProjectID: 12}); err != nil {
		t.Fatal(err)
	}
	for name, scope := range map[string]task.WorkflowScope{
		"project without team": {ProjectID: 12},
		"wrong team":           {TeamID: 20},
		"wrong project":        {TeamID: 10, ProjectID: 22},
	} {
		t.Run(name, func(t *testing.T) {
			if err := repository.ValidateWorkflowScope(context.Background(), 1, scope); !errors.Is(err, task.ErrWorkflowReferenceInvalid) {
				t.Fatalf("err=%v", err)
			}
		})
	}
	if err := repository.ValidateWorkflowScope(context.Background(), 2, task.WorkflowScope{TeamID: 10}); !errors.Is(err, task.ErrWorkflowReferenceInvalid) {
		t.Fatalf("cross-workspace err=%v", err)
	}
}
