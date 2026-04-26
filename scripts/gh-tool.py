#!/usr/bin/env python3

"""GitHub helper utility for LLM-friendly repository research.

This script provides a small CLI around GitHub REST API and local git commands.
It is designed to print concise Markdown-like output that is easy for an LLM to
consume.

Authentication:
  - Requests are performed with anonymous access (no authorization header).

Repository handling:
  - Repositories are cloned under --tmpdir and reused across calls.
  - Existing clones are fetched and can be switched to --ref when requested.
  - Cloning always uses --recurse-submodules.

Examples:
  # Clone and checkout a branch
  ./scripts/gh-tool.py clone --repo owner/repo --tmpdir ./tmp/gh --ref main

  # Search issues, pull requests, and commits
  ./scripts/gh-tool.py search --repo owner/repo --query "panic in parser" --limit 5

  # Retrieve issue content with comments/events
  ./scripts/gh-tool.py get --repo owner/repo --issue 123 --limit 20

  # Retrieve pull request content with comments/review comments/commits
  ./scripts/gh-tool.py get --repo owner/repo --pr 456 --limit 20

  # Show file history and related artifacts
  ./scripts/gh-tool.py file-history --repo owner/repo --path pkg/core/session.go --limit 10

Use --help for command details.
"""

from __future__ import annotations

import argparse
import json
import subprocess
import sys
from pathlib import Path
from typing import Any
from urllib import error, parse, request


API_BASE = "https://api.github.com"
DEFAULT_TMPDIR = ".cswdata/tmp/git"


def parse_repo(repo: str) -> tuple[str, str]:
    """Parse repository value into owner and repo name."""
    value = repo.strip()
    if value.startswith("git@github.com:"):
        value = value.split(":", 1)[1]
    elif value.startswith("https://github.com/") or value.startswith("http://github.com/"):
        parsed = parse.urlparse(value)
        value = parsed.path.lstrip("/")

    value = value.removesuffix(".git").strip("/")
    parts = value.split("/")
    if len(parts) != 2 or not parts[0] or not parts[1]:
        raise ValueError(f"Invalid repo '{repo}', expected owner/repo or GitHub URL")
    return parts[0], parts[1]


def repo_http_url(owner: str, repo: str) -> str:
    """Build HTTPS clone URL for repository."""
    return f"https://github.com/{owner}/{repo}.git"


def clone_path(tmpdir: Path, owner: str, repo: str) -> Path:
    """Return local clone path under tmp directory."""
    return tmpdir / owner / repo


def run_git(args: list[str], cwd: Path | None = None) -> str:
    """Run git command and return stdout, raising on failure."""
    cmd = ["git", *args]
    proc = subprocess.run(
        cmd,
        cwd=str(cwd) if cwd is not None else None,
        check=False,
        text=True,
        capture_output=True,
    )
    if proc.returncode != 0:
        stderr = proc.stderr.strip()
        stdout = proc.stdout.strip()
        details = stderr or stdout or "git command failed"
        raise RuntimeError(f"Git command failed: {' '.join(cmd)}\n{details}")
    return proc.stdout.strip()


def ensure_clone(tmpdir: Path, owner: str, repo: str, ref: str | None = None) -> Path:
    """Ensure repo exists locally and optionally checkout ref."""
    target = clone_path(tmpdir, owner, repo)
    target.parent.mkdir(parents=True, exist_ok=True)

    if (target / ".git").is_dir():
        run_git(["-C", str(target), "fetch", "--all", "--prune"])
    else:
        run_git([
            "clone",
            "--recurse-submodules",
            repo_http_url(owner, repo),
            str(target),
        ])

    run_git(["-C", str(target), "submodule", "update", "--init", "--recursive"])
    if ref:
        run_git(["-C", str(target), "checkout", ref])
    return target


def api_headers(extra: dict[str, str] | None = None) -> dict[str, str]:
    """Build headers for GitHub API requests."""
    headers = {
        "Accept": "application/vnd.github+json",
        "X-GitHub-Api-Version": "2022-11-28",
        "User-Agent": "gh-tool-script",
    }
    if extra:
        headers.update(extra)
    return headers


def api_get_json(url: str, extra_headers: dict[str, str] | None = None) -> Any:
    """Execute GET request and decode JSON response."""
    req = request.Request(url=url, headers=api_headers(extra_headers), method="GET")
    try:
        with request.urlopen(req, timeout=30) as resp:
            body = resp.read().decode("utf-8")
            return json.loads(body)
    except error.HTTPError as err:
        try:
            payload = err.read().decode("utf-8")
        except Exception:
            payload = ""
        raise RuntimeError(f"GitHub API error {err.code} for {url}: {payload}") from err
    except error.URLError as err:
        raise RuntimeError(f"Network error for {url}: {err}") from err


def fetch_search_items(url: str, limit: int, extra_headers: dict[str, str] | None = None) -> list[dict[str, Any]]:
    """Fetch top N search result items from search endpoint."""
    per_page = min(100, max(1, limit))
    params = parse.urlencode({"per_page": str(per_page), "page": "1"})
    data = api_get_json(f"{url}&{params}", extra_headers=extra_headers)
    items = data.get("items", [])
    if not isinstance(items, list):
        return []
    return items[:limit]


def is_rate_limit_error(err: RuntimeError) -> bool:
    """Return true when error indicates GitHub API rate limiting."""
    return "rate limit exceeded" in str(err).casefold()


def print_header(title: str) -> None:
    """Print Markdown section header."""
    print(f"## {title}")


def cmd_clone(args: argparse.Namespace) -> int:
    """Handle clone subcommand."""
    owner, repo = parse_repo(args.repo)
    local = ensure_clone(Path(args.tmpdir), owner, repo, args.ref)
    print_header("Clone")
    print(f"- Repository: `{owner}/{repo}`")
    print(f"- Local path: `{local}`")
    print(f"- Ref: `{args.ref or 'current default'}`")
    return 0


def cmd_search(args: argparse.Namespace) -> int:
    """Handle search subcommand."""
    owner, repo = parse_repo(args.repo)
    query = args.query.strip()
    limit = max(1, args.limit)
    kinds = set(args.types)

    print_header(f"Search in {owner}/{repo}")
    print(f"- Query: `{query}`")
    print(f"- Limit: `{limit}`")
    print()

    if "issues" in kinds:
        q = parse.quote_plus(f"{query} repo:{owner}/{repo} is:issue in:title,body,comments")
        url = f"{API_BASE}/search/issues?q={q}&sort=updated&order=desc"
        print("### Issues")
        try:
            items = fetch_search_items(url, limit)
            if not items:
                print("- No results")
            for item in items:
                print(f"- {item.get('html_url', '')}")
        except RuntimeError as err:
            if is_rate_limit_error(err):
                print("- Search unavailable: GitHub API rate limit exceeded for anonymous access")
            else:
                raise
        print()

    if "prs" in kinds:
        q = parse.quote_plus(f"{query} repo:{owner}/{repo} is:pr in:title,body,comments")
        url = f"{API_BASE}/search/issues?q={q}&sort=updated&order=desc"
        print("### Pull requests")
        try:
            items = fetch_search_items(url, limit)
            if not items:
                print("- No results")
            for item in items:
                print(f"- {item.get('html_url', '')}")
        except RuntimeError as err:
            if is_rate_limit_error(err):
                print("- Search unavailable: GitHub API rate limit exceeded for anonymous access")
            else:
                raise
        print()

    if "commits" in kinds:
        q = parse.quote_plus(f"{query} repo:{owner}/{repo}")
        url = f"{API_BASE}/search/commits?q={q}&sort=author-date&order=desc"
        print("### Commits")
        try:
            items = fetch_search_items(
                url,
                limit,
                extra_headers={"Accept": "application/vnd.github+json"},
            )
            if not items:
                print("- No results")
            for item in items:
                print(f"- {item.get('html_url', '')}")
        except RuntimeError as err:
            if is_rate_limit_error(err):
                print("- Search unavailable: GitHub API rate limit exceeded for anonymous access")
            else:
                raise
        print()

    return 0


def print_comments(comments: list[dict[str, Any]], title: str, limit: int) -> None:
    """Print comment list as Markdown bullets."""
    print(f"### {title}")
    if not comments:
        print("- No comments")
        print()
        return
    for comment in comments[:limit]:
        user = (comment.get("user") or {}).get("login", "unknown")
        url = comment.get("html_url", "")
        body = (comment.get("body") or "").strip().replace("\n", " ")
        snippet = body[:240] + ("..." if len(body) > 240 else "")
        print(f"- `{user}`: {url}")
        if snippet:
            print(f"  - {snippet}")
    print()


def cmd_get(args: argparse.Namespace) -> int:
    """Handle get subcommand."""
    owner, repo = parse_repo(args.repo)
    limit = max(1, args.limit)

    if args.issue is None and args.pr is None:
        raise ValueError("Provide at least one selector: --issue or --pr")

    print_header(f"Details from {owner}/{repo}")

    if args.issue is not None:
        issue_num = args.issue
        try:
            issue = api_get_json(f"{API_BASE}/repos/{owner}/{repo}/issues/{issue_num}")
            comments = api_get_json(
                f"{API_BASE}/repos/{owner}/{repo}/issues/{issue_num}/comments?per_page={min(limit, 100)}"
            )
            events = api_get_json(
                f"{API_BASE}/repos/{owner}/{repo}/issues/{issue_num}/events?per_page={min(limit, 100)}"
            )
        except RuntimeError as err:
            if is_rate_limit_error(err):
                print(f"### Issue #{issue_num}")
                print("- Details unavailable: GitHub API rate limit exceeded for anonymous access")
                print()
                issue = None
                comments = []
                events = []
            else:
                raise

        if issue is None:
            pass
        else:
            print(f"### Issue #{issue_num}")
            print(f"- URL: {issue.get('html_url', '')}")
            print(f"- Title: {issue.get('title', '')}")
            print(f"- State: {issue.get('state', '')}")
            print("- Description:")
            print("```text")
            print((issue.get("body") or "").strip())
            print("```")
            print()

            if isinstance(comments, list):
                print_comments(comments, "Issue comments", limit)

            print("### Issue events")
            if isinstance(events, list) and events:
                for event in events[:limit]:
                    actor = (event.get("actor") or {}).get("login", "unknown")
                    ev = event.get("event", "")
                    commit_id = event.get("commit_id") or ""
                    print(f"- `{ev}` by `{actor}` {commit_id}".rstrip())
            else:
                print("- No events")
            print()

            sub_url = f"{API_BASE}/repos/{owner}/{repo}/issues/{issue_num}/sub_issues?per_page={min(limit, 100)}"
            try:
                sub_issues = api_get_json(sub_url)
                print("### Sub-issues")
                if isinstance(sub_issues, list) and sub_issues:
                    for sub in sub_issues[:limit]:
                        print(f"- {sub.get('html_url', '')}")
                else:
                    print("- No sub-issues")
                print()
            except RuntimeError:
                print("### Sub-issues")
                print("- Not available for this repository or API access level")
                print()

    if args.pr is not None:
        pr_num = args.pr
        try:
            pr = api_get_json(f"{API_BASE}/repos/{owner}/{repo}/pulls/{pr_num}")
            issue_comments = api_get_json(
                f"{API_BASE}/repos/{owner}/{repo}/issues/{pr_num}/comments?per_page={min(limit, 100)}"
            )
            review_comments = api_get_json(
                f"{API_BASE}/repos/{owner}/{repo}/pulls/{pr_num}/comments?per_page={min(limit, 100)}"
            )
            commits = api_get_json(
                f"{API_BASE}/repos/{owner}/{repo}/pulls/{pr_num}/commits?per_page={min(limit, 100)}"
            )
        except RuntimeError as err:
            if is_rate_limit_error(err):
                print(f"### Pull request #{pr_num}")
                print("- Details unavailable: GitHub API rate limit exceeded for anonymous access")
                print()
                pr = None
                issue_comments = []
                review_comments = []
                commits = []
            else:
                raise

        if pr is None:
            pass
        else:
            print(f"### Pull request #{pr_num}")
            print(f"- URL: {pr.get('html_url', '')}")
            print(f"- Title: {pr.get('title', '')}")
            print(f"- State: {pr.get('state', '')}")
            print(f"- Base: `{(pr.get('base') or {}).get('ref', '')}`")
            print(f"- Head: `{(pr.get('head') or {}).get('ref', '')}`")
            print("- Description:")
            print("```text")
            print((pr.get("body") or "").strip())
            print("```")
            print()

            if isinstance(issue_comments, list):
                print_comments(issue_comments, "PR conversation comments", limit)
            if isinstance(review_comments, list):
                print_comments(review_comments, "PR review comments", limit)

            print("### PR commits")
            if isinstance(commits, list) and commits:
                for commit in commits[:limit]:
                    sha = commit.get("sha", "")
                    html_url = commit.get("html_url", "")
                    message = ((commit.get("commit") or {}).get("message") or "").splitlines()[0]
                    print(f"- `{sha}` {html_url}")
                    print(f"  - {message}")
            else:
                print("- No commits")
            print()

    return 0


def shas_for_file(repo_dir: Path, file_path: str, limit: int) -> list[str]:
    """Return newest commit SHAs that changed the given file path."""
    output = run_git(
        [
            "-C",
            str(repo_dir),
            "log",
            "--format=%H",
            f"-n{max(1, limit)}",
            "--",
            file_path,
        ]
    )
    return [line.strip() for line in output.splitlines() if line.strip()]


def cmd_file_history(args: argparse.Namespace) -> int:
    """Handle file-history subcommand."""
    owner, repo = parse_repo(args.repo)
    limit = max(1, args.limit)
    repo_dir = ensure_clone(Path(args.tmpdir), owner, repo, args.ref)
    shas = shas_for_file(repo_dir, args.path, limit)

    print_header(f"File history for `{args.path}` in {owner}/{repo}")
    print(f"- Local clone: `{repo_dir}`")
    print(f"- Ref: `{args.ref or 'current default'}`")
    print()

    print("### Commits")
    if not shas:
        print("- No matching commits")
        print()
        return 0

    for sha in shas:
        print(f"- https://github.com/{owner}/{repo}/commit/{sha}")
    print()

    try:
        commit_comments = api_get_json(f"{API_BASE}/repos/{owner}/{repo}/comments?per_page=100")
    except RuntimeError as err:
        if is_rate_limit_error(err):
            commit_comments = None
        else:
            raise
    print("### Commit comments")
    if isinstance(commit_comments, list):
        matched = [c for c in commit_comments if c.get("commit_id") in set(shas)]
        if not matched:
            print("- No commit comments for listed commits")
        else:
            for c in matched[:limit]:
                author = (c.get("user") or {}).get("login", "unknown")
                print(f"- `{author}` on `{c.get('commit_id', '')}`: {c.get('html_url', '')}")
    else:
        print("- Unable to fetch commit comments")
    print()

    print("### Related issues / pull requests by commit SHA mention")
    seen: set[str] = set()
    any_found = False
    for sha in shas[:limit]:
        short_sha = sha[:12]
        q = parse.quote_plus(f"{short_sha} repo:{owner}/{repo}")
        url = f"{API_BASE}/search/issues?q={q}&sort=updated&order=desc"
        try:
            items = fetch_search_items(url, limit)
        except RuntimeError as err:
            if is_rate_limit_error(err):
                print("- Related search unavailable: GitHub API rate limit exceeded for anonymous access")
                print()
                return 0
            raise
        for item in items:
            html_url = item.get("html_url", "")
            if html_url and html_url not in seen:
                any_found = True
                seen.add(html_url)
                print(f"- {html_url}")
    if not any_found:
        print("- No related issues or pull requests found")
    print()

    return 0


def build_parser() -> argparse.ArgumentParser:
    """Create command-line parser for gh-tool."""
    parser = argparse.ArgumentParser(
        description=__doc__,
        formatter_class=argparse.RawDescriptionHelpFormatter,
    )
    parser.add_argument(
        "--tmpdir",
        default=DEFAULT_TMPDIR,
        help=f"Directory used for local repository clones (default: {DEFAULT_TMPDIR})",
    )

    sub = parser.add_subparsers(dest="command", required=True)

    clone = sub.add_parser("clone", help="Clone repository locally (or refresh existing clone)")
    clone.add_argument("--repo", required=True, help="Repository in owner/repo or GitHub URL format")
    clone.add_argument("--ref", default=None, help="Optional branch/tag/commit to checkout")
    clone.set_defaults(func=cmd_clone)

    search = sub.add_parser("search", help="Search issues, pull requests, and commits")
    search.add_argument("--repo", required=True, help="Repository in owner/repo or GitHub URL format")
    search.add_argument("--query", required=True, help="Search query")
    search.add_argument("--limit", type=int, default=10, help="Max number of results per type")
    search.add_argument(
        "--types",
        nargs="+",
        choices=["issues", "prs", "commits"],
        default=["issues", "prs", "commits"],
        help="Result types to include (default: issues prs commits)",
    )
    search.set_defaults(func=cmd_search)

    get = sub.add_parser("get", help="Retrieve issue or pull request content and comments")
    get.add_argument("--repo", required=True, help="Repository in owner/repo or GitHub URL format")
    get.add_argument("--issue", type=int, default=None, help="Issue number")
    get.add_argument("--pr", type=int, default=None, help="Pull request number")
    get.add_argument("--limit", type=int, default=20, help="Max items for comments/events/commits")
    get.set_defaults(func=cmd_get)

    fh = sub.add_parser(
        "file-history",
        help="Search file change history and related GitHub artifacts",
    )
    fh.add_argument("--repo", required=True, help="Repository in owner/repo or GitHub URL format")
    fh.add_argument("--path", required=True, help="Path of file inside repository")
    fh.add_argument("--limit", type=int, default=20, help="Max commits/results to include")
    fh.add_argument("--ref", default=None, help="Optional branch/tag/commit to checkout before log")
    fh.set_defaults(func=cmd_file_history)

    return parser


def main() -> int:
    """Run gh-tool CLI."""
    parser = build_parser()
    args = parser.parse_args()

    try:
        return int(args.func(args))
    except Exception as err:  # noqa: BLE001
        print(f"## Error\n- {err}", file=sys.stderr)
        return 1


if __name__ == "__main__":
    raise SystemExit(main())
