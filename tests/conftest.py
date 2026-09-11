import os
import shutil
from pathlib import Path

import pytest

from claude_atlas import product
from claude_atlas.home import Config, Home

ENV_PRODUCT = "CLAUDE_ATLAS_TEST_PRODUCT"
ENV_RELEASE = "CLAUDE_ATLAS_TEST_RELEASE_ZIP"


def _candidate_paths(env: str, default: Path) -> list[Path]:
    paths = []
    if os.environ.get(env):
        paths.append(Path(os.environ[env]).expanduser())
    paths.append(default)
    return paths


@pytest.fixture(scope="session")
def product_root() -> Path:
    """An extracted claude-obsidian release, or skip the test."""
    default = Home.resolve().default_product_path
    for path in _candidate_paths(ENV_PRODUCT, default):
        if product.is_installed(path):
            return path
    pytest.skip(f"no claude-obsidian install; set {ENV_PRODUCT} or run setup")


@pytest.fixture(scope="session")
def release_zip() -> Path:
    """The pinned release zip, or skip the test. Tests never download."""
    default = Home.resolve().cache_dir / product.RELEASE_ASSET
    for path in _candidate_paths(ENV_RELEASE, default):
        if product.cached_release_ok(path):
            return path
    pytest.skip(f"no cached release zip; set {ENV_RELEASE} or run setup")


@pytest.fixture
def core(product_root: Path) -> product.ClaudeObsidian:
    return product.ClaudeObsidian(product_root)


@pytest.fixture
def home(tmp_path: Path) -> Home:
    return Home(tmp_path / "home")


@pytest.fixture
def config(home: Home, tmp_path: Path, product_root: Path) -> Config:
    cfg = Config(
        vaults_dir=tmp_path / "Vaults",
        atlas_vault=home.default_atlas_vault,
        product_path=product_root,
        product_version=product.PRODUCT_VERSION,
    )
    home.save_config(cfg)
    (cfg.atlas_vault / "tree").mkdir(parents=True)
    return cfg


@pytest.fixture
def seeded_home(home: Home, release_zip: Path) -> Home:
    """A home whose cache already holds the release, so setup stays offline."""
    home.cache_dir.mkdir(parents=True)
    shutil.copy(release_zip, home.cache_dir / product.RELEASE_ASSET)
    return home
