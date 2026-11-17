package task

import (
	"context"
	"reflect"
	"testing"
)

func TestBatchDeleteUsesCanonicalDeletionExecutorWithoutCascade(t *testing.T) {
	store := &batchMutationStoreFake{ids: []uint{82, 81}}
	deletions := &batchDeletionExecutorFake{result: TaskDeletionResult{Deleted: true, DeletedIDs: []uint{82, 81}, DeletedCount: 2}}
	service, err := NewBatchMutationService(store, deletions)
	if err != nil {
		t.Fatal(err)
	}
	command := BatchMutation{
		Operation: BatchDelete, WorkspaceID: 7, PrincipalID: 41, ApplyKey: "apply-batch-delete-0001",
		ChangeSetID: "changeset-batch-delete-0001", InputDigest: batchMutationTestDigest(),
		Selector: BatchTaskSelector{IDs: []uint{81, 82}},
	}
	result, err := service.ApplyTaskBatchMutation(context.Background(), command)
	if err != nil || result.DeletedCount != 2 || !result.Atomic || !reflect.DeepEqual(result.DeletedIDs, []uint{82, 81}) {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	if len(deletions.commands) != 1 {
		t.Fatalf("deletion commands=%+v", deletions.commands)
	}
	deleted := deletions.commands[0]
	if deleted.WorkspaceID != command.WorkspaceID || deleted.PrincipalID != command.PrincipalID || deleted.ApplyKey != command.ApplyKey ||
		deleted.ChangeSetID != command.ChangeSetID || deleted.InputDigest != command.InputDigest || deleted.Cascade || !reflect.DeepEqual(deleted.RootIDs, store.ids) {
		t.Fatalf("deletion command=%+v", deleted)
	}
}

func batchMutationTestDigest() string {
	return "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
}

func TestBatchDeleteForwardsExplicitCascade(t *testing.T) {
	store := &batchMutationStoreFake{ids: []uint{81}}
	deletions := &batchDeletionExecutorFake{result: TaskDeletionResult{Deleted: true, DeletedIDs: []uint{81, 82}, DeletedCount: 2}}
	service, err := NewBatchMutationService(store, deletions)
	if err != nil {
		t.Fatal(err)
	}
	result, err := service.ApplyTaskBatchMutation(context.Background(), BatchMutation{
		Operation: BatchDelete, WorkspaceID: 7, PrincipalID: 41, ApplyKey: "cascade-apply", ChangeSetID: "cascade-changeset",
		InputDigest: batchMutationTestDigest(), Selector: BatchTaskSelector{IDs: []uint{81}}, Cascade: true,
	})
	if err != nil || result.DeletedCount != 2 || len(deletions.commands) != 1 || !deletions.commands[0].Cascade {
		t.Fatalf("result=%+v commands=%+v err=%v", result, deletions.commands, err)
	}
}

type batchMutationStoreFake struct {
	ids []uint
}

func (store *batchMutationStoreFake) ApplyTaskBatchMutation(context.Context, BatchMutation) (BatchMutationResult, error) {
	return BatchMutationResult{}, nil
}

func (store *batchMutationStoreFake) ResolveTaskBatchDeletion(context.Context, BatchMutation) ([]uint, error) {
	return append([]uint(nil), store.ids...), nil
}

type batchDeletionExecutorFake struct {
	commands []TaskDeletionCommand
	result   TaskDeletionResult
}

func (executor *batchDeletionExecutorFake) DeleteTasks(_ context.Context, command TaskDeletionCommand) (TaskDeletionResult, error) {
	executor.commands = append(executor.commands, command)
	return executor.result, nil
}
