package quality

import (
	"context"
	"errors"
	"testing"
)

func TestBugResolutionAliasesNormalizeWhileDomainWritesStayCanonical(t *testing.T) {
	for input, want := range map[string]string{
		"已修复": "fixed", "已解决": "fixed", " FIXED ": "fixed",
		"符合设计": "as_designed", "设计如此": "as_designed",
		"重复问题": "duplicate", "重复Bug": "duplicate", "外部原因": "external",
		"无法复现": "cannot_reproduce", "无法重现": "cannot_reproduce",
		"延期处理": "postponed", "不予修复": "wont_fix", "不予解决": "wont_fix",
		"": "", "unknown": "unknown",
	} {
		if got := NormalizeBugResolutionReason(input); got != want {
			t.Errorf("stored reason %q=%q want=%q", input, got, want)
		}
	}
	for _, reason := range BugResolutionReasons() {
		if !ValidBugResolutionReason(reason) {
			t.Errorf("canonical reason %q rejected", reason)
		}
	}
	for _, reason := range []string{"已修复", "已解决", "符合设计", "unknown", "not_a_bug"} {
		if ValidBugResolutionReason(reason) {
			t.Errorf("noncanonical write %q accepted", reason)
		}
	}
}

func TestBugMutationsRejectNoncanonicalResolutionReasonsBeforeStorage(t *testing.T) {
	for _, operation := range []BugMutationOperation{BugCreate, BugUpdate, BugBatchCreate} {
		for _, reason := range []string{"已修复", "已解决", "invalid_reason"} {
			t.Run(string(operation)+"/"+reason, func(t *testing.T) {
				store := &defectDomainMutationStore{}
				service, err := NewBugMutationService(store)
				if err != nil {
					t.Fatal(err)
				}
				values := BugMutationValues{
					RequirementID: 11, Title: "resolution validation", Status: "review",
					ResolutionReason: reason, Fields: map[string]bool{"status": true, "resolution_reason": true},
				}
				command := BugMutation{
					Operation: operation, ID: 17, WorkspaceID: 7, PrincipalID: 9,
					ApplyKey: "resolution-validation", ChangeSetID: "resolution-validation",
					InputDigest: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
					Values:      values, Items: []BatchBugMutationItem{{ClientID: "first", Values: values}},
				}
				if _, err := service.ApplyBugMutation(context.Background(), command); !errors.Is(err, ErrBugMutationInvalid) {
					t.Fatalf("invalid reason error=%v", err)
				}
				if store.command.Operation != "" {
					t.Fatal("invalid reason reached storage")
				}
			})
		}
	}
}

func TestBugMutationNormalizesCanonicalResolutionReason(t *testing.T) {
	store := &defectDomainMutationStore{}
	service, err := NewBugMutationService(store)
	if err != nil {
		t.Fatal(err)
	}
	_, err = service.ApplyBugMutation(context.Background(), BugMutation{
		Operation: BugCreate, WorkspaceID: 7, PrincipalID: 9,
		ApplyKey: "canonical-reason", ChangeSetID: "canonical-reason",
		InputDigest: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		Values:      BugMutationValues{RequirementID: 11, Title: "canonical reason", Status: "review", ResolutionReason: " FIXED "},
	})
	if err != nil || store.command.Values.ResolutionReason != "fixed" {
		t.Fatalf("canonical reason=%q err=%v", store.command.Values.ResolutionReason, err)
	}
}
