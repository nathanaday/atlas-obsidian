"""Derive state.json for every node and render the atlas page."""

from __future__ import annotations

import os
import re
from datetime import date, datetime
from pathlib import Path
from typing import Any

from .home import Config, display_path, tree_root
from .obsidian import open_uri
from .product import ClaudeObsidian, ProductError, now_utc
from .tree import STATE_SCHEMA, Node, walk, write_state

HOT_DAYS = 7
WARM_DAYS = 30
HEAT_ORDER = {"hot": 0, "warm": 1, "cold": 2, None: 3}
PRIORITY_ORDER = {"high": 0, "normal": 1, "low": 2, "someday": 3}
UNFINISHED_KEYS = ("empty_sections", "seed_pages", "dead_links")

_LOG_HEADING = re.compile(r"^##\s+(\d{4}-\d{2}-\d{2})\b", re.MULTILINE)
_WIKILINK = re.compile(r"\[\[([^\]|]+)(?:\|([^\]]+))?\]\]")
_STATUS_LINE = re.compile(r"^status:\s*(.+?)\s*$", re.MULTILINE)


def heat_of(days_idle: int | None) -> str | None:
    if days_idle is None:
        return None
    if days_idle < HOT_DAYS:
        return "hot"
    if days_idle < WARM_DAYS:
        return "warm"
    return "cold"


def newest_log_date(vault: Path) -> date | None:
    log = vault / "wiki" / "log.md"
    if not log.is_file():
        return None
    dates: list[date] = []
    for match in _LOG_HEADING.finditer(log.read_text(encoding="utf-8", errors="replace")):
        try:
            dates.append(date.fromisoformat(match.group(1)))
        except ValueError:
            continue
    return max(dates) if dates else None


def newest_wiki_mtime(vault: Path) -> date | None:
    wiki = vault / "wiki"
    if not wiki.is_dir():
        return None
    newest: float | None = None
    for root, _dirs, files in os.walk(wiki):
        for name in files:
            try:
                stamp = os.stat(os.path.join(root, name)).st_mtime
            except OSError:
                continue
            if newest is None or stamp > newest:
                newest = stamp
    return datetime.fromtimestamp(newest).date() if newest is not None else None


def active_threads(vault: Path) -> list[str]:
    """Bullets under `## Active Threads` in wiki/hot.md; prose, so best effort."""
    hot = vault / "wiki" / "hot.md"
    if not hot.is_file():
        return []
    threads: list[str] = []
    inside = False
    for line in hot.read_text(encoding="utf-8", errors="replace").splitlines():
        if line.startswith("## "):
            inside = line[3:].strip().lower() == "active threads"
            continue
        if not inside:
            continue
        stripped = line.strip()
        if stripped.startswith(("- ", "* ")):
            threads.append(stripped[2:].strip())
        elif stripped and threads and not stripped.startswith("#"):
            threads[-1] += " " + stripped
    return threads


def plain_text(text: str) -> str:
    """Strip wikilinks: a link copied from another vault resolves to nothing here."""
    return _WIKILINK.sub(lambda m: m.group(2) or m.group(1), text)


def frontmatter(text: str) -> str | None:
    if not text.startswith("---\n"):
        return None
    end = text.find("\n---", 4)
    return text[4:end] if end != -1 else None


def seed_pages(vault: Path) -> int:
    wiki = vault / "wiki"
    if not wiki.is_dir():
        return 0
    count = 0
    for page in wiki.rglob("*.md"):
        try:
            block = frontmatter(page.read_text(encoding="utf-8", errors="replace"))
        except OSError:
            continue
        if block is None:
            continue
        match = _STATUS_LINE.search(block)
        if match and match.group(1).strip("\"'") == "seed":
            count += 1
    return count


def _base_state(generated_at: str) -> dict[str, Any]:
    return {
        "schema": STATE_SCHEMA,
        "generated_at": generated_at,
        "vault_ok": False,
        "vault_error": "",
        "last_operation": None,
        "last_touched": None,
        "days_idle": None,
        "heat": None,
        "pages": None,
        "open_threads": [],
        "unfinished": {key: None for key in UNFINISHED_KEYS},
    }


def derive_leaf(
    product: ClaudeObsidian | None, node: Node, *, today: date, generated_at: str
) -> dict[str, Any]:
    state = _base_state(generated_at)
    vault = node.vault
    assert vault is not None
    if not (vault / ".claude-obsidian.json").is_file():
        state["vault_error"] = (
            "not found" if not vault.exists() else "not a claude-obsidian vault"
        )
        return state

    last_operation = newest_log_date(vault)
    touched = [value for value in (last_operation, newest_wiki_mtime(vault)) if value]
    last_touched = max(touched) if touched else None
    days_idle = (today - last_touched).days if last_touched else None
    state.update(
        last_operation=last_operation.isoformat() if last_operation else None,
        last_touched=last_touched.isoformat() if last_touched else None,
        days_idle=days_idle,
        heat=heat_of(days_idle),
        open_threads=active_threads(vault),
    )
    state["unfinished"]["seed_pages"] = seed_pages(vault)

    if product is None:
        state["vault_error"] = "claude-obsidian is not installed"
        return state
    try:
        doctor = product.doctor(vault)
    except ProductError as exc:
        state["vault_error"] = str(exc)
        return state
    if not doctor.get("ok"):
        failed = [key for key, ok in doctor.get("checks", {}).items() if not ok]
        state["vault_error"] = "doctor: " + ", ".join(failed)
        return state
    try:
        summary = product.lint(vault).get("summary", {})
    except ProductError as exc:
        state["vault_error"] = str(exc)
        return state
    counts = summary.get("category_counts", {})
    state["vault_ok"] = True
    state["pages"] = summary.get("pages_scanned")
    state["unfinished"]["empty_sections"] = counts.get("empty_sections", 0)
    state["unfinished"]["dead_links"] = counts.get("dead_links", 0)
    return state


def derive_cluster(children: list[dict[str, Any]], *, generated_at: str) -> dict[str, Any]:
    state = _base_state(generated_at)
    state["vault_ok"] = bool(children) and all(child["vault_ok"] for child in children)
    state["vault_error"] = "" if state["vault_ok"] else "one or more vaults unreachable"
    operations = [c["last_operation"] for c in children if c["last_operation"]]
    touched = [c["last_touched"] for c in children if c["last_touched"]]
    idle = [c["days_idle"] for c in children if c["days_idle"] is not None]
    pages = [c["pages"] for c in children if c["pages"] is not None]
    state["last_operation"] = max(operations) if operations else None
    state["last_touched"] = max(touched) if touched else None
    state["days_idle"] = min(idle) if idle else None
    state["heat"] = heat_of(state["days_idle"])
    state["pages"] = sum(pages) if pages else None
    state["open_threads"] = [thread for c in children for thread in c["open_threads"]]
    for key in UNFINISHED_KEYS:
        values = [c["unfinished"][key] for c in children if c["unfinished"][key] is not None]
        state["unfinished"][key] = sum(values) if values else None
    state["leaves"] = sum(1 + c.get("leaves", 0) if "leaves" in c else 1 for c in children)
    return state


def refresh_tree(
    config: Config,
    product: ClaudeObsidian | None,
    *,
    today: date | None = None,
    generated_at: str | None = None,
) -> list[tuple[Node, dict[str, Any]]]:
    today = today or date.today()
    generated_at = generated_at or now_utc()
    nodes = walk(tree_root(config))
    states: dict[str, dict[str, Any]] = {}
    for node in sorted(nodes, key=lambda n: -n.depth):
        if node.is_leaf:
            state = derive_leaf(product, node, today=today, generated_at=generated_at)
        else:
            prefix = node.rel + "/"
            children = [
                states[other.rel]
                for other in nodes
                if other.rel.startswith(prefix) and other.rel.count("/") == node.depth + 1
            ]
            state = derive_cluster(children, generated_at=generated_at)
        write_state(node.dir, state)
        states[node.rel] = state
    return [(node, states[node.rel]) for node in nodes]


def _row_key(item: tuple[Node, dict[str, Any]]) -> tuple:
    node, state = item
    return (HEAT_ORDER[state["heat"]], PRIORITY_ORDER[node.priority], node.rel)


def _idle(state: dict[str, Any]) -> str:
    days = state["days_idle"]
    if days is None:
        return "—"
    if days == 0:
        return "today"
    return f"{days}d"


def _unfinished_total(state: dict[str, Any]) -> str:
    values = [v for v in state["unfinished"].values() if v is not None]
    return str(sum(values)) if values else "—"


def signals(node: Node, state: dict[str, Any], *, today: date) -> list[str]:
    notes: list[str] = []
    if not state["vault_ok"]:
        notes.append(f"vault unreachable: {state['vault_error']}")
    if state["heat"] == "cold" and node.state == "active" and node.priority in ("high", "normal"):
        notes.append(
            f"declared priority {node.priority}, active, but cold for {state['days_idle']} days"
        )
    if node.state == "blocked":
        notes.append("blocked on: " + (node.blocked_on or "(nothing recorded)"))
    if node.review_after:
        try:
            if date.fromisoformat(node.review_after) < today:
                notes.append(f"review date {node.review_after} has passed; intent may be stale")
        except ValueError:
            notes.append(f"review_after {node.review_after!r} is not a date")
    return notes


def render_atlas(
    rows: list[tuple[Node, dict[str, Any]]], *, generated_at: str, today: date
) -> str:
    stamp = datetime.fromisoformat(generated_at.replace("Z", "+00:00")).astimezone()
    leaf_rows = [(n, s) for n, s in rows if n.is_leaf]
    heats = [s["heat"] for _, s in leaf_rows]
    summary = " · ".join(
        part
        for part in (
            f"{len(leaf_rows)} vault{'s' if len(leaf_rows) != 1 else ''}",
            f"{heats.count('hot')} hot" if heats.count("hot") else "",
            f"{heats.count('warm')} warm" if heats.count("warm") else "",
            f"{heats.count('cold')} cold" if heats.count("cold") else "",
            f"{heats.count(None)} unreachable" if heats.count(None) else "",
        )
        if part
    )
    out: list[str] = [
        "---",
        "title: Atlas",
        f"generated_at: {generated_at}",
        "---",
        "",
        "# Atlas",
        "",
        f"{summary} · refreshed {stamp.strftime('%Y-%m-%d %H:%M')}",
        "",
        "This page is generated by `claude-atlas refresh`. Edit intent in each",
        "node's `node.json`; everything else here is derived and will be overwritten.",
        "",
        "| Heat | Node | Priority | State | Idle | Pages | Threads | Unfinished | |",
        "|:--|:--|:--|:--|--:|--:|--:|--:|:--|",
    ]
    for node, state in sorted(rows, key=_row_key):
        label = node.rel + ("/" if not node.is_leaf else "")
        link = f"[open]({open_uri(node.vault)})" if node.vault else ""
        out.append(
            "| "
            + " | ".join(
                (
                    state["heat"] or "off",
                    f"[[#{node.rel}|{label}]]",
                    node.priority,
                    node.state,
                    _idle(state),
                    str(state["pages"]) if state["pages"] is not None else "—",
                    str(len(state["open_threads"])),
                    _unfinished_total(state),
                    link,
                )
            )
            + " |"
        )
    flagged = [(n, note) for n, s in rows for note in signals(n, s, today=today)]
    if flagged:
        out += ["", "## Signals", ""]
        out += [f"- **{n.rel}** — {note}" for n, note in flagged]
    for node, state in rows:
        out += ["", f"## {node.rel}", ""]
        if node.vault:
            line = f"`{display_path(node.vault)}` · [open]({open_uri(node.vault)})"
        else:
            line = f"cluster · {state.get('leaves', 0)} vaults"
        if state["last_touched"]:
            line += f" · last touched {state['last_touched']} ({_idle(state)})"
        if state["pages"] is not None:
            line += f" · {state['pages']} pages"
        out.append(line)
        if node.purpose:
            out += ["", node.purpose]
        if node.definition_of_done:
            out += ["", f"**Done when:** {node.definition_of_done}"]
        if state["open_threads"]:
            out += ["", "**Open threads**", ""]
            out += [f"- {plain_text(thread)}" for thread in state["open_threads"]]
        unfinished = state["unfinished"]
        if any(value is not None for value in unfinished.values()):
            parts = [
                f"{key.replace('_', ' ')} {value}"
                for key, value in unfinished.items()
                if value is not None
            ]
            out += ["", "**Unfinished** " + " · ".join(parts)]
        if node.repos:
            out += ["", "**Repos** " + " · ".join(node.repos)]
    out.append("")
    return "\n".join(out)


def refresh(config: Config, product: ClaudeObsidian | None, *, today: date | None = None) -> Path:
    today = today or date.today()
    generated_at = now_utc()
    rows = refresh_tree(config, product, today=today, generated_at=generated_at)
    page = config.atlas_vault / "Atlas.md"
    config.atlas_vault.mkdir(parents=True, exist_ok=True)
    page.write_text(render_atlas(rows, generated_at=generated_at, today=today), encoding="utf-8")
    return page
