package httpapi

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	aigorm "github.com/xizhou0606/aurora-pm/backend/internal/module/ai/gorm"
	requirementgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/requirement/gorm"
	taskgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/task/gorm"
)

func TestAITestCaseContextRequirementLimit(t *testing.T) {
	for _, test := range []struct {
		name       string
		count      int
		wantErr    string
		wantLoaded int
	}{
		{name: "at limit", count: aiTestCaseRequirementLimit, wantLoaded: aiTestCaseRequirementLimit},
		{name: "over limit", count: aiTestCaseRequirementLimit + 1, wantErr: "requirement limit"},
	} {
		t.Run(test.name, func(t *testing.T) {
			app, _, workspace, project, unit := newAIWorkflowFixture(t)
			requirements := make([]requirementgorm.Requirement, 0, test.count)
			for index := 0; index < test.count; index++ {
				requirements = append(requirements, requirementgorm.Requirement{
					WorkspaceID:  workspace.ID,
					TeamID:       project.TeamID,
					ProjectSetID: project.ID,
					PlanUnitID:   unit.ID,
					Title:        fmt.Sprintf("需求-%03d", index+1),
					Version:      1,
				})
			}
			if err := app.db.CreateInBatches(&requirements, 100).Error; err != nil {
				t.Fatal(err)
			}

			context, err := (&Server{}).aiTestCaseContext(app.db, workspace.ID, "plan_unit", unit.ID)
			if test.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), test.wantErr) {
					t.Fatalf("error=%v, want containing %q", err, test.wantErr)
				}
				if context != nil {
					t.Fatalf("context=%v, want nil", context)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			assertAITestCaseContextMetadata(t, context, test.wantLoaded, 0)
		})
	}
}

func TestAITestCaseContextTaskLimit(t *testing.T) {
	for _, test := range []struct {
		name       string
		count      int
		wantErr    string
		wantLoaded int
	}{
		{name: "at limit", count: aiTestCaseTaskLimit, wantLoaded: aiTestCaseTaskLimit},
		{name: "over limit", count: aiTestCaseTaskLimit + 1, wantErr: "task limit"},
	} {
		t.Run(test.name, func(t *testing.T) {
			app, _, workspace, project, unit := newAIWorkflowFixture(t)
			requirement := requirementgorm.Requirement{
				WorkspaceID:  workspace.ID,
				TeamID:       project.TeamID,
				ProjectSetID: project.ID,
				PlanUnitID:   unit.ID,
				Title:        "复杂需求",
				Version:      1,
			}
			if err := app.db.Create(&requirement).Error; err != nil {
				t.Fatal(err)
			}
			tasks := make([]taskgorm.WorkTask, 0, test.count)
			for index := 0; index < test.count; index++ {
				tasks = append(tasks, taskgorm.WorkTask{
					WorkspaceID:   workspace.ID,
					TeamID:        project.TeamID,
					ProjectSetID:  project.ID,
					ProjectID:     unit.ID,
					RequirementID: requirement.ID,
					Kind:          taskgorm.WorkTaskKindTask,
					TaskKey:       fmt.Sprintf("QA-CONTEXT-%03d", index+1),
					Title:         fmt.Sprintf("任务-%03d", index+1),
					Status:        "todo",
					Priority:      "normal",
				})
			}
			if err := app.db.CreateInBatches(&tasks, 100).Error; err != nil {
				t.Fatal(err)
			}

			context, err := (&Server{}).aiTestCaseContext(app.db, workspace.ID, "plan_unit", unit.ID)
			if test.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), test.wantErr) {
					t.Fatalf("error=%v, want containing %q", err, test.wantErr)
				}
				if context != nil {
					t.Fatalf("context=%v, want nil", context)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			assertAITestCaseContextMetadata(t, context, 1, test.wantLoaded)
		})
	}
}

func TestAITestCaseContextRequirementUsesOnlyRequirementDocument(t *testing.T) {
	app, _, workspace, project, unit := newAIWorkflowFixture(t)
	requirement := requirementgorm.Requirement{
		WorkspaceID: workspace.ID, TeamID: project.TeamID, ProjectSetID: project.ID,
		PlanUnitID: unit.ID, Title: "单需求文档", ProductDocURL: "https://acme.feishu.cn/docx/requirement", Version: 1,
	}
	if err := app.db.Create(&requirement).Error; err != nil {
		t.Fatal(err)
	}
	tasks := []taskgorm.WorkTask{
		{
			WorkspaceID: workspace.ID, TeamID: project.TeamID, ProjectSetID: project.ID,
			ProjectID: unit.ID, RequirementID: requirement.ID, Kind: taskgorm.WorkTaskKindTask,
			TaskKey: "DEV-001", Title: "开发实现", Status: "todo", Priority: "normal",
		},
		{
			WorkspaceID: workspace.ID, TeamID: project.TeamID, ProjectSetID: project.ID,
			ProjectID: unit.ID, RequirementID: requirement.ID, Kind: taskgorm.WorkTaskKindTask,
			TaskKey: "DEV-002", Title: "联调实现", Status: "todo", Priority: "normal",
		},
	}
	if err := app.db.Create(&tasks).Error; err != nil {
		t.Fatal(err)
	}

	context, err := (&Server{}).aiTestCaseContext(app.db, workspace.ID, "requirement", requirement.ID)
	if err != nil {
		t.Fatal(err)
	}
	assertAITestCaseContextMetadata(t, context, 1, 0)
	if context["mode"] != "single_requirement_document" {
		t.Fatalf("mode=%v, want single_requirement_document", context["mode"])
	}
}

func TestAITestCaseContextRejectsOversizedItem(t *testing.T) {
	app, _, workspace, project, unit := newAIWorkflowFixture(t)
	requirement := requirementgorm.Requirement{
		WorkspaceID: workspace.ID, TeamID: project.TeamID, ProjectSetID: project.ID,
		PlanUnitID: unit.ID, Title: "超长需求", Description: strings.Repeat("界", aiTestCaseContextByteLimit), Version: 1,
	}
	if err := app.db.Create(&requirement).Error; err != nil {
		t.Fatal(err)
	}
	context, err := (&Server{}).aiTestCaseContext(app.db, workspace.ID, "plan_unit", unit.ID)
	if err == nil || !strings.Contains(err.Error(), "128 KiB") || context != nil {
		t.Fatalf("context=%v error=%v", context, err)
	}
}

func TestAITestCaseContextRejectsOversizedCombinedInput(t *testing.T) {
	app, _, workspace, project, unit := newAIWorkflowFixture(t)
	requirements := make([]requirementgorm.Requirement, 0, 9)
	for index := 0; index < 9; index++ {
		requirements = append(requirements, requirementgorm.Requirement{
			WorkspaceID: workspace.ID, TeamID: project.TeamID, ProjectSetID: project.ID,
			PlanUnitID: unit.ID, Title: fmt.Sprintf("大需求-%d", index+1),
			Description: strings.Repeat("x", 120<<10), Version: 1,
		})
	}
	if err := app.db.CreateInBatches(&requirements, 9).Error; err != nil {
		t.Fatal(err)
	}
	context, err := (&Server{}).aiTestCaseContext(app.db, workspace.ID, "plan_unit", unit.ID)
	if err == nil || !strings.Contains(err.Error(), "1 MiB") || context != nil {
		t.Fatalf("context=%v error=%v", context, err)
	}
}

func TestPlanUnitTestCaseRunCreatesEveryUniqueFeishuSource(t *testing.T) {
	app, token, workspace, project, unit := newAIWorkflowFixture(t)
	urls := []string{
		"https://acme.feishu.cn/docx/source-a",
		"https://acme.feishu.cn/docx/source-b",
		"https://acme.feishu.cn/docx/source-a",
	}
	for index, sourceURL := range urls {
		requirement := requirementgorm.Requirement{
			WorkspaceID: workspace.ID, TeamID: project.TeamID, ProjectSetID: project.ID,
			PlanUnitID: unit.ID, Title: fmt.Sprintf("需求-%d", index+1), ProductDocURL: sourceURL, Version: 1,
		}
		if err := app.db.Create(&requirement).Error; err != nil {
			t.Fatal(err)
		}
	}
	response := app.request(t, http.MethodPost, "/api/v1/platform/ai/runs", token, workspace.ID, map[string]any{
		"capability": "qa.test_cases", "subject_type": "plan_unit", "subject_id": unit.ID,
	})
	requireStatus(t, response, http.StatusCreated)
	runID := uint(decodeData(t, response).(map[string]any)["id"].(float64))
	var sources []aigorm.AIRunSource
	if err := app.db.Where("run_id = ?", runID).Order("id").Find(&sources).Error; err != nil {
		t.Fatal(err)
	}
	if len(sources) != 2 || sources[0].SourceURL != urls[0] || sources[1].SourceURL != urls[1] {
		t.Fatalf("sources=%+v", sources)
	}
	var run aigorm.AIRun
	if err := app.db.First(&run, runID).Error; err != nil {
		t.Fatal(err)
	}
	var input map[string]any
	if err := json.Unmarshal([]byte(run.InputJSON), &input); err != nil {
		t.Fatal(err)
	}
	fingerprint, _ := input["test_case_context_sha256"].(string)
	if len(fingerprint) != 64 {
		t.Fatalf("context fingerprint=%q", fingerprint)
	}
}

func TestRequirementTestCaseRunUsesSingleDocumentContext(t *testing.T) {
	app, token, workspace, project, unit := newAIWorkflowFixture(t)
	requirement := requirementgorm.Requirement{
		WorkspaceID: workspace.ID, TeamID: project.TeamID, ProjectSetID: project.ID,
		PlanUnitID: unit.ID, Title: "单需求测试", ProductDocURL: "https://acme.feishu.cn/docx/requirement-only", Version: 1,
	}
	if err := app.db.Create(&requirement).Error; err != nil {
		t.Fatal(err)
	}
	task := taskgorm.WorkTask{
		WorkspaceID: workspace.ID, TeamID: project.TeamID, ProjectSetID: project.ID,
		ProjectID: unit.ID, RequirementID: requirement.ID, Kind: taskgorm.WorkTaskKindTask,
		TaskKey: "DEV-001", Title: "不应进入测试输入", Status: "todo", Priority: "normal",
	}
	if err := app.db.Create(&task).Error; err != nil {
		t.Fatal(err)
	}

	response := app.request(t, http.MethodPost, "/api/v1/platform/ai/runs", token, workspace.ID, map[string]any{
		"capability": "qa.test_cases", "subject_type": "requirement", "subject_id": requirement.ID,
	})
	requireStatus(t, response, http.StatusCreated)
	runID := uint(decodeData(t, response).(map[string]any)["id"].(float64))

	var run aigorm.AIRun
	if err := app.db.First(&run, runID).Error; err != nil {
		t.Fatal(err)
	}
	var input map[string]any
	if err := json.Unmarshal([]byte(run.InputJSON), &input); err != nil {
		t.Fatal(err)
	}
	context := input["test_case_context"].(map[string]any)
	if context["mode"] != "single_requirement_document" ||
		len(context["requirements"].([]any)) != 1 || len(context["tasks"].([]any)) != 0 {
		t.Fatalf("single requirement context=%v", context)
	}
	var sources []aigorm.AIRunSource
	if err := app.db.Where("run_id = ?", runID).Find(&sources).Error; err != nil {
		t.Fatal(err)
	}
	if len(sources) != 1 || sources[0].SourceURL != requirement.ProductDocURL {
		t.Fatalf("sources=%+v, want only requirement document", sources)
	}
}

func assertAITestCaseContextMetadata(t *testing.T, context gin.H, requirements, tasks int) {
	t.Helper()
	if context["coverageRequired"] != true {
		t.Fatalf("coverageRequired=%v, want true", context["coverageRequired"])
	}
	counts, ok := context["counts"].(gin.H)
	if !ok {
		t.Fatalf("counts=%T, want gin.H", context["counts"])
	}
	if counts["requirements"] != requirements || counts["tasks"] != tasks {
		t.Fatalf("counts=%v, want requirements=%d tasks=%d", counts, requirements, tasks)
	}
	if len(context["requirements"].([]gin.H)) != requirements || len(context["tasks"].([]gin.H)) != tasks {
		t.Fatalf("context lengths do not match counts: %v", counts)
	}
}
