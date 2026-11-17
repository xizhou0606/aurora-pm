package qualitygorm

import (
	"context"
	"encoding/json"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/quality"
	"strings"
	"testing"
)

func TestFlowEvidenceIsAtomicAndReplayDoesNotDuplicate(t *testing.T) {
	db, requirementID := openQualityMutationDB(t)
	repo := newQualityMutationRepository(t, db)
	created, err := repo.ApplyBugMutation(context.Background(), qualityCreateCommand(requirementID, "flow-create"))
	if err != nil {
		t.Fatal(err)
	}
	for _, status := range []string{"review", "reopened"} {
		cmd := qualityCreateCommand(requirementID, "flow-"+status)
		cmd.Operation, cmd.ID, cmd.StatusTransition = quality.BugUpdate, created.Bug.ID, true
		url := "https://example.test/" + status + ".png"
		cmd.Values = quality.BugMutationValues{Status: status, Fields: map[string]bool{"status": true}}
		if status == "review" {
			cmd.Values.ResolutionReason, cmd.Values.ResolutionNote = "fixed", "已修复"
			cmd.Values.ResolutionMaterials = []quality.Material{{URL: url}}
			cmd.Values.Fields["resolution_reason"], cmd.Values.Fields["resolution_note"], cmd.Values.Fields["resolution_materials"] = true, true, true
		} else {
			cmd.Values.RegressionNote = `{"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"不行"}]}]}`
			cmd.Values.RegressionMaterials = []quality.Material{{URL: url}}
			cmd.Values.Fields["regression_note"], cmd.Values.Fields["regression_materials"] = true, true
		}
		result, err := repo.ApplyBugMutation(context.Background(), cmd)
		if err != nil || result.Bug.Status != status {
			t.Fatalf("result=%+v err=%v", result, err)
		}
		if _, err = repo.ApplyBugMutation(context.Background(), cmd); err != nil {
			t.Fatal(err)
		}
		var rows []QualityBugActivity
		db.Where("quality_bug_id = ? AND field_key = ? AND new_value = ?", cmd.ID, "status", status).Find(&rows)
		if len(rows) != 1 || rows[0].Action != "status_transition" || rows[0].OldValue == status || !strings.Contains(rows[0].Summary, url) {
			t.Fatalf("activities=%+v", rows)
		}
		if status == "reopened" {
			raw := strings.SplitN(rows[0].Summary, "；备注：", 2)[1]
			if !json.Valid([]byte(raw)) || !strings.Contains(raw, "不行") || result.Bug.ReproductionSteps != "" || len(result.Bug.ReproductionMaterials) != 0 {
				t.Fatalf("reopen evidence=%s bug=%+v", raw, result.Bug)
			}
		}
	}
}

func TestFlowActivityFailureRollsBackStatusAndReceipt(t *testing.T) {
	db, requirementID := openQualityMutationDB(t)
	repo := newQualityMutationRepository(t, db)
	created, err := repo.ApplyBugMutation(context.Background(), qualityCreateCommand(requirementID, "rollback-create"))
	if err != nil {
		t.Fatal(err)
	}
	if err = db.Exec("CREATE TRIGGER reject_flow BEFORE INSERT ON quality_bug_activities BEGIN SELECT RAISE(ABORT, 'test rejection'); END").Error; err != nil {
		t.Fatal(err)
	}
	cmd := qualityCreateCommand(requirementID, "rollback-flow")
	cmd.Operation, cmd.ID, cmd.StatusTransition = quality.BugUpdate, created.Bug.ID, true
	cmd.Values = quality.BugMutationValues{Status: "reopened", RegressionNote: "不行", Fields: map[string]bool{"status": true, "regression_note": true}}
	if _, err = repo.ApplyBugMutation(context.Background(), cmd); err == nil {
		t.Fatal("expected activity failure")
	}
	var bug QualityBug
	db.First(&bug, cmd.ID)
	if bug.Status != "triage" {
		t.Fatalf("status committed: %s", bug.Status)
	}
	assertQualityMutationCounts(t, db, 1, 1, 1)
}
