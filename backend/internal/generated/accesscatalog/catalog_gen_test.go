package accesscatalog

import "testing"

func TestGeneratedCatalogPublicAPI(t *testing.T) {
	tree := Tree()
	if len(tree) == 0 {
		t.Fatal("generated permission tree is empty")
	}
	codes := ActionCodes()
	if len(codes) == 0 {
		t.Fatal("generated action-code set is empty")
	}
	records := BootstrapActionRoutes()
	if len(records) == 0 {
		t.Fatal("generated bootstrap route records are empty")
	}

	seen := make(map[ActionCode]struct{}, len(codes))
	for _, code := range codes {
		if _, exists := seen[code]; exists {
			t.Fatalf("duplicate generated action code %q", code)
		}
		seen[code] = struct{}{}
	}
	interfaceCodes := make(map[string]struct{}, len(records))
	for _, record := range records {
		if len(record.InterfaceCode) == 0 || len(record.InterfaceCode) > 96 {
			t.Fatalf("invalid generated interface code %q", record.InterfaceCode)
		}
		if _, exists := interfaceCodes[record.InterfaceCode]; exists {
			t.Fatalf("duplicate generated interface code %q", record.InterfaceCode)
		}
		interfaceCodes[record.InterfaceCode] = struct{}{}
		resolved, ok := ActionForRoute(record.HTTPMethod, record.RouteTemplate)
		if !ok || resolved != record.ActionCode {
			t.Fatalf("route lookup mismatch for %s %s: got %q, %v; want %q",
				record.HTTPMethod, record.RouteTemplate, resolved, ok, record.ActionCode)
		}
	}
}

func TestTreeReturnsAnIndependentCopy(t *testing.T) {
	first := Tree()
	second := Tree()
	first[0].Name = "mutated"
	first[0].FrontendRoutes[0] = "mutated"
	first[0].Modules[0].Name = "mutated"
	first[0].Modules[0].Actions[0].Name = "mutated"
	first[0].Modules[0].Actions[0].Routes[0].RouteTemplate = "mutated"

	if second[0].Name == "mutated" || second[0].FrontendRoutes[0] == "mutated" ||
		second[0].Modules[0].Name == "mutated" || second[0].Modules[0].Actions[0].Name == "mutated" ||
		second[0].Modules[0].Actions[0].Routes[0].RouteTemplate == "mutated" {
		t.Fatal("Tree exposed mutable generated backing storage")
	}
}

func TestPermissionTreeUsesRecursiveFrontendViewsAndReturnsIndependentCopies(t *testing.T) {
	first := PermissionTree()
	second := PermissionTree()
	var taskTable *PermissionView
	for pageIndex := range first {
		if first[pageIndex].Code != "pm.work-items" {
			continue
		}
		for viewIndex := range first[pageIndex].Views {
			if first[pageIndex].Views[viewIndex].Code == "pm.work-items.views.task-table" {
				taskTable = &first[pageIndex].Views[viewIndex]
				break
			}
		}
	}
	if taskTable == nil {
		t.Fatal("work-items task-table frontend view is missing")
	}
	if len(taskTable.Children) != 1 || taskTable.Children[0].Code != "pm.work-items.views.task-detail" {
		t.Fatalf("task-table child views = %#v; want recursive task-detail view", taskTable.Children)
	}
	if len(taskTable.Actions) == 0 || len(taskTable.Children[0].Actions) == 0 {
		t.Fatal("recursive frontend views must retain their action references")
	}

	taskTable.Name = "mutated"
	taskTable.Children[0].Name = "mutated"
	taskTable.Actions[0].Routes[0].RouteTemplate = "mutated"
	for _, page := range second {
		if page.Code != "pm.work-items" {
			continue
		}
		for _, view := range page.Views {
			if view.Code == "pm.work-items.views.task-table" && (view.Name == "mutated" || view.Children[0].Name == "mutated" ||
				view.Actions[0].Routes[0].RouteTemplate == "mutated") {
				t.Fatal("PermissionTree exposed mutable generated backing storage")
			}
		}
	}
}

func TestPermissionTreeGroupsDetailSubresources(t *testing.T) {
	tree := PermissionTree()
	taskDetail := findPermissionView(tree, "pm.work-items.views.task-detail")
	if taskDetail == nil {
		t.Fatal("work-items task-detail frontend view is missing")
	}
	assertPermissionViewChildren(t, *taskDetail, map[string]string{
		"pm.work-items.views.task-detail.attachments": "任务附件",
		"pm.work-items.views.task-detail.comments":    "任务评论",
	})

	bugCreate := findPermissionView(tree, "pm.quality.views.bug-create")
	if bugCreate == nil {
		t.Fatal("quality bug-create frontend view is missing")
	}
	assertPermissionViewChildren(t, *bugCreate, map[string]string{
		"pm.quality.views.bug-create.materials":           "缺陷素材",
		"pm.quality.views.bug-create.environment-options": "ENV维护（发现环境/OS/浏览器）",
	})

	bugDetail := findPermissionView(tree, "pm.quality.views.bug-detail")
	if bugDetail == nil {
		t.Fatal("quality bug-detail frontend view is missing")
	}
	assertPermissionViewChildren(t, *bugDetail, map[string]string{
		"pm.quality.views.bug-detail.comments":            "缺陷评论",
		"pm.quality.views.bug-detail.materials":           "缺陷素材",
		"pm.quality.views.bug-detail.environment-options": "ENV维护（发现环境/OS/浏览器）",
	})
}

func findPermissionView(tree []PermissionPage, code string) *PermissionView {
	var find func(views []PermissionView) *PermissionView
	find = func(views []PermissionView) *PermissionView {
		for index := range views {
			if views[index].Code == code {
				return &views[index]
			}
			if result := find(views[index].Children); result != nil {
				return result
			}
		}
		return nil
	}
	for index := range tree {
		if result := find(tree[index].Views); result != nil {
			return result
		}
	}
	return nil
}

func assertPermissionViewChildren(t *testing.T, view PermissionView, want map[string]string) {
	t.Helper()
	got := make(map[string]string, len(view.Children))
	for _, child := range view.Children {
		got[child.Code] = child.Name
	}
	if len(got) != len(want) {
		t.Fatalf("permission view %s children = %#v; want %#v", view.Code, got, want)
	}
	for code, name := range want {
		if got[code] != name {
			t.Fatalf("permission view %s child %s = %q; want %q", view.Code, code, got[code], name)
		}
	}
}

func TestWorkItemViewsOwnSeparateReadInterfaces(t *testing.T) {
	if got, ok := ActionForRoute("GET", "/api/v1/platform/tasks"); !ok || got != ActionWorkItemsTaskTableView {
		t.Fatalf("task table route resolved to %q, %v", got, ok)
	}
	if got, ok := ActionForRoute("GET", "/api/v1/platform/tasks/:id"); !ok || got != ActionWorkItemsTaskDetailView {
		t.Fatalf("task detail route resolved to %q, %v", got, ok)
	}
	if got, ok := ActionForRoute("GET", "/api/v1/platform/tasks/:id/deletion-impact"); !ok || got != ActionWorkItemsTasksDelete {
		t.Fatalf("deletion-impact route resolved to %q, %v", got, ok)
	}
	if got, ok := ActionForRoute("GET", "/api/v1/platform/tasks/:id/due-at-descendant-conflicts"); !ok || got != ActionWorkItemsTasksUpdate {
		t.Fatalf("due-at conflict route resolved to %q, %v", got, ok)
	}
}

func TestActionForRouteNormalizesOnlyMethodWhitespaceAndCase(t *testing.T) {
	want := ActionRequirementsMaterialsView
	got, ok := ActionForRoute(" get ", " /api/v1/platform/requirements/:id/materials ")
	if !ok || got != want {
		t.Fatalf("ActionForRoute() = %q, %v; want %q, true", got, ok, want)
	}
	if _, ok := ActionForRoute("GET", "/platform/requirements/:id/materials"); ok {
		t.Fatal("compatibility alias unexpectedly resolved as a canonical route")
	}
}

func TestFixedBaselineRoutesKeepRelatedToolsIndependentFromBusinessActions(t *testing.T) {
	route, ok := BaselineCapabilityForRoute("GET", "/api/v1/platform/project-requirement-scope-options")
	if !ok || route.Capability != "system.workspace.scope-directory" {
		t.Fatalf("baseline scope route = %#v, %v", route, ok)
	}
	if len(route.RelatedToolNames) != 1 || route.RelatedToolNames[0] != "project.scope_options.list" {
		t.Fatalf("related tools = %#v", route.RelatedToolNames)
	}
	if _, ok := ActionForTool("project.scope_options.list"); ok {
		t.Fatal("baseline-related Tool unexpectedly inherited a business Action")
	}

	first := BaselineRoutes()
	originalCount := len(first[0].RelatedToolNames)
	first[0].RelatedToolNames = append(first[0].RelatedToolNames, "mutated.tool")
	second := BaselineRoutes()
	if len(second[0].RelatedToolNames) != originalCount {
		t.Fatal("BaselineRoutes exposed mutable generated backing storage")
	}
}

func TestCurrentUserToolIsAReadableSessionBaseline(t *testing.T) {
	route, ok := BaselineCapabilityForRoute("GET", "/api/v1/core/auth/me")
	if !ok || route.Capability != "system.session.inspect" {
		t.Fatalf("current-user baseline route = %#v, %v", route, ok)
	}
	if len(route.RelatedToolNames) != 1 || route.RelatedToolNames[0] != "user.me" {
		t.Fatalf("current-user related tools = %#v", route.RelatedToolNames)
	}
	if _, ok := ActionForTool("user.me"); ok {
		t.Fatal("user.me unexpectedly inherited a business Action")
	}
}

func TestBusinessOperationBindingsCoverGovernedActionsAndTools(t *testing.T) {
	bindings := OperationBindings()
	if len(bindings) != 186 {
		t.Fatalf("operation bindings = %d; want 186", len(bindings))
	}
	seen := make(map[ActionCode]struct{}, len(bindings))
	for _, binding := range bindings {
		if binding.FrontendOwner == "" {
			t.Fatalf("operation %q has no frontend owner", binding.ActionCode)
		}
		if _, exists := seen[binding.ActionCode]; exists {
			t.Fatalf("duplicate operation binding %q", binding.ActionCode)
		}
		seen[binding.ActionCode] = struct{}{}
		for _, toolName := range binding.ToolNames {
			resolved, ok := ActionForTool(toolName)
			if !ok || resolved != binding.ActionCode {
				t.Fatalf("Tool lookup mismatch for %q: got %q, %v; want %q",
					toolName, resolved, ok, binding.ActionCode)
			}
		}
	}

	if got, ok := ActionForTool("project.set.create"); !ok || got != ActionProjectsProjectSetsCreate {
		t.Fatalf("project.set.create resolved to %q, %v", got, ok)
	}
	if got, ok := ActionForTool("workspace.member.list"); !ok || got != ActionAccessManagementWorkspacesMembersView {
		t.Fatalf("workspace.member.list resolved to %q, %v", got, ok)
	}
	tools := ToolNamesForAction(ActionInboxTodosView)
	if len(tools) != 2 || tools[0] != "workbench.todo.summary" || tools[1] != "workbench.todo.feed" {
		t.Fatalf("inbox tools = %#v", tools)
	}
	tools[0] = "mutated.tool"
	if ToolNamesForAction(ActionInboxTodosView)[0] == "mutated.tool" {
		t.Fatal("ToolNamesForAction exposed generated backing storage")
	}
	if _, ok := ActionForTool("unknown.tool"); ok {
		t.Fatal("unknown Tool unexpectedly resolved to an action")
	}
}
