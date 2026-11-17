package taskgorm

import (
	"context"
	"errors"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/task"
	"gorm.io/gorm"
)

func TestCascadeDeletionGuardFailsClosedForAttachmentsAndDeletingTasks(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&WorkTask{}, &WorkTaskAttachment{}); err != nil {
		t.Fatal(err)
	}
	rows := []WorkTask{
		{WorkspaceID: 7, ProjectSetID: 11, RequirementID: 21, TaskKey: "A-1", MutationState: workTaskStateActive},
		{WorkspaceID: 7, ProjectSetID: 12, RequirementID: 22, TaskKey: "A-2", MutationState: workTaskStateDeleting},
		{WorkspaceID: 8, ProjectSetID: 11, RequirementID: 21, TaskKey: "B-1", MutationState: workTaskStateActive},
	}
	if err := db.Create(&rows).Error; err != nil {
		t.Fatal(err)
	}
	attachments := []WorkTaskAttachment{
		{WorkspaceID: 7, WorkTaskID: rows[0].ID, UploaderUserID: 1, Filename: "active", StorageBackend: "s3", ObjectKey: "active", MutationState: attachmentStateActive},
		{WorkspaceID: 7, WorkTaskID: rows[0].ID, UploaderUserID: 1, Filename: "deleting", StorageBackend: "s3", ObjectKey: "deleting", MutationState: attachmentStateDeleting},
	}
	if err := db.Create(&attachments).Error; err != nil {
		t.Fatal(err)
	}
	service, err := task.NewCascadeDeletionGuardService(NewCascadeDeletionGuardRepository(db))
	if err != nil {
		t.Fatal(err)
	}
	for _, scope := range []task.CascadeDeletionScope{
		{WorkspaceID: 7, RequirementID: 21},
		{WorkspaceID: 7, ProjectSetID: 11},
		{WorkspaceID: 7, RequirementID: 22},
		{WorkspaceID: 7, TaskIDs: []uint{rows[0].ID}},
	} {
		if err := service.ValidateCascadeDeletion(context.Background(), scope); !errors.Is(err, task.ErrCascadeDeletionConflict) {
			t.Fatalf("scope=%+v err=%v", scope, err)
		}
	}
	if err := db.Delete(&attachments[0]).Error; err != nil {
		t.Fatal(err)
	}
	if err := service.ValidateCascadeDeletion(context.Background(), task.CascadeDeletionScope{WorkspaceID: 7, RequirementID: 21}); !errors.Is(err, task.ErrCascadeDeletionConflict) {
		t.Fatalf("deleting attachment must remain a conflict: %v", err)
	}
	if err := service.ValidateCascadeDeletion(context.Background(), task.CascadeDeletionScope{WorkspaceID: 8, ProjectSetID: 11}); err != nil {
		t.Fatalf("foreign workspace attachment leaked into guard: %v", err)
	}
}
