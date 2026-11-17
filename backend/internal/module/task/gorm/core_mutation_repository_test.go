package taskgorm

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/task"
	"gorm.io/gorm"
)

type coreEffectTestRecord struct {
	PreviousStatus string
	ID             uint `gorm:"primaryKey"`
	WorkspaceID    uint
	TeamID         uint
	PlanUnitID     uint
	TaskID         uint
	TaskKey        string
	Action         string
	NotifyAssignee bool
}
type coreCleanerTestRecord struct {
	ID              uint `gorm:"primaryKey"`
	WorkspaceID     uint
	TaskID          uint
	TestExecutionID uint
	ExecutionExists bool
}

type coreFactsTestAdapter struct {
	scopes map[uint]task.CoreMutationScope
	err    error
}

func (adapter *coreFactsTestAdapter) ResolveCoreMutationScope(_ context.Context, workspaceID, requirementID uint) (task.CoreMutationScope, error) {
	if adapter.err != nil {
		return task.CoreMutationScope{}, adapter.err
	}
	scope, ok := adapter.scopes[requirementID]
	if !ok || scope.WorkspaceID != workspaceID {
		return task.CoreMutationScope{}, task.ErrCoreMutationNotFound
	}
	return scope, nil
}
func (adapter *coreFactsTestAdapter) ValidateActiveTaskUsers(_ context.Context, _ uint, ids []uint) error {
	if adapter.err != nil {
		return adapter.err
	}
	for _, id := range ids {
		if id == 99 {
			return task.ErrCoreMutationConflict
		}
	}
	return nil
}

type coreEffectsTestAdapter struct {
	tx   *gorm.DB
	fail *bool
}

func (adapter coreEffectsTestAdapter) RecordCoreTaskMutation(_ context.Context, effect task.CoreMutationEffect) error {
	if adapter.fail != nil && *adapter.fail {
		return errors.New("effect unavailable")
	}
	return adapter.tx.Create(&coreEffectTestRecord{
		WorkspaceID: effect.WorkspaceID, TeamID: effect.TeamID, PlanUnitID: effect.PlanUnitID,
		PreviousStatus: effect.PreviousStatus, TaskID: effect.TaskID, TaskKey: effect.TaskKey, Action: effect.Action, NotifyAssignee: effect.NotifyAssignee,
	}).Error
}

type coreCleanerTestAdapter struct{ tx *gorm.DB }

func (adapter coreCleanerTestAdapter) ClearTaskReferences(_ context.Context, facts task.TaskDependentReferenceFacts) error {
	for _, id := range facts.TaskIDs {
		if err := adapter.tx.Create(&coreCleanerTestRecord{WorkspaceID: facts.WorkspaceID, TaskID: id}).Error; err != nil {
			return err
		}
	}
	for _, id := range facts.TestExecutionIDs {
		var count int64
		if err := adapter.tx.Model(&TaskTestExecution{}).Where("workspace_id = ? AND id = ?", facts.WorkspaceID, id).Count(&count).Error; err != nil {
			return err
		}
		if err := adapter.tx.Create(&coreCleanerTestRecord{WorkspaceID: facts.WorkspaceID, TestExecutionID: id, ExecutionExists: count == 1}).Error; err != nil {
			return err
		}
	}
	return nil
}

type coreRemovalTestAdapter struct {
	err  error
	keys []string
}

func (adapter *coreRemovalTestAdapter) RemoveObject(_ context.Context, key string) error {
	adapter.keys = append(adapter.keys, key)
	return adapter.err
}

func TestCoreMutationCreateUpdateCloneAndReplayAreAtomic(t *testing.T) {
	db, facts, effectFail := openCoreMutationDB(t)
	repository := newCoreTestRepository(t, db, facts, effectFail)
	service, _ := task.NewCoreMutationService(repository)
	assigneeID := uint(42)
	create := task.CoreMutation{Operation: task.CoreCreate, WorkspaceID: 7, PrincipalID: 3, ApplyKey: "apply-create", ChangeSetID: "changeset-create", InputDigest: strings.Repeat("a", 64), Values: task.CoreMutationValues{RequirementID: 11, Title: " First ", Status: "draft", Priority: "medium", AssigneeID: &assigneeID}}
	created, err := service.ApplyCoreMutation(context.Background(), create)
	if err != nil || created.Task == nil || created.Task.TaskKey != "PRJ-1" || created.Task.Priority != "normal" || created.Task.ReporterUserID == nil || *created.Task.ReporterUserID != 3 {
		t.Fatalf("created=%+v err=%v", created, err)
	}
	replay, err := service.ApplyCoreMutation(context.Background(), create)
	if err != nil || replay.Task == nil || replay.Task.ID != created.Task.ID {
		t.Fatalf("replay=%+v err=%v", replay, err)
	}
	nextAssigneeID := uint(43)
	update := task.CoreMutation{Operation: task.CoreUpdate, WorkspaceID: 7, PrincipalID: 3, ApplyKey: "apply-update", ChangeSetID: "changeset-update", InputDigest: strings.Repeat("b", 64), ID: created.Task.ID, ExpectedUpdatedAt: created.Task.UpdatedAt, Values: task.CoreMutationValues{Title: "Updated", Status: "completed", AssigneeID: &nextAssigneeID, Fields: map[string]bool{"title": true, "status": true, "assignee_id": true}}}
	updated, err := service.ApplyCoreMutation(context.Background(), update)
	if err != nil || updated.Task == nil || updated.Task.Title != "Updated" || updated.Task.CompletedAt == nil {
		t.Fatalf("updated=%+v err=%v", updated, err)
	}
	if _, err := service.ApplyCoreMutation(context.Background(), update); err != nil {
		t.Fatal(err)
	}
	clone := task.CoreMutation{Operation: task.CoreClone, WorkspaceID: 7, PrincipalID: 3, ApplyKey: "apply-clone", ChangeSetID: "changeset-clone", InputDigest: strings.Repeat("c", 64), ID: updated.Task.ID, ExpectedUpdatedAt: updated.Task.UpdatedAt, TitleSuffix: " - copy", TitleSuffixSet: true}
	cloned, err := service.ApplyCoreMutation(context.Background(), clone)
	if err != nil || cloned.Task == nil || cloned.Task.Title != "Updated- copy" || cloned.Task.Status != "draft" || cloned.ClonedFrom == nil || cloned.ClonedFrom.ID != updated.Task.ID {
		t.Fatalf("clone=%+v err=%v", cloned, err)
	}
	if _, err := service.ApplyCoreMutation(context.Background(), clone); err != nil {
		t.Fatal(err)
	}
	var tasks, effects, ledgers int64
	db.Model(&WorkTask{}).Count(&tasks)
	db.Model(&coreEffectTestRecord{}).Count(&effects)
	db.Model(&TaskToolMutationApply{}).Count(&ledgers)
	if tasks != 2 || effects != 3 || ledgers != 3 {
		t.Fatalf("tasks=%d effects=%d ledgers=%d", tasks, effects, ledgers)
	}
	var createdEffect coreEffectTestRecord
	if err := db.Where("task_id = ? AND action = ?", created.Task.ID, "created").First(&createdEffect).Error; err != nil ||
		!createdEffect.NotifyAssignee || createdEffect.TeamID != 8 || createdEffect.PlanUnitID != 10 || createdEffect.TaskKey != created.Task.TaskKey {
		t.Fatalf("created effect=%+v err=%v", createdEffect, err)
	}
	var updatedEffect coreEffectTestRecord
	if err := db.Where("task_id = ? AND action = ?", created.Task.ID, "updated").First(&updatedEffect).Error; err != nil || !updatedEffect.NotifyAssignee || updatedEffect.PreviousStatus != "draft" {
		t.Fatalf("updated effect=%+v err=%v", updatedEffect, err)
	}
}

func TestTaskDeletionSagaRetriesObjectsAndKeepsAtomicDatabaseEffects(t *testing.T) {
	db, facts, effectFail := openCoreMutationDB(t)
	repository := newCoreTestRepository(t, db, facts, effectFail)
	parent := WorkTask{WorkspaceID: 7, TeamID: 8, ProjectSetID: 9, ProjectID: 10, RequirementID: 11, Kind: task.WorkTaskKindTask, TaskKey: "PRJ-1", Title: "Parent", Status: "draft", Priority: "normal", MutationState: workTaskStateActive}
	if err := db.Create(&parent).Error; err != nil {
		t.Fatal(err)
	}
	parentID := parent.ID
	child := WorkTask{WorkspaceID: 7, TeamID: 8, ProjectSetID: 9, ProjectID: 10, RequirementID: 11, Kind: task.WorkTaskKindSubtask, TaskKey: "PRJ-2", Title: "Child", Status: "draft", Priority: "normal", ParentID: &parentID, MutationState: workTaskStateActive}
	if err := db.Create(&child).Error; err != nil {
		t.Fatal(err)
	}
	attachment := WorkTaskAttachment{WorkspaceID: 7, WorkTaskID: child.ID, UploaderUserID: 3, Filename: "file", Size: 1, ObjectKey: "workspace/7/file", StorageBackend: "s3", MutationState: attachmentStateActive}
	if err := db.Create(&attachment).Error; err != nil {
		t.Fatal(err)
	}
	execution := TaskTestExecution{WorkspaceID: 7, WorkTaskID: child.ID, TaskTestDefinitionID: 1, DefinitionVersion: 1, Environment: "uat", ExecutorUserID: 3, Status: "pending"}
	if err := db.Create(&execution).Error; err != nil {
		t.Fatal(err)
	}
	conflict := deletionCommand(parent.ID, "no-cascade")
	remover := &coreRemovalTestAdapter{}
	deletions, _ := task.NewTaskDeletionService(repository, remover)
	if _, err := deletions.DeleteTasks(context.Background(), conflict); !errors.Is(err, task.ErrTaskDeletionConflict) {
		t.Fatalf("cascade=false error=%v", err)
	}
	remover.err = errors.New("storage disabled")
	command := deletionCommand(parent.ID, "cascade")
	command.Cascade = true
	if _, err := deletions.DeleteTasks(context.Background(), command); err == nil {
		t.Fatal("storage failure expected")
	}
	var stored WorkTaskAttachment
	if err := db.First(&stored, attachment.ID).Error; err != nil {
		t.Fatalf("metadata lost: %v", err)
	}
	var deleting int64
	db.Model(&WorkTask{}).Where("mutation_state = ?", workTaskStateDeleting).Count(&deleting)
	if deleting != 2 {
		t.Fatalf("deleting=%d", deleting)
	}
	recovered := &coreRemovalTestAdapter{}
	deletions, _ = task.NewTaskDeletionService(repository, recovered)
	result, err := deletions.DeleteTasks(context.Background(), command)
	if err != nil || !result.Deleted || result.DeletedCount != 2 {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	if _, err := deletions.DeleteTasks(context.Background(), command); err != nil {
		t.Fatal(err)
	}
	if len(recovered.keys) != 1 || recovered.keys[0] != attachment.ObjectKey {
		t.Fatalf("remove calls=%v", recovered.keys)
	}
	var tasks, attachments, effects, cleaners int64
	db.Model(&WorkTask{}).Count(&tasks)
	db.Model(&WorkTaskAttachment{}).Count(&attachments)
	db.Model(&coreEffectTestRecord{}).Count(&effects)
	db.Model(&coreCleanerTestRecord{}).Count(&cleaners)
	if tasks != 0 || attachments != 0 || effects != 2 || cleaners != 3 {
		t.Fatalf("tasks=%d attachments=%d effects=%d cleaners=%d", tasks, attachments, effects, cleaners)
	}
	var executionFact coreCleanerTestRecord
	if err := db.Where("test_execution_id = ?", execution.ID).First(&executionFact).Error; err != nil || !executionFact.ExecutionExists {
		t.Fatalf("execution fact=%+v err=%v", executionFact, err)
	}
}

func TestCoreMutationRollbackAndWorkspaceCASFailClosed(t *testing.T) {
	db, facts, effectFail := openCoreMutationDB(t)
	repository := newCoreTestRepository(t, db, facts, effectFail)
	service, _ := task.NewCoreMutationService(repository)
	*effectFail = true
	command := task.CoreMutation{Operation: task.CoreCreate, WorkspaceID: 7, PrincipalID: 3, ApplyKey: "apply-fail", ChangeSetID: "changeset-fail", InputDigest: strings.Repeat("d", 64), Values: task.CoreMutationValues{RequirementID: 11, Title: "Rollback"}}
	if _, err := service.ApplyCoreMutation(context.Background(), command); err == nil {
		t.Fatal("effect failure expected")
	}
	var tasks, ledgers int64
	db.Model(&WorkTask{}).Count(&tasks)
	db.Model(&TaskToolMutationApply{}).Count(&ledgers)
	if tasks != 0 || ledgers != 0 {
		t.Fatalf("tasks=%d ledgers=%d", tasks, ledgers)
	}
	*effectFail = false
	foreign := command
	foreign.WorkspaceID = 8
	foreign.ApplyKey = "apply-foreign"
	foreign.ChangeSetID = "changeset-foreign"
	if _, err := service.ApplyCoreMutation(context.Background(), foreign); !errors.Is(err, task.ErrCoreMutationNotFound) {
		t.Fatalf("foreign error=%v", err)
	}
}

func TestTaskDeletionCompletionRollbackResumesWithoutRepeatingRemovedObject(t *testing.T) {
	db, facts, effectFail := openCoreMutationDB(t)
	repository := newCoreTestRepository(t, db, facts, effectFail)
	row := WorkTask{WorkspaceID: 7, TeamID: 8, ProjectSetID: 9, ProjectID: 10, RequirementID: 11, Kind: task.WorkTaskKindTask, TaskKey: "PRJ-1", Title: "Task", Status: "draft", Priority: "normal", MutationState: workTaskStateActive}
	if err := db.Create(&row).Error; err != nil {
		t.Fatal(err)
	}
	attachment := WorkTaskAttachment{WorkspaceID: 7, WorkTaskID: row.ID, UploaderUserID: 3, Filename: "file", Size: 1, ObjectKey: "workspace/7/retry", StorageBackend: "s3", MutationState: attachmentStateActive}
	if err := db.Create(&attachment).Error; err != nil {
		t.Fatal(err)
	}
	*effectFail = true
	firstRemover := &coreRemovalTestAdapter{}
	service, _ := task.NewTaskDeletionService(repository, firstRemover)
	command := deletionCommand(row.ID, "completion-rollback")
	if _, err := service.DeleteTasks(context.Background(), command); err == nil {
		t.Fatal("effect failure must rollback completion")
	}
	var stored WorkTaskAttachment
	if err := db.First(&stored, attachment.ID).Error; err != nil {
		t.Fatalf("metadata must survive completion rollback: %v", err)
	}
	var object TaskDeletionObject
	if err := db.Where("attachment_id = ?", attachment.ID).First(&object).Error; err != nil || object.Status != deletionObjectRemoved {
		t.Fatalf("object progress=%+v err=%v", object, err)
	}
	*effectFail = false
	secondRemover := &coreRemovalTestAdapter{}
	service, _ = task.NewTaskDeletionService(repository, secondRemover)
	result, err := service.DeleteTasks(context.Background(), command)
	if err != nil || !result.Deleted || result.DeletedCount != 1 {
		t.Fatalf("recovered=%+v err=%v", result, err)
	}
	if len(firstRemover.keys) != 1 || len(secondRemover.keys) != 0 {
		t.Fatalf("first calls=%v second calls=%v", firstRemover.keys, secondRemover.keys)
	}
}

func openCoreMutationDB(t *testing.T) (*gorm.DB, *coreFactsTestAdapter, *bool) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{NowFunc: func() time.Time { return time.Now().UTC() }})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, _ := db.DB()
	sqlDB.SetMaxOpenConns(1)
	records := []any{&WorkTask{}, &KeySequence{}, &WorkTaskActivity{}, &WorkTaskComment{}, &WorkTaskWatcher{}, &WorkTaskAttachment{}, &TaskTestDefinition{}, &TaskTestExecution{}, &TaskTestExecutionRun{}, &TaskToolMutationApply{}, &TaskDeletionSaga{}, &TaskDeletionObject{}, &coreEffectTestRecord{}, &coreCleanerTestRecord{}}
	if err := db.AutoMigrate(records...); err != nil {
		t.Fatal(err)
	}
	facts := &coreFactsTestAdapter{scopes: map[uint]task.CoreMutationScope{11: {WorkspaceID: 7, TeamID: 8, ProjectSetID: 9, ProjectCode: "PRJ", PlanUnitID: 10, RequirementID: 11, RequirementVersion: 1}, 21: {WorkspaceID: 7, TeamID: 18, ProjectSetID: 19, ProjectCode: "NEXT", PlanUnitID: 20, RequirementID: 21, RequirementVersion: 1}}}
	fail := false
	return db, facts, &fail
}

func newCoreTestRepository(t *testing.T, db *gorm.DB, facts *coreFactsTestAdapter, effectFail *bool) *CoreMutationRepository {
	t.Helper()
	repository, err := NewCoreMutationRepository(db, func(*gorm.DB) (task.CoreMutationFacts, error) { return facts, nil }, func(tx *gorm.DB) (task.CoreMutationEffectSink, error) {
		return coreEffectsTestAdapter{tx: tx, fail: effectFail}, nil
	}, func(tx *gorm.DB) (task.TaskDependentCleaner, error) { return coreCleanerTestAdapter{tx: tx}, nil })
	if err != nil {
		t.Fatal(err)
	}
	return repository
}

func deletionCommand(id uint, suffix string) task.TaskDeletionCommand {
	return task.TaskDeletionCommand{WorkspaceID: 7, PrincipalID: 3, ApplyKey: "apply-" + suffix, ChangeSetID: "changeset-" + suffix, InputDigest: strings.Repeat("e", 64), RootIDs: []uint{id}}
}
