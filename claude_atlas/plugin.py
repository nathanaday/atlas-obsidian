"""Install the claude-obsidian plugin into Claude Code from the local release."""

from __future__ import annotations

import shutil
import subprocess
from pathlib import Path

from .errors import AtlasError

MARKETPLACE_NAME = "agricidaniel-claude-obsidian"
PLUGIN_NAME = "claude-obsidian"
PLUGIN_ID = f"{PLUGIN_NAME}@{MARKETPLACE_NAME}"


def claude_cli() -> str | None:
    return shutil.which("claude")


def _run(*args: str) -> subprocess.CompletedProcess[str]:
    cli = claude_cli()
    if cli is None:
        raise AtlasError("the `claude` command is not on PATH")
    return subprocess.run(
        [cli, "plugin", *args], capture_output=True, text=True, encoding="utf-8"
    )


def marketplace_present() -> bool:
    result = _run("marketplace", "list")
    return MARKETPLACE_NAME in result.stdout


def plugin_present() -> bool:
    result = _run("list")
    return PLUGIN_ID in result.stdout


def commands(product_root: Path) -> list[list[str]]:
    return [
        ["claude", "plugin", "marketplace", "add", str(product_root)],
        ["claude", "plugin", "install", PLUGIN_ID],
    ]


def install(product_root: Path) -> list[str]:
    """Register the release directory as a marketplace and install from it."""
    ran: list[str] = []
    if not marketplace_present():
        result = _run("marketplace", "add", str(product_root))
        ran.append(" ".join(commands(product_root)[0]))
        if result.returncode != 0:
            raise AtlasError(
                "claude plugin marketplace add failed:\n"
                + (result.stderr or result.stdout).strip()
            )
    if not plugin_present():
        result = _run("install", PLUGIN_ID)
        ran.append(" ".join(commands(product_root)[1]))
        if result.returncode != 0:
            raise AtlasError(
                "claude plugin install failed:\n"
                + (result.stderr or result.stdout).strip()
            )
    return ran
