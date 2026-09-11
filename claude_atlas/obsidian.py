"""Open vaults in the Obsidian desktop app."""

from __future__ import annotations

import shutil
import subprocess
import sys
from pathlib import Path
from urllib.parse import quote


def open_uri(vault: Path) -> str:
    return "obsidian://open?path=" + quote(str(vault), safe="")


def open_vault(vault: Path) -> bool:
    """Ask the desktop to open the vault. Returns False when no opener exists."""
    uri = open_uri(vault)
    if sys.platform == "darwin":
        command = ["open", uri]
    elif sys.platform.startswith("win"):
        command = ["cmd", "/c", "start", "", uri]
    else:
        opener = shutil.which("xdg-open")
        if opener is None:
            return False
        command = [opener, uri]
    try:
        subprocess.run(command, check=True, capture_output=True)
    except (OSError, subprocess.CalledProcessError):
        return False
    return True
