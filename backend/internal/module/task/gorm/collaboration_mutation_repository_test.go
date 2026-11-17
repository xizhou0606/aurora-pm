package taskgorm

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/task"
	"gorm.io/gorm"
)

type collaborationRemover struct {
	err  error
	keys []string
}

type collaborationNotificationNoop struct{}

func (collaborationNotificationNoop) RecordTaskComment(context.Context, task.CommentNotification) error {
	return nil
}

func newCollaborationMutationRepository(t *testing.T, db *gorm.DB) *CollaborationMutationRepository {
	t.Helper()
	repository, err := NewCollaborationMutationRepository(db, func(*gorm.DB) (task.CollaborationNotificationWriter, error) {
		return collaborationNotificationNoop{}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return repository
}

func (remover *collaborationRemover) RemoveObject(_ context.Context, key string) error {
	remover.keys = append(remover.keys, key)
	return remover.err
}

func TestCollaborationMutationRepositoryAppliesCommentAndWatchWithDurableReplay(t *testing.T) {
	db, workTask := openCollaborationMutationDB(t)
	repository := newCollaborationMutationRepository(t, db)
	service, _ := task.NewCollaborationMutationService(repository, &collaborationRemover{})

	create := collaborationCommand(task.CommentCreate, workTask, "comment-create")
	create.Body = " hello "
	created, err := service.ApplyCollaborationMutation(context.Background(), create)
	if err != nil || created.Comment == nil || created.Comment.Body != "hello" || created.Comment.UserID != create.PrincipalID {
		t.Fatalf("created=%+v err=%v", created, err)
	}
	replayed, err := service.ApplyCollaborationMutation(context.Background(), create)
	if err != nil || replayed.Comment == nil || replayed.Comment.ID != created.Comment.ID {
		t.Fatalf("replayed=%+v err=%v", replayed, err)
	}
	watch := collaborationCommand(task.WatchUpdate, workTask, "watch")
	watch.Watch = true
	if result, err := service.ApplyCollaborationMutation(context.Background(), watch); err != nil || result.Watched == nil || !*result.Watched {
		t.Fatalf("watch=%+v err=%v", result, err)
	}
	if _, err := service.ApplyCollaborationMutation(context.Background(), watch); err != nil {
		t.Fatal(err)
	}
	var comments, watchers, activities, ledgers int64
	_ = db.Model(&WorkTaskComment{}).Count(&comments).Error
	_ = db.Model(&WorkTaskWatcher{}).Count(&watchers).Error
	_ = db.Model(&WorkTaskActivity{}).Count(&activities).Error
	_ = db.Model(&TaskToolMutationApply{}).Count(&ledgers).Error
	if comments != 1 || watchers != 1 || activities != 2 || ledgers != 2 {
		t.Fatalf("comments=%d watchers=%d activities=%d ledgers=%d", comments, watchers, activities, ledgers)
	}

	foreignAuthor := WorkTaskComment{WorkspaceID: workTask.WorkspaceID, WorkTaskID: workTask.ID, UserID: 99, Body: "foreign"}
	if err := db.Create(&foreignAuthor).Error; err != nil {
		t.Fatal(err)
	}
	forbidden := collaborationCommand(task.CommentDelete, workTask, "forbidden")
	forbidden.CommentID = foreignAuthor.ID
	if _, err := service.ApplyCollaborationMutation(context.Background(), forbidden); !errors.Is(err, task.ErrCollaborationMutationForbidden) {
		t.Fatalf("forbidden error=%v", err)
	}
	var stillThere int64
	_ = db.Model(&WorkTaskComment{}).Where("id = ?", foreignAuthor.ID).Count(&stillThere).Error
	if stillThere != 1 {
		t.Fatal("foreign author's comment was deleted")
	}
	remove := collaborationCommand(task.CommentDelete, workTask, "comment-delete")
	remove.CommentID = created.Comment.ID
	if result, err := service.ApplyCollaborationMutation(context.Background(), remove); err != nil || !result.Deleted || result.DeletedID != created.Comment.ID {
		t.Fatalf("delete=%+v err=%v", result, err)
	}
	if _, err := service.ApplyCollaborationMutation(context.Background(), remove); err != nil {
		t.Fatal(err)
	}
	stillThere = 0
	_ = db.Model(&WorkTaskComment{}).Where("id = ?", created.Comment.ID).Count(&stillThere).Error
	if stillThere != 0 {
		t.Fatal("owned comment remains")
	}
}

func TestAttachmentDeletionSagaRetriesStorageWithoutLosingMetadata(t *testing.T) {
	db, workTask := openCollaborationMutationDB(t)
	attachment := WorkTaskAttachment{
		WorkspaceID: workTask.WorkspaceID, WorkTaskID: workTask.ID, UploaderUserID: 12,
		Filename: "report.txt", Size: 4, ObjectKey: "workspace/7/task/1/report.txt", StorageBackend: "s3", MutationState: attachmentStateActive,
	}
	if err := db.Create(&attachment).Error; err != nil {
		t.Fatal(err)
	}
	repository := newCollaborationMutationRepository(t, db)
	disabled := &collaborationRemover{err: errors.New("storage disabled")}
	service, _ := task.NewCollaborationMutationService(repository, disabled)
	command := collaborationCommand(task.AttachmentDelete, workTask, "attachment")
	command.AttachmentID = attachment.ID
	if _, err := service.ApplyCollaborationMutation(context.Background(), command); err == nil {
		t.Fatal("disabled storage must fail")
	}
	var stored WorkTaskAttachment
	if err := db.First(&stored, attachment.ID).Error; err != nil || stored.MutationState != attachmentStateDeleting {
		t.Fatalf("metadata=%+v err=%v", stored, err)
	}
	var saga AttachmentDeletionSaga
	if err := db.Where("attachment_id = ?", attachment.ID).First(&saga).Error; err != nil || saga.Status != taskMutationPending {
		t.Fatalf("saga=%+v err=%v", saga, err)
	}
	var ledger TaskToolMutationApply
	if err := db.Where("apply_key = ?", command.ApplyKey).First(&ledger).Error; err != nil || ledger.Status != taskMutationPending {
		t.Fatalf("ledger=%+v err=%v", ledger, err)
	}

	recovered := &collaborationRemover{}
	recoveredService, _ := task.NewCollaborationMutationService(repository, recovered)
	result, err := recoveredService.ApplyCollaborationMutation(context.Background(), command)
	if err != nil || !result.Deleted || result.DeletedID != attachment.ID {
		t.Fatalf("recovered=%+v err=%v", result, err)
	}
	if _, err := recoveredService.ApplyCollaborationMutation(context.Background(), command); err != nil {
		t.Fatal(err)
	}
	if len(recovered.keys) != 1 || recovered.keys[0] != attachment.ObjectKey {
		t.Fatalf("remove calls=%v", recovered.keys)
	}
	var attachments int64
	_ = db.Model(&WorkTaskAttachment{}).Where("id = ?", attachment.ID).Count(&attachments).Error
	if attachments != 0 {
		t.Fatal("attachment metadata remains after completed saga")
	}
	if err := db.First(&saga, saga.ID).Error; err != nil || saga.Status != taskMutationSucceeded || saga.CompletedAt == nil {
		t.Fatalf("completed saga=%+v err=%v", saga, err)
	}
}

func TestCollaborationMutationFailsClosedAndRollsBackAtomicWrites(t *testing.T) {
	db, workTask := openCollaborationMutationDB(t)
	repository := newCollaborationMutationRepository(t, db)
	service, _ := task.NewCollaborationMutationService(repository, &collaborationRemover{})
	foreign := collaborationCommand(task.CommentCreate, workTask, "foreign-workspace")
	foreign.WorkspaceID = 8
	foreign.Body = "no"
	if _, err := service.ApplyCollaborationMutation(context.Background(), foreign); !errors.Is(err, task.ErrCollaborationMutationNotFound) {
		t.Fatalf("foreign error=%v", err)
	}
	moved := collaborationCommand(task.CommentCreate, workTask, "moved")
	moved.TaskScope.RequirementID = 99
	moved.Body = "no"
	if _, err := service.ApplyCollaborationMutation(context.Background(), moved); !errors.Is(err, task.ErrCollaborationMutationConflict) {
		t.Fatalf("moved error=%v", err)
	}
	callback := "fail_collaboration_activity"
	if err := db.Callback().Create().Before("gorm:create").Register(callback, func(tx *gorm.DB) {
		if tx.Statement != nil && tx.Statement.Table == "work_task_activities" {
			tx.AddError(errors.New("activity unavailable"))
		}
	}); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Callback().Create().Remove(callback) })
	rollback := collaborationCommand(task.CommentCreate, workTask, "rollback")
	rollback.Body = "rollback"
	if _, err := service.ApplyCollaborationMutation(context.Background(), rollback); err == nil {
		t.Fatal("injected failure must rollback")
	}
	var comments, ledgers int64
	_ = db.Model(&WorkTaskComment{}).Where("body = ?", "rollback").Count(&comments).Error
	_ = db.Model(&TaskToolMutationApply{}).Where("apply_key = ?", rollback.ApplyKey).Count(&ledgers).Error
	if comments != 0 || ledgers != 0 {
		t.Fatalf("comments=%d ledgers=%d", comments, ledgers)
	}
}

func openCollaborationMutationDB(t *testing.T) (*gorm.DB, WorkTask) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, _ := db.DB()
	sqlDB.SetMaxOpenConns(1)
	if err := db.AutoMigrate(&WorkTask{}, &WorkTaskComment{}, &WorkTaskActivity{}, &WorkTaskWatcher{}, &WorkTaskAttachment{}, &TaskToolMutationApply{}, &AttachmentDeletionSaga{}); err != nil {
		t.Fatal(err)
	}
	row := WorkTask{WorkspaceID: 7, TeamID: 8, ProjectSetID: 9, ProjectID: 10, RequirementID: 11, Kind: "task", TaskKey: "TASK-1", Title: "Task", Status: "todo", Priority: "normal"}
	if err := db.Create(&row).Error; err != nil {
		t.Fatal(err)
	}
	return db, row
}

func collaborationCommand(operation task.CollaborationMutationOperation, workTask WorkTask, suffix string) task.CollaborationMutation {
	return task.CollaborationMutation{
		Operation: operation, WorkspaceID: workTask.WorkspaceID, PrincipalID: 12,
		ApplyKey: "apply-" + suffix, ChangeSetID: "changeset-" + suffix, InputDigest: strings.Repeat("b", 64),
		TaskID: workTask.ID, TaskScope: task.MutationTaskScope{
			TeamID: workTask.TeamID, ProjectSetID: workTask.ProjectSetID, PlanUnitID: workTask.ProjectID, RequirementID: workTask.RequirementID,
		},
	}
}
