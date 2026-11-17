package qualitygorm

import (
	"context"
	"errors"
	"testing"

	"github.com/xizhou0606/aurora-pm/backend/internal/module/quality"
)

func TestBugReadServiceNormalizesLegacyResolutionWithoutRewriting(t *testing.T) {
	db, requirementID := openQualityMutationDB(t)
	repository := newQualityMutationRepository(t, db)
	created, err := repository.ApplyBugMutation(context.Background(), qualityCreateCommand(requirementID, "legacy-read"))
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&QualityBug{}).Where("id = ?", created.Bug.ID).
		Updates(map[string]any{"status": "review", "resolution_reason": "已修复"}).Error; err != nil {
		t.Fatal(err)
	}
	service, err := quality.NewBugReadService(NewBugReadRepository(db))
	if err != nil {
		t.Fatal(err)
	}
	bug, err := service.GetBug(context.Background(), 7, created.Bug.ID)
	if err != nil || bug.ResolutionReason != "fixed" {
		t.Fatalf("get reason=%q err=%v", bug.ResolutionReason, err)
	}
	page, err := service.ListBugs(context.Background(), 7, quality.BugReadFilter{})
	if err != nil || len(page.Items) != 1 || page.Items[0].ResolutionReason != "fixed" {
		t.Fatalf("list=%+v err=%v", page, err)
	}
	var stored QualityBug
	if err := db.First(&stored, bug.ID).Error; err != nil || stored.ResolutionReason != "已修复" {
		t.Fatalf("read rewrote reason=%q err=%v", stored.ResolutionReason, err)
	}
	assertQualityMutationCounts(t, db, 1, 1, 1)
	if _, err := service.GetBug(context.Background(), 8, bug.ID); !errors.Is(err, quality.ErrBugReadNotFound) {
		t.Fatalf("cross-workspace read error=%v", err)
	}
	mutationService, err := quality.NewBugMutationService(repository)
	if err != nil {
		t.Fatal(err)
	}
	command := qualityCreateCommand(requirementID, "legacy-complete")
	command.Operation, command.ID, command.StatusTransition = quality.BugUpdate, bug.ID, true
	command.Values = quality.BugMutationValues{Status: "done", Fields: map[string]bool{"status": true}}
	result, err := mutationService.ApplyBugMutation(context.Background(), command)
	if err != nil || result.Bug == nil || result.Bug.Status != "done" || result.Bug.ResolutionReason != "fixed" {
		t.Fatalf("legacy completion=%+v err=%v", result, err)
	}
	if err := db.First(&stored, bug.ID).Error; err != nil || stored.ResolutionReason != "已修复" {
		t.Fatalf("completion rewrote reason=%q err=%v", stored.ResolutionReason, err)
	}
}

func TestBugCannotEnterReviewWithUnknownInheritedResolution(t *testing.T) {
	db, requirementID := openQualityMutationDB(t)
	repository := newQualityMutationRepository(t, db)
	created, err := repository.ApplyBugMutation(context.Background(), qualityCreateCommand(requirementID, "unknown-create"))
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&QualityBug{}).Where("id = ?", created.Bug.ID).Update("resolution_reason", "unknown").Error; err != nil {
		t.Fatal(err)
	}
	command := qualityCreateCommand(requirementID, "unknown-review")
	command.Operation, command.ID, command.StatusTransition = quality.BugUpdate, created.Bug.ID, true
	command.Values = quality.BugMutationValues{Status: "review", Fields: map[string]bool{"status": true}}
	if _, err := repository.ApplyBugMutation(context.Background(), command); !errors.Is(err, quality.ErrBugMutationInvalid) {
		t.Fatalf("inherited invalid reason error=%v", err)
	}
	assertQualityMutationCounts(t, db, 1, 1, 1)
}
