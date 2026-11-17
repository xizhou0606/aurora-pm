package pmtools

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/xizhou0606/aurora-pm/backend/internal/module/quality"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/toolruntime"
)

func TestQualityToolsMapChineseReasonsBeforeDomainWrites(t *testing.T) {
	for _, name := range []string{QualityBugCreate, QualityBugUpdate, QualityBugStatusUpdate, QualityBugBatchCreate} {
		for reason, want := range map[string]string{
			"已修复": "fixed", "已解决": "fixed", "符合设计": "as_designed",
			"无法复现": "cannot_reproduce", "不予修复": "wont_fix", "fixed": "fixed",
			"随便处理": "",
		} {
			t.Run(name+"/"+reason, func(t *testing.T) {
				capture := &qualityMutationCapture{result: quality.BugMutationResult{Bug: &quality.BugSnapshot{ID: 9}}}
				service, err := quality.NewBugMutationService(capture)
				if err != nil {
					t.Fatal(err)
				}
				handlers, err := NewQualityWriteHandlers(service)
				if err != nil {
					t.Fatal(err)
				}
				input := map[string]any{
					"id": 9, "requirement_id": 5, "title": "Bug", "status": "review", "resolution_reason": reason,
				}
				switch name {
				case QualityBugCreate:
					delete(input, "id")
				case QualityBugStatusUpdate:
					delete(input, "requirement_id")
					delete(input, "title")
				case QualityBugBatchCreate:
					delete(input, "id")
					input["client_id"] = "first"
					input = map[string]any{"bugs": []any{input}}
				}
				raw, err := json.Marshal(input)
				if err != nil {
					t.Fatal(err)
				}
				_, err = handlers[name].(toolruntime.IdempotentMutationHandler).Apply(context.Background(), toolruntime.MutationCall{
					Authorization: toolruntime.ExecutionAuthorization{PrincipalID: 3, WorkspaceID: 7},
					Input:         raw, ChangeSetID: "reason-mapping", ApplyKey: "reason-mapping",
				})
				if want == "" {
					if err == nil || capture.command.Operation != "" {
						t.Fatalf("unknown reason reached storage: command=%+v err=%v", capture.command, err)
					}
					return
				}
				if err != nil {
					t.Fatal(err)
				}
				values := capture.command.Values
				if name == QualityBugBatchCreate {
					if len(capture.command.Items) != 1 {
						t.Fatalf("batch items=%+v", capture.command.Items)
					}
					values = capture.command.Items[0].Values
				}
				if values.ResolutionReason != want || !values.Fields["resolution_reason"] {
					t.Fatalf("storage received reason=%q fields=%v want=%q", values.ResolutionReason, values.Fields, want)
				}
			})
		}
	}
}
