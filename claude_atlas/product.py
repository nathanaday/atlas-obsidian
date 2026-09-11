"""The pinned claude-obsidian release: download, verify, extract, and run."""

from __future__ import annotations

import hashlib
import json
import os
import shutil
import subprocess
import sys
import tempfile
import zipfile
from datetime import datetime, timezone
from pathlib import Path, PurePosixPath
from typing import Any, Callable
from urllib.request import Request, urlopen

from .errors import AtlasError

PRODUCT_NAME = "claude-obsidian"
PRODUCT_VERSION = "2.2.0"
RELEASE_TAG = f"v{PRODUCT_VERSION}"
RELEASE_ASSET = f"{PRODUCT_NAME}-{RELEASE_TAG}.zip"
RELEASE_URL = (
    "https://github.com/AgriciDaniel/claude-obsidian/releases/download/"
    f"{RELEASE_TAG}/{RELEASE_ASSET}"
)
RELEASE_SHA256 = "6207f8aff60366adb441f4311e542fed42ef1edec1b303e78ee215cec8b64ffa"
RELEASE_SIZE = 3_145_865

PLUGIN_MANIFEST = Path(".claude-plugin") / "plugin.json"
CLI_SCRIPT = Path("scripts") / "claude-obsidian.py"


class ProductError(AtlasError):
    """claude-obsidian returned an error or unparseable output."""


def sha256_file(path: Path) -> str:
    digest = hashlib.sha256()
    with path.open("rb") as handle:
        for chunk in iter(lambda: handle.read(1 << 20), b""):
            digest.update(chunk)
    return digest.hexdigest()


def cached_release_ok(zip_path: Path) -> bool:
    return zip_path.is_file() and sha256_file(zip_path) == RELEASE_SHA256


def download_release(
    dest: Path, *, progress: Callable[[int, int | None], None] | None = None
) -> Path:
    """Fetch the pinned release zip to dest and refuse it unless the hash matches."""
    dest.parent.mkdir(parents=True, exist_ok=True)
    request = Request(RELEASE_URL, headers={"User-Agent": "claude-atlas"})
    digest = hashlib.sha256()
    handle = tempfile.NamedTemporaryFile(dir=dest.parent, delete=False)
    temp = Path(handle.name)
    try:
        with handle, urlopen(request, timeout=60) as response:
            length = response.headers.get("Content-Length")
            total = int(length) if length else None
            received = 0
            for chunk in iter(lambda: response.read(1 << 16), b""):
                handle.write(chunk)
                digest.update(chunk)
                received += len(chunk)
                if progress:
                    progress(received, total)
    except OSError as exc:
        temp.unlink(missing_ok=True)
        raise ProductError(f"download failed: {exc}") from exc
    if digest.hexdigest() != RELEASE_SHA256:
        temp.unlink(missing_ok=True)
        raise ProductError(
            f"{RELEASE_ASSET} did not match the pinned sha256; refusing to install"
        )
    os.replace(temp, dest)
    return dest


def _safe_member(name: str) -> PurePosixPath:
    member = PurePosixPath(name)
    if member.is_absolute() or ".." in member.parts or "\\" in name:
        raise ProductError(f"unsafe path in release archive: {name}")
    return member


def extract_release(zip_path: Path, dest: Path) -> None:
    """Unpack the release into dest, replacing any previous install atomically."""
    staging = dest.parent / (dest.name + ".extracting")
    if staging.exists():
        shutil.rmtree(staging)
    staging.mkdir(parents=True)
    try:
        with zipfile.ZipFile(zip_path) as archive:
            for info in archive.infolist():
                member = _safe_member(info.filename)
                target = staging / member
                if info.is_dir():
                    target.mkdir(parents=True, exist_ok=True)
                    continue
                target.parent.mkdir(parents=True, exist_ok=True)
                with archive.open(info) as source, target.open("wb") as sink:
                    shutil.copyfileobj(source, sink)
                mode = (info.external_attr >> 16) & 0o777
                if mode:
                    target.chmod(mode)
    except zipfile.BadZipFile as exc:
        shutil.rmtree(staging, ignore_errors=True)
        raise ProductError(f"release archive is corrupt: {exc}") from exc
    if dest.exists():
        shutil.rmtree(dest)
    os.replace(staging, dest)


def verify_tree(root: Path) -> list[str]:
    """Check every file listed in the release's SHA256SUMS. Returns problems."""
    sums = root / "SHA256SUMS"
    if not sums.is_file():
        return ["SHA256SUMS is missing"]
    problems: list[str] = []
    for line in sums.read_text(encoding="utf-8").splitlines():
        if not line.strip():
            continue
        expected, _, relative = line.partition("  ")
        path = root / relative
        if not path.is_file():
            problems.append(f"missing: {relative}")
        elif sha256_file(path) != expected:
            problems.append(f"modified: {relative}")
    return problems


def installed_version(root: Path) -> str | None:
    manifest = root / PLUGIN_MANIFEST
    if not manifest.is_file():
        return None
    try:
        with manifest.open(encoding="utf-8") as handle:
            return str(json.load(handle).get("version"))
    except (OSError, ValueError):
        return None


def is_installed(root: Path) -> bool:
    return installed_version(root) == PRODUCT_VERSION and (root / CLI_SCRIPT).is_file()


def now_utc() -> str:
    return (
        datetime.now(timezone.utc)
        .replace(microsecond=0)
        .isoformat()
        .replace("+00:00", "Z")
    )


def operation_id(prefix: str, generated_at: str) -> str:
    return prefix + "-" + generated_at.replace(":", "").replace("-", "")


class ClaudeObsidian:
    """Runs the product CLI by absolute path, the way its own skills do."""

    def __init__(self, root: Path):
        self.root = root
        self.cli = root / CLI_SCRIPT
        if not self.cli.is_file():
            raise ProductError(f"claude-obsidian is not installed at {root}")

    def run(self, *args: str) -> subprocess.CompletedProcess[str]:
        env = dict(os.environ, PYTHONDONTWRITEBYTECODE="1")
        return subprocess.run(
            [sys.executable, str(self.cli), *args],
            capture_output=True,
            text=True,
            encoding="utf-8",
            env=env,
        )

    def run_json(self, *args: str) -> dict[str, Any]:
        result = self.run(*args)
        try:
            return json.loads(result.stdout)
        except ValueError:
            detail = (result.stderr or result.stdout).strip()
            raise ProductError(
                f"claude-obsidian {args[0]} failed (exit {result.returncode}): {detail}"
            ) from None

    def version(self) -> str:
        return self.run("--version").stdout.strip()

    def doctor(self, vault: Path) -> dict[str, Any]:
        return self.run_json("doctor", "--vault", str(vault))

    def lint(self, vault: Path) -> dict[str, Any]:
        return self.run_json("lint", "--vault", str(vault), "--format", "json")

    def init_plan(
        self, path: Path, *, generated_at: str, operation: str
    ) -> dict[str, Any]:
        return self.run_json(
            "init",
            str(path),
            "--generated-at",
            generated_at,
            "--operation-id",
            operation,
        )

    def init_apply(
        self, path: Path, *, generated_at: str, operation: str, approval: str
    ) -> dict[str, Any]:
        return self.run_json(
            "init",
            str(path),
            "--generated-at",
            generated_at,
            "--operation-id",
            operation,
            "--approved-plan-sha256",
            approval,
            "--apply",
        )
