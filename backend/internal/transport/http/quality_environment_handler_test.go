package httpapi

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"

	identitygorm "github.com/xizhou0606/aurora-pm/backend/internal/module/identity/gorm"
	qualitygorm "github.com/xizhou0606/aurora-pm/backend/internal/module/quality/gorm"
	qualityhttp "github.com/xizhou0606/aurora-pm/backend/internal/module/quality/http"
)

func sceneOptionsPath(kind string) string {
	return "/api/v1/platform/quality/scene-options/" + kind
}

func TestQualitySceneOptionCatalogsIncludeBuiltInCustomAndLegacyValues(t *testing.T) {
	fix := setupQualityPermissionFixture(t)
	legacy := fix.createBug(t, 980, "triage", false)
	if err := fix.app.db.Model(&legacy).Updates(map[string]any{
		"found_environment": "staging",
		"operating_system":  "macOS",
		"browser":           "Chrome",
	}).Error; err != nil {
		t.Fatal(err)
	}
	token := fix.app.token(t, fix.users["reporter"])

	for _, testCase := range []struct {
		kind       string
		customName string
		legacyName string
		wantCount  int
	}{
		{kind: "environment", customName: "Pre Release", legacyName: "staging", wantCount: 3},
		{kind: "operating_system", customName: "Windows 11", legacyName: "macOS", wantCount: 2},
		{kind: "browser", customName: "Safari", legacyName: "Chrome", wantCount: 2},
	} {
		t.Run(testCase.kind, func(t *testing.T) {
			createdResponse := fix.app.request(t, http.MethodPost,
				sceneOptionsPath(testCase.kind), token, fix.workspaceID,
				map[string]any{"name": "  " + testCase.customName + "  "})
			requireStatus(t, createdResponse, http.StatusCreated)
			createdData := decodeData(t, createdResponse).(map[string]any)
			if createdData["name"] != testCase.customName || createdData["kind"] != testCase.kind {
				t.Fatalf("created option=%v", createdData)
			}

			list := fix.app.request(t, http.MethodGet,
				sceneOptionsPath(testCase.kind), token, fix.workspaceID, nil)
			requireStatus(t, list, http.StatusOK)
			items := decodeData(t, list).(map[string]any)["items"].([]any)
			if len(items) != testCase.wantCount {
				t.Fatalf("%s options=%v body=%s", testCase.kind, items, list.Body.String())
			}
			if testCase.kind == "environment" && (items[0].(map[string]any)["name"] != "test" ||
				items[0].(map[string]any)["built_in"] != true) {
				t.Fatalf("default environment=%v", items[0])
			}
			foundLegacy := false
			for _, item := range items {
				row := item.(map[string]any)
				if row["name"] == testCase.legacyName {
					foundLegacy = row["legacy"] == true && row["usage_count"] == float64(1)
				}
			}
			if !foundLegacy {
				t.Fatalf("legacy option missing usage metadata: %v", items)
			}

			duplicate := fix.app.request(t, http.MethodPost,
				sceneOptionsPath(testCase.kind), token, fix.workspaceID,
				map[string]any{"name": strings.ToLower(testCase.customName)})
			requireStatus(t, duplicate, http.StatusConflict)
			var duplicateBody map[string]any
			if err := json.Unmarshal(duplicate.Body.Bytes(), &duplicateBody); err != nil {
				t.Fatal(err)
			}
			if duplicateBody["code"] != qualityhttp.ErrorSceneOptionNameConflict {
				t.Fatalf("duplicate response=%v", duplicateBody)
			}
		})
	}
}

func TestQualitySceneOptionDeleteProtectsReferencedValues(t *testing.T) {
	fix := setupQualityPermissionFixture(t)
	token := fix.app.token(t, fix.users["reporter"])
	create := func(name string) uint {
		response := fix.app.request(t, http.MethodPost,
			sceneOptionsPath("browser"), token, fix.workspaceID, map[string]any{"name": name})
		requireStatus(t, response, http.StatusCreated)
		return uint(decodeData(t, response).(map[string]any)["id"].(float64))
	}

	unusedID := create("Firefox")
	deleted := fix.app.request(t, http.MethodDelete,
		fmt.Sprintf("%s/%d", sceneOptionsPath("browser"), unusedID), token, fix.workspaceID, nil)
	requireStatus(t, deleted, http.StatusOK)

	inUseID := create("Edge")
	bug := fix.createBug(t, 981, "triage", false)
	if err := fix.app.db.Model(&bug).Update("browser", "Edge").Error; err != nil {
		t.Fatal(err)
	}
	blocked := fix.app.request(t, http.MethodDelete,
		fmt.Sprintf("%s/%d", sceneOptionsPath("browser"), inUseID), token, fix.workspaceID, nil)
	requireStatus(t, blocked, http.StatusConflict)
	var body map[string]any
	if err := json.Unmarshal(blocked.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body["code"] != qualityhttp.ErrorSceneOptionInUse || body["usage_count"] != float64(1) {
		t.Fatalf("blocked delete=%v", body)
	}

	var stored qualitygorm.QualitySceneOption
	if err := fix.app.db.First(&stored, inUseID).Error; err != nil {
		t.Fatalf("referenced option should remain: %v", err)
	}
}

func TestQualitySceneOptionCatalogIsWorkspaceScoped(t *testing.T) {
	fix := setupQualityPermissionFixture(t)
	member := fix.users["reporter"]
	workspace := identitygorm.Workspace{ID: 2, Name: "Other", Slug: "quality-other", Enabled: true}
	for _, row := range []any{
		&workspace,
		&identitygorm.WorkspaceMember{WorkspaceID: workspace.ID, UserID: member.ID, Role: "member", Enabled: true},
	} {
		if err := fix.app.db.Create(row).Error; err != nil {
			t.Fatal(err)
		}
	}
	token := fix.app.token(t, member)
	requireStatus(t, fix.app.request(t, http.MethodPost,
		sceneOptionsPath("operating_system"), token, fix.workspaceID,
		map[string]any{"name": "Linux"}), http.StatusCreated)

	other := fix.app.request(t, http.MethodGet,
		sceneOptionsPath("operating_system"), token, workspace.ID, nil)
	requireStatus(t, other, http.StatusOK)
	items := decodeData(t, other).(map[string]any)["items"].([]any)
	if len(items) != 0 {
		t.Fatalf("other workspace operating systems=%v", items)
	}
}

func TestQualityBugCreateDefaultsEnvironmentAndDefectDomain(t *testing.T) {
	fix := setupQualityPermissionFixture(t)
	token := fix.app.token(t, fix.users["reporter"])
	response := fix.app.request(t, http.MethodPost,
		"/api/v1/platform/quality/bugs", token, fix.workspaceID,
		map[string]any{"requirement_id": fix.requirement.ID, "title": "Default environment"})
	requireStatus(t, response, http.StatusCreated)
	data := decodeData(t, response).(map[string]any)
	if data["found_environment"] != "test" {
		t.Fatalf("create response=%s", response.Body.String())
	}
	if data["defect_domain"] != "functional" {
		t.Fatalf("default defect domain response=%s", response.Body.String())
	}
	var stored qualitygorm.QualityBug
	if err := fix.app.db.Where("title = ?", "Default environment").First(&stored).Error; err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(stored.FoundEnvironment) != "test" {
		t.Fatalf("stored environment=%q", stored.FoundEnvironment)
	}
	if stored.DefectDomain != "functional" {
		t.Fatalf("stored defect domain=%q", stored.DefectDomain)
	}
	updated := fix.app.request(t, http.MethodPatch,
		fmt.Sprintf("/api/v1/platform/quality/bugs/%d", stored.ID), token, fix.workspaceID,
		map[string]any{"defect_domain": "ui"})
	requireStatus(t, updated, http.StatusOK)
	if decodeData(t, updated).(map[string]any)["defect_domain"] != "ui" {
		t.Fatalf("updated defect domain response=%s", updated.Body.String())
	}
}
