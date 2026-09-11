"""The guided `claude-atlas setup` flow."""

from __future__ import annotations

import json
from pathlib import Path

from . import plugin, product
from .console import Console
from .errors import AtlasError
from .home import DEFAULT_VAULTS_DIR, Config, Home, display_path, tree_root
from .obsidian import open_uri, open_vault
from .refresh import refresh
from .tree import leaves, walk
from .vaults import create_vault, register_vault, resolve_new_vault_path

ATLAS_APP_JSON = {"newLinkFormat": "absolute"}


def _plan_line(console: Console, label: str, action: str, target: str = "") -> None:
    console.say(f"  {label:<16} {action:<10} {target}")


def _progress(console: Console):
    last = {"pct": -1}

    def report(received: int, total: int | None) -> None:
        if total:
            pct = received * 100 // total
            if pct // 10 != last["pct"] // 10:
                last["pct"] = pct
                console.stream.write(f"\r  → downloading {pct:3d}%")
                console.stream.flush()

    return report


def ensure_atlas_vault(config: Config) -> bool:
    """Create the root Obsidian vault if missing. Returns True when created."""
    vault = config.atlas_vault
    created = not (vault / ".obsidian").is_dir()
    (vault / ".obsidian").mkdir(parents=True, exist_ok=True)
    app = vault / ".obsidian" / "app.json"
    if not app.exists():
        app.write_text(json.dumps(ATLAS_APP_JSON, indent=2) + "\n", encoding="utf-8")
    tree_root(config).mkdir(exist_ok=True)
    return created


def ensure_product(home: Home, config: Config, console: Console) -> None:
    zip_path = home.cache_dir / product.RELEASE_ASSET
    if not product.cached_release_ok(zip_path):
        product.download_release(zip_path, progress=_progress(console))
        console.stream.write("\r")
        console.step("downloaded", f"{product.RELEASE_ASSET} (sha256 verified)")
    else:
        console.step("cached", product.RELEASE_ASSET, status="skip")
    product.extract_release(zip_path, config.product_path)
    problems = product.verify_tree(config.product_path)
    if problems:
        raise AtlasError(
            "extracted release failed verification:\n  " + "\n  ".join(problems[:10])
        )
    console.step("installed", f"claude-obsidian v{product.PRODUCT_VERSION} → {display_path(config.product_path)}")


def run_setup(
    home: Home,
    console: Console,
    *,
    vaults_dir: str | None = None,
    first_vault: str | None = None,
    with_plugin: bool = True,
    open_after: bool | None = None,
) -> int:
    fresh = not home.exists()
    if fresh:
        chosen = vaults_dir or console.ask(
            "Where should new vaults live?", str(DEFAULT_VAULTS_DIR)
        )
        config = home.default_config(Path(chosen))
    else:
        config = home.load_config()
        if vaults_dir:
            config = Config(
                vaults_dir=Path(vaults_dir).expanduser(),
                atlas_vault=config.atlas_vault,
                product_path=config.product_path,
                product_version=config.product_version,
            )

    product_ready = product.is_installed(config.product_path)
    claude = plugin.claude_cli()
    plugin_ready = bool(claude) and with_plugin and plugin.plugin_present()
    atlas_ready = (config.atlas_vault / ".obsidian").is_dir()
    registered = leaves(walk(tree_root(config))) if atlas_ready else []
    first_path: Path | None = None
    if not registered:
        name = first_vault or console.ask("Name for your first vault", "welcome")
        first_path = resolve_new_vault_path(name, config.vaults_dir)
        if first_path.exists():
            raise AtlasError(
                f"{display_path(first_path)} already exists; choose another name "
                "or register it with `claude-atlas vault add`"
            )

    console.say()
    console.say("claude-atlas setup")
    console.say()
    _plan_line(console, "home", "exists" if not fresh else "create", display_path(home.root))
    _plan_line(
        console,
        "claude-obsidian",
        "installed" if product_ready else "download",
        f"v{product.PRODUCT_VERSION}"
        + ("" if product_ready else f" ({product.RELEASE_SIZE / 1e6:.1f} MB from github.com; verified by sha256)"),
    )
    if not with_plugin:
        plugin_action, plugin_target = "skip", "--no-plugin"
    elif not claude:
        plugin_action, plugin_target = "skip", "`claude` is not on PATH"
    elif plugin_ready:
        plugin_action, plugin_target = "installed", plugin.PLUGIN_ID
    else:
        plugin_action, plugin_target = "install", f"{plugin.PLUGIN_ID} into Claude Code"
    _plan_line(console, "Claude Code", plugin_action, plugin_target)
    _plan_line(console, "atlas vault", "exists" if atlas_ready else "create", display_path(config.atlas_vault))
    _plan_line(console, "vaults dir", "use", display_path(config.vaults_dir))
    if first_path:
        _plan_line(console, "first vault", "create", f"{display_path(first_path)} (claude-obsidian init)")
    else:
        _plan_line(console, "vaults", "keep", f"{len(registered)} registered")
    console.say()
    if not console.confirm("Proceed?"):
        return 1
    console.say()

    home.save_config(config)
    console.step("home", display_path(home.root))

    if product_ready:
        console.step("claude-obsidian", f"v{product.PRODUCT_VERSION} already installed", status="skip")
    else:
        ensure_product(home, config, console)
    core = product.ClaudeObsidian(config.product_path)

    if with_plugin and claude and not plugin_ready:
        try:
            for command in plugin.install(config.product_path):
                console.step("ran", command)
            console.step("Claude Code", f"{plugin.PLUGIN_ID} installed")
        except AtlasError as exc:
            console.step("Claude Code", str(exc), status="fail")
            console.say("    Install it by hand:")
            for command in plugin.commands(config.product_path):
                console.say("      " + " ".join(command))
    elif with_plugin and claude:
        console.step("Claude Code", "plugin already installed", status="skip")
    elif with_plugin:
        console.step("Claude Code", "`claude` not on PATH; manual commands below", status="skip")
    else:
        console.step("Claude Code", "skipped (--no-plugin)", status="skip")

    if ensure_atlas_vault(config):
        console.step("atlas vault", display_path(config.atlas_vault))
    else:
        console.step("atlas vault", "already present", status="skip")

    if first_path:
        create_vault(core, first_path, console, confirm=False)
        node = register_vault(config, first_path, purpose="Created by claude-atlas setup to verify the installation.")
        console.step("first vault", f"{display_path(first_path)} → node {node.rel}")

    page = refresh(config, core)
    console.step("refreshed", display_path(page))

    console.say()
    console.say("Setup complete.")
    console.say()
    console.say(f"  Atlas       {display_path(config.atlas_vault)}")
    console.say(f"              {open_uri(config.atlas_vault)}")
    if first_path:
        console.say(f"  First vault {display_path(first_path)}")
        console.say(f"              {open_uri(first_path)}")
    console.say()
    console.say("Next:")
    console.say("  claude-atlas vault new <name>    create another vault")
    console.say("  claude-atlas vault add <path>    register an existing claude-obsidian vault")
    console.say("  claude-atlas refresh             rebuild Atlas.md from every vault")
    console.say("  claude-atlas open                open the atlas in Obsidian")
    if with_plugin and not claude:
        console.say()
        console.say("Claude Code was not found on PATH. To install the skills later:")
        for command in plugin.commands(config.product_path):
            console.say("  " + " ".join(command))
    console.say()

    if open_after is None:
        open_after = console.confirm("Open the atlas in Obsidian now?")
    if open_after and not open_vault(config.atlas_vault):
        console.say("Could not launch Obsidian; open the path above from its vault picker.")
    return 0
