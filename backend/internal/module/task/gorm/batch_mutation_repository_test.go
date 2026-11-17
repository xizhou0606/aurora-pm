package taskgorm

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/task"
	"gorm.io/gorm"
)

type batchScopeFake struct {
	scopes map[uint]task.BatchRequirementScope
}

func (fake batchScopeFake) ResolveTaskRequirementScope(_ context.Context, workspaceID, requirementID uint) (task.BatchRequirementScope, error) {
	scope, found := fake.scopes[requirementID]
	if !found || scope.WorkspaceID != workspaceID {
		return task.BatchRequirementScope{}, task.ErrBatchMutationConflict
	}
	return scope, nil
}

type batchUserFake struct{}

func (batchUserFake) VerifyTaskBatchUsers(_ context.Context, _ uint, ids []uint) error {
	for _, id := range ids {
		if id == 99 {
			return task.ErrBatchMutationConflict
		}
	}
	return nil
}

type batchEffectFake struct {
	calls  *int
	failAt int
}

func (fake batchEffectFake) RecordTaskBatchEffect(context.Context, task.BatchEffect) error {
	*fake.calls++
	if fake.failAt > 0 && *fake.calls == fake.failAt {
		return errors.New("effect failed")
	}
	return nil
}

type batchNotificationFake struct{ calls *int }

func (fake batchNotificationFake) RecordTaskAssignment(context.Context, task.AssignmentNotification) error {
	*fake.calls++
	return nil
}

func TestTaskBatchCreateIsAtomicScopedAndReplaySafe(t *testing.T) {
	repository, db, effects, notifications := newTaskBatchFixture(t, 0)
	command := task.BatchMutation{
		Operation: task.BatchCreate, WorkspaceID: 7, PrincipalID: 41,
		ApplyKey: "task-batch-create-0001", ChangeSetID: "changeset-batch-create-0001", InputDigest: batchDigest("create"),
		Tasks: []task.BatchTaskInput{{
			ClientID: "parent", RequirementID: 13, Title: "父任务",
			Children: []task.BatchTaskInput{{ClientID: "child", Title: "子任务"}},
		}},
	}
	first, err := repository.ApplyTaskBatchMutation(context.Background(), command)
	if err != nil || first.CreatedCount != 2 || len(first.Created) != 1 || len(first.Created[0].Children) != 1 {
		t.Fatalf("first=%+v err=%v", first, err)
	}
	second, err := repository.ApplyTaskBatchMutation(context.Background(), command)
	if err != nil || second.CreatedCount != 2 || *effects != 2 || *notifications != 2 {
		t.Fatalf("replay=%+v effects=%d notifications=%d err=%v", second, *effects, *notifications, err)
	}
	var rows []WorkTask
	if err := db.Order("id").Find(&rows).Error; err != nil || len(rows) != 2 || rows[1].ParentID == nil || *rows[1].ParentID != rows[0].ID ||
		rows[0].WorkspaceID != 7 || rows[0].RequirementID != 13 || rows[0].AssigneeID == nil || *rows[0].AssigneeID != 41 {
		t.Fatalf("rows=%+v err=%v", rows, err)
	}
	assertBatchModelCount(t, db, &WorkTaskActivity{}, 2)
	assertBatchModelCount(t, db, &TaskToolMutationApply{}, 1)
}

func TestTaskBatchCreateRollsBackEveryWriteWhenEffectFails(t *testing.T) {
	repository, db, _, _ := newTaskBatchFixture(t, 2)
	_, err := repository.ApplyTaskBatchMutation(context.Background(), task.BatchMutation{
		Operation: task.BatchCreate, WorkspaceID: 7, PrincipalID: 41,
		ApplyKey: "task-batch-create-fail-0001", ChangeSetID: "changeset-batch-create-fail-0001", InputDigest: batchDigest("failure"),
		Tasks: []task.BatchTaskInput{{ClientID: "a", RequirementID: 13, Title: "A"}, {ClientID: "b", RequirementID: 13, Title: "B"}},
	})
	if err == nil {
		t.Fatal("expected effect failure")
	}
	assertBatchModelCount(t, db, &WorkTask{}, 0)
	assertBatchModelCount(t, db, &WorkTaskActivity{}, 0)
	assertBatchModelCount(t, db, &TaskToolMutationApply{}, 0)
}

func TestTaskChildrenBatchCreateFreezesParentScope(t *testing.T) {
	repository, db, _, _ := newTaskBatchFixture(t, 0)
	parent := WorkTask{ID: 71, WorkspaceID: 7, TeamID: 8, ProjectSetID: 11, ProjectID: 12, RequirementID: 13,
		Kind: WorkTaskKindTask, TaskKey: "PM-71", Title: "Parent", Status: "draft", Priority: "high", AssigneeID: uintBatchPointer(42), MutationState: workTaskStateActive}
	if err := db.Create(&parent).Error; err != nil {
		t.Fatal(err)
	}
	result, err := repository.ApplyTaskBatchMutation(context.Background(), task.BatchMutation{
		Operation: task.ChildrenBatchCreate, WorkspaceID: 7, PrincipalID: 41, ParentID: parent.ID,
		ApplyKey: "task-children-create-0001", ChangeSetID: "changeset-children-create-0001", InputDigest: batchDigest("children"),
		Tasks: []task.BatchTaskInput{{ClientID: "child-a", Title: "Child A"}},
	})
	if err != nil || result.ParentID != parent.ID || result.CreatedCount != 1 {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	var child WorkTask
	if err := db.Where("parent_id = ?", parent.ID).First(&child).Error; err != nil || child.ProjectID != parent.ProjectID || child.AssigneeID == nil || *child.AssigneeID != 42 {
		t.Fatalf("child=%+v err=%v", child, err)
	}
}

func TestTaskBatchUpdateCASPropagatesDueAndAutoCompletesAncestors(t *testing.T) {
	repository, db, effects, _ := newTaskBatchFixture(t, 0)
	parent := WorkTask{ID: 81, WorkspaceID: 7, TeamID: 8, ProjectSetID: 11, ProjectID: 12, RequirementID: 13,
		Kind: WorkTaskKindTask, TaskKey: "PM-81", Title: "Parent", Status: "draft", Priority: "normal", MutationState: workTaskStateActive}
	first := WorkTask{ID: 82, WorkspaceID: 7, TeamID: 8, ProjectSetID: 11, ProjectID: 12, RequirementID: 13,
		Kind: WorkTaskKindSubtask, ParentID: &parent.ID, TaskKey: "PM-82", Title: "One", Status: "draft", Priority: "normal", MutationState: workTaskStateActive}
	second := first
	second.ID, second.TaskKey, second.Title = 83, "PM-83", "Two"
	if err := db.Create(&[]WorkTask{parent, first, second}).Error; err != nil {
		t.Fatal(err)
	}
	completed := "completed"
	command := task.BatchMutation{
		Operation: task.BatchUpdate, WorkspaceID: 7, PrincipalID: 41,
		ApplyKey: "task-batch-update-0001", ChangeSetID: "changeset-batch-update-0001", InputDigest: batchDigest("update"),
		Selector: task.BatchTaskSelector{TaskRefs: []string{"PM-82", "PM-83"}},
		Updates:  task.BatchTaskPatch{Status: task.Optional[string]{Set: true, Value: &completed}},
	}
	result, err := repository.ApplyTaskBatchMutation(context.Background(), command)
	if err != nil || result.UpdatedCount != 2 || len(result.AutoCompletedAncestorIDs) != 1 || result.AutoCompletedAncestorIDs[0] != parent.ID {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	if _, err := repository.ApplyTaskBatchMutation(context.Background(), command); err != nil || *effects != 3 {
		t.Fatalf("replay effects=%d err=%v", *effects, err)
	}
	var actualParent WorkTask
	if err := db.First(&actualParent, parent.ID).Error; err != nil || actualParent.Status != "completed" || actualParent.CompletedAt == nil {
		t.Fatalf("parent=%+v err=%v", actualParent, err)
	}

	due := time.Now().UTC().Add(48 * time.Hour).Truncate(time.Second)
	dueCommand := task.BatchMutation{
		Operation: task.BatchUpdate, WorkspaceID: 7, PrincipalID: 41,
		ApplyKey: "task-batch-update-0002", ChangeSetID: "changeset-batch-update-0002", InputDigest: batchDigest("due"),
		Selector: task.BatchTaskSelector{IDs: []uint{parent.ID}},
		Updates:  task.BatchTaskPatch{DueAt: task.Optional[time.Time]{Set: true, Value: &due}},
	}
	dueResult, err := repository.ApplyTaskBatchMutation(context.Background(), dueCommand)
	if err != nil || len(dueResult.DuePropagatedDescendantIDs) != 2 {
		t.Fatalf("due result=%+v err=%v", dueResult, err)
	}
}

func TestTaskBatchSelectionIsWorkspaceScopedAndRejectsAmbiguousTitles(t *testing.T) {
	repository, db, _, _ := newTaskBatchFixture(t, 0)
	rows := []WorkTask{
		{ID: 91, WorkspaceID: 7, TeamID: 8, ProjectSetID: 11, ProjectID: 12, RequirementID: 13, Kind: WorkTaskKindTask, TaskKey: "PM-91", Title: "Same", Status: "draft", Priority: "normal", MutationState: workTaskStateActive},
		{ID: 92, WorkspaceID: 7, TeamID: 8, ProjectSetID: 11, ProjectID: 12, RequirementID: 13, Kind: WorkTaskKindTask, TaskKey: "PM-92", Title: "Same", Status: "draft", Priority: "normal", MutationState: workTaskStateActive},
		{ID: 93, WorkspaceID: 8, TeamID: 8, ProjectSetID: 11, ProjectID: 12, RequirementID: 13, Kind: WorkTaskKindTask, TaskKey: "PM-93", Title: "Foreign", Status: "draft", Priority: "normal", MutationState: workTaskStateActive},
		{ID: 95, WorkspaceID: 7, TeamID: 8, ProjectSetID: 11, ProjectID: 12, RequirementID: 13, Kind: WorkTaskKindTask, TaskKey: "PM-95", Title: "Deleting", Status: "draft", Priority: "normal", MutationState: workTaskStateDeleting},
	}
	if err := db.Create(&rows).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := repository.ResolveTaskBatchSelection(context.Background(), 7, task.BatchTaskSelector{TaskRefs: []string{"Same"}}); !errors.Is(err, task.ErrBatchMutationAmbiguous) {
		t.Fatalf("ambiguous err=%v", err)
	}
	if _, err := repository.ResolveTaskBatchSelection(context.Background(), 7, task.BatchTaskSelector{IDs: []uint{93}}); !errors.Is(err, task.ErrBatchMutationNotFound) {
		t.Fatalf("cross-workspace err=%v", err)
	}
	if _, err := repository.ResolveTaskBatchSelection(context.Background(), 7, task.BatchTaskSelector{IDs: []uint{95}}); !errors.Is(err, task.ErrBatchMutationNotFound) {
		t.Fatalf("deleting-state err=%v", err)
	}
	ids, err := repository.ResolveTaskBatchSelection(context.Background(), 7, task.BatchTaskSelector{
		TaskRefs: []string{"PM-91"}, ProjectSetID: 11, PlanUnitID: 12, RequirementID: 13,
	})
	if err != nil || len(ids) != 1 || ids[0] != 91 {
		t.Fatalf("ids=%v err=%v", ids, err)
	}
}

func TestTaskBatchDeletionFreezesTaskRefsForReplayAfterRowsAreDeleted(t *testing.T) {
	repository, db, _, _ := newTaskBatchFixture(t, 0)
	row := WorkTask{ID: 94, WorkspaceID: 7, TeamID: 8, ProjectSetID: 11, ProjectID: 12, RequirementID: 13,
		Kind: WorkTaskKindTask, TaskKey: "PM-94", Title: "Delete Me", Status: "draft", Priority: "normal", MutationState: workTaskStateActive}
	if err := db.Create(&row).Error; err != nil {
		t.Fatal(err)
	}
	command := task.BatchMutation{
		Operation: task.BatchDelete, WorkspaceID: 7, PrincipalID: 41,
		ApplyKey: "task-batch-delete-0001", ChangeSetID: "changeset-batch-delete-0001", InputDigest: batchDigest("delete"),
		Selector: task.BatchTaskSelector{TaskRefs: []string{"PM-94"}},
	}
	first, err := repository.ResolveTaskBatchDeletion(context.Background(), command)
	if err != nil || len(first) != 1 || first[0] != row.ID {
		t.Fatalf("first=%v err=%v", first, err)
	}
	if err := db.Delete(&row).Error; err != nil {
		t.Fatal(err)
	}
	second, err := repository.ResolveTaskBatchDeletion(context.Background(), command)
	if err != nil || len(second) != 1 || second[0] != row.ID {
		t.Fatalf("replay=%v err=%v", second, err)
	}
	assertBatchModelCount(t, db, &TaskBatchDeletionApply{}, 1)
	conflict := command
	conflict.InputDigest = batchDigest("different")
	if _, err := repository.ResolveTaskBatchDeletion(context.Background(), conflict); !errors.Is(err, task.ErrBatchMutationConflict) {
		t.Fatalf("conflict err=%v", err)
	}
}

func newTaskBatchFixture(t *testing.T, effectFailAt int) (*BatchMutationRepository, *gorm.DB, *int, *int) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&WorkTask{}, &WorkTaskActivity{}, &KeySequence{}, &TaskToolMutationApply{}, &TaskBatchDeletionApply{}); err != nil {
		t.Fatal(err)
	}
	effects, notifications := 0, 0
	repository, err := NewBatchMutationRepository(db, func(*gorm.DB) (BatchMutationDependencies, error) {
		return BatchMutationDependencies{
			Scopes: batchScopeFake{scopes: map[uint]task.BatchRequirementScope{13: {
				WorkspaceID: 7, TeamID: 8, ProjectSetID: 11, PlanUnitID: 12, RequirementID: 13, ProjectCode: "PM",
			}}}, Users: batchUserFake{}, Effects: batchEffectFake{calls: &effects, failAt: effectFailAt},
			Notifications: batchNotificationFake{calls: &notifications},
		}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return repository, db, &effects, &notifications
}

func assertBatchModelCount(t *testing.T, db *gorm.DB, model any, want int64) {
	t.Helper()
	var got int64
	if err := db.Model(model).Count(&got).Error; err != nil || got != want {
		t.Fatalf("count %T=%d/%d err=%v", model, got, want, err)
	}
}

func batchDigest(value string) string {
	for len(value) < 64 {
		value += "0"
	}
	return value[:64]
}
