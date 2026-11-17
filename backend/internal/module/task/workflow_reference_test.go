package task

import (
	"context"
	"errors"
	"testing"
)

type workflowScopeFake struct {
	workspace uint
	scope     WorkflowScope
	err       error
}

func (fake *workflowScopeFake) ValidateWorkflowScope(_ context.Context, workspaceID uint, scope WorkflowScope) error {
	fake.workspace, fake.scope = workspaceID, scope
	return fake.err
}

func TestWorkflowReferenceServiceReturnsCanonicalPMFacts(t *testing.T) {
	fake := &workflowScopeFake{}
	service, err := NewWorkflowReferenceService(fake)
	if err != nil {
		t.Fatal(err)
	}
	kinds, err := service.ListKinds(context.Background(), 7, WorkflowScope{})
	if err != nil || len(kinds) != 2 || kinds[0].Key != WorkTaskKindTask || kinds[1].Key != WorkTaskKindSubtask {
		t.Fatalf("kinds=%+v err=%v", kinds, err)
	}
	statuses, err := service.ListStatuses(context.Background(), 7, WorkTaskKindTask, WorkflowScope{TeamID: 10})
	if err != nil || len(statuses) != 5 || statuses[0].StatusKey != "draft" || fake.workspace != 7 || fake.scope.TeamID != 10 {
		t.Fatalf("statuses=%+v fake=%+v err=%v", statuses, fake, err)
	}
	profile, err := service.ListTransitions(context.Background(), 7, WorkTaskKindSubtask, WorkflowScope{})
	if err != nil || profile.Enforce || !profile.Standard || len(profile.Transitions) != 10 {
		t.Fatalf("profile=%+v err=%v", profile, err)
	}
}

func TestWorkflowReferenceServiceRejectsInvalidKindAndScope(t *testing.T) {
	service, _ := NewWorkflowReferenceService(&workflowScopeFake{})
	if _, err := service.ListStatuses(context.Background(), 7, "story", WorkflowScope{}); !errors.Is(err, ErrWorkflowReferenceInvalid) {
		t.Fatalf("kind err=%v", err)
	}
	if _, err := service.ListKinds(context.Background(), 7, WorkflowScope{ProjectID: 12}); !errors.Is(err, ErrWorkflowReferenceInvalid) {
		t.Fatalf("scope err=%v", err)
	}
}
