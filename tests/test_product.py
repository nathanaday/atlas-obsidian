import hashlib
import json
import zipfile
from pathlib import Path

import pytest

from claude_atlas import product


def _release(tmp_path: Path, files: dict[str, bytes], *, sums: bool = True) -> Path:
    zip_path = tmp_path / "release.zip"
    lines = [f"{hashlib.sha256(data).hexdigest()}  {name}" for name, data in files.items()]
    with zipfile.ZipFile(zip_path, "w") as archive:
        for name, data in files.items():
            archive.writestr(name, data)
        if sums:
            archive.writestr("SHA256SUMS", "\n".join(lines) + "\n")
    return zip_path


FILES = {
    ".claude-plugin/plugin.json": json.dumps({"version": product.PRODUCT_VERSION}).encode(),
    "scripts/claude-obsidian.py": b"print('hi')\n",
    "README.md": b"# r\n",
}


def test_extract_then_verify_round_trip(tmp_path: Path):
    dest = tmp_path / "install"
    product.extract_release(_release(tmp_path, FILES), dest)
    assert product.verify_tree(dest) == []
    assert product.is_installed(dest)
    assert product.installed_version(dest) == product.PRODUCT_VERSION


def test_extract_replaces_previous_install(tmp_path: Path):
    dest = tmp_path / "install"
    dest.mkdir()
    (dest / "stale.txt").write_text("old")
    product.extract_release(_release(tmp_path, FILES), dest)
    assert not (dest / "stale.txt").exists()
    assert (dest / "README.md").read_text() == "# r\n"


def test_verify_reports_modified_and_missing(tmp_path: Path):
    dest = tmp_path / "install"
    product.extract_release(_release(tmp_path, FILES), dest)
    (dest / "README.md").write_text("tampered")
    (dest / "scripts" / "claude-obsidian.py").unlink()
    problems = product.verify_tree(dest)
    assert "modified: README.md" in problems
    assert "missing: scripts/claude-obsidian.py" in problems
    assert product.verify_tree(tmp_path / "nowhere") == ["SHA256SUMS is missing"]


def test_extract_refuses_zip_slip(tmp_path: Path):
    zip_path = tmp_path / "evil.zip"
    with zipfile.ZipFile(zip_path, "w") as archive:
        archive.writestr("../escape.txt", b"x")
    with pytest.raises(product.ProductError, match="unsafe path"):
        product.extract_release(zip_path, tmp_path / "install")
    assert not (tmp_path / "escape.txt").exists()


def test_cached_release_ok_checks_the_pinned_hash(tmp_path: Path):
    bogus = tmp_path / product.RELEASE_ASSET
    bogus.write_bytes(b"not the release")
    assert product.cached_release_ok(bogus) is False
    assert product.cached_release_ok(tmp_path / "absent.zip") is False


def test_operation_id_is_derived_from_the_timestamp():
    assert product.operation_id("init", "2026-09-11T20:14:03Z") == "init-20260911T201403Z"


def test_real_release_verifies(product_root: Path):
    assert product.verify_tree(product_root) == []
    assert product.ClaudeObsidian(product_root).version() == product.PRODUCT_VERSION
