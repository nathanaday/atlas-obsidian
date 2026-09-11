"""claude-atlas command line."""

from __future__ import annotations

import argparse
import sys
from pathlib import Path

from . import __version__, plugin, product
from .console import Console
from .errors import AtlasError
from .home import Home, display_path, tree_root
from .obsidian import open_uri, open_vault
from .refresh import refresh
from .tree import PRIORITIES, find_by_rel, leaves, read_state, walk
from .vaults import create_vault, register_vault, resolve_new_vault_path
from .wizard import run_setup


def _core(config) -> product.ClaudeObsidian:
    if not product.is_installed(config.product_path):
        raise AtlasError(
            f"claude-obsidian v{product.PRODUCT_VERSION} is not installed at "
            f"{display_path(config.product_path)}; run `claude-atlas setup`"
        )
    return product.ClaudeObsidian(config.product_path)


def _add_node_options(parser: argparse.ArgumentParser) -> None:
    parser.add_argument("--parent", help="cluster path to place the node under, e.g. work")
    parser.add_argument("--purpose", default="", help="one paragraph: why this vault exists")
    parser.add_argument("--priority", choices=PRIORITIES, default="normal")


def cmd_setup(args: argparse.Namespace, console: Console) -> int:
    home = Home.resolve(args.home)
    return run_setup(
        home,
        console,
        vaults_dir=args.vaults_dir,
        first_vault=args.first_vault,
        with_plugin=not args.no_plugin,
        open_after=False if args.no_open else None,
    )


def cmd_vault_new(args: argparse.Namespace, console: Console) -> int:
    config = Home.resolve(args.home).load_config()
    core = _core(config)
    path = resolve_new_vault_path(args.name, config.vaults_dir)
    create_vault(core, path, console)
    node = register_vault(
        config, path, parent=args.parent, purpose=args.purpose, priority=args.priority
    )
    page = refresh(config, core)
    console.say()
    console.step("created", display_path(path))
    console.step("registered", f"node {node.rel}")
    console.step("refreshed", display_path(page))
    console.say()
    console.say(f"  {open_uri(path)}")
    console.say(f"  cd {display_path(path)} && claude    # then /claude-obsidian:wiki")
    console.say()
    if not args.no_open and console.confirm("Open it in Obsidian now?"):
        open_vault(path)
    return 0


def cmd_vault_add(args: argparse.Namespace, console: Console) -> int:
    config = Home.resolve(args.home).load_config()
    node = register_vault(
        config,
        Path(args.path),
        name=args.name,
        parent=args.parent,
        purpose=args.purpose,
        priority=args.priority,
    )
    page = refresh(config, _core(config))
    console.step("registered", f"{display_path(node.vault)} → node {node.rel}")
    console.step("refreshed", display_path(page))
    return 0


def cmd_vault_list(args: argparse.Namespace, console: Console) -> int:
    config = Home.resolve(args.home).load_config()
    nodes = walk(tree_root(config))
    if not nodes:
        console.say("no vaults registered; run `claude-atlas vault new <name>`")
        return 0
    for node in nodes:
        state = read_state(node.dir) or {}
        heat = state.get("heat") or ("off" if state else "?")
        target = display_path(node.vault) if node.vault else "(cluster)"
        console.say(f"  {heat:<5} {node.priority:<7} {node.state:<8} {node.rel:<32} {target}")
    return 0


def cmd_refresh(args: argparse.Namespace, console: Console) -> int:
    config = Home.resolve(args.home).load_config()
    page = refresh(config, _core(config))
    for node in walk(tree_root(config)):
        state = read_state(node.dir) or {}
        if node.is_leaf and not state.get("vault_ok"):
            console.step(node.rel, state.get("vault_error", ""), status="fail")
        else:
            console.step(node.rel, f"{state.get('heat') or '-'}, idle {state.get('days_idle')}d", status="ok")
    console.say(f"  wrote {display_path(page)}")
    return 0


def cmd_open(args: argparse.Namespace, console: Console) -> int:
    config = Home.resolve(args.home).load_config()
    if args.node:
        node = find_by_rel(walk(tree_root(config)), args.node)
        if node is None or node.vault is None:
            raise AtlasError(f"no leaf node named {args.node!r}")
        target = node.vault
    else:
        target = config.atlas_vault
    if not open_vault(target):
        console.say(open_uri(target))
    return 0


def cmd_doctor(args: argparse.Namespace, console: Console) -> int:
    home = Home.resolve(args.home)
    console.say(f"  home            {display_path(home.root)}  {'ok' if home.exists() else 'missing'}")
    if not home.exists():
        return 1
    config = home.load_config()
    version = product.installed_version(config.product_path)
    problems = product.verify_tree(config.product_path) if version else ["not installed"]
    console.say(
        f"  claude-obsidian {display_path(config.product_path)}  "
        f"v{version or '?'}  {'verified' if not problems else problems[0]}"
    )
    if plugin.claude_cli():
        console.say(f"  Claude Code     {plugin.PLUGIN_ID}  {'installed' if plugin.plugin_present() else 'not installed'}")
    else:
        console.say("  Claude Code     `claude` not on PATH")
    console.say(f"  atlas vault     {display_path(config.atlas_vault)}  {'ok' if (config.atlas_vault / '.obsidian').is_dir() else 'missing'}")
    console.say(f"  vaults dir      {display_path(config.vaults_dir)}")
    nodes = leaves(walk(tree_root(config)))
    console.say(f"  vaults          {len(nodes)} registered")
    ok = not problems
    for node in nodes:
        reachable = node.vault is not None and (node.vault / ".claude-obsidian.json").is_file()
        ok = ok and reachable
        console.say(f"    {'ok ' if reachable else 'off'} {node.rel:<24} {display_path(node.vault)}")
    return 0 if ok else 1


def build_parser() -> argparse.ArgumentParser:
    parser = argparse.ArgumentParser(
        prog="claude-atlas",
        description="One view across many claude-obsidian vaults.",
    )
    parser.add_argument("--version", action="version", version=__version__)
    parser.add_argument("--home", help="atlas home directory (default ~/.claude-atlas or $CLAUDE_ATLAS_HOME)")
    parser.add_argument("-y", "--yes", action="store_true", help="answer yes to every prompt")
    sub = parser.add_subparsers(dest="command", required=True)

    setup = sub.add_parser("setup", help="install claude-obsidian, create the atlas, and your first vault")
    setup.add_argument("--vaults-dir", help="where new vaults are created (default ~/Vaults)")
    setup.add_argument("--first-vault", help="name or path of the first vault (default welcome)")
    setup.add_argument("--no-plugin", action="store_true", help="do not touch Claude Code plugins")
    setup.add_argument("--no-open", action="store_true", help="do not offer to open Obsidian")
    setup.set_defaults(handler=cmd_setup)

    vault = sub.add_parser("vault", help="create, register, and list vaults")
    vault_sub = vault.add_subparsers(dest="vault_command", required=True)
    new = vault_sub.add_parser("new", help="create a claude-obsidian vault and register it")
    new.add_argument("name", help="vault name (under the vaults dir) or a path")
    _add_node_options(new)
    new.add_argument("--no-open", action="store_true")
    new.set_defaults(handler=cmd_vault_new)
    add = vault_sub.add_parser("add", help="register an existing claude-obsidian vault")
    add.add_argument("path")
    add.add_argument("--name", help="display name (default: directory name)")
    _add_node_options(add)
    add.set_defaults(handler=cmd_vault_add)
    lst = vault_sub.add_parser("list", help="list registered vaults")
    lst.set_defaults(handler=cmd_vault_list)

    ref = sub.add_parser("refresh", help="recompute every state.json and rewrite Atlas.md")
    ref.set_defaults(handler=cmd_refresh)

    opn = sub.add_parser("open", help="open the atlas (or one vault) in Obsidian")
    opn.add_argument("node", nargs="?", help="node path, e.g. work/sensor-triage")
    opn.set_defaults(handler=cmd_open)

    doc = sub.add_parser("doctor", help="report the state of the installation")
    doc.set_defaults(handler=cmd_doctor)
    return parser


def main(argv: list[str] | None = None) -> int:
    args = build_parser().parse_args(argv)
    console = Console(assume_yes=args.yes)
    try:
        return args.handler(args, console)
    except AtlasError as exc:
        print(f"error: {exc}", file=sys.stderr)
        return 1
    except KeyboardInterrupt:
        print()
        return 130
