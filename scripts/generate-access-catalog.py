#!/usr/bin/env python3
"""Generate typed Go access-catalog data from contracts/access/catalog.json."""

from __future__ import annotations

import argparse
import json
from pathlib import Path

from access_catalog_generator import (
    GenerationError,
    generated_operation_matrix_output_path,
    generated_output_path,
    generated_typescript_output_path,
    render_go,
    render_operation_matrix,
    render_typescript,
)


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--root", type=Path, default=Path(__file__).resolve().parents[1])
    parser.add_argument("--catalog", type=Path)
    parser.add_argument("--check", action="store_true", help="fail when the checked-in generated file differs")
    args = parser.parse_args()
    root = args.root.resolve()
    catalog_path = (args.catalog or root / "contracts/access/catalog.json").resolve()
    try:
        catalog = json.loads(catalog_path.read_text(encoding="utf-8"))
        source_label = catalog_path.relative_to(root).as_posix()
        outputs = {
            generated_output_path(catalog, root): render_go(catalog, source_label),
            generated_typescript_output_path(catalog, root): render_typescript(catalog, source_label),
            generated_operation_matrix_output_path(catalog, root): render_operation_matrix(catalog, source_label),
        }
    except (OSError, json.JSONDecodeError, GenerationError, KeyError, TypeError, ValueError) as exc:
        print(f"access catalog generation failed: {exc}")
        return 1
    if args.check:
        stale = []
        for output, rendered in outputs.items():
            try:
                current = output.read_text(encoding="utf-8")
            except OSError:
                current = ""
            if current != rendered:
                stale.append(output.relative_to(root))
        if stale:
            for output in stale:
                print(f"generated access catalog is stale: {output}")
            return 1
        for output in outputs:
            print(f"generated access catalog is current: {output.relative_to(root)}")
        return 0
    for output, rendered in outputs.items():
        output.parent.mkdir(parents=True, exist_ok=True)
        output.write_text(rendered, encoding="utf-8")
        print(f"generated access catalog: {output.relative_to(root)}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
