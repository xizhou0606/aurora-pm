from __future__ import annotations

import copy
import json
import sys
import tempfile
import unittest
from pathlib import Path


SCRIPT_DIR = Path(__file__).resolve().parents[1]
REPOSITORY_ROOT = SCRIPT_DIR.parent
FIXTURE_ROOT = Path(__file__).resolve().parent / "fixtures/access_catalog_valid"
sys.path.insert(0, str(SCRIPT_DIR))

from access_catalog_validator import (  # noqa: E402
    extract_pm_tool_contracts,
    extract_pm_tool_names,
    load_json,
    validate_catalog,
    validate_path,
)
from access_catalog_generator import (  # noqa: E402
    GenerationError,
    render_go,
    render_operation_matrix,
    render_typescript,
)


class AccessCatalogValidatorTest(unittest.TestCase):
    def setUp(self) -> None:
        self.catalog = load_json(FIXTURE_ROOT / "catalog.json")

    def test_accepts_strict_three_level_catalog_with_complete_route_coverage(self) -> None:
        self.assertEqual(validate_catalog(self.catalog, FIXTURE_ROOT), [])

    def test_rejects_duplicate_codes_and_route_owners(self) -> None:
        catalog = copy.deepcopy(self.catalog)
        action = catalog["pages"][0]["modules"][0]["actions"][0]
        duplicate = copy.deepcopy(action)
        duplicate["copy_key"] = "access.actions.things.list.duplicate"
        catalog["pages"][0]["modules"][0]["actions"].append(duplicate)

        errors = validate_catalog(catalog, FIXTURE_ROOT)

        self.assertTrue(any("duplicate code" in error for error in errors), errors)
        self.assertTrue(any("route mapped more than once" in error for error in errors), errors)

    def test_rejects_action_without_route_or_reviewed_frontend_reason(self) -> None:
        catalog = copy.deepcopy(self.catalog)
        catalog["pages"][0]["modules"][0]["actions"][0].pop("routes")

        errors = validate_catalog(catalog, FIXTURE_ROOT)

        self.assertTrue(any("action needs a route" in error for error in errors), errors)
        self.assertTrue(any("protected Gin route has no action mapping" in error for error in errors), errors)

    def test_rejects_untracked_gin_registration_function(self) -> None:
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            for source in FIXTURE_ROOT.rglob("*"):
                if source.is_file():
                    destination = root / source.relative_to(FIXTURE_ROOT)
                    destination.parent.mkdir(parents=True, exist_ok=True)
                    destination.write_bytes(source.read_bytes())
            untracked = root / "backend/untracked.go"
            untracked.write_text(
                'package fixture\nfunc mountOther(scoped *gin.RouterGroup) {\n'
                '  scoped.POST("/other", createOther)\n}\n',
                encoding="utf-8",
            )

            errors = validate_path(root / "catalog.json", root)

        self.assertTrue(any("untracked Gin registration" in error for error in errors), errors)

    def test_rejects_exact_catalog_code_literal_in_business_source(self) -> None:
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            for source in FIXTURE_ROOT.rglob("*"):
                if source.is_file():
                    destination = root / source.relative_to(FIXTURE_ROOT)
                    destination.parent.mkdir(parents=True, exist_ok=True)
                    destination.write_bytes(source.read_bytes())
            catalog_path = root / "catalog.json"
            catalog = json.loads(catalog_path.read_text(encoding="utf-8"))
            catalog["permission_literal_scan"]["paths"] = ["frontend"]
            catalog_path.write_text(json.dumps(catalog), encoding="utf-8")
            (root / "frontend/button.tsx").write_text(
                "const permission = 'pm.things.list.view'\n", encoding="utf-8"
            )

            errors = validate_path(catalog_path, root)

        self.assertTrue(any("scattered permission literal" in error for error in errors), errors)

    def test_repository_catalog_is_current(self) -> None:
        errors = validate_path(
            REPOSITORY_ROOT / "contracts/access/catalog.json",
            REPOSITORY_ROOT,
        )
        self.assertEqual(errors, [])

    def test_go_generation_is_deterministic_and_typed(self) -> None:
        first = render_go(self.catalog, "fixture/catalog.json")
        second = render_go(copy.deepcopy(self.catalog), "fixture/catalog.json")

        self.assertEqual(first, second)
        self.assertIn("DO NOT EDIT", first)
        self.assertIn("type ActionCode string", first)
        self.assertIn("func ActionForRoute", first)
        self.assertIn("func ToolNamesForAction", first)
        self.assertIn('ActionThingsListView ActionCode = "pm.things.list.view"', first)

    def test_go_generation_exposes_canonical_object_policy_lookups(self) -> None:
        catalog = load_json(REPOSITORY_ROOT / "contracts/access/catalog.json")

        rendered = render_go(catalog, "contracts/access/catalog.json")

        self.assertRegex(
            rendered, r'ObjectPolicyTaskCreator\s+ObjectPolicyCode = "task\.creator"'
        )
        self.assertRegex(
            rendered,
            r'ObjectPolicyRequirementCreator\s+ObjectPolicyCode = "requirement\.creator"',
        )
        self.assertIn("func ObjectPolicyForAction", rendered)
        self.assertIn("func ObjectPolicyForRoute", rendered)
        self.assertIn("func AuthorizationRouteForAction", rendered)

    def test_generation_exposes_canonical_action_implications(self) -> None:
        catalog = load_json(REPOSITORY_ROOT / "contracts/access/catalog.json")

        rendered_go = render_go(catalog, "contracts/access/catalog.json")
        rendered_typescript = render_typescript(catalog, "contracts/access/catalog.json")

        self.assertIn("func ActionImplies", rendered_go)
        self.assertIn("func GrantingActions", rendered_go)
        self.assertIn("export function pmActionImplies", rendered_typescript)
        self.assertIn("export function pmGrantingActions", rendered_typescript)

    def test_rejects_unknown_or_cyclic_action_implications(self) -> None:
        catalog = load_json(REPOSITORY_ROOT / "contracts/access/catalog.json")
        catalog["operation_governance"]["action_implications"] = {
            "pm.work-items.tasks.update": ["pm.work-items.tasks.transition-status"],
            "pm.work-items.tasks.transition-status": ["pm.work-items.tasks.update"],
            "pm.unknown": ["pm.work-items.tasks.update"],
        }

        errors = validate_catalog(catalog, REPOSITORY_ROOT, scan_literals=False)

        self.assertTrue(any("unknown source action" in error for error in errors), errors)
        self.assertTrue(any("contains a cycle" in error for error in errors), errors)

    def test_rejects_object_policy_without_registered_handler_enforcement(self) -> None:
        catalog = load_json(REPOSITORY_ROOT / "contracts/access/catalog.json")
        catalog["operation_governance"]["object_policies"]["task.creator"][
            "enforcement_markers"
        ] = ["missingCreatorEnforcement"]

        errors = validate_catalog(catalog, REPOSITORY_ROOT, scan_literals=False)

        self.assertTrue(
            any("does not call a registered enforcement marker" in error for error in errors),
            errors,
        )

    def test_rejects_action_with_unknown_object_policy(self) -> None:
        catalog = load_json(REPOSITORY_ROOT / "contracts/access/catalog.json")
        task_actions = next(
            module["actions"]
            for page in catalog["pages"] if page["code"] == "pm.work-items"
            for module in page["modules"] if module["code"] == "pm.work-items.tasks"
        )
        next(
            action for action in task_actions
            if action["code"] == "pm.work-items.tasks.update"
        )["object_policy"] = "task.unknown"

        errors = validate_catalog(catalog, REPOSITORY_ROOT, scan_literals=False)

        self.assertTrue(any("references unknown object policy" in error for error in errors), errors)

    def test_resource_access_object_policy_kind_is_supported(self) -> None:
        catalog = load_json(REPOSITORY_ROOT / "contracts/access/catalog.json")
        catalog["operation_governance"]["object_policies"]["task.creator"]["kind"] = "resource_access"

        errors = validate_catalog(catalog, REPOSITORY_ROOT, scan_literals=False)

        self.assertFalse(
            any("unsupported object policy kind 'resource_access'" in error for error in errors),
            errors,
        )

    def test_authenticated_baseline_capability_generates_route_matcher(self) -> None:
        catalog = copy.deepcopy(self.catalog)
        catalog["route_inventory"]["authenticated_exemptions"] = [{
            "method": "GET",
            "path": "/api/v1/core/auth/me/workspaces",
            "capability": "system.workspace.select",
            "access_stage": "admitted",
            "related_tools": ["workspace.member.list"],
            "reason": "Workspace selection is required before business authorization.",
        }]

        rendered = render_go(catalog, "fixture/catalog.json")

        self.assertIn("type BaselineRoute struct", rendered)
        self.assertIn('Capability: "system.workspace.select"', rendered)
        self.assertIn('RelatedToolNames: []string{"workspace.member.list"}', rendered)
        self.assertIn("func BaselineCapabilityForRoute", rendered)

    def test_rejects_unknown_baseline_related_tool(self) -> None:
        catalog = copy.deepcopy(self.catalog)
        catalog["route_inventory"]["authenticated_exemptions"] = [{
            "method": "GET",
            "path": "/api/v1/core/auth/me/workspaces",
            "capability": "system.workspace.select",
            "access_stage": "admitted",
            "related_tools": ["unknown.tool"],
            "reason": "Workspace selection is required before business authorization.",
        }]

        errors = validate_catalog(catalog, FIXTURE_ROOT)

        self.assertTrue(any("related Tool is not present" in error for error in errors), errors)

    def test_extracts_pm_tool_name_from_string_constant(self) -> None:
        names, errors = extract_pm_tool_names(
            'const PMCurrentUserToolName = "user.me"\n'
            'func specs() { pmSpec(PMCurrentUserToolName, "users") }\n',
            "backend/catalog.go",
        )

        self.assertEqual(errors, [])
        self.assertEqual(names, {"user.me"})

    def test_extracts_pm_tool_contract_from_string_constant(self) -> None:
        contracts, errors = extract_pm_tool_contracts(
            'const PMCurrentUserToolName = "user.me"\n'
            'func specs() { pmSpec(PMCurrentUserToolName, users, "GET", "users/me", "me", "L0") }\n',
            "backend/catalog.go",
        )

        self.assertEqual(errors, [])
        self.assertEqual(contracts["user.me"].method, "GET")
        self.assertEqual(contracts["user.me"].path, "users/me")

    def test_rejects_business_tool_mapped_to_action_with_different_route_semantics(self) -> None:
        catalog = load_json(REPOSITORY_ROOT / "contracts/access/catalog.json")
        catalog["operation_governance"]["tool_bindings"]["pm.work-items.tasks.update"].remove(
            "task.due_conflict.list"
        )
        catalog["operation_governance"]["tool_bindings"]["pm.work-items.task-table.view"].append(
            "task.due_conflict.list"
        )

        errors = validate_catalog(catalog, REPOSITORY_ROOT, scan_literals=False)

        self.assertTrue(
            any(
                "task.due_conflict.list" in error
                and "does not match any route owned by Action pm.work-items.task-table.view" in error
                for error in errors
            ),
            errors,
        )

    def test_rejects_protocol_neutral_tool_without_permission_classification(self) -> None:
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            for source in FIXTURE_ROOT.rglob("*"):
                if source.is_file():
                    destination = root / source.relative_to(FIXTURE_ROOT)
                    destination.parent.mkdir(parents=True, exist_ok=True)
                    destination.write_bytes(source.read_bytes())
            tool_source = root / "backend/tools.go"
            tool_source.write_text(
                'package fixture\nfunc specs() { pmSpec("unclassified.tool", "tools") }\n',
                encoding="utf-8",
            )
            catalog_path = root / "catalog.json"
            catalog = json.loads(catalog_path.read_text(encoding="utf-8"))
            catalog["operation_governance"]["tool_source_glob"] = "backend/tools.go"
            catalog_path.write_text(json.dumps(catalog), encoding="utf-8")

            errors = validate_path(catalog_path, root)

        self.assertTrue(any("Tool has no permission classification" in error for error in errors), errors)

    def test_rejects_pm_authenticated_exemption_without_fixed_capability(self) -> None:
        catalog = copy.deepcopy(self.catalog)
        catalog["route_inventory"]["authenticated_exemptions"] = [{
            "method": "GET",
            "path": "/api/v1/core/auth/me/workspaces",
            "reason": "Workspace selection is required before business authorization.",
        }]

        errors = validate_catalog(catalog, FIXTURE_ROOT)

        self.assertTrue(any("fixed PM baseline capability is required" in error for error in errors), errors)

    def test_go_generation_rejects_identifier_collisions(self) -> None:
        catalog = copy.deepcopy(self.catalog)
        duplicate = copy.deepcopy(catalog["pages"][0]["modules"][0]["actions"][0])
        duplicate["code"] = "pm.things-list-view"
        duplicate["copy_key"] = "access.actions.things.list.collision"
        catalog["pages"][0]["modules"][0]["actions"].append(duplicate)

        with self.assertRaises(GenerationError):
            render_go(catalog)

    def test_generation_rejects_missing_canonical_name(self) -> None:
        catalog = copy.deepcopy(self.catalog)
        catalog["pages"][0]["modules"][0]["actions"][0].pop("name")

        with self.assertRaises(GenerationError):
            render_go(catalog)

    def test_typescript_generation_is_typed_without_copy_duplication(self) -> None:
        rendered = render_typescript(self.catalog, "fixture/catalog.json")

        self.assertIn("DO NOT EDIT", rendered)
        self.assertIn("export type PMActionCode", rendered)
        self.assertIn('ThingsListView: "pm.things.list.view"', rendered)
        self.assertIn('"/workspace/things": "pm.things"', rendered)
        self.assertNotIn("事项", rendered)

    def test_operation_matrix_generation_is_derived_from_the_catalog(self) -> None:
        rendered = render_operation_matrix(self.catalog, "fixture/catalog.json")

        self.assertIn("PM 业务动作映射表", rendered)
        self.assertIn("按钮位置", rendered)
        self.assertIn("组件/交互入口", rendered)
        self.assertNotIn("pm.things.list.view", rendered)

    def test_recursive_frontend_views_drive_permission_tree_and_matrix(self) -> None:
        catalog = copy.deepcopy(self.catalog)
        catalog["operation_governance"]["domains"] = [
            {"owner": "things", "page_codes": ["pm.things"]},
        ]
        catalog["operation_governance"]["permission_views"] = [{
            "page_code": "pm.things",
            "views": [{
                "code": "pm.things.views.table",
                "name": "事项表格",
                "copy_key": "access.views.things.table",
                "kind": "ui",
                "components": ["frontend/router.tsx"],
                "actions": [],
                "children": [{
                    "code": "pm.things.views.detail",
                    "name": "事项详情",
                    "copy_key": "access.views.things.detail",
                    "kind": "ui",
                    "components": ["frontend/router.tsx"],
                    "actions": ["pm.things.list.view"],
                }],
            }],
        }]
        catalog["operation_governance"]["frontend_surfaces"] = [{
            "component": "frontend/router.tsx",
            "capability_owner": "frontend/operation-capabilities.ts",
            "status": "ui",
            "actions": ["pm.things.list.view"],
        }]
        catalog["operation_governance"]["button_locations"] = {
            "pm.things.list.view": ["事项管理 > 事项详情 > 查看"],
        }
        catalog["operation_governance"]["tool_not_applicable"] = {
            "pm.things.list.view": "no_protocol_neutral_contract",
        }
        catalog["operation_governance"]["tool_decision_reasons"] = {
            "no_protocol_neutral_contract": "尚无可复用的协议无关 Tool 契约。",
        }

        self.assertEqual(validate_catalog(catalog, FIXTURE_ROOT), [])
        rendered_go = render_go(catalog, "fixture/catalog.json")
        rendered_matrix = render_operation_matrix(catalog, "fixture/catalog.json")
        self.assertIn('Code:        "pm.things.views.detail"', rendered_go)
        self.assertIn("事项表格 > 事项详情", rendered_matrix)
        self.assertIn("GET /api/v1/things", rendered_matrix)

    def test_rejects_governed_action_without_frontend_view_or_capability_placement(self) -> None:
        catalog = copy.deepcopy(self.catalog)
        catalog["operation_governance"]["domains"] = [
            {"owner": "things", "page_codes": ["pm.things"]},
        ]
        catalog["operation_governance"]["permission_views"] = [{
            "page_code": "pm.things",
            "views": [{
                "code": "pm.things.views.table",
                "name": "事项表格",
                "copy_key": "access.views.things.table",
                "kind": "ui",
                "components": ["frontend/router.tsx"],
                "actions": [],
                "children": [{
                    "code": "pm.things.views.empty-detail",
                    "name": "事项详情",
                    "copy_key": "access.views.things.emptyDetail",
                    "kind": "ui",
                    "components": ["frontend/router.tsx"],
                    "actions": [],
                }],
            }],
        }]

        errors = validate_catalog(catalog, FIXTURE_ROOT)

        self.assertTrue(any("view requires actions or child views" in error for error in errors), errors)
        self.assertTrue(any("no frontend view/capability placement" in error for error in errors), errors)

    def test_rejects_operation_without_tool_or_frontend_surface_decision(self) -> None:
        catalog = copy.deepcopy(self.catalog)
        catalog["operation_governance"]["domains"] = [
            {"owner": "things", "page_codes": ["pm.things"]},
        ]
        action = catalog["pages"][0]["modules"][0]["actions"][0]
        action["code"] = "pm.things.list.create"
        action["copy_key"] = "access.actions.things.list.create"

        errors = validate_catalog(catalog, FIXTURE_ROOT)

        self.assertTrue(any("no Tool adaptation decision" in error for error in errors), errors)
        self.assertTrue(any("no component/interface mapping" in error for error in errors), errors)
        self.assertTrue(any("no button location decision" in error for error in errors), errors)

    def test_rejects_false_frontend_permission_enforcement_claim(self) -> None:
        catalog = copy.deepcopy(self.catalog)
        catalog["operation_governance"]["domains"] = [
            {"owner": "things", "page_codes": ["pm.things"]},
        ]
        catalog["operation_governance"]["permission_views"] = [{
            "page_code": "pm.things",
            "views": [{
                "code": "pm.things.views.table",
                "name": "事项表格",
                "copy_key": "access.views.things.table",
                "kind": "ui",
                "components": ["frontend/router.tsx"],
                "actions": ["pm.things.list.view"],
            }],
        }]
        catalog["operation_governance"]["frontend_surfaces"] = [{
            "component": "frontend/router.tsx",
            "capability_owner": "frontend/operation-capabilities.ts",
            "status": "ui",
            "permission_enforcement": "implemented",
            "actions": ["pm.things.list.view"],
        }]
        catalog["operation_governance"]["button_locations"] = {
            "pm.things.list.view": ["事项管理 > 事项表格 > 查看"],
        }
        catalog["operation_governance"]["tool_not_applicable"] = {
            "pm.things.list.view": "no_protocol_neutral_contract",
        }
        catalog["operation_governance"]["tool_decision_reasons"] = {
            "no_protocol_neutral_contract": "尚无可复用的协议无关 Tool 契约。",
        }

        errors = validate_catalog(catalog, FIXTURE_ROOT)

        self.assertTrue(any("implemented capability owner must consume" in error for error in errors), errors)

    def test_validator_rejects_stale_generated_go(self) -> None:
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            for source in FIXTURE_ROOT.rglob("*"):
                if source.is_file():
                    destination = root / source.relative_to(FIXTURE_ROOT)
                    destination.parent.mkdir(parents=True, exist_ok=True)
                    destination.write_bytes(source.read_bytes())
            catalog_path = root / "catalog.json"
            catalog = json.loads(catalog_path.read_text(encoding="utf-8"))
            catalog["generation"] = {
                "go_output": "backend/internal/generated/accesscatalog/catalog_gen.go",
                "typescript_output": "frontend/src/generated/access-catalog.ts",
                "operation_matrix_output": "docs/access/business-action-matrix.md",
            }
            catalog_path.write_text(json.dumps(catalog), encoding="utf-8")
            for key in ("go_output", "typescript_output", "operation_matrix_output"):
                output = root / catalog["generation"][key]
                output.parent.mkdir(parents=True, exist_ok=True)
                output.write_text("stale\n", encoding="utf-8")

            errors = validate_path(catalog_path, root)

        self.assertTrue(any("generated access catalog is stale" in error for error in errors), errors)


if __name__ == "__main__":
    unittest.main()
