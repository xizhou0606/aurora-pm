#!/usr/bin/env python3
"""Validate PM's canonical page/module/action permission catalog.

The catalog deliberately remains independent of runtime code generation. This
validator extracts the Gin registration facts from their owning functions and
checks that every canonical protected route has exactly one action owner or one
reviewed exemption.
"""

from __future__ import annotations

import json
import re
from dataclasses import dataclass
from pathlib import Path
from typing import Iterable

from access_catalog_generator import (
    GenerationError,
    baseline_routes,
    generated_operation_matrix_output_path,
    generated_output_path,
    generated_typescript_output_path,
    operation_actions_by_domain,
    render_go,
    render_operation_matrix,
    render_typescript,
)


HTTP_METHODS = {"GET", "POST", "PUT", "PATCH", "DELETE", "HEAD"}
CODE_RE = re.compile(r"^[a-z][a-z0-9]*(?:[.-][a-z0-9]+)*$")
COPY_KEY_RE = re.compile(r"^access\.(pages|modules|views|actions)\.[A-Za-z0-9_.-]+$")
ROUTE_CALL_RE = re.compile(
    r"(?P<receiver>[A-Za-z_][A-Za-z0-9_]*)\.(?P<method>GET|POST|PUT|PATCH|DELETE|HEAD)"
    r"\(\s*\"(?P<path>[^\"]*)\""
)
ROUTE_HANDLER_CALL_RE = re.compile(
    r"(?P<receiver>[A-Za-z_][A-Za-z0-9_]*)\.(?P<method>GET|POST|PUT|PATCH|DELETE|HEAD)"
    r"\(\s*\"(?P<path>[^\"]*)\"\s*,\s*"
    r"(?:(?P<qualifier>[A-Za-z_][A-Za-z0-9_]*)\.)?(?P<handler>[A-Za-z_][A-Za-z0-9_]*)"
)
GROUP_RE = re.compile(
    r"(?P<child>[A-Za-z_][A-Za-z0-9_]*)\s*:=\s*"
    r"(?P<parent>[A-Za-z_][A-Za-z0-9_]*)\.Group\(\s*\"(?P<path>[^\"]*)\"\s*\)"
)
FUNC_RE = re.compile(r"^func\s+(?:\([^)]*\)\s*)?(?P<name>[A-Za-z_][A-Za-z0-9_]*)\s*\(")
ANY_ROUTE_RE = re.compile(r"\.(GET|POST|PUT|PATCH|DELETE|HEAD)\(\s*\"[^\"]*\"")
STRING_LITERAL_RE = re.compile(r"['\"](?P<value>pm\.[a-z0-9.-]+)['\"]")
FRONTEND_ROUTE_RE = re.compile(r"^\s*path:\s*['\"](?P<path>/[^'\"]*)['\"],?\s*$", re.MULTILINE)
TOOL_NAME_RE = re.compile(r"^[a-z][a-z0-9_-]*(?:\.[a-z][a-z0-9_-]*)+$")
DECISION_REASON_RE = re.compile(r"^[a-z][a-z0-9_]*$")
PM_TOOL_SPEC_RE = re.compile(
    r'pmSpec\(\s*(?:"(?P<literal>[^"]+)"|(?P<constant>[A-Za-z_][A-Za-z0-9_]*))'
)
PM_TOOL_CONTRACT_RE = re.compile(
    r'pmSpec\(\s*(?:"(?P<literal>[^"]+)"|(?P<constant>[A-Za-z_][A-Za-z0-9_]*))'
    r'\s*,\s*[^,\n]+\s*,\s*"(?P<method>GET|POST|PUT|PATCH|DELETE|HEAD)"'
    r'\s*,\s*"(?P<path>[^"]+)"'
)
GO_STRING_CONST_RE = re.compile(
    r'^\s*(?:const\s+)?(?P<name>[A-Za-z_][A-Za-z0-9_]*)\s*(?:string\s*)?=\s*"(?P<value>[^"]+)"',
    re.MULTILINE,
)
CJK_RE = re.compile(r"[\u3400-\u9fff]")


def _typescript_action_member(code: str) -> str:
    words = re.findall(r"[a-zA-Z0-9]+", code.removeprefix("pm."))
    return "".join(word[:1].upper() + word[1:] for word in words)


@dataclass(frozen=True, order=True)
class Route:
    method: str
    path: str

    @classmethod
    def from_mapping(cls, value: dict) -> "Route":
        return cls(str(value.get("method", "")).upper(), str(value.get("path", "")))

    def display(self) -> str:
        return f"{self.method} {self.path}"


class CatalogError(Exception):
    """Raised for a malformed catalog or repository route inventory."""


def load_json(path: Path) -> dict:
    try:
        return json.loads(path.read_text(encoding="utf-8"))
    except (OSError, json.JSONDecodeError) as exc:
        raise CatalogError(f"cannot read catalog {path}: {exc}") from exc


def extract_pm_tool_names(source: str, source_name: str) -> tuple[set[str], list[str]]:
    """Extract canonical Tool names from pmSpec calls, including string constants."""
    constants = {
        match.group("name"): match.group("value")
        for match in GO_STRING_CONST_RE.finditer(source)
    }
    names: set[str] = set()
    errors: list[str] = []
    for match in PM_TOOL_SPEC_RE.finditer(source):
        name = match.group("literal")
        constant = match.group("constant")
        if name is None:
            name = constants.get(constant or "")
            if name is None:
                errors.append(
                    f"operation Tool source {source_name}: unresolved pmSpec Tool-name constant "
                    f"{constant!r}"
                )
                continue
        names.add(name)
    return names, errors


def extract_pm_tool_contracts(source: str, source_name: str) -> tuple[dict[str, Route], list[str]]:
    """Extract the canonical HTTP method/route shape declared by full pmSpec calls."""
    constants = {
        match.group("name"): match.group("value")
        for match in GO_STRING_CONST_RE.finditer(source)
    }
    contracts: dict[str, Route] = {}
    errors: list[str] = []
    for match in PM_TOOL_CONTRACT_RE.finditer(source):
        name = match.group("literal")
        constant = match.group("constant")
        if name is None:
            name = constants.get(constant or "")
            if name is None:
                errors.append(
                    f"operation Tool source {source_name}: unresolved pmSpec Tool-name constant "
                    f"{constant!r}"
                )
                continue
        route = Route(match.group("method"), match.group("path"))
        previous = contracts.get(name)
        if previous is not None and previous != route:
            errors.append(
                f"operation Tool source {source_name}: duplicate Tool contract {name} "
                f"({previous.display()} and {route.display()})"
            )
            continue
        contracts[name] = route
    return contracts, errors


def _normalized_tool_route_shape(path: str) -> str:
    normalized = str(path).strip()
    normalized = re.sub(r"^/?api/v1/(?:platform|core)/", "", normalized)
    segments = []
    for segment in normalized.strip("/").split("/"):
        if re.fullmatch(r"(?::[A-Za-z_][A-Za-z0-9_]*|\{[A-Za-z_][A-Za-z0-9_]*\})", segment):
            segments.append("{param}")
        else:
            segments.append(segment)
    return "/".join(segments)


def _function_body(source: str, function_name: str) -> str:
    lines = source.splitlines()
    start = None
    for index, line in enumerate(lines):
        match = FUNC_RE.match(line.strip())
        if match and match.group("name") == function_name:
            start = index
            break
    if start is None:
        raise CatalogError(f"route source function not found: {function_name}")

    depth = 0
    began = False
    result: list[str] = []
    for line in lines[start:]:
        if "{" in line:
            began = True
        if began:
            result.append(line)
            depth += line.count("{") - line.count("}")
            if depth == 0:
                return "\n".join(result)
    raise CatalogError(f"unterminated route source function: {function_name}")


def _join_route(prefix: str, suffix: str) -> str:
    if not prefix:
        prefix = "/"
    if not suffix:
        return prefix.rstrip("/") or "/"
    return f"{prefix.rstrip('/')}/{suffix.lstrip('/')}"


def extract_routes(root: Path, sources: Iterable[dict]) -> tuple[set[Route], set[tuple[str, str]]]:
    routes: set[Route] = set()
    tracked_calls: set[tuple[str, str]] = set()
    for source_spec in sources:
        relative = str(source_spec.get("file", ""))
        function = str(source_spec.get("function", ""))
        source_path = root / relative
        try:
            source = source_path.read_text(encoding="utf-8")
        except OSError as exc:
            raise CatalogError(f"cannot read route source {relative}: {exc}") from exc
        body = _function_body(source, function)
        prefixes = {str(key): str(value) for key, value in source_spec.get("receivers", {}).items()}
        for match in GROUP_RE.finditer(body):
            parent = match.group("parent")
            if parent in prefixes:
                prefixes[match.group("child")] = _join_route(prefixes[parent], match.group("path"))
        calls = list(ROUTE_CALL_RE.finditer(body))
        for call in calls:
            receiver = call.group("receiver")
            if receiver not in prefixes:
                raise CatalogError(
                    f"unresolved route receiver {receiver} in {relative}:{function}; "
                    "declare its canonical prefix in route_inventory.sources.receivers"
                )
            route = Route(call.group("method"), _join_route(prefixes[receiver], call.group("path")))
            if route in routes:
                raise CatalogError(f"duplicate canonical Gin route extracted: {route.display()}")
            routes.add(route)
            tracked_calls.add((relative, call.group(0)))
    return routes, tracked_calls


def extract_route_handlers(root: Path, sources: Iterable[dict]) -> dict[Route, tuple[str, str]]:
    """Resolve canonical Gin routes to their directly registered handler names."""
    handlers: dict[Route, tuple[str, str]] = {}
    for source_spec in sources:
        relative = str(source_spec.get("file", ""))
        function = str(source_spec.get("function", ""))
        source_path = root / relative
        try:
            source = source_path.read_text(encoding="utf-8")
        except OSError as exc:
            raise CatalogError(f"cannot read route source {relative}: {exc}") from exc
        body = _function_body(source, function)
        prefixes = {str(key): str(value) for key, value in source_spec.get("receivers", {}).items()}
        for match in GROUP_RE.finditer(body):
            parent = match.group("parent")
            if parent in prefixes:
                prefixes[match.group("child")] = _join_route(prefixes[parent], match.group("path"))
        for call in ROUTE_HANDLER_CALL_RE.finditer(body):
            receiver = call.group("receiver")
            if receiver not in prefixes:
                continue
            route = Route(call.group("method"), _join_route(prefixes[receiver], call.group("path")))
            handlers[route] = (relative, call.group("handler"))
    return handlers


def _go_function_source(root: Path, function_name: str) -> tuple[str, str] | None:
    for path in sorted((root / "backend").rglob("*.go")):
        if path.name.endswith("_test.go"):
            continue
        source = path.read_text(encoding="utf-8")
        try:
            body = _function_body(source, function_name)
        except CatalogError:
            continue
        return path.relative_to(root).as_posix(), body
    return None


def find_untracked_route_registrations(root: Path, sources: Iterable[dict]) -> list[str]:
    configured = {(str(item["file"]), str(item["function"])) for item in sources}
    errors: list[str] = []
    backend = root / "backend"
    if not backend.exists():
        return errors
    for path in backend.rglob("*.go"):
        if path.name.endswith("_test.go"):
            continue
        relative = path.relative_to(root).as_posix()
        source = path.read_text(encoding="utf-8")
        lines = source.splitlines()
        current = ""
        for number, line in enumerate(lines, 1):
            match = FUNC_RE.match(line.strip())
            if match:
                current = match.group("name")
            if ANY_ROUTE_RE.search(line) and (relative, current) not in configured:
                errors.append(f"untracked Gin registration {relative}:{number} ({current})")
    return errors


def _validate_route(route: Route, context: str, errors: list[str]) -> None:
    if route.method not in HTTP_METHODS:
        errors.append(f"{context}: invalid HTTP method {route.method!r}")
    if not route.path.startswith("/") or "//" in route.path:
        errors.append(f"{context}: route must be a canonical absolute template: {route.path!r}")
    if route.path.startswith("/core") or route.path.startswith("/platform"):
        errors.append(f"{context}: compatibility aliases cannot be catalog bindings: {route.path}")


def _validate_operation_governance(catalog: dict, root: Path, page_codes: set[str], errors: list[str]) -> None:
    governance = catalog.get("operation_governance")
    if not isinstance(governance, dict):
        errors.append("operation_governance is required")
        return

    domains = governance.get("domains")
    if not isinstance(domains, list):
        errors.append("operation_governance.domains must be a list")
        return

    owner_pages: dict[str, str] = {}
    for index, domain in enumerate(domains):
        context = f"operation_governance.domains[{index}]"
        if not isinstance(domain, dict):
            errors.append(f"{context}: domain must be an object")
            continue
        owner = str(domain.get("owner", ""))
        if not CODE_RE.fullmatch(owner):
            errors.append(f"{context}: invalid frontend owner {owner!r}")
        raw_page_codes = domain.get("page_codes")
        if not isinstance(raw_page_codes, list) or not raw_page_codes:
            errors.append(f"{context}: page_codes must be a non-empty list")
            continue
        for raw_page_code in raw_page_codes:
            page_code = str(raw_page_code)
            if page_code not in page_codes:
                errors.append(f"{context}: unknown permission page {page_code!r}")
            elif page_code in owner_pages:
                errors.append(
                    f"operation permission page has multiple frontend owners: {page_code} "
                    f"({owner_pages[page_code]} and {owner})"
                )
            else:
                owner_pages[page_code] = owner

    pages_by_code = {
        str(page.get("code", "")): page
        for page in catalog.get("pages", [])
        if isinstance(page, dict)
    }
    action_page: dict[str, str] = {}
    action_routes: dict[str, list[Route]] = {}
    action_object_policies: dict[str, str] = {}
    for page_code, page in pages_by_code.items():
        for module in page.get("modules", []):
            for action in module.get("actions", []):
                action_code = str(action.get("code", ""))
                action_page[action_code] = page_code
                raw_routes = action.get("routes", [])
                action_routes[action_code] = [
                    Route.from_mapping(route)
                    for route in raw_routes
                    if isinstance(route, dict)
                ]
                policy_code = str(action.get("object_policy", "")).strip()
                if policy_code:
                    action_object_policies[action_code] = policy_code

    raw_action_implications = governance.get("action_implications", {})
    if not isinstance(raw_action_implications, dict):
        errors.append("operation_governance.action_implications must be an object")
        raw_action_implications = {}
    action_implications: dict[str, list[str]] = {}
    for raw_source, raw_targets in raw_action_implications.items():
        source = str(raw_source).strip()
        context = f"operation_governance.action_implications[{source!r}]"
        if source not in action_page:
            errors.append(f"{context}: unknown source action")
        if not isinstance(raw_targets, list) or not raw_targets:
            errors.append(f"{context}: targets must be a non-empty list")
            continue
        targets = [str(target).strip() for target in raw_targets]
        if len(set(targets)) != len(targets):
            errors.append(f"{context}: duplicate target action")
        for target in targets:
            if target not in action_page:
                errors.append(f"{context}: unknown target action {target!r}")
            if target == source:
                errors.append(f"{context}: action cannot imply itself")
        action_implications[source] = targets

    def implication_reaches(source: str, target: str, visiting: set[str]) -> bool:
        if source in visiting:
            return False
        visiting.add(source)
        for candidate in action_implications.get(source, []):
            if candidate == target or implication_reaches(candidate, target, visiting):
                return True
        visiting.remove(source)
        return False

    for source in action_implications:
        if any(
            implication_reaches(target, source, set())
            for target in action_implications.get(source, [])
        ):
            errors.append(
                f"operation_governance.action_implications contains a cycle through {source!r}"
            )

    raw_object_policies = governance.get("object_policies", {})
    if not isinstance(raw_object_policies, dict):
        errors.append("operation_governance.object_policies must be an object")
        raw_object_policies = {}
    object_policies: dict[str, dict] = {}
    for raw_code, raw_policy in raw_object_policies.items():
        code = str(raw_code).strip()
        context = f"operation_governance.object_policies[{code!r}]"
        if not CODE_RE.fullmatch(code):
            errors.append(f"{context}: invalid object policy code")
        if not isinstance(raw_policy, dict):
            errors.append(f"{context}: policy must be an object")
            continue
        kind = str(raw_policy.get("kind", "")).strip()
        if kind not in {"creator_override", "resource_access"}:
            errors.append(f"{context}: unsupported object policy kind {kind!r}")
        markers = raw_policy.get("enforcement_markers")
        if not isinstance(markers, list) or not markers:
            errors.append(f"{context}: enforcement_markers must be a non-empty list")
            markers = []
        for marker in markers:
            if not re.fullmatch(r"[A-Za-z_][A-Za-z0-9_]*", str(marker)):
                errors.append(f"{context}: invalid enforcement marker {marker!r}")
        object_policies[code] = raw_policy

    used_object_policies = set(action_object_policies.values())
    for action_code, policy_code in action_object_policies.items():
        if policy_code not in object_policies:
            errors.append(
                f"permission action {action_code} references unknown object policy {policy_code!r}"
            )
    for policy_code in sorted(set(object_policies) - used_object_policies):
        errors.append(f"object policy is not attached to any permission action: {policy_code}")

    if action_object_policies:
        try:
            route_handlers = extract_route_handlers(
                root, catalog.get("route_inventory", {}).get("sources", [])
            )
        except CatalogError as exc:
            errors.append(str(exc))
            route_handlers = {}
        function_cache: dict[str, tuple[str, str] | None] = {}
        for action_code, policy_code in action_object_policies.items():
            policy = object_policies.get(policy_code)
            if policy is None:
                continue
            markers = [str(item) for item in policy.get("enforcement_markers", [])]
            for route in action_routes.get(action_code, []):
                registration = route_handlers.get(route)
                if registration is None:
                    errors.append(
                        f"object-policy action {action_code} route has no resolvable Gin handler: "
                        f"{route.display()}"
                    )
                    continue
                _, handler_name = registration
                if handler_name not in function_cache:
                    function_cache[handler_name] = _go_function_source(root, handler_name)
                handler_source = function_cache[handler_name]
                if handler_source is None:
                    errors.append(
                        f"object-policy action {action_code} handler source is missing: {handler_name}"
                    )
                    continue
                handler_file, handler_body = handler_source
                if not any(re.search(rf"\b{re.escape(marker)}\s*\(", handler_body) for marker in markers):
                    errors.append(
                        f"object-policy action {action_code} route {route.display()} handler "
                        f"{handler_file}:{handler_name} does not call a registered enforcement marker"
                    )

    permission_views = governance.get("permission_views")
    if not isinstance(permission_views, list):
        if owner_pages:
            errors.append("operation_governance.permission_views must be a list")
        permission_views = []
    configured_pages: set[str] = set()
    view_codes: set[str] = set()
    view_kinds_by_action: dict[str, set[str]] = {}

    def validate_views(raw_views: object, page_code: str, context: str) -> None:
        if not isinstance(raw_views, list) or not raw_views:
            errors.append(f"{context}: views must be a non-empty list")
            return
        for view_index, view in enumerate(raw_views):
            view_context = f"{context}[{view_index}]"
            if not isinstance(view, dict):
                errors.append(f"{view_context}: view must be an object")
                continue
            view_code = str(view.get("code", ""))
            name = str(view.get("name", "")).strip()
            copy_key = str(view.get("copy_key", ""))
            kind = str(view.get("kind", ""))
            if not CODE_RE.fullmatch(view_code):
                errors.append(f"{view_context}: invalid view code {view_code!r}")
            elif view_code in view_codes:
                errors.append(f"{view_context}: duplicate view code {view_code!r}")
            view_codes.add(view_code)
            if not name or not CJK_RE.search(name):
                errors.append(f"{view_context}: view name must be a non-empty Chinese label")
            if not COPY_KEY_RE.fullmatch(copy_key) or not copy_key.startswith("access.views."):
                errors.append(f"{view_context}: invalid view copy_key {copy_key!r}")
            if kind not in {"ui", "capability"}:
                errors.append(f"{view_context}: kind must be ui or capability")
            components = view.get("components")
            if not isinstance(components, list):
                errors.append(f"{view_context}: components must be a list")
                components = []
            if kind == "ui" and not components:
                errors.append(f"{view_context}: ui view requires at least one component")
            for raw_component in components:
                component = str(raw_component).strip()
                if not component or not (root / component).is_file():
                    errors.append(f"{view_context}: component file is missing: {component or '<unset>'}")
            raw_actions = view.get("actions", [])
            children = view.get("children", [])
            if not isinstance(raw_actions, list):
                errors.append(f"{view_context}: actions must be a list")
                raw_actions = []
            if not isinstance(children, list):
                errors.append(f"{view_context}: children must be a list")
                children = []
            if not raw_actions and not children:
                errors.append(f"{view_context}: view requires actions or child views")
            if kind == "capability" and children:
                errors.append(f"{view_context}: capability view cannot contain child views")
            seen_actions: set[str] = set()
            for raw_action_code in raw_actions:
                action_code = str(raw_action_code)
                if action_code in seen_actions:
                    errors.append(f"{view_context}: duplicate action {action_code!r}")
                seen_actions.add(action_code)
                if action_code not in action_page:
                    errors.append(f"{view_context}: unknown action {action_code!r}")
                elif action_page[action_code] != page_code:
                    errors.append(
                        f"{view_context}: action {action_code!r} belongs to {action_page[action_code]!r}"
                    )
                view_kinds_by_action.setdefault(action_code, set()).add(kind)
            if children:
                validate_views(children, page_code, f"{view_context}.children")

    for page_index, page_views in enumerate(permission_views):
        context = f"operation_governance.permission_views[{page_index}]"
        if not isinstance(page_views, dict):
            errors.append(f"{context}: page view mapping must be an object")
            continue
        page_code = str(page_views.get("page_code", ""))
        if page_code not in owner_pages:
            errors.append(f"{context}: page_code must reference a governed page: {page_code!r}")
        if page_code in configured_pages:
            errors.append(f"{context}: duplicate permission view page {page_code!r}")
        configured_pages.add(page_code)
        validate_views(page_views.get("views"), page_code, f"{context}.views")

    for page_code in sorted(set(owner_pages) - configured_pages):
        errors.append(f"governed permission page has no frontend view mapping: {page_code}")
    governed_tree_actions = {
        action_code for action_code, page_code in action_page.items() if page_code in owner_pages
    }
    for action_code in sorted(governed_tree_actions - set(view_kinds_by_action)):
        errors.append(f"permission action has no frontend view/capability placement: {action_code}")

    operations_by_domain = operation_actions_by_domain(catalog)
    governed_actions = {
        str(action.get("code", ""))
        for actions in operations_by_domain.values()
        for action in actions
    }
    surfaces = governance.get("frontend_surfaces")
    if not isinstance(surfaces, list):
        errors.append("operation_governance.frontend_surfaces must be a list")
        surfaces = []
    surface_owners: dict[str, str] = {}
    surface_statuses: dict[str, str] = {}
    for index, surface in enumerate(surfaces):
        context = f"operation_governance.frontend_surfaces[{index}]"
        if not isinstance(surface, dict):
            errors.append(f"{context}: surface must be an object")
            continue
        component = str(surface.get("component", "")).strip()
        raw_components = surface.get("components")
        if raw_components is None:
            components = [component] if component else []
        elif not isinstance(raw_components, list):
            errors.append(f"{context}: components must be a list")
            components = []
        else:
            components = [str(item).strip() for item in raw_components if str(item).strip()]
        if not components:
            errors.append(f"{context}: component or components must contain at least one file")
        capability_owner = str(surface.get("capability_owner", "")).strip()
        status = str(surface.get("status", ""))
        permission_enforcement = str(surface.get("permission_enforcement", "mapped_only"))
        if status not in {"ui", "api_only"}:
            errors.append(f"{context}: status must be ui or api_only")
        if permission_enforcement not in {"mapped_only", "implemented"}:
            errors.append(f"{context}: permission_enforcement must be mapped_only or implemented")
        if status == "api_only" and permission_enforcement == "implemented":
            errors.append(f"{context}: api_only action cannot claim frontend permission enforcement")
        for label, relative in [
            *(("components", relative) for relative in components),
            ("capability_owner", capability_owner),
        ]:
            if not relative or not (root / relative).is_file():
                errors.append(f"{context}: {label} file is missing: {relative or '<unset>'}")
        raw_actions = surface.get("actions")
        if not isinstance(raw_actions, list) or not raw_actions:
            errors.append(f"{context}: actions must be a non-empty list")
            continue
        for raw_action_code in raw_actions:
            action_code = str(raw_action_code)
            if permission_enforcement == "implemented" and (root / capability_owner).is_file():
                owner_source = (root / capability_owner).read_text(encoding="utf-8")
                marker = f"PM_ACTION.{_typescript_action_member(action_code)}"
                if "useOperationCapabilities" not in owner_source or marker not in owner_source:
                    errors.append(
                        f"{context}: implemented capability owner must consume {marker} "
                        f"through useOperationCapabilities"
                    )
            if action_code in surface_owners:
                errors.append(
                    f"operation action has multiple frontend surfaces: {action_code} "
                    f"({surface_owners[action_code]} and {', '.join(components)})"
                )
            else:
                surface_owners[action_code] = ", ".join(components)
                surface_statuses[action_code] = status
    for action_code in sorted(governed_actions - set(surface_owners)):
        errors.append(f"operation action has no component/interface mapping: {action_code}")
    for action_code in sorted(set(surface_owners) - governed_actions):
        errors.append(f"frontend surface maps a non-governed operation action: {action_code}")
    for action_code, status in surface_statuses.items():
        kinds = view_kinds_by_action.get(action_code, set())
        if status == "ui" and "ui" not in kinds:
            errors.append(f"ui action has no frontend ui view placement: {action_code}")
        if status == "api_only" and kinds != {"capability"}:
            errors.append(f"api_only action must only use capability view placement: {action_code}")

    button_locations = governance.get("button_locations")
    if not isinstance(button_locations, dict):
        errors.append("operation_governance.button_locations must be an object")
        button_locations = {}
    for action_code in sorted(governed_actions - set(button_locations)):
        errors.append(f"operation action has no button location decision: {action_code}")
    for action_code in sorted(set(button_locations) - governed_actions):
        errors.append(f"button location maps a non-governed operation action: {action_code}")
    for action_code, raw_locations in button_locations.items():
        context = f"operation_governance.button_locations[{action_code!r}]"
        if not isinstance(raw_locations, list):
            errors.append(f"{context}: locations must be a list")
            continue
        status = surface_statuses.get(action_code)
        if status == "ui" and not raw_locations:
            errors.append(f"{context}: ui action requires at least one button location")
        if status == "api_only" and raw_locations:
            errors.append(f"{context}: api_only action must not declare a button location")
        seen_locations: set[str] = set()
        for raw_location in raw_locations:
            location = str(raw_location).strip()
            segments = [segment.strip() for segment in location.split(" > ")]
            if not location or not CJK_RE.search(location) or len(segments) < 3 or any(not segment for segment in segments):
                errors.append(
                    f"{context}: use a Chinese '页面 > 区域 > 按钮/交互' location, got {location!r}"
                )
            if location in seen_locations:
                errors.append(f"{context}: duplicate button location {location!r}")
            seen_locations.add(location)

    tool_bindings = governance.get("tool_bindings")
    tool_not_applicable = governance.get("tool_not_applicable")
    tool_no_action = governance.get("tool_no_action")
    decision_reasons = governance.get("tool_decision_reasons")
    if not isinstance(tool_bindings, dict):
        errors.append("operation_governance.tool_bindings must be an object")
        tool_bindings = {}
    if not isinstance(tool_not_applicable, dict):
        errors.append("operation_governance.tool_not_applicable must be an object")
        tool_not_applicable = {}
    if not isinstance(tool_no_action, dict):
        errors.append("operation_governance.tool_no_action must be an object")
        tool_no_action = {}
    if not isinstance(decision_reasons, dict):
        errors.append("operation_governance.tool_decision_reasons must be an object")
        decision_reasons = {}

    mapped_actions = set(tool_bindings)
    exempt_actions = set(tool_not_applicable)
    for action_code in sorted(mapped_actions & exempt_actions):
        errors.append(f"operation action cannot be both Tool-mapped and not-applicable: {action_code}")
    for action_code in sorted(governed_actions - mapped_actions - exempt_actions):
        errors.append(f"operation action has no Tool adaptation decision: {action_code}")
    all_action_codes = set(action_page)
    for action_code in sorted(mapped_actions - all_action_codes):
        errors.append(f"Tool binding references an unknown permission action: {action_code}")
    for action_code in sorted(exempt_actions - governed_actions):
        errors.append(f"Tool adaptation decision is not a governed operation action: {action_code}")

    source_glob = str(governance.get("tool_source_glob", "")).strip()
    available_tools: set[str] = set()
    available_tool_contracts: dict[str, Route] = {}
    if not source_glob:
        errors.append("operation_governance.tool_source_glob is required")
    else:
        source_paths = sorted(root.glob(source_glob))
        if not source_paths:
            errors.append(f"operation Tool source glob matched no files: {source_glob}")
        for source_path in source_paths:
            source_name = source_path.relative_to(root).as_posix()
            source_text = source_path.read_text(encoding="utf-8")
            source_tools, source_errors = extract_pm_tool_names(
                source_text,
                source_name,
            )
            available_tools.update(source_tools)
            errors.extend(source_errors)
            source_contracts, contract_errors = extract_pm_tool_contracts(source_text, source_name)
            errors.extend(contract_errors)
            for tool_name, route in source_contracts.items():
                previous = available_tool_contracts.get(tool_name)
                if previous is not None:
                    errors.append(
                        f"protocol-neutral Tool is declared more than once: {tool_name} "
                        f"({previous.display()} and {route.display()})"
                    )
                    continue
                available_tool_contracts[tool_name] = route
    tool_owners: dict[str, str] = {}
    for action_code, raw_tools in tool_bindings.items():
        context = f"operation_governance.tool_bindings[{action_code!r}]"
        if not isinstance(raw_tools, list) or not raw_tools:
            errors.append(f"{context}: one or more canonical Tool names are required")
            continue
        for raw_tool in raw_tools:
            tool_name = str(raw_tool)
            if not TOOL_NAME_RE.fullmatch(tool_name):
                errors.append(f"{context}: invalid Tool name {tool_name!r}")
            elif tool_name not in available_tools:
                errors.append(f"{context}: Tool is not present in the protocol-neutral catalog: {tool_name}")
            elif tool_name in tool_owners:
                errors.append(
                    f"Tool maps to multiple permission actions: {tool_name} "
                    f"({tool_owners[tool_name]} and {action_code})"
                )
            else:
                tool_owners[tool_name] = action_code
                tool_route = available_tool_contracts.get(tool_name)
                if tool_route is None:
                    errors.append(
                        f"{context}: Tool has no complete pmSpec HTTP contract: {tool_name}"
                    )
                    continue
                expected_routes = action_routes.get(action_code, [])
                tool_shape = _normalized_tool_route_shape(tool_route.path)
                if not any(
                    route.method == tool_route.method
                    and _normalized_tool_route_shape(route.path) == tool_shape
                    for route in expected_routes
                ):
                    rendered_routes = ", ".join(route.display() for route in expected_routes) or "<none>"
                    errors.append(
                        f"{context}: Tool contract {tool_name} ({tool_route.display()}) "
                        f"does not match any route owned by Action {action_code}: {rendered_routes}"
                    )

    baseline_tool_owners: dict[str, str] = {}
    for route in baseline_routes(catalog):
        route_name = f"{str(route.get('method', '')).upper()} {route.get('path', '')}"
        raw_tools = route.get("related_tools", [])
        if not isinstance(raw_tools, list):
            errors.append(f"baseline route {route_name}: related_tools must be a list")
            continue
        for raw_tool in raw_tools:
            tool_name = str(raw_tool)
            if not TOOL_NAME_RE.fullmatch(tool_name):
                errors.append(f"baseline route {route_name}: invalid related Tool name {tool_name!r}")
            elif tool_name not in available_tools:
                errors.append(
                    f"baseline route {route_name}: related Tool is not present in the "
                    f"protocol-neutral catalog: {tool_name}"
                )
            elif tool_name in tool_owners:
                errors.append(
                    f"baseline-related Tool must not inherit a business Action: {tool_name} "
                    f"({tool_owners[tool_name]})"
                )
            elif tool_name in baseline_tool_owners:
                errors.append(
                    f"related Tool maps to multiple baseline routes: {tool_name} "
                    f"({baseline_tool_owners[tool_name]} and {route_name})"
                )
            else:
                baseline_tool_owners[tool_name] = route_name

    no_action_tool_owners: dict[str, str] = {}
    for raw_tool, raw_reason in tool_no_action.items():
        tool_name = str(raw_tool)
        reason = str(raw_reason)
        context = f"operation_governance.tool_no_action[{tool_name!r}]"
        explanation = str(decision_reasons.get(reason, "")).strip()
        if not TOOL_NAME_RE.fullmatch(tool_name):
            errors.append(f"{context}: invalid Tool name")
        elif tool_name not in available_tools:
            errors.append(f"{context}: Tool is not present in the protocol-neutral catalog")
        elif tool_name in tool_owners:
            errors.append(
                f"Tool cannot be both business Action-mapped and no-Action: {tool_name} "
                f"({tool_owners[tool_name]})"
            )
        elif tool_name in baseline_tool_owners:
            errors.append(
                f"Tool cannot be both baseline-related and no-Action: {tool_name} "
                f"({baseline_tool_owners[tool_name]})"
            )
        else:
            no_action_tool_owners[tool_name] = reason
        if not DECISION_REASON_RE.fullmatch(reason) or not explanation:
            errors.append(f"{context}: unknown or unexplained reason {reason!r}")

    classified_tools = set(tool_owners) | set(baseline_tool_owners) | set(no_action_tool_owners)
    for tool_name in sorted(available_tools - classified_tools):
        errors.append(
            f"protocol-neutral Tool has no permission classification: {tool_name}; "
            "map it to one business Action, one baseline capability, or tool_no_action"
        )

    for action_code, raw_reason in tool_not_applicable.items():
        reason = str(raw_reason)
        explanation = str(decision_reasons.get(reason, "")).strip()
        if not DECISION_REASON_RE.fullmatch(reason) or not explanation:
            errors.append(
                f"operation_governance.tool_not_applicable[{action_code!r}]: "
                f"unknown or unexplained reason {reason!r}"
            )

    resolver = str(governance.get("frontend_resolver", "")).strip()
    resolver_path = root / resolver
    if not resolver or not resolver_path.is_file():
        errors.append(f"operation frontend capability resolver is missing: {resolver or '<unset>'}")
    else:
        source = resolver_path.read_text(encoding="utf-8")
        for marker in ("PM_OPERATION_ACTIONS_BY_DOMAIN", "useActionPermissions", "useOperationCapabilities"):
            if marker not in source:
                errors.append(f"operation frontend capability resolver is missing {marker}: {resolver}")


def validate_catalog(catalog: dict, root: Path, scan_literals: bool = True) -> list[str]:
    errors: list[str] = []
    if catalog.get("schema_version") != 2:
        errors.append("schema_version must be 2")

    code_owners: dict[str, str] = {}
    copy_owners: dict[str, str] = {}
    mapped_routes: dict[Route, str] = {}
    pages = catalog.get("pages")
    if not isinstance(pages, list) or not pages:
        errors.append("pages must be a non-empty list")
        pages = []

    def node_identity(node: dict, level: str, owner: str) -> None:
        code = str(node.get("code", ""))
        name = str(node.get("name", "")).strip()
        copy_key = str(node.get("copy_key", ""))
        if not name:
            errors.append(f"{owner}: canonical Chinese display name is required")
        if not CODE_RE.fullmatch(code):
            errors.append(f"{owner}: invalid stable code {code!r}")
        elif code in code_owners:
            errors.append(f"duplicate code {code!r}: {code_owners[code]} and {owner}")
        else:
            code_owners[code] = owner
        if not COPY_KEY_RE.fullmatch(copy_key) or not copy_key.startswith(f"access.{level}."):
            errors.append(f"{owner}: invalid {level} copy_key {copy_key!r}")
        elif copy_key in copy_owners:
            errors.append(f"duplicate copy_key {copy_key!r}: {copy_owners[copy_key]} and {owner}")
        else:
            copy_owners[copy_key] = owner

    mapped_frontend_routes: dict[str, str] = {}
    page_codes: set[str] = set()
    for page_index, page in enumerate(pages):
        page_owner = f"pages[{page_index}]"
        if not isinstance(page, dict):
            errors.append(f"{page_owner}: page must be an object")
            continue
        node_identity(page, "pages", page_owner)
        page_codes.add(str(page.get("code", "")))
        raw_frontend_routes = page.get("frontend_routes", [])
        if not isinstance(raw_frontend_routes, list):
            errors.append(f"{page_owner}: frontend_routes must be a list")
        else:
            for frontend_route in raw_frontend_routes:
                frontend_route = str(frontend_route)
                if not frontend_route.startswith("/"):
                    errors.append(f"{page_owner}: invalid frontend route {frontend_route!r}")
                elif frontend_route in mapped_frontend_routes:
                    errors.append(
                        f"frontend route mapped more than once: {frontend_route} "
                        f"({mapped_frontend_routes[frontend_route]} and {page.get('code', page_owner)})"
                    )
                else:
                    mapped_frontend_routes[frontend_route] = str(page.get("code", page_owner))
        modules = page.get("modules")
        if not isinstance(modules, list) or not modules:
            errors.append(f"{page_owner}: modules must be a non-empty list")
            continue
        forbidden_page_keys = {"actions", "routes", "frontend_only_reason"} & set(page)
        if forbidden_page_keys:
            errors.append(f"{page_owner}: strict hierarchy forbids {sorted(forbidden_page_keys)}")
        for module_index, module in enumerate(modules):
            module_owner = f"{page_owner}.modules[{module_index}]"
            if not isinstance(module, dict):
                errors.append(f"{module_owner}: module must be an object")
                continue
            node_identity(module, "modules", module_owner)
            actions = module.get("actions")
            if not isinstance(actions, list) or not actions:
                errors.append(f"{module_owner}: actions must be a non-empty list")
                continue
            forbidden_module_keys = {"modules", "routes", "frontend_only_reason"} & set(module)
            if forbidden_module_keys:
                errors.append(f"{module_owner}: strict hierarchy forbids {sorted(forbidden_module_keys)}")
            for action_index, action in enumerate(actions):
                action_owner = f"{module_owner}.actions[{action_index}]"
                if not isinstance(action, dict):
                    errors.append(f"{action_owner}: action must be an object")
                    continue
                node_identity(action, "actions", action_owner)
                if "modules" in action or "actions" in action:
                    errors.append(f"{action_owner}: action must be a leaf")
                raw_routes = action.get("routes", [])
                reason = str(action.get("frontend_only_reason", "")).strip()
                if raw_routes and reason:
                    errors.append(f"{action_owner}: use routes or frontend_only_reason, not both")
                if not raw_routes and not reason:
                    errors.append(f"{action_owner}: action needs a route or explicit frontend_only_reason")
                if raw_routes and not isinstance(raw_routes, list):
                    errors.append(f"{action_owner}: routes must be a list")
                    continue
                for raw_route in raw_routes:
                    if not isinstance(raw_route, dict):
                        errors.append(f"{action_owner}: route must be an object")
                        continue
                    route = Route.from_mapping(raw_route)
                    _validate_route(route, action_owner, errors)
                    if route in mapped_routes:
                        errors.append(
                            f"route mapped more than once: {route.display()} "
                            f"({mapped_routes[route]} and {action.get('code', action_owner)})"
                        )
                    else:
                        mapped_routes[route] = str(action.get("code", action_owner))

    _validate_operation_governance(catalog, root, page_codes, errors)

    route_inventory = catalog.get("route_inventory", {})
    sources = route_inventory.get("sources", [])
    try:
        actual_routes, _ = extract_routes(root, sources)
        errors.extend(find_untracked_route_registrations(root, sources))
    except CatalogError as exc:
        errors.append(str(exc))
        actual_routes = set()

    exemption_routes: dict[Route, str] = {}
    for kind in ("public_exemptions", "authenticated_exemptions"):
        entries = route_inventory.get(kind, [])
        if not isinstance(entries, list):
            errors.append(f"route_inventory.{kind} must be a list")
            continue
        for index, entry in enumerate(entries):
            context = f"route_inventory.{kind}[{index}]"
            if not isinstance(entry, dict):
                errors.append(f"{context}: exemption must be an object")
                continue
            route = Route.from_mapping(entry)
            _validate_route(route, context, errors)
            reason = str(entry.get("reason", "")).strip()
            if not reason:
                errors.append(f"{context}: reviewed reason is required")
            if kind == "authenticated_exemptions":
                capability = str(entry.get("capability", "")).strip()
                access_stage = str(entry.get("access_stage", "")).strip()
                external_protocol = route.path == "/api/v1/mcp" or route.path.startswith(
                    ("/api/v1/tool-runtime/", "/api/v1/tool-auth/")
                )
                pm_session_route = route.path.startswith(
                    ("/api/v1/core/", "/api/v1/platform/")
                ) and not external_protocol
                if pm_session_route and not capability:
                    errors.append(f"{context}: fixed PM baseline capability is required")
                if capability and not CODE_RE.fullmatch(capability):
                    errors.append(f"{context}: invalid baseline capability {capability!r}")
                if capability and access_stage not in {"session", "admitted", "workspace"}:
                    errors.append(
                        f"{context}: access_stage must be session, admitted or workspace"
                    )
                if access_stage and not capability:
                    errors.append(f"{context}: access_stage requires a baseline capability")
                frontend_surfaces = entry.get("frontend_surfaces", [])
                if not isinstance(frontend_surfaces, list):
                    errors.append(f"{context}: frontend_surfaces must be a list")
                else:
                    for surface in frontend_surfaces:
                        relative = str(surface).strip()
                        if not relative or not (root / relative).is_file():
                            errors.append(
                                f"{context}: frontend surface does not exist: {relative!r}"
                            )
            if route in exemption_routes:
                errors.append(f"duplicate route exemption: {route.display()}")
            exemption_routes[route] = reason

    overlap = set(mapped_routes) & set(exemption_routes)
    for route in sorted(overlap):
        errors.append(f"route cannot be both mapped and exempt: {route.display()}")
    for route in sorted(actual_routes - set(mapped_routes) - set(exemption_routes)):
        errors.append(f"protected Gin route has no action mapping or exemption: {route.display()}")
    for route in sorted((set(mapped_routes) | set(exemption_routes)) - actual_routes):
        errors.append(f"catalog route is not in current Gin inventory: {route.display()}")

    frontend_inventory = catalog.get("frontend_inventory", {})
    router_source = root / str(frontend_inventory.get("router_file", ""))
    router_dir = router_source if router_source.is_dir() else router_source.parent
    if router_source.is_file() or router_dir.is_dir():
        actual_frontend_routes: set[str] = set()
        if router_source.is_file():
            actual_frontend_routes.update(FRONTEND_ROUTE_RE.findall(router_source.read_text(encoding="utf-8")))
        if router_dir.is_dir():
            for child in sorted(router_dir.iterdir()):
                if child.suffix == '.tsx' and child.name != router_source.name:
                    try:
                        actual_frontend_routes.update(FRONTEND_ROUTE_RE.findall(child.read_text(encoding="utf-8")))
                    except Exception:
                        pass
        frontend_exemptions: set[str] = set()
        for index, entry in enumerate(frontend_inventory.get("exemptions", [])):
            context = f"frontend_inventory.exemptions[{index}]"
            path = str(entry.get("path", "")) if isinstance(entry, dict) else ""
            reason = str(entry.get("reason", "")).strip() if isinstance(entry, dict) else ""
            if not path.startswith("/") or not reason:
                errors.append(f"{context}: absolute path and reviewed reason are required")
            frontend_exemptions.add(path)
        for path in sorted(actual_frontend_routes - set(mapped_frontend_routes) - frontend_exemptions):
            errors.append(f"frontend route has no page mapping or exemption: {path}")
        for path in sorted((set(mapped_frontend_routes) | frontend_exemptions) - actual_frontend_routes):
            errors.append(f"catalog frontend route is not in current router: {path}")

        shell_file = root / str(frontend_inventory.get("shell_catalog_file", ""))
        if shell_file.is_file():
            try:
                shell_paths = {str(item["path"]) for item in json.loads(shell_file.read_text(encoding="utf-8"))}
            except (json.JSONDecodeError, KeyError, TypeError) as exc:
                errors.append(f"cannot read frontend shell catalog: {exc}")
            else:
                for path in sorted(shell_paths - set(mapped_frontend_routes)):
                    errors.append(f"shell page has no permission page mapping: {path}")

    if scan_literals:
        scan = catalog.get("permission_literal_scan", {})
        excludes = tuple(str(item) for item in scan.get("exclude_prefixes", []))
        permission_codes = set(code_owners)
        for relative in scan.get("paths", []):
            base = root / str(relative)
            if not base.exists():
                continue
            candidates = [base] if base.is_file() else base.rglob("*")
            for path in candidates:
                if not path.is_file() or path.suffix not in {".go", ".ts", ".tsx", ".js", ".jsx"}:
                    continue
                rel = path.relative_to(root).as_posix()
                if rel.startswith(excludes):
                    continue
                for number, line in enumerate(path.read_text(encoding="utf-8").splitlines(), 1):
                    if any(match.group("value") in permission_codes for match in STRING_LITERAL_RE.finditer(line)):
                        errors.append(f"scattered permission literal {rel}:{number}")

    if catalog.get("generation", {}).get("go_output"):
        try:
            generated = {
                generated_output_path(catalog, root): render_go(catalog),
                generated_typescript_output_path(catalog, root): render_typescript(catalog),
                generated_operation_matrix_output_path(catalog, root): render_operation_matrix(catalog),
            }
        except (GenerationError, OSError) as exc:
            errors.append(f"generated access catalog unavailable: {exc}")
        else:
            for output, expected in generated.items():
                try:
                    current = output.read_text(encoding="utf-8")
                except OSError as exc:
                    errors.append(f"generated access catalog unavailable: {exc}")
                    continue
                if current != expected:
                    errors.append(
                        f"generated access catalog is stale: {output.relative_to(root).as_posix()} "
                        "(run python3 scripts/generate-access-catalog.py)"
                    )
    return errors


def validate_path(catalog_path: Path, root: Path, scan_literals: bool = True) -> list[str]:
    return validate_catalog(load_json(catalog_path), root, scan_literals=scan_literals)
