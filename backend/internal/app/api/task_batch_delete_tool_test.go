package api

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/xizhou0606/aurora-pm/backend/internal/integration/objectstore"
	projectgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/project/gorm"
	requirementgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/requirement/gorm"
	taskgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/task/gorm"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/toolruntime"
	toolruntimegorm "github.com/xizhou0606/aurora-pm/backend/internal/module/toolruntime/gorm"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/toolruntime/pmtools"
	"github.com/xizhou0606/aurora-pm/backend/internal/platform/config"
	"github.com/xizhou0606/aurora-pm/backend/internal/platform/database"
	"gorm.io/gorm"
)

type taskBatchDeleteAuthorizer struct{}

func (taskBatchDeleteAuthorizer) AuthorizeGrant(context.Context, toolruntime.ExecutionAuthorization) (toolruntime.AuthorizedGrant, error) {
	return toolruntime.NewAuthorizedGrant([]string{pmtools.TaskBatchDelete}, []string{"task:write"})
}

func TestTaskBatchDeleteToolDeletes196ExplicitTasksAndKeepsExcludedTask(t *testing.T) {
	app, db, auth := newTaskBatchDeleteToolFixture(t)
	ids := make([]uint, 196)
	rows := make([]taskgorm.WorkTask, 0, 198)
	for i := range ids {
		ids[i] = uint(100 + i)
		rows = append(rows, batchDeleteToolTask(ids[i], 7))
	}
	rows = append(rows, batchDeleteToolTask(1, 7), batchDeleteToolTask(999, 8))
	if err := db.Create(&rows).Error; err != nil {
		t.Fatal(err)
	}
	input, _ := json.Marshal(map[string]any{"ids": ids, "cascade": false})
	pending := prepareTaskBatchDelete(t, app, auth, input)
	result, err := app.ConfirmChangeSet(context.Background(), pending.ID, auth)
	if err != nil || result.Status != toolruntime.ChangeSetSucceeded {
		t.Fatalf("confirm status=%s code=%s err=%v", result.Status, result.ErrorCode, err)
	}
	var output struct {
		DeletedCount int    `json:"deleted_count"`
		DeletedIDs   []uint `json:"deleted_ids"`
	}
	if json.Unmarshal(result.Result, &output) != nil || output.DeletedCount != 196 || len(output.DeletedIDs) != 196 {
		t.Fatalf("result=%s", result.Result)
	}
	var remaining []taskgorm.WorkTask
	if err := db.Order("id").Find(&remaining).Error; err != nil || len(remaining) != 2 || remaining[0].ID != 1 || remaining[1].ID != 999 {
		t.Fatalf("remaining=%+v err=%v", remaining, err)
	}
	if replay, err := app.ConfirmChangeSet(context.Background(), pending.ID, auth); err != nil || replay.Status != toolruntime.ChangeSetSucceeded {
		t.Fatalf("replay=%+v err=%v", replay, err)
	}
}

func TestTaskBatchDeleteToolCascadeIsExplicitAndConflictsArePublic(t *testing.T) {
	for _, cascade := range []bool{false, true} {
		t.Run(fmt.Sprint(cascade), func(t *testing.T) {
			app, db, auth := newTaskBatchDeleteToolFixture(t)
			parent, child := batchDeleteToolTask(100, 7), batchDeleteToolTask(101, 7)
			child.ParentID, child.Kind = &parent.ID, "subtask"
			if err := db.Create(&[]taskgorm.WorkTask{parent, child}).Error; err != nil {
				t.Fatal(err)
			}
			input, _ := json.Marshal(map[string]any{"ids": []uint{parent.ID}, "cascade": cascade})
			pending := prepareTaskBatchDelete(t, app, auth, input)
			result, err := app.ConfirmChangeSet(context.Background(), pending.ID, auth)
			if err != nil {
				t.Fatal(err)
			}
			var count int64
			if err := db.Model(&taskgorm.WorkTask{}).Count(&count).Error; err != nil {
				t.Fatal(err)
			}
			if cascade {
				if result.Status != toolruntime.ChangeSetSucceeded || count != 0 {
					t.Fatalf("result=%+v count=%d", result, count)
				}
			} else {
				failure := toolruntime.LookupBusinessError(result.ErrorCode)
				if result.Status != toolruntime.ChangeSetFailed || result.ErrorCode != "TASK_BATCH_CONFLICT" || failure == nil || failure.Status != 409 || count != 2 {
					t.Fatalf("result=%+v failure=%+v count=%d", result, failure, count)
				}
				var deleting int64
				if err := db.Model(&taskgorm.WorkTask{}).Where("mutation_state <> ?", "active").Count(&deleting).Error; err != nil || deleting != 0 {
					t.Fatalf("conflict changed task state: count=%d err=%v", deleting, err)
				}
			}
		})
	}
}

func TestTaskBatchDeleteToolRejectsForeignWorkspaceWithoutPartialDeletion(t *testing.T) {
	app, db, auth := newTaskBatchDeleteToolFixture(t)
	if err := db.Create(&[]taskgorm.WorkTask{batchDeleteToolTask(100, 7), batchDeleteToolTask(101, 8)}).Error; err != nil {
		t.Fatal(err)
	}
	pending := prepareTaskBatchDelete(t, app, auth, json.RawMessage(`{"ids":[100,101],"cascade":false}`))
	result, err := app.ConfirmChangeSet(context.Background(), pending.ID, auth)
	if err != nil || result.Status != toolruntime.ChangeSetFailed || result.ErrorCode != "TASK_BATCH_NOT_FOUND" {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	var count int64
	if err := db.Model(&taskgorm.WorkTask{}).Where("mutation_state = ?", "active").Count(&count).Error; err != nil || count != 2 {
		t.Fatalf("partially deleted or changed task state: count=%d err=%v", count, err)
	}
}

func batchDeleteToolTask(id, workspaceID uint) taskgorm.WorkTask {
	return taskgorm.WorkTask{ID: id, WorkspaceID: workspaceID, TeamID: 8, ProjectSetID: 11, ProjectID: 12,
		RequirementID: 13, Kind: "task", TaskKey: fmt.Sprintf("PM18-%d", id), Title: fmt.Sprintf("Task %d", id),
		Status: "todo", Priority: "normal", MutationState: "active"}
}

func prepareTaskBatchDelete(t *testing.T, app *toolruntime.Application, auth toolruntime.ExecutionAuthorization, input json.RawMessage) toolruntime.ChangeSet {
	t.Helper()
	response, err := app.Invoke(context.Background(), toolruntime.Invocation{Name: pmtools.TaskBatchDelete, Input: input,
		IdempotencyKey: "batch-delete-regression-0001", Authorization: auth})
	if err != nil || response.ChangeSet == nil || response.ChangeSet.Status != toolruntime.ChangeSetPending {
		t.Fatalf("prepare=%+v err=%v", response, err)
	}
	return *response.ChangeSet
}

func newTaskBatchDeleteToolFixture(t *testing.T) (*toolruntime.Application, *gorm.DB, toolruntime.ExecutionAuthorization) {
	t.Helper()
	cfg := config.Config{Environment: "test", DatabaseDriver: "sqlite", DatabaseDSN: "file:" + t.Name() + "?mode=memory&cache=shared"}
	db, err := database.Open(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if err := database.Migrate(db); err != nil {
		t.Fatal(err)
	}
	for _, row := range []any{
		&projectgorm.ProjectSet{ID: 11, WorkspaceID: 7, TeamID: 8, Name: "Project", Code: "PM18", Status: "active"},
		&projectgorm.PlanUnit{ID: 12, WorkspaceID: 7, TeamID: 8, ProjectSetID: 11, Title: "Plan", Type: "version", Status: "planning"},
		&requirementgorm.Requirement{ID: 13, WorkspaceID: 7, TeamID: 8, ProjectSetID: 11, PlanUnitID: 12, Title: "Requirement", Version: 1},
	} {
		if err := db.Create(row).Error; err != nil {
			t.Fatal(err)
		}
	}
	dependencies, err := newPMToolRuntimeDependencies(db, objectstore.NewRemovalAdapter(cfg.ObjectStorage), cfg)
	if err != nil {
		t.Fatal(err)
	}
	changes, err := toolruntime.NewChangeSetService(toolruntimegorm.NewChangeSetRepository(db), toolruntime.DefaultChangeSetPolicy())
	if err != nil {
		t.Fatal(err)
	}
	app, err := toolruntime.NewApplication(dependencies.Catalog, taskBatchDeleteAuthorizer{}, dependencies.Handlers, toolruntime.WithChangeSetService(changes))
	if err != nil {
		t.Fatal(err)
	}
	return app, db, toolruntime.ExecutionAuthorization{PrincipalID: 3, WorkspaceID: 7, GrantID: 5, ClientID: "pm-cli", Channel: toolruntime.DeliveryCLI, Audience: toolruntime.AudienceCLI}
}
