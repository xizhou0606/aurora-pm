package quality

import (
	"context"
	"errors"
	"testing"
)

type defectDomainMutationStore struct{ command BugMutation }

func (store *defectDomainMutationStore) ApplyBugMutation(_ context.Context, command BugMutation) (BugMutationResult, error) {
	store.command = command
	return BugMutationResult{}, nil
}

func TestBugMutationDefaultsAndValidatesDefectDomain(t *testing.T) {
	store := &defectDomainMutationStore{}
	service, err := NewBugMutationService(store)
	if err != nil {
		t.Fatal(err)
	}
	command := BugMutation{
		Operation: BugCreate, WorkspaceID: 7, PrincipalID: 9,
		ApplyKey: "apply-key", ChangeSetID: "change-set",
		InputDigest: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		Values:      BugMutationValues{RequirementID: 11, Title: "视觉错位"},
	}
	if _, err := service.ApplyBugMutation(context.Background(), command); err != nil {
		t.Fatal(err)
	}
	if store.command.Values.DefectDomain != BugDefectDomainFunctional {
		t.Fatalf("default defect domain=%q", store.command.Values.DefectDomain)
	}

	command.ApplyKey = "invalid-domain"
	command.ChangeSetID = "invalid-domain"
	command.Values.DefectDomain = "visual"
	if _, err := service.ApplyBugMutation(context.Background(), command); !errors.Is(err, ErrBugMutationInvalid) {
		t.Fatalf("invalid domain error=%v", err)
	}
}

func TestNormalizeBugDefectDomainTreatsOnlyExplicitUIAsUI(t *testing.T) {
	for input, want := range map[string]string{
		"": BugDefectDomainFunctional, "functional": BugDefectDomainFunctional,
		"legacy": BugDefectDomainFunctional, " UI ": BugDefectDomainUI,
	} {
		if got := NormalizeBugDefectDomain(input); got != want {
			t.Fatalf("NormalizeBugDefectDomain(%q)=%q want=%q", input, got, want)
		}
	}
}
