#!/usr/bin/env python3

"""jira_ticket hook script.

Reads CSW_JIRA_TICKET from hook context environment, fetches issue details and comments
from JIRA REST API, and writes extracted values back through CSWFEEDBACK context.
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
        if not line or line.startswith("#"):
            continue
        if ":" not in line:
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

        parts: list[str] = []
        content = node.get("content")
        if isinstance(content, list):
            for item in content:
                text = _extract_text(item)
                if text:
                    parts.append(text)

        if node_type in {"paragraph", "heading"}:
            return "".join(parts).strip()
        if node_type in {"bulletList", "orderedList"}:
            return "\n".join(parts).strip()
        if node_type == "listItem":
            body = "\n".join(parts).strip()
            if body:
                return f"- {body}"
            return ""
        if node_type == "hardBreak":
            return "\n"

        joined = "\n".join(p for p in parts if p).strip()
        if joined:
            return joined

        text_value = node.get("text")
        if isinstance(text_value, str):
            return text_value
    return ""


def _http_get_json(url: str, api_key: str) -> Any:
    """Send GET request and parse JSON response."""
    req = request.Request(
        url,
        headers={
            "Accept": "application/json",
            "Authorization": f"Bearer {api_key}",
        },
        method="GET",
    )
    with request.urlopen(req, timeout=30) as resp:
        payload = resp.read().decode("utf-8")
        return json.loads(payload)


def _build_comments(comments_payload: dict[str, Any]) -> str:
    """Build a readable comment block from JIRA comments payload."""
    entries: list[str] = []
    for item in comments_payload.get("comments", []) or []:
        author = (item.get("author") or {}).get("displayName") or "unknown"
        created = item.get("created") or ""
        body = _extract_text(item.get("body"))
        block = f"[{author} | {created}]\n{body}".strip()
        if block:
            entries.append(block)
    return "\n\n".join(entries)


def _emit_context_feedback(context: dict[str, str]) -> None:
    """Emit context update through CSWFEEDBACK channel."""
    message = {
        "fn": "context",
        "args": context,
    }
    print(f"CSWFEEDBACK: {json.dumps(message, ensure_ascii=False)}")


def main() -> int:
    """Run jira ticket fetch and emit hook context."""
    ticket_id = os.environ.get("CSW_JIRA_TICKET", "").strip()
    if not ticket_id:
        print("jira_ticket.py: missing CSW_JIRA_TICKET", file=sys.stderr)
        return 1

    script_dir = Path(__file__).resolve().parent
    cfg_path = script_dir / "jira_config.yaml"
    if not cfg_path.exists():
        print(f"jira_ticket.py: missing config file: {cfg_path}", file=sys.stderr)
        return 1

    cfg = _load_simple_yaml(cfg_path)
    base_url = cfg.get("jira_url", "").rstrip("/")
    api_key = cfg.get("jira_api_key", "").strip()
    if not base_url or not api_key:
        print("jira_ticket.py: jira_url or jira_api_key is missing in jira_config.yaml", file=sys.stderr)
        return 1

    encoded_ticket = parse.quote(ticket_id, safe="")
    issue_url = f"{base_url}/rest/api/3/issue/{encoded_ticket}"
    comments_url = f"{base_url}/rest/api/3/issue/{encoded_ticket}/comment"

    try:
        issue_data = _http_get_json(issue_url, api_key)
        comments_data = _http_get_json(comments_url, api_key)
    except error.HTTPError as exc:
        print(f"jira_ticket.py: JIRA HTTP error {exc.code}: {exc.reason}", file=sys.stderr)
        return 1
    except error.URLError as exc:
        print(f"jira_ticket.py: JIRA request failed: {exc.reason}", file=sys.stderr)
        return 1
    except json.JSONDecodeError as exc:
        print(f"jira_ticket.py: invalid JSON from JIRA: {exc}", file=sys.stderr)
        return 1

    fields = issue_data.get("fields") or {}
    jira_title = str(fields.get("summary") or "")
    jira_description = _extract_text(fields.get("description"))
    jira_status = str(((fields.get("status") or {}).get("name")) or "")
    reporter_obj = fields.get("reporter") or {}
    jira_reporter = str(
        reporter_obj.get("displayName")
        or reporter_obj.get("name")
        or reporter_obj.get("emailAddress")
        or ""
    )
    jira_comments = _build_comments(comments_data)

    _emit_context_feedback(
        {
            "jira_title": jira_title,
            "jira_description": jira_description,
            "jira_comments": jira_comments,
            "jira_status": jira_status,
            "jira_reporter": jira_reporter,
        }
    )
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
