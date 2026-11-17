#!/usr/bin/env python3
"""CLI entry point for PM access catalog governance."""

from __future__ import annotations

import argparse
from pathlib import Path

from access_catalog_validator import CatalogError, validate_path


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--root", type=Path, default=Path(__file__).resolve().parents[1])
    parser.add_argument("--catalog", type=Path)
    parser.add_argument("--no-literal-scan", action="store_true")
    args = parser.parse_args()
    root = args.root.resolve()
    catalog = (args.catalog or root / "contracts/access/catalog.json").resolve()
    try:
        errors = validate_path(catalog, root, scan_literals=not args.no_literal_scan)
    except CatalogError as exc:
        errors = [str(exc)]
    if errors:
        print(f"access catalog validation failed ({len(errors)} error(s)):")
        for error in errors:
            print(f"- {error}")
        return 1
    print(f"access catalog valid: {catalog.relative_to(root)}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
