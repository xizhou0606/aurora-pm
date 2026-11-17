package qualitygorm

import (
	"encoding/json"
	"strconv"
	"strings"

	"github.com/xizhou0606/aurora-pm/backend/internal/module/quality"
)

// Flow evidence belongs to the activity committed with the state change. In
// particular, reopening must not overwrite the original reproduction report.
func bugMutationActivity(before quality.BugSnapshot, after QualityBug, command quality.BugMutation) *QualityBugActivity {
	row := &QualityBugActivity{WorkspaceID: command.WorkspaceID, QualityBugID: after.ID,
		UserID: command.PrincipalID, Action: "updated", Summary: "更新质量 Bug"}
	if before.Status == after.Status {
		if actorID(before.AssigneeUserID) != actorID(after.AssigneeUserID) || command.Values.Fields["assignment_note"] || command.Values.Fields["assignment_materials"] {
			row.Action, row.FieldKey = "assigned", "assignee_user_id"
			row.OldValue, row.NewValue = actorID(before.AssigneeUserID), actorID(after.AssigneeUserID)
			row.Summary = activityEvidence("指派", command.Values.AssignmentNote, command.Values.AssignmentMaterials)
		}
		return row
	}
	row.Action, row.FieldKey, row.OldValue, row.NewValue = "status_transition", "status", before.Status, after.Status
	values := command.Values
	note := strings.TrimSpace(values.ActivityNote)
	materials := []quality.Material{}
	switch after.Status {
	case "review", "done":
		if strings.TrimSpace(values.ResolutionNote) != "" {
			note = strings.TrimSpace(values.ResolutionNote)
		}
		materials = values.ResolutionMaterials
	case "reopened", "in_progress":
		if strings.TrimSpace(values.RegressionNote) != "" {
			note = strings.TrimSpace(values.RegressionNote)
		}
		materials = values.RegressionMaterials
	}
	row.Summary = activityEvidence("变更状态", note, materials)
	return row
}

func actorID(id *uint) string {
	if id == nil {
		return ""
	}
	return strconv.FormatUint(uint64(*id), 10)
}

// Keep evidence inside the note consumed by the shared activity presenter.
// A rich-text note must remain valid JSON after attachment URLs are appended.
func activityEvidence(title, note string, materials []quality.Material) string {
	note = strings.TrimSpace(note)
	if len(materials) > 0 {
		urls := make([]string, 0, len(materials))
		for _, m := range materials {
			urls = append(urls, m.URL)
		}
		var doc map[string]any
		if json.Unmarshal([]byte(note), &doc) == nil && doc["type"] == "doc" {
			content, _ := doc["content"].([]any)
			content = append(content, map[string]any{"type": "paragraph", "content": []any{map[string]any{"type": "text", "text": "材料：" + strings.Join(urls, "、")}}})
			doc["content"] = content
			raw, _ := json.Marshal(doc)
			note = string(raw)
		} else {
			if note != "" {
				note += "\n"
			}
			note += "材料：" + strings.Join(urls, "、")
		}
	}
	if note != "" {
		return title + "；备注：" + note
	}
	return title
}
