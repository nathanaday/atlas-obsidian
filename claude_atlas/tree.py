"""The node tree: authored node.json files and derived state.json files."""

from __future__ import annotations

import json
import re
from dataclasses import dataclass, field
from pathlib import Path
from typing import Any

from .errors import AtlasError

NODE_SCHEMA = "atlas.node.v1"
STATE_SCHEMA = "atlas.state.v1"
NODE_FILE = "node.json"
STATE_FILE = "state.json"
PRIORITIES = ("high", "normal", "low", "someday")
STATES = ("active", "paused", "blocked", "archived")


@dataclass
class Node:
    dir: Path
    rel: str
    id: str
    name: str
    kind: str
    vault: Path | None
    purpose: str = ""
    definition_of_done: str = ""
    priority: str = "normal"
    state: str = "active"
    blocked_on: str = ""
    review_after: str = ""
    repos: list[str] = field(default_factory=list)

    @property
    def is_leaf(self) -> bool:
        return self.kind == "leaf"

    @property
    def depth(self) -> int:
        return self.rel.count("/")


def slugify(name: str) -> str:
    slug = re.sub(r"[^a-z0-9]+", "-", name.lower()).strip("-")
    if not slug:
        raise AtlasError(f"cannot derive a node id from {name!r}")
    return slug


def _read_json(path: Path) -> dict[str, Any]:
    try:
        with path.open(encoding="utf-8") as handle:
            data = json.load(handle)
    except ValueError as exc:
        raise AtlasError(f"{path}: invalid JSON: {exc}") from exc
    if not isinstance(data, dict):
        raise AtlasError(f"{path}: expected a JSON object")
    return data


def _write_json(path: Path, data: dict[str, Any]) -> None:
    path.write_text(json.dumps(data, indent=2) + "\n", encoding="utf-8")


def load_node(directory: Path, tree_root: Path) -> Node:
    path = directory / NODE_FILE
    data = _read_json(path)
    if data.get("schema") != NODE_SCHEMA:
        raise AtlasError(f"{path}: unsupported schema {data.get('schema')!r}")
    kind = data.get("kind")
    if kind not in ("leaf", "cluster"):
        raise AtlasError(f"{path}: kind must be leaf or cluster")
    vault = data.get("vault")
    if kind == "leaf" and not vault:
        raise AtlasError(f"{path}: a leaf must name a vault")
    if kind == "cluster" and vault:
        raise AtlasError(f"{path}: a cluster must not name a vault")
    priority = data.get("priority", "normal")
    state = data.get("state", "active")
    if priority not in PRIORITIES:
        raise AtlasError(f"{path}: priority must be one of {', '.join(PRIORITIES)}")
    if state not in STATES:
        raise AtlasError(f"{path}: state must be one of {', '.join(STATES)}")
    rel = directory.relative_to(tree_root).as_posix() if directory != tree_root else ""
    return Node(
        dir=directory,
        rel=rel,
        id=str(data.get("id") or directory.name),
        name=str(data.get("name") or directory.name),
        kind=kind,
        vault=Path(vault).expanduser() if vault else None,
        purpose=str(data.get("purpose", "")),
        definition_of_done=str(data.get("definition_of_done", "")),
        priority=priority,
        state=state,
        blocked_on=str(data.get("blocked_on", "")),
        review_after=str(data.get("review_after", "")),
        repos=list(data.get("repos", [])),
    )


def walk(tree_root: Path) -> list[Node]:
    """Every node below the tree root, parents before children, sorted by path."""
    if not tree_root.is_dir():
        return []
    nodes: list[Node] = []
    found = [path.parent for path in tree_root.rglob(NODE_FILE) if path.parent != tree_root]
    for directory in sorted(found, key=lambda d: d.relative_to(tree_root).parts):
        nodes.append(load_node(directory, tree_root))
    vaults: dict[Path, Node] = {}
    for node in nodes:
        if node.vault is None:
            continue
        key = node.vault.resolve()
        if key in vaults:
            raise AtlasError(
                f"two leaves name the same vault {node.vault}: "
                f"{vaults[key].rel} and {node.rel}"
            )
        vaults[key] = node
    return nodes


def leaves(nodes: list[Node]) -> list[Node]:
    return [node for node in nodes if node.is_leaf]


def find_by_vault(nodes: list[Node], vault: Path) -> Node | None:
    target = vault.resolve()
    for node in nodes:
        if node.vault is not None and node.vault.resolve() == target:
            return node
    return None


def find_by_rel(nodes: list[Node], rel: str) -> Node | None:
    rel = rel.strip("/")
    for node in nodes:
        if node.rel == rel or node.id == rel:
            return node
    return None


def cluster_document(node_id: str, name: str) -> dict[str, Any]:
    return {
        "schema": NODE_SCHEMA,
        "id": node_id,
        "name": name,
        "kind": "cluster",
        "purpose": "",
        "priority": "normal",
        "state": "active",
    }


def leaf_document(
    node_id: str,
    name: str,
    vault: Path,
    *,
    purpose: str = "",
    priority: str = "normal",
) -> dict[str, Any]:
    return {
        "schema": NODE_SCHEMA,
        "id": node_id,
        "name": name,
        "kind": "leaf",
        "vault": str(vault),
        "purpose": purpose,
        "definition_of_done": "",
        "priority": priority,
        "state": "active",
        "blocked_on": "",
        "review_after": "",
        "repos": [],
        "outputs": "outputs/",
    }


def ensure_cluster_path(tree_root: Path, parent: str) -> Path:
    """Create cluster nodes for every missing segment of parent; return its dir."""
    directory = tree_root
    for segment in parent.strip("/").split("/"):
        if not segment:
            continue
        directory = directory / segment
        node_file = directory / NODE_FILE
        if node_file.is_file():
            existing = load_node(directory, tree_root)
            if existing.is_leaf:
                raise AtlasError(f"{existing.rel} is a leaf and cannot hold children")
            continue
        directory.mkdir(parents=True, exist_ok=True)
        _write_json(node_file, cluster_document(slugify(segment), segment))
    return directory


def create_leaf(
    tree_root: Path,
    *,
    node_id: str,
    name: str,
    vault: Path,
    parent: str | None = None,
    purpose: str = "",
    priority: str = "normal",
) -> Path:
    if priority not in PRIORITIES:
        raise AtlasError(f"priority must be one of {', '.join(PRIORITIES)}")
    container = ensure_cluster_path(tree_root, parent) if parent else tree_root
    directory = container / node_id
    if (directory / NODE_FILE).exists():
        rel = directory.relative_to(tree_root).as_posix()
        raise AtlasError(f"node {rel} already exists")
    directory.mkdir(parents=True, exist_ok=True)
    (directory / "outputs").mkdir(exist_ok=True)
    _write_json(
        directory / NODE_FILE,
        leaf_document(node_id, name, vault, purpose=purpose, priority=priority),
    )
    return directory


def read_state(directory: Path) -> dict[str, Any] | None:
    path = directory / STATE_FILE
    if not path.is_file():
        return None
    try:
        return _read_json(path)
    except AtlasError:
        return None


def write_state(directory: Path, state: dict[str, Any]) -> None:
    _write_json(directory / STATE_FILE, state)
