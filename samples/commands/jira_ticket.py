#!/usr/bin/env python3

"""JIRA helper for command-driven ticket workflow.

Usage:
  python3 samples/commands/jira_ticket.py <ticket-id>
  python3 samples/commands/jira_ticket.py <ticket-id> <new-status> <resolution>
"""

from __future__ import annotations

import json
import os
import sys
from pathlib import Path
from typing import Any
from urllib import error, parse, request


def _load_simple_yaml(path: Path) -> dict[str, str]:
    """Load a simple key:value YAML file without external dependencies."""
    data: dict[str, str] = {}
    for raw_line in path.read_text(encoding="utf-8").splitlines():
        line = raw_line.strip()
        if not line or line.startswith("#") or ":" not in line:
            continue
        key, value = line.split(":", 1)
        k = key.strip()
        v = value.strip()
        if len(v) >= 2 and ((v[0] == '"' and v[-1] == '"') or (v[0] == "'" and v[-1] == "'")):
            v = v[1:-1]
        data[k] = v
    return data


def _extract_text(node: Any) -> str:
    """Extract plain text from JIRA ADF nodes or primitive values."""
    if node is None:
        return ""
    if isinstance(node, str):
        return node
    if isinstance(node, (int, float, bool)):
        return str(node)
    if isinstance(node, list):
        parts = [_extract_text(item) for item in node]
        return "\n".join(p for p in parts if p)
    if isinstance(node, dict):
        node_type = node.get("type")
        if node_type == "text":
            return str(node.get("text", ""))

        content = node.get("content")
        parts: list[str] = []
        if isinstance(content, list):
            for item in content:
                part = _extract_text(item)
                if part:
                    parts.append(part)

        if node_type in {"paragraph", "heading"}:
            return "".join(parts).strip()
        if node_type in {"bulletList", "orderedList"}:
            return "\n".join(parts).strip()
        if node_type == "listItem":
            body = "\n".join(parts).strip()
            return f"- {body}" if body else ""
        if node_type == "hardBreak":
            return "\n"

        joined = "\n".join(p for p in parts if p).strip()
        if joined:
            return joined

        text_value = node.get("text")
        if isinstance(text_value, str):
            return text_value
    return ""


def _load_config(script_dir: Path) -> tuple[str, str]:
    """Load JIRA URL and API key from env or config files."""
    env_url = os.environ.get("JIRA_URL", "").strip()
    env_key = os.environ.get("JIRA_API_KEY", "").strip()
    if env_url and env_key:
        return env_url.rstrip("/"), env_key

    config_candidates = [
        script_dir / "jira_config.yaml",
        script_dir.parent / "hooks" / "jira_ticket" / "jira_config.yaml",
    ]
    for cfg_path in config_candidates:
        if not cfg_path.exists():
            continue
        cfg = _load_simple_yaml(cfg_path)
        url = cfg.get("jira_url", "").strip().rstrip("/")
        key = cfg.get("jira_api_key", "").strip()
        if url and key:
            return url, key

    raise ValueError(
        "_load_config() [jira_ticket.py]: missing JIRA credentials; set JIRA_URL/JIRA_API_KEY "
        "or provide jira_config.yaml"
    )


def _http_json(method: str, url: str, api_key: str, payload: Any | None = None) -> Any:
    """Send JSON request and parse JSON response body when present."""
    body: bytes | None = None
    headers = {
        "Accept": "application/json",
        "Authorization": f"Bearer {api_key}",
    }
    if payload is not None:
        body = json.dumps(payload).encode("utf-8")
        headers["Content-Type"] = "application/json"

    req = request.Request(url, data=body, headers=headers, method=method)
    with request.urlopen(req, timeout=30) as resp:
        raw = resp.read().decode("utf-8")
        if not raw:
            return {}
        return json.loads(raw)


def _format_issue(issue_data: dict[str, Any], comments_data: dict[str, Any]) -> str:
    """Format issue details and comments in LLM-friendly plain text."""
    fields = issue_data.get("fields") or {}
    key = str(issue_data.get("key") or "")
    summary = str(fields.get("summary") or "")
    status = str(((fields.get("status") or {}).get("name")) or "")
    issue_type = str(((fields.get("issuetype") or {}).get("name")) or "")
    priority = str(((fields.get("priority") or {}).get("name")) or "")

    reporter_obj = fields.get("reporter") or {}
    reporter = str(
        reporter_obj.get("displayName")
        or reporter_obj.get("name")
        or reporter_obj.get("emailAddress")
        or ""
    )
    assignee_obj = fields.get("assignee") or {}
    assignee = str(
        assignee_obj.get("displayName")
        or assignee_obj.get("name")
        or assignee_obj.get("emailAddress")
        or ""
    )

    description = _extract_text(fields.get("description"))

    lines = [
        f"Ticket: {key}",
        f"Summary: {summary}",
        f"Status: {status}",
        f"Type: {issue_type}",
        f"Priority: {priority}",
        f"Reporter: {reporter}",
        f"Assignee: {assignee}",
        "",
        "Description:",
        description or "(empty)",
        "",
        "Comments:",
    ]

    comments = comments_data.get("comments", []) or []
    if not comments:
        lines.append("(no comments)")
        return "\n".join(lines)

    for idx, item in enumerate(comments, start=1):
        author = (item.get("author") or {}).get("displayName") or "unknown"
        created = str(item.get("created") or "")
        body = _extract_text(item.get("body")) or "(empty)"
        lines.append(f"{idx}. [{author} | {created}]")
        lines.append(body)
        lines.append("")

    return "\n".join(lines).rstrip()


def _find_transition_id(transitions_payload: dict[str, Any], target_status: str) -> str:
    """Find transition ID by target status name (case-insensitive)."""
    wanted = target_status.strip().casefold()
    for item in transitions_payload.get("transitions", []) or []:
        name = str(item.get("name") or "")
        to_name = str(((item.get("to") or {}).get("name")) or "")
        if name.casefold() == wanted or to_name.casefold() == wanted:
            return str(item.get("id") or "")
    return ""


def _update_issue(base_url: str, api_key: str, ticket_id: str, new_status: str, resolution: str) -> None:
    """Transition issue status and post resolution comment."""
    encoded_ticket = parse.quote(ticket_id, safe="")
    transitions_url = f"{base_url}/rest/api/3/issue/{encoded_ticket}/transitions"
    comment_url = f"{base_url}/rest/api/3/issue/{encoded_ticket}/comment"

    transitions = _http_json("GET", transitions_url, api_key)
    transition_id = _find_transition_id(transitions, new_status)
    if not transition_id:
        available = [str(item.get("name") or "") for item in transitions.get("transitions", []) or []]
        raise ValueError(
            "_update_issue() [jira_ticket.py]: status transition not found for "
            f"{new_status!r}. Available transitions: {', '.join(available)}"
        )

    _http_json("POST", transitions_url, api_key, payload={"transition": {"id": transition_id}})

    comment_payload = {
        "body": {
            "version": 1,
            "type": "doc",
            "content": [
                {
                    "type": "paragraph",
                    "content": [
                        {
                            "type": "text",
                            "text": resolution,
                        }
                    ],
                }
            ],
        }
    }
    _http_json("POST", comment_url, api_key, payload=comment_payload)


def main() -> int:
    """CLI entrypoint."""
    args = sys.argv[1:]
    if len(args) not in {1, 3}:
        print(
            "Usage:\n"
            "  jira_ticket.py <ticket-id>\n"
            "  jira_ticket.py <ticket-id> <new-status> <resolution>",
            file=sys.stderr,
        )
        return 1

    ticket_id = args[0].strip()
    if not ticket_id:
        print("main() [jira_ticket.py]: ticket-id cannot be empty", file=sys.stderr)
        return 1

    script_dir = Path(__file__).resolve().parent
    try:
        base_url, api_key = _load_config(script_dir)
    except ValueError as exc:
        print(str(exc), file=sys.stderr)
        return 1

    encoded_ticket = parse.quote(ticket_id, safe="")
    issue_url = f"{base_url}/rest/api/3/issue/{encoded_ticket}"
    comments_url = f"{base_url}/rest/api/3/issue/{encoded_ticket}/comment"

    try:
        if len(args) == 1:
            issue_data = _http_json("GET", issue_url, api_key)
            comments_data = _http_json("GET", comments_url, api_key)
            print(_format_issue(issue_data, comments_data))
            return 0

        new_status = args[1].strip()
        resolution = args[2].strip()
        if not new_status:
            print("main() [jira_ticket.py]: new-status cannot be empty", file=sys.stderr)
            return 1
        if not resolution:
            print("main() [jira_ticket.py]: resolution cannot be empty", file=sys.stderr)
            return 1

        _update_issue(base_url, api_key, ticket_id, new_status, resolution)
        print(f"Updated {ticket_id}: status -> {new_status}; comment added.")
        return 0
    except error.HTTPError as exc:
        print(f"main() [jira_ticket.py]: JIRA HTTP error {exc.code}: {exc.reason}", file=sys.stderr)
        return 1
    except error.URLError as exc:
        print(f"main() [jira_ticket.py]: JIRA request failed: {exc.reason}", file=sys.stderr)
        return 1
    except json.JSONDecodeError as exc:
        print(f"main() [jira_ticket.py]: invalid JSON from JIRA: {exc}", file=sys.stderr)
        return 1
    except ValueError as exc:
        print(str(exc), file=sys.stderr)
        return 1


if __name__ == "__main__":
    raise SystemExit(main())
