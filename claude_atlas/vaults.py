"""Create new claude-obsidian vaults and register vaults as tree leaves."""

from __future__ import annotations

from pathlib import Path
from typing import Any

from .console import Console
from .errors import AtlasError
from .home import Config, display_path, tree_root
from .product import ClaudeObsidian, now_utc, operation_id
from .tree import Node, create_leaf, find_by_vault, slugify, walk


def resolve_new_vault_path(arg: str, vaults_dir: Path) -> Path:
    """A bare name lands in the vaults directory; anything path-like is a path."""
    if "/" in arg or arg.startswith(("~", ".")):
        return Path(arg).expanduser().resolve()
    return (vaults_dir / arg).resolve()


def create_vault(
    product: ClaudeObsidian, path: Path, console: Console, *, confirm: bool = True
) -> dict[str, Any]:
    """Run claude-obsidian's plan-then-apply init as one reviewed step."""
    if path.exists():
        raise AtlasError(
            f"{display_path(path)} already exists; use `claude-atlas vault add` "
            "to register an existing vault"
        )
    path.parent.mkdir(parents=True, exist_ok=True)
    generated_at = now_utc()
    operation = operation_id("init", generated_at)
    plan = product.init_plan(path, generated_at=generated_at, operation=operation)
    if plan.get("status") != "dry-run":
        raise AtlasError(f"unexpected init plan status {plan.get('status')!r}")
    changed = plan.get("changed_paths", [])
    if confirm:
        console.say(f"claude-obsidian will create {display_path(path)} with {len(changed)} files:")
        for item in changed:
            console.say(f"    {item}")
        console.say()
        if not console.confirm("Create this vault?"):
            raise AtlasError("cancelled")
    return product.init_apply(
        path,
        generated_at=generated_at,
        operation=operation,
        approval=str(plan["approved_plan_sha256"]),
    )


def register_vault(
    config: Config,
    vault: Path,
    *,
    name: str | None = None,
    parent: str | None = None,
    purpose: str = "",
    priority: str = "normal",
) -> Node:
    vault = vault.expanduser().resolve()
    if not (vault / ".claude-obsidian.json").is_file():
        raise AtlasError(
            f"{display_path(vault)} is not a claude-obsidian vault "
            "(no .claude-obsidian.json)"
        )
    root = tree_root(config)
    existing = find_by_vault(walk(root), vault)
    if existing is not None:
        raise AtlasError(f"{display_path(vault)} is already registered as {existing.rel}")
    display = name or vault.name
    directory = create_leaf(
        root,
        node_id=slugify(display),
        name=display,
        vault=vault,
        parent=parent,
        purpose=purpose,
        priority=priority,
    )
    from .tree import load_node

    return load_node(directory, root)
