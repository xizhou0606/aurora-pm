package pmtools

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/xizhou0606/aurora-pm/backend/internal/module/task"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/toolruntime"
)

type creatorPolicyStub struct {
	workspaceID uint
	userID      uint
	ids         []uint
	allowed     bool
}

func (stub *creatorPolicyStub) AllCreatedBy(_ context.Context, workspaceID, userID uint, ids []uint) (bool, error) {
	stub.workspaceID, stub.userID, stub.ids = workspaceID, userID, append([]uint(nil), ids...)
	return stub.allowed, nil
}

type factUserResolverStub struct{ id uint }

func (stub factUserResolverStub) FactUserID(context.Context, uint) (uint, error) { return stub.id, nil }

type objectTaskQueriesStub struct{}

func (objectTaskQueriesStub) GetTask(_ context.Context, _ uint, lookup task.Lookup) (task.Snapshot, error) {
	if lookup.Reference == "PM-12" {
		return task.Snapshot{ID: 12}, nil
	}
	return task.Snapshot{}, task.ErrTaskNotFound
}

func TestTaskObjectActionAuthorizerUsesFactIdentityAndRequiresAllBatchTargets(t *testing.T) {
	creators := &creatorPolicyStub{allowed: true}
	authorizer, err := NewTaskObjectActionAuthorizer(objectTaskQueriesStub{}, creators, factUserResolverStub{id: 47})
	if err != nil {
		t.Fatal(err)
	}
	allowed, err := authorizer.AuthorizeObjectAction(context.Background(), toolruntime.ObjectActionRequest{
		PolicyCode: taskCreatorObjectPolicy, ToolName: TaskBatchUpdate,
		Authorization: toolruntime.ExecutionAuthorization{PrincipalID: 3, WorkspaceID: 7},
		Input:         json.RawMessage(`{"ids":[11],"task_refs":["PM-12"],"updates":{"status":"in_dev"}}`),
	})
	if err != nil || !allowed {
		t.Fatalf("allowed=%t err=%v", allowed, err)
	}
	if creators.workspaceID != 7 || creators.userID != 47 || len(creators.ids) != 2 || creators.ids[0] != 11 || creators.ids[1] != 12 {
		t.Fatalf("creator request=%+v", creators)
	}

	allowed, err = authorizer.AuthorizeObjectAction(context.Background(), toolruntime.ObjectActionRequest{
		PolicyCode: "unknown", ToolName: TaskUpdate,
		Authorization: toolruntime.ExecutionAuthorization{PrincipalID: 3, WorkspaceID: 7},
		Input:         json.RawMessage(`{"id":11,"title":"x"}`),
	})
	if err != nil || allowed {
		t.Fatalf("unknown policy allowed=%t err=%v", allowed, err)
	}
}
