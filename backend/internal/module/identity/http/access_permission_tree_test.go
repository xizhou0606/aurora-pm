package identityhttp

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/xizhou0606/aurora-pm/backend/internal/generated/accesscatalog"
)

func TestPermissionTreeActionIncludesCanonicalToolNames(t *testing.T) {
	node := permissionTreeViewNode(accesscatalog.PermissionView{
		Code: "pm.inbox.views.todos",
		Name: "我的待办",
		Actions: []accesscatalog.Action{{
			Code: accesscatalog.ActionInboxTodosView,
			Name: "查看",
			Routes: []accesscatalog.RouteBinding{{
				HTTPMethod: "GET", RouteTemplate: "/api/v1/platform/workbench/todos/summary",
			}},
		}},
	})

	if len(node.Children) != 1 {
		t.Fatalf("permission tree children = %d; want 1", len(node.Children))
	}
	action := node.Children[0]
	if len(action.Tools) != 2 || action.Tools[0] != "workbench.todo.summary" || action.Tools[1] != "workbench.todo.feed" {
		t.Fatalf("permission action tools = %#v", action.Tools)
	}
	payload, err := json.Marshal(action)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(payload), `"tools":["workbench.todo.summary","workbench.todo.feed"]`) {
		t.Fatalf("permission action JSON = %s", payload)
	}
}

func TestPermissionTreeActionOmitsEmptyToolList(t *testing.T) {
	node := permissionTreeViewNode(accesscatalog.PermissionView{
		Code: "pm.bi-screen.views.metrics",
		Name: "指标配置",
		Actions: []accesscatalog.Action{{
			Code: accesscatalog.ActionBiScreenMetricsView,
			Name: "查看",
			Routes: []accesscatalog.RouteBinding{{
				HTTPMethod: "GET", RouteTemplate: "/api/v1/platform/executive-dashboard/metrics",
			}},
		}},
	})

	payload, err := json.Marshal(node.Children[0])
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(payload), `"tools"`) {
		t.Fatalf("empty tools should be omitted: %s", payload)
	}
}

func TestPermissionTreeActionIncludesCanonicalObjectPolicy(t *testing.T) {
	node := permissionTreeViewNode(accesscatalog.PermissionView{
		Code: "pm.work-items.views.task-table",
		Name: "任务表格",
		Actions: []accesscatalog.Action{{
			Code: accesscatalog.ActionWorkItemsTasksUpdate,
			Name: "编辑任务",
			Routes: []accesscatalog.RouteBinding{{
				HTTPMethod: "PATCH", RouteTemplate: "/api/v1/platform/tasks/:id",
			}},
		}},
	})

	action := node.Children[0]
	if action.ObjectPolicy == nil || action.ObjectPolicy.Code != "task.creator" ||
		action.ObjectPolicy.Kind != "creator_override" {
		t.Fatalf("permission action object policy = %#v", action.ObjectPolicy)
	}
	payload, err := json.Marshal(action)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(payload), `"object_policy":{"code":"task.creator","kind":"creator_override"}`) {
		t.Fatalf("permission action JSON = %s", payload)
	}
}
