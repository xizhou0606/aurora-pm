package httpapi

import (
	"fmt"
	"net/http"
	"strings"
	"testing"

	requirementgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/requirement/gorm"
	"github.com/xizhou0606/aurora-pm/backend/internal/platform/config"
)

func TestPendingDesignNodeAcceptsLibraryMaterialDownloadPath(t *testing.T) {
	app, token, workspace, _, unit := newAIWorkflowFixture(t)
	store := &fakeMaterialStore{}
	app.cfg.ObjectStorage = config.ObjectStorage{
		Enabled: true, Endpoint: "cos.ap-shanghai.myqcloud.com", Bucket: "pm-materials",
		AccessKey: "access", SecretKey: "secret", UseSSL: true, Region: "ap-shanghai",
		GlobalPrefix: "pm", TeamPrefix: true, PublicBaseURL: "https://pm-materials.cos.ap-shanghai.myqcloud.com",
	}
	app.rebuildEngine(t, store, nil)

	definition := map[string]any{
		"nodes": []map[string]any{
			{"id": "start", "type": "start", "name": "开始"},
			{"id": "input", "type": "form", "name": "需求录入"},
			{"id": "design", "type": "human_task", "name": "UI/UX设计", "config": map[string]any{"require_design_image": true}},
			{"id": "end", "type": "end", "name": "结束"},
		},
		"edges": []map[string]any{
			{"id": "e1", "source": "start", "target": "input"},
			{"id": "e2", "source": "input", "target": "design"},
			{"id": "e3", "source": "design", "target": "end"},
		},
	}
	created := app.request(t, http.MethodPost, "/api/v1/platform/pm/workflows/templates", token, workspace.ID, map[string]any{
		"name": "设计提前上传流程", "subject_type": "requirement", "definition": definition,
	})
	requireStatus(t, created, http.StatusCreated)
	templateID := uint(decodeData(t, created).(map[string]any)["id"].(float64))
	requireStatus(t, app.request(t, http.MethodPost, "/api/v1/platform/pm/workflows/templates/"+uintString(templateID)+"/publish", token, workspace.ID, nil), http.StatusOK)
	createdRequirement := createRequirementWithWorkflow(t, app, token, workspace.ID, unit.ID, templateID, map[string]any{"title": "提前上传设计稿"})
	requirementID := uint(createdRequirement["id"].(float64))
	instanceID := uint(createdRequirement["workflow_instance_id"].(float64))

	pngBody := append([]byte("\x89PNG\r\n\x1a\n"), make([]byte, 512)...)
	uploaded := requirementMaterialUploadRequest(
		t, app, token, workspace.ID,
		fmt.Sprintf("/api/v1/platform/requirements/%d/materials", requirementID),
		"design", "checkout.png", pngBody,
	)
	requireStatus(t, uploaded, http.StatusCreated)
	listed := decodeData(t, uploaded).(map[string]any)
	downloadURL, _ := listed["url"].(string)
	originURL, _ := listed["origin_url"].(string)
	if !strings.HasSuffix(downloadURL, "/download") {
		t.Fatalf("library material url should be the auth download path, got %q", downloadURL)
	}
	if originURL == "" || !strings.HasPrefix(originURL, "https://pm-materials.cos.ap-shanghai.myqcloud.com/") {
		t.Fatalf("library material origin_url=%q", originURL)
	}

	supplemented := app.workflowAction(t, workspace.ID, token, instanceID, map[string]any{
		"action": "supplement_design", "node_id": "design", "design_url": downloadURL,
	})
	requireStatus(t, supplemented, http.StatusOK)
	data := decodeData(t, supplemented).(map[string]any)
	if data["status"] != "running" {
		t.Fatalf("attaching a library design advanced the workflow: %v", data)
	}
	history := data["transition_history"].([]any)
	last := history[len(history)-1].(map[string]any)
	materials, materialsOK := last["materials"].([]any)
	if last["action"] != "material_supplemented" || last["node_id"] != "design" || !materialsOK || len(materials) != 1 || materials[0].(map[string]any)["url"] != originURL {
		t.Fatalf("supplemented material history=%v origin=%q", last, originURL)
	}
	var updated requirementgorm.Requirement
	if err := app.db.Where("id = ? AND workspace_id = ?", requirementID, workspace.ID).First(&updated).Error; err != nil || updated.DesignURL != originURL {
		t.Fatalf("requirement design_url=%q origin=%q err=%v", updated.DesignURL, originURL, err)
	}
}
