#!/usr/bin/env python3

"""Convert YAML files passed as CLI arguments to JSON files."""

from __future__ import annotations

import argparse
import json
from pathlib import Path

import yaml


def output_path_for(input_path: Path) -> Path:
    """Return JSON output path for a YAML input path."""
    return input_path.with_suffix(".json")


def convert_file(input_path: Path) -> None:
    """Convert one YAML file to JSON in the same directory."""
    with input_path.open("r", encoding="utf-8") as input_file:
        yaml_data = yaml.safe_load(input_file)

    output_path = output_path_for(input_path)
    with output_path.open("w", encoding="utf-8") as output_file:
        json.dump(yaml_data, output_file, indent=2, ensure_ascii=False)
        output_file.write("\n")


def main() -> int:
    """Run yaml2json CLI."""
    parser = argparse.ArgumentParser(
        description="Convert YAML files to JSON files in the same directory.",
    )
    parser.add_argument("files", nargs="+", help="YAML file paths")
    args = parser.parse_args()

    failed = False
    for file_name in args.files:
        input_path = Path(file_name)
        try:
            convert_file(input_path)
        except Exception as err:
            failed = True
            print(f"Failed to convert {input_path}: {err}")

    return 1 if failed else 0


if __name__ == "__main__":
    raise SystemExit(main())
