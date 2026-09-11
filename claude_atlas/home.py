"""The atlas home directory: settings, product install, cache, and root vault."""

from __future__ import annotations

import json
import os
from dataclasses import dataclass
from pathlib import Path

from .errors import AtlasError

DEFAULT_HOME = Path("~/.claude-atlas")
DEFAULT_VAULTS_DIR = Path("~/Vaults")
CONFIG_SCHEMA = "claude-atlas.config.v1"
ENV_HOME = "CLAUDE_ATLAS_HOME"


@dataclass(frozen=True)
class Config:
    vaults_dir: Path
    atlas_vault: Path
    product_path: Path
    product_version: str

    def to_json(self, home: Path) -> dict:
        return {
            "schema": CONFIG_SCHEMA,
            "vaults_dir": str(self.vaults_dir),
            "atlas_vault": _relative_if_inside(self.atlas_vault, home),
            "product": {
                "name": "claude-obsidian",
                "version": self.product_version,
                "path": _relative_if_inside(self.product_path, home),
            },
        }

    @classmethod
    def from_json(cls, data: dict, home: Path) -> "Config":
        if data.get("schema") != CONFIG_SCHEMA:
            raise AtlasError(f"unsupported config schema in {home / 'config.json'}")
        product = data.get("product", {})
        return cls(
            vaults_dir=Path(data["vaults_dir"]).expanduser(),
            atlas_vault=_absolute(data["atlas_vault"], home),
            product_path=_absolute(product["path"], home),
            product_version=str(product["version"]),
        )


def _relative_if_inside(path: Path, home: Path) -> str:
    try:
        return str(path.relative_to(home))
    except ValueError:
        return str(path)


def _absolute(value: str, home: Path) -> Path:
    path = Path(value).expanduser()
    return path if path.is_absolute() else home / path


class Home:
    def __init__(self, root: Path):
        self.root = root.expanduser().resolve()

    @classmethod
    def resolve(cls, explicit: str | None = None) -> "Home":
        value = explicit or os.environ.get(ENV_HOME) or str(DEFAULT_HOME)
        return cls(Path(value))

    @property
    def config_path(self) -> Path:
        return self.root / "config.json"

    @property
    def cache_dir(self) -> Path:
        return self.root / "cache"

    @property
    def default_product_path(self) -> Path:
        return self.root / "claude-obsidian"

    @property
    def default_atlas_vault(self) -> Path:
        return self.root / "atlas"

    def exists(self) -> bool:
        return self.config_path.is_file()

    def load_config(self) -> Config:
        if not self.exists():
            raise AtlasError(
                f"no atlas at {self.root}; run `claude-atlas setup` first"
            )
        with self.config_path.open(encoding="utf-8") as handle:
            return Config.from_json(json.load(handle), self.root)

    def save_config(self, config: Config) -> None:
        self.root.mkdir(parents=True, exist_ok=True)
        text = json.dumps(config.to_json(self.root), indent=2, sort_keys=True) + "\n"
        self.config_path.write_text(text, encoding="utf-8")

    def default_config(self, vaults_dir: Path | None = None) -> Config:
        from .product import PRODUCT_VERSION

        return Config(
            vaults_dir=(vaults_dir or DEFAULT_VAULTS_DIR).expanduser(),
            atlas_vault=self.default_atlas_vault,
            product_path=self.default_product_path,
            product_version=PRODUCT_VERSION,
        )


def tree_root(config: Config) -> Path:
    return config.atlas_vault / "tree"


def display_path(path: Path) -> str:
    """Render a path with ~ for the home directory, for terminal output."""
    home = Path.home()
    try:
        return "~/" + str(path.relative_to(home))
    except ValueError:
        return str(path)
