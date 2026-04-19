#!/usr/bin/env python3

"""Print biggest Go files in cmd/ and pkg/ by line count."""

from __future__ import annotations

import argparse
from pathlib import Path


def count_lines(path: Path) -> int:
    """Return number of lines in file."""
    with path.open("r", encoding="utf-8") as file:
        return sum(1 for _ in file)


def collect_go_files(base_dirs: list[Path]) -> list[tuple[Path, int]]:
    """Collect Go files and their line counts from provided directories."""
    results: list[tuple[Path, int]] = []
    for base_dir in base_dirs:
        if not base_dir.exists():
            continue
        for path in base_dir.rglob("*.go"):
            if path.is_file():
                results.append((path, count_lines(path)))
    return results


def main() -> int:
    """Run bfscan CLI."""
    parser = argparse.ArgumentParser(
        description=(
            "Find biggest Go files in cmd/ and pkg/ directories, filtered by "
            "minimum line count."
        ),
    )
    parser.add_argument(
        "min_lines",
        type=int,
        help="Minimum line count required for a file to be reported",
    )
    parser.add_argument(
        "n",
        type=int,
        help="Number of biggest files to print",
    )
    args = parser.parse_args()

    if args.min_lines < 0:
        parser.error("min_lines must be non-negative")

    if args.n < 0:
        parser.error("n must be non-negative")

    base_dirs = [Path("cmd"), Path("pkg")]
    files_with_counts = collect_go_files(base_dirs)
    files_with_counts.sort(key=lambda entry: (-entry[1], str(entry[0])))
    filtered_files = [entry for entry in files_with_counts if entry[1] >= args.min_lines]

    for path, line_count in filtered_files[: args.n]:
        print(f"{path}\t{line_count}")

    return 0


if __name__ == "__main__":
    raise SystemExit(main())
