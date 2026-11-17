#!/usr/bin/env python3
"""Deterministic renderers for the canonical PM access catalog."""

from __future__ import annotations

import hashlib
import json
import re
import subprocess
from pathlib import Path


class GenerationError(Exception):
    """Raised when the catalog cannot produce a safe typed artifact."""


def _go_string(value: object) -> str:
    return json.dumps(str(value), ensure_ascii=False)


def _identifier(code: str) -> str:
    words = re.findall(r"[a-zA-Z0-9]+", code.removeprefix("pm."))
    if not words:
        raise GenerationError(f"action code cannot form a Go identifier: {code!r}")
    return "Action" + "".join(word[:1].upper() + word[1:] for word in words)


def _object_policy_identifier(code: str) -> str:
    words = re.findall(r"[a-zA-Z0-9]+", code)
    if not words:
        raise GenerationError(f"object policy code cannot form a Go identifier: {code!r}")
    return "ObjectPolicy" + "".join(word[:1].upper() + word[1:] for word in words)


def _interface_code(method: object, route_template: object) -> str:
    identity = f"{str(method).upper().strip()} {str(route_template).strip()}"
    return "route." + hashlib.sha256(identity.encode("utf-8")).hexdigest()[:24]


def _format_go(source: str) -> str:
    try:
        result = subprocess.run(
            ["gofmt"], input=source, text=True, capture_output=True, check=False
        )
    except OSError as exc:
        raise GenerationError(f"gofmt is required to generate the Go catalog: {exc}") from exc
    if result.returncode != 0:
        raise GenerationError(f"gofmt rejected generated catalog: {result.stderr.strip()}")
    return result.stdout


def operation_actions_by_domain(catalog: dict) -> dict[str, list[dict]]:
    """Project operation registry derived from the canonical permission tree."""
    governance = catalog.get("operation_governance", {})
    domains = governance.get("domains", [])
    pages_by_code = {str(page.get("code", "")): page for page in catalog.get("pages", [])}
    result: dict[str, list[dict]] = {}
    for domain in domains:
        owner = str(domain.get("owner", ""))
        actions: list[dict] = []
        for page_code in domain.get("page_codes", []):
            page = pages_by_code.get(str(page_code), {})
            for module in page.get("modules", []):
                actions.extend(module.get("actions", []))
        result[owner] = actions
    return result


def permission_views_by_page(catalog: dict) -> dict[str, list[dict]]:
    """Frontend-first permission views keyed by their permission page."""
    result: dict[str, list[dict]] = {}
    for page_views in catalog.get("operation_governance", {}).get("permission_views", []):
        result[str(page_views.get("page_code", ""))] = list(page_views.get("views", []))
    return result


def iter_permission_view_nodes(views: list[dict]):
    """Yield recursive frontend view nodes in display order."""
    for view in views:
        yield view
        yield from iter_permission_view_nodes(list(view.get("children", [])))


def permission_view_placements(catalog: dict) -> list[dict]:
    """Expand recursive view nodes into auditable action placements."""
    pages_by_code = {str(page.get("code", "")): page for page in catalog.get("pages", [])}
    actions_by_code = {
        str(action.get("code", "")): action
        for page in catalog.get("pages", [])
        for module in page.get("modules", [])
        for action in module.get("actions", [])
    }
    result: list[dict] = []

    def append_views(page_code: str, views: list[dict], ancestors: list[str]) -> None:
        page = pages_by_code.get(page_code, {})
        for view in views:
            path = [*ancestors, str(view.get("name", ""))]
            for action_code in view.get("actions", []):
                result.append({
                    "page_code": page_code,
                    "page_name": str(page.get("name", "")),
                    "view_path": " > ".join(path),
                    "kind": str(view.get("kind", "")),
                    "components": list(view.get("components", [])),
                    "action": actions_by_code.get(str(action_code), {"code": str(action_code)}),
                })
            append_views(page_code, list(view.get("children", [])), path)

    for page_views in catalog.get("operation_governance", {}).get("permission_views", []):
        append_views(str(page_views.get("page_code", "")), list(page_views.get("views", [])), [])
    return result


def baseline_routes(catalog: dict) -> list[dict]:
    """Return fixed PM-session capabilities that stay outside grantable RBAC."""
    return [
        entry
        for entry in catalog.get("route_inventory", {}).get("authenticated_exemptions", [])
        if str(entry.get("capability", "")).strip()
    ]


def render_go(catalog: dict, source_label: str = "contracts/access/catalog.json") -> str:
    pages = catalog.get("pages", [])
    actions = [action for page in pages for module in page.get("modules", []) for action in module.get("actions", [])]
    modules = [module for page in pages for module in page.get("modules", [])]
    for level, nodes in (("page", pages), ("module", modules), ("action", actions)):
        for node in nodes:
            for field in ("code", "name", "copy_key"):
                if not str(node.get(field, "")).strip():
                    raise GenerationError(f"{level} {field} is required for generation")
    identifiers: dict[str, str] = {}
    interface_codes: dict[str, str] = {}
    for action in actions:
        code = str(action.get("code", ""))
        identifier = _identifier(code)
        if identifier in identifiers:
            raise GenerationError(
                f"Go action identifier collision: {identifiers[identifier]!r} and {code!r} -> {identifier}"
            )
        identifiers[identifier] = code
        for route in action.get("routes", []):
            route_identity = f"{str(route.get('method', '')).upper().strip()} {str(route.get('path', '')).strip()}"
            interface_code = _interface_code(route.get("method", ""), route.get("path", ""))
            if len(interface_code) > 96:
                raise GenerationError(f"generated interface code exceeds 96 characters: {interface_code}")
            if interface_code in interface_codes and interface_codes[interface_code] != route_identity:
                raise GenerationError(
                    f"generated interface-code collision: {interface_codes[interface_code]!r} and "
                    f"{route_identity!r} -> {interface_code}"
                )
            interface_codes[interface_code] = route_identity

    canonical = json.dumps(catalog, ensure_ascii=False, sort_keys=True, separators=(",", ":"))
    digest = hashlib.sha256(canonical.encode("utf-8")).hexdigest()
    operations_by_domain = operation_actions_by_domain(catalog)
    permission_views = permission_views_by_page(catalog)
    governance = catalog.get("operation_governance", {})
    tool_bindings = governance.get("tool_bindings", {})
    object_policies = governance.get("object_policies", {})
    action_implications = governance.get("action_implications", {})
    object_policy_identifiers: dict[str, str] = {}
    for code in object_policies:
        identifier = _object_policy_identifier(str(code))
        if identifier in object_policy_identifiers:
            raise GenerationError(
                "Go object-policy identifier collision: "
                f"{object_policy_identifiers[identifier]!r} and {code!r} -> {identifier}"
            )
        object_policy_identifiers[identifier] = str(code)
    fixed_baseline_routes = baseline_routes(catalog)
    page_owner = {
        str(page_code): str(domain.get("owner", ""))
        for domain in catalog.get("operation_governance", {}).get("domains", [])
        for page_code in domain.get("page_codes", [])
    }
    action_owner = {
        str(action.get("code", "")): page_owner.get(
            str(page.get("code", "")),
            str(page.get("code", "")).removeprefix("pm."),
        )
        for page in pages
        for module in page.get("modules", [])
        for action in module.get("actions", [])
    }
    action_codes = {str(action["code"]) for action in actions}
    for source, targets in action_implications.items():
        if str(source) not in action_codes:
            raise GenerationError(f"action implication references unknown source action {source!r}")
        for target in targets:
            if str(target) not in action_codes:
                raise GenerationError(
                    f"action implication {source!r} references unknown target action {target!r}"
                )
    for page_code, views in permission_views.items():
        for view in iter_permission_view_nodes(views):
            for action_code in view.get("actions", []):
                if str(action_code) not in action_codes:
                    raise GenerationError(
                        f"permission view {page_code!r}/{view.get('code', '')!r} references "
                        f"unknown action {action_code!r}"
                    )
    lines = [
        f"// Code generated by scripts/generate-access-catalog.py from {source_label}; DO NOT EDIT.",
        f"// Catalog SHA-256: {digest}",
        "",
        "package accesscatalog",
        "",
        'import "strings"',
        "",
        "type ActionCode string",
        "",
        "type ObjectPolicyCode string",
        "",
        "type RouteBinding struct {",
        "HTTPMethod string",
        "RouteTemplate string",
        "}",
        "",
        "type BaselineRoute struct {",
        "Capability string",
        "AccessStage string",
        "HTTPMethod string",
        "RouteTemplate string",
        "RelatedToolNames []string",
        "}",
        "",
        "type ObjectPolicy struct {",
        "Code ObjectPolicyCode",
        "Kind string",
        "EnforcementMarkers []string",
        "}",
        "",
        "type Action struct {",
        "Code ActionCode",
        "Name string",
        "CopyKey string",
        "Routes []RouteBinding",
        "FrontendOnlyReason string",
        "ObjectPolicyCode ObjectPolicyCode",
        "}",
        "",
        "type Module struct {",
        "Code string",
        "Name string",
        "CopyKey string",
        "Actions []Action",
        "}",
        "",
        "type Page struct {",
        "Code string",
        "Name string",
        "CopyKey string",
        "FrontendRoutes []string",
        "Modules []Module",
        "}",
        "",
        "type PermissionViewDefinition struct {",
        "Code string",
        "Name string",
        "CopyKey string",
        "Kind string",
        "Components []string",
        "ActionCodes []ActionCode",
        "Children []PermissionViewDefinition",
        "}",
        "",
        "type PermissionView struct {",
        "Code string",
        "Name string",
        "CopyKey string",
        "Kind string",
        "Components []string",
        "Actions []Action",
        "Children []PermissionView",
        "}",
        "",
        "type PermissionPage struct {",
        "Code string",
        "Name string",
        "CopyKey string",
        "FrontendRoutes []string",
        "Views []PermissionView",
        "}",
        "",
        "type ActionRouteRecord struct {",
        "InterfaceCode string",
        "ActionCode ActionCode",
        "ActionName string",
        "CopyKey string",
        "HTTPMethod string",
        "RouteTemplate string",
        "}",
        "",
        "type OperationBinding struct {",
        "ActionCode ActionCode",
        "FrontendOwner string",
        "ToolNames []string",
        "}",
        "",
        "const (",
    ]
    for identifier, code in identifiers.items():
        lines.append(f"{identifier} ActionCode = {_go_string(code)}")
    lines.extend([")", "", "const ("])
    for identifier, code in object_policy_identifiers.items():
        lines.append(f"{identifier} ObjectPolicyCode = {_go_string(code)}")
    lines.extend([")", "", "var objectPolicies = map[ObjectPolicyCode]ObjectPolicy{"])
    for code, policy in object_policies.items():
        identifier = _object_policy_identifier(str(code))
        lines.append(
            f"{identifier}: {{Code: {identifier}, Kind: {_go_string(policy.get('kind', ''))}, "
            "EnforcementMarkers: []string{"
            + ", ".join(_go_string(item) for item in policy.get("enforcement_markers", []))
            + "}},"
        )
    lines.extend(["}", "", "var catalogPages = []Page{"])
    for page in pages:
        lines.extend([
            "{",
            f"Code: {_go_string(page['code'])},",
            f"Name: {_go_string(page['name'])},",
            f"CopyKey: {_go_string(page['copy_key'])},",
            "FrontendRoutes: []string{" + ", ".join(_go_string(item) for item in page.get("frontend_routes", [])) + "},",
            "Modules: []Module{",
        ])
        for module in page.get("modules", []):
            lines.extend([
                "{",
                f"Code: {_go_string(module['code'])},",
                f"Name: {_go_string(module['name'])},",
                f"CopyKey: {_go_string(module['copy_key'])},",
                "Actions: []Action{",
            ])
            for action in module.get("actions", []):
                identifier = _identifier(str(action["code"]))
                lines.extend([
                    "{",
                    f"Code: {identifier},",
                    f"Name: {_go_string(action['name'])},",
                    f"CopyKey: {_go_string(action['copy_key'])},",
                    "Routes: []RouteBinding{",
                ])
                for route in action.get("routes", []):
                    lines.append(
                        "{HTTPMethod: " + _go_string(str(route["method"]).upper())
                        + ", RouteTemplate: " + _go_string(route["path"]) + "},"
                    )
                lines.extend([
                    "},",
                    f"FrontendOnlyReason: {_go_string(action.get('frontend_only_reason', ''))},",
                    # Python 3.9 兼容：f-string 内不再嵌套同类引号（PEP 701 之前的
                    # 版本会把内层 " 当成字符串终点），先算好值再插值。
                    "ObjectPolicyCode: %s," % (
                        _object_policy_identifier(str(action["object_policy"]))
                        if action.get("object_policy") else '""'
                    ),
                    "},",
                ])
            lines.extend(["},", "},"])
        lines.extend(["},", "},"])
    lines.extend(["}", "", "var permissionViewDefinitionsByPage = map[string][]PermissionViewDefinition{"])
    def append_permission_view_definition(view: dict) -> None:
        lines.extend([
            "{",
            f"Code: {_go_string(view['code'])},",
            f"Name: {_go_string(view['name'])},",
            f"CopyKey: {_go_string(view['copy_key'])},",
            f"Kind: {_go_string(view['kind'])},",
            "Components: []string{" + ", ".join(
                _go_string(item) for item in view.get("components", [])
            ) + "},",
            "ActionCodes: []ActionCode{" + ", ".join(
                _identifier(str(action_code)) for action_code in view.get("actions", [])
            ) + "},",
            "Children: []PermissionViewDefinition{",
        ])
        for child in view.get("children", []):
            append_permission_view_definition(child)
        lines.extend(["},", "},"])

    for page_code, views in permission_views.items():
        lines.append(_go_string(page_code) + ": {")
        for view in views:
            append_permission_view_definition(view)
        lines.append("},")
    lines.extend(["}", "", "var actionCodes = []ActionCode{"])
    for action in actions:
        lines.append(f"{_identifier(str(action['code']))},")
    lines.extend(["}", "", "var actionImplications = map[ActionCode][]ActionCode{"])
    for source, targets in action_implications.items():
        lines.append(
            f"{_identifier(str(source))}: {{"
            + ", ".join(_identifier(str(target)) for target in targets)
            + "},"
        )
    lines.extend(["}", "", "var actionRouteRecords = []ActionRouteRecord{"])
    for action in actions:
        for route in action.get("routes", []):
            lines.append(
                "{ActionCode: " + _identifier(str(action["code"]))
                + ", InterfaceCode: " + _go_string(_interface_code(route["method"], route["path"]))
                + ", ActionName: " + _go_string(action["name"])
                + ", CopyKey: " + _go_string(action["copy_key"])
                + ", HTTPMethod: " + _go_string(str(route["method"]).upper())
                + ", RouteTemplate: " + _go_string(route["path"]) + "},"
            )
    lines.extend(["}", "", "var baselineRoutes = []BaselineRoute{"])
    for route in fixed_baseline_routes:
        lines.append(
            "{Capability: " + _go_string(route["capability"])
            + ", AccessStage: " + _go_string(route["access_stage"])
            + ", HTTPMethod: " + _go_string(str(route["method"]).upper())
            + ", RouteTemplate: " + _go_string(route["path"])
            + ", RelatedToolNames: []string{"
            + ", ".join(_go_string(item) for item in route.get("related_tools", []))
            + "}},"
        )
    lines.extend(["}", "", "var operationBindings = []OperationBinding{"])
    generated_binding_codes: set[str] = set()
    for owner, domain_actions in operations_by_domain.items():
        for action in domain_actions:
            code = str(action["code"])
            tools = tool_bindings.get(code, [])
            generated_binding_codes.add(code)
            lines.append(
                "{ActionCode: " + _identifier(code)
                + ", FrontendOwner: " + _go_string(owner)
                + ", ToolNames: []string{" + ", ".join(_go_string(item) for item in tools) + "}},"
            )
    for code, tools in tool_bindings.items():
        if code in generated_binding_codes:
            continue
        lines.append(
            "{ActionCode: " + _identifier(str(code))
            + ", FrontendOwner: " + _go_string(action_owner.get(str(code), ""))
            + ", ToolNames: []string{" + ", ".join(_go_string(item) for item in tools) + "}},"
        )
    lines.extend([
        "}",
        "",
        "var actionByRoute = func() map[string]ActionCode {",
        "result := make(map[string]ActionCode, len(actionRouteRecords))",
        "for _, record := range actionRouteRecords {",
        'result[record.HTTPMethod+"\\x00"+record.RouteTemplate] = record.ActionCode',
        "}",
        "return result",
        "}()",
        "",
        "var baselineByRoute = func() map[string]BaselineRoute {",
        "result := make(map[string]BaselineRoute, len(baselineRoutes))",
        "for _, route := range baselineRoutes {",
        'result[route.HTTPMethod+"\\x00"+route.RouteTemplate] = route',
        "}",
        "return result",
        "}()",
        "",
        "var actionByCode = func() map[ActionCode]Action {",
        "result := make(map[ActionCode]Action, len(actionCodes))",
        "for _, page := range catalogPages {",
        "for _, module := range page.Modules {",
        "for _, action := range module.Actions {",
        "result[action.Code] = action",
        "}",
        "}",
        "}",
        "return result",
        "}()",
        "",
        "func Tree() []Page {",
        "result := make([]Page, len(catalogPages))",
        "for pageIndex, page := range catalogPages {",
        "result[pageIndex] = page",
        "result[pageIndex].FrontendRoutes = append([]string(nil), page.FrontendRoutes...)",
        "result[pageIndex].Modules = make([]Module, len(page.Modules))",
        "for moduleIndex, module := range page.Modules {",
        "result[pageIndex].Modules[moduleIndex] = module",
        "result[pageIndex].Modules[moduleIndex].Actions = make([]Action, len(module.Actions))",
        "for actionIndex, action := range module.Actions {",
        "result[pageIndex].Modules[moduleIndex].Actions[actionIndex] = action",
        "result[pageIndex].Modules[moduleIndex].Actions[actionIndex].Routes = append([]RouteBinding(nil), action.Routes...)",
        "}",
        "}",
        "}",
        "return result",
        "}",
        "",
        "func materializePermissionViews(definitions []PermissionViewDefinition) []PermissionView {",
        "result := make([]PermissionView, len(definitions))",
        "for viewIndex, definition := range definitions {",
        "view := PermissionView{",
        "Code: definition.Code, Name: definition.Name, CopyKey: definition.CopyKey, Kind: definition.Kind,",
        "Components: append([]string(nil), definition.Components...),",
        "Actions: make([]Action, 0, len(definition.ActionCodes)),",
        "Children: materializePermissionViews(definition.Children),",
        "}",
        "for _, actionCode := range definition.ActionCodes {",
        "if action, ok := actionByCode[actionCode]; ok {",
        "action.Routes = append([]RouteBinding(nil), action.Routes...)",
        "view.Actions = append(view.Actions, action)",
        "}",
        "}",
        "result[viewIndex] = view",
        "}",
        "return result",
        "}",
        "",
        "func PermissionTree() []PermissionPage {",
        "pages := Tree()",
        "result := make([]PermissionPage, len(pages))",
        "for pageIndex, page := range pages {",
        "result[pageIndex] = PermissionPage{",
        "Code: page.Code, Name: page.Name, CopyKey: page.CopyKey,",
        "FrontendRoutes: append([]string(nil), page.FrontendRoutes...),",
        "}",
        "definitions, configured := permissionViewDefinitionsByPage[page.Code]",
        "if configured {",
        "result[pageIndex].Views = materializePermissionViews(definitions)",
        "continue",
        "}",
        "result[pageIndex].Views = make([]PermissionView, len(page.Modules))",
        "for moduleIndex, module := range page.Modules {",
        "view := PermissionView{",
        "Code: module.Code, Name: module.Name, CopyKey: module.CopyKey, Kind: \"legacy\",",
        "Actions: make([]Action, len(module.Actions)),",
        "}",
        "for actionIndex, action := range module.Actions {",
        "action.Routes = append([]RouteBinding(nil), action.Routes...)",
        "view.Actions[actionIndex] = action",
        "}",
        "result[pageIndex].Views[moduleIndex] = view",
        "}",
        "}",
        "return result",
        "}",
        "",
        "func ActionCodes() []ActionCode {",
        "return append([]ActionCode(nil), actionCodes...)",
        "}",
        "",
        "// ActionImplies reports whether one granted broad Action includes the requested Action.",
        "// Implications are canonical catalog data and are traversed transitively.",
        "func ActionImplies(granted, requested ActionCode) bool {",
        "if granted == requested {",
        "return true",
        "}",
        "visited := map[ActionCode]bool{granted: true}",
        "pending := append([]ActionCode(nil), actionImplications[granted]...)",
        "for len(pending) > 0 {",
        "candidate := pending[0]",
        "pending = pending[1:]",
        "if candidate == requested {",
        "return true",
        "}",
        "if visited[candidate] {",
        "continue",
        "}",
        "visited[candidate] = true",
        "pending = append(pending, actionImplications[candidate]...)",
        "}",
        "return false",
        "}",
        "",
        "// GrantingActions returns the requested Action followed by broad Actions that imply it.",
        "func GrantingActions(requested ActionCode) []ActionCode {",
        "result := []ActionCode{requested}",
        "for _, candidate := range actionCodes {",
        "if candidate != requested && ActionImplies(candidate, requested) {",
        "result = append(result, candidate)",
        "}",
        "}",
        "return result",
        "}",
        "",
        "func ActionForRoute(method, canonicalRoute string) (ActionCode, bool) {",
        'key := strings.ToUpper(strings.TrimSpace(method)) + "\\x00" + strings.TrimSpace(canonicalRoute)',
        "code, ok := actionByRoute[key]",
        "return code, ok",
        "}",
        "",
        "func AuthorizationRouteForAction(actionCode ActionCode) (RouteBinding, bool) {",
        "action, ok := actionByCode[actionCode]",
        "if !ok || len(action.Routes) == 0 {",
        "return RouteBinding{}, false",
        "}",
        "return action.Routes[0], true",
        "}",
        "",
        "func ObjectPolicyForAction(actionCode ActionCode) (ObjectPolicy, bool) {",
        "action, ok := actionByCode[actionCode]",
        "if !ok || action.ObjectPolicyCode == \"\" {",
        "return ObjectPolicy{}, false",
        "}",
        "policy, ok := objectPolicies[action.ObjectPolicyCode]",
        "if !ok {",
        "return ObjectPolicy{}, false",
        "}",
        "policy.EnforcementMarkers = append([]string(nil), policy.EnforcementMarkers...)",
        "return policy, true",
        "}",
        "",
        "func ObjectPolicyForRoute(method, canonicalRoute string) (ObjectPolicy, bool) {",
        "actionCode, ok := ActionForRoute(method, canonicalRoute)",
        "if !ok {",
        "return ObjectPolicy{}, false",
        "}",
        "return ObjectPolicyForAction(actionCode)",
        "}",
        "",
        "func BootstrapActionRoutes() []ActionRouteRecord {",
        "return append([]ActionRouteRecord(nil), actionRouteRecords...)",
        "}",
        "",
        "func BaselineRoutes() []BaselineRoute {",
        "result := make([]BaselineRoute, len(baselineRoutes))",
        "for index, route := range baselineRoutes {",
        "result[index] = route",
        "result[index].RelatedToolNames = append([]string(nil), route.RelatedToolNames...)",
        "}",
        "return result",
        "}",
        "",
        "func BaselineCapabilityForRoute(method, canonicalRoute string) (BaselineRoute, bool) {",
        'key := strings.ToUpper(strings.TrimSpace(method)) + "\\x00" + strings.TrimSpace(canonicalRoute)',
        "route, ok := baselineByRoute[key]",
        "return route, ok",
        "}",
        "",
        "func OperationBindings() []OperationBinding {",
        "result := make([]OperationBinding, len(operationBindings))",
        "for index, binding := range operationBindings {",
        "result[index] = binding",
        "result[index].ToolNames = append([]string(nil), binding.ToolNames...)",
        "}",
        "return result",
        "}",
        "",
        "func ActionForTool(toolName string) (ActionCode, bool) {",
        "toolName = strings.TrimSpace(toolName)",
        "for _, binding := range operationBindings {",
        "for _, candidate := range binding.ToolNames {",
        "if candidate == toolName {",
        "return binding.ActionCode, true",
        "}",
        "}",
        "}",
        "return \"\", false",
        "}",
        "",
        "func ToolNamesForAction(actionCode ActionCode) []string {",
        "for _, binding := range operationBindings {",
        "if binding.ActionCode == actionCode {",
        "return append([]string(nil), binding.ToolNames...)",
        "}",
        "}",
        "return nil",
        "}",
        "",
    ])
    return _format_go("\n".join(lines))


def render_typescript(catalog: dict, source_label: str = "contracts/access/catalog.json") -> str:
    pages = catalog.get("pages", [])
    actions = [action for page in pages for module in page.get("modules", []) for action in module.get("actions", [])]
    identifiers: dict[str, str] = {}
    for action in actions:
        code = str(action.get("code", ""))
        identifier = _identifier(code).removeprefix("Action")
        if identifier in identifiers:
            raise GenerationError(
                f"TypeScript action identifier collision: {identifiers[identifier]!r} and {code!r} -> {identifier}"
            )
        identifiers[identifier] = code
    canonical = json.dumps(catalog, ensure_ascii=False, sort_keys=True, separators=(",", ":"))
    digest = hashlib.sha256(canonical.encode("utf-8")).hexdigest()
    operations_by_domain = operation_actions_by_domain(catalog)
    governance = catalog.get("operation_governance", {})
    tool_bindings = governance.get("tool_bindings", {})
    action_implications = governance.get("action_implications", {})
    lines = [
        f"// Code generated by scripts/generate-access-catalog.py from {source_label}; DO NOT EDIT.",
        f"// Catalog SHA-256: {digest}",
        "",
        "export const PM_ACTION = {",
    ]
    for identifier, code in identifiers.items():
        lines.append(f"  {identifier}: {_go_string(code)},")
    lines.extend([
        "} as const",
        "",
        "export type PMActionCode = (typeof PM_ACTION)[keyof typeof PM_ACTION]",
        "",
        "export const PM_ACTION_CODES = Object.freeze(",
        "  Object.values(PM_ACTION),",
        ") as readonly PMActionCode[]",
        "",
        "export const PM_ACTION_IMPLICATIONS = {",
    ])
    for source, targets in action_implications.items():
        source_identifier = _identifier(str(source)).removeprefix("Action")
        rendered_targets = ", ".join(
            f"PM_ACTION.{_identifier(str(target)).removeprefix('Action')}"
            for target in targets
        )
        lines.append(f"  [PM_ACTION.{source_identifier}]: [{rendered_targets}],")
    lines.extend([
        "} as const satisfies Partial<Record<PMActionCode, readonly PMActionCode[]>>",
        "",
        "export function pmActionImplies(granted: PMActionCode, requested: PMActionCode): boolean {",
        "  if (granted === requested) return true",
        "  const visited = new Set<PMActionCode>([granted])",
        "  const pending: PMActionCode[] = [...(PM_ACTION_IMPLICATIONS[granted as keyof typeof PM_ACTION_IMPLICATIONS] || [])]",
        "  while (pending.length) {",
        "    const candidate = pending.shift()!",
        "    if (candidate === requested) return true",
        "    if (visited.has(candidate)) continue",
        "    visited.add(candidate)",
        "    pending.push(...(PM_ACTION_IMPLICATIONS[candidate as keyof typeof PM_ACTION_IMPLICATIONS] || []))",
        "  }",
        "  return false",
        "}",
        "",
        "export function pmGrantingActions(requested: PMActionCode): readonly PMActionCode[] {",
        "  return PM_ACTION_CODES.filter((candidate) => pmActionImplies(candidate, requested))",
        "}",
        "",
        "export const PM_PAGE_CODES = [",
    ])
    for page in pages:
        lines.append(f"  {_go_string(page['code'])},")
    lines.extend([
        "] as const",
        "",
        "export type PMPageCode = (typeof PM_PAGE_CODES)[number]",
        "",
        "export const PM_PAGE_BY_ROUTE = {",
    ])
    for page in pages:
        for route in page.get("frontend_routes", []):
            lines.append(f"  {_go_string(route)}: {_go_string(page['code'])},")
    lines.extend([
        "} as const satisfies Readonly<Record<string, PMPageCode>>",
        "",
        "export const PM_OPERATION_ACTIONS_BY_DOMAIN = {",
    ])
    for owner, domain_actions in operations_by_domain.items():
        lines.append(f"  {_go_string(owner)}: [")
        for action in domain_actions:
            lines.append(f"    PM_ACTION.{_identifier(str(action['code'])).removeprefix('Action')},")
        lines.append("  ],")
    lines.extend([
        "} as const",
        "",
        "export type PMOperationDomain = keyof typeof PM_OPERATION_ACTIONS_BY_DOMAIN",
        "",
        "export type PMOperationActionCode<D extends PMOperationDomain = PMOperationDomain> =",
        "  (typeof PM_OPERATION_ACTIONS_BY_DOMAIN)[D][number]",
        "",
        "export const PM_TOOL_NAMES_BY_ACTION = {",
    ])
    for code, tools in tool_bindings.items():
        identifier = _identifier(str(code)).removeprefix("Action")
        rendered_tools = ", ".join(_go_string(item) for item in tools)
        lines.append(f"  [PM_ACTION.{identifier}]: [{rendered_tools}],")
    lines.extend([
        "} as const satisfies Partial<Record<PMActionCode, readonly string[]>>",
        "",
    ])
    return "\n".join(lines)


def render_operation_matrix(
    catalog: dict,
    source_label: str = "contracts/access/catalog.json",
) -> str:
    governance = catalog.get("operation_governance", {})
    operations_by_domain = operation_actions_by_domain(catalog)
    tool_bindings = governance.get("tool_bindings", {})
    tool_not_applicable = governance.get("tool_not_applicable", {})
    decision_reasons = governance.get("tool_decision_reasons", {})
    button_locations = governance.get("button_locations", {})
    surfaces_by_action: dict[str, dict] = {}
    for surface in governance.get("frontend_surfaces", []):
        for action_code in surface.get("actions", []):
            surfaces_by_action[str(action_code)] = surface

    placements = permission_view_placements(catalog)
    placements_by_action: dict[str, list[dict]] = {}
    for placement in placements:
        action_code = str(placement.get("action", {}).get("code", ""))
        placements_by_action.setdefault(action_code, []).append(placement)

    lines = [
        f"<!-- 由 scripts/generate-access-catalog.py 根据 {source_label} 生成；请勿手工编辑。 -->",
        "# PM 业务动作映射表",
        "",
        "本表把业务按钮/交互位置、组件入口、集中 capability owner、权限 action、后端 HTTP 接口和可选 Tool 关联在一起。",
        "`ui` 表示当前存在用户交互组件；`api_only` 表示当前只有接口或 mutation owner，没有独立按钮。",
        "按钮显隐、禁用与提交守卫必须由 capability owner 统一组合全局权限、对象能力和业务状态。",
        "`已接入` 才表示前端已消费 Action；`待接入` 只表示映射完成，不得宣称按钮权限已生效。",
        "",
        "## 系统固定基础能力",
        "",
        "基础能力不进入角色/用户组的可编辑授权集合；仍必须通过对应的会话、账号准入或工作区边界。",
        "",
        "| 能力 | 前置阶段 | 前端调用入口 | HTTP 接口 | 相关 Tool（独立授权） | 审核理由 |",
        "| --- | --- | --- | --- | --- | --- |",
    ]
    for route in baseline_routes(catalog):
        frontend = "<br>".join(
            f"`{item}`" for item in route.get("frontend_surfaces", [])
        ) or "后端/协议自用"
        related_tools = "<br>".join(
            f"`{item}`" for item in route.get("related_tools", [])
        ) or "—"
        lines.append(
            f"| `{route.get('capability', '')}` | `{route.get('access_stage', '')}` | "
            f"{frontend} | `{str(route.get('method', '')).upper()} {route.get('path', '')}` | "
            f"{related_tools} | "
            f"{route.get('reason', '')} |"
        )
    lines.extend([
        "",
        "## 前端视图 → Action → 接口",
        "",
        "视图允许递归嵌套；同一 Action 可以被多个视图引用，但授权只保存一份 Action code。",
        "`capability` 节点表示没有当前前端入口的接口或 Tool 能力，不得伪造成按钮。",
        "",
        "| 页面 | 视图路径 | 类型 | 组件 | Action | HTTP 接口 |",
        "| --- | --- | --- | --- | --- | --- |",
    ])
    for placement in placements:
        action = placement.get("action", {})
        routes = "<br>".join(
            f"`{str(route.get('method', '')).upper()} {route.get('path', '')}`"
            for route in action.get("routes", [])
        ) or "无独立接口"
        components = "<br>".join(f"`{item}`" for item in placement.get("components", [])) or "—"
        lines.append(
            f"| {placement.get('page_name', '')} | {placement.get('view_path', '')} | "
            f"`{placement.get('kind', '')}` | {components} | "
            f"`{action.get('code', '')}` | {routes} |"
        )
    lines.extend([
        "",
        "## 业务动作执行映射",
        "",
        "| 领域 | 前端视图 | Action | UI 状态 | 权限接入 | 按钮位置 | 主要组件/交互入口 | Capability owner | HTTP 接口 | Tool 结论 |",
        "| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |",
    ])
    for owner, actions in operations_by_domain.items():
        for action in actions:
            action_code = str(action["code"])
            surface = surfaces_by_action.get(action_code, {})
            routes = "<br>".join(
                f"`{str(route.get('method', '')).upper()} {route.get('path', '')}`"
                for route in action.get("routes", [])
            ) or "无独立接口"
            tools = tool_bindings.get(action_code, [])
            if tools:
                tool_decision = "<br>".join(f"`{tool}`" for tool in tools)
            else:
                reason_code = str(tool_not_applicable.get(action_code, ""))
                explanation = str(decision_reasons.get(reason_code, ""))
                tool_decision = f"`not_applicable:{reason_code}`<br>{explanation}"
            locations = button_locations.get(action_code, [])
            button_location = "<br>".join(str(location) for location in locations) or "无按钮（仅接口）"
            view_paths = "<br>".join(
                f"{placement.get('page_name', '')} > {placement.get('view_path', '')}"
                for placement in placements_by_action.get(action_code, [])
            ) or "未归属"
            enforcement = "已接入" if surface.get("permission_enforcement") == "implemented" else (
                "后端强制" if surface.get("status") == "api_only" else "待接入"
            )
            surface_components = surface.get("components")
            if not isinstance(surface_components, list):
                surface_components = [surface.get("component", "")]
            component_entry = "<br>".join(
                f"`{component}`" for component in surface_components if component
            ) or "—"
            lines.append(
                f"| `{owner}` | {view_paths} | `{action_code}` | `{surface.get('status', '')}` | "
                f"{enforcement} | {button_location} | {component_entry} | "
                f"`{surface.get('capability_owner', '')}` | "
                f"{routes} | {tool_decision} |"
            )
    lines.append("")
    return "\n".join(lines)


def generated_output_path(catalog: dict, root: Path) -> Path:
    relative = str(catalog.get("generation", {}).get("go_output", "")).strip()
    if not relative:
        raise GenerationError("generation.go_output is required")
    return root / relative


def generated_typescript_output_path(catalog: dict, root: Path) -> Path:
    relative = str(catalog.get("generation", {}).get("typescript_output", "")).strip()
    if not relative:
        raise GenerationError("generation.typescript_output is required")
    return root / relative


def generated_operation_matrix_output_path(catalog: dict, root: Path) -> Path:
    relative = str(catalog.get("generation", {}).get("operation_matrix_output", "")).strip()
    if not relative:
        raise GenerationError("generation.operation_matrix_output is required")
    return root / relative
