import json
from pathlib import Path

from claude_atlas.cli import main


def run(*args: str) -> int:
    return main([*args])


def test_setup_creates_home_atlas_and_first_vault(seeded_home, tmp_path: Path, capsys):
    vaults = tmp_path / "Vaults"
    code = run(
        "--home", str(seeded_home.root), "-y", "setup",
        "--no-plugin", "--no-open", "--vaults-dir", str(vaults), "--first-vault", "welcome",
    )
    assert code == 0, capsys.readouterr().out
    out = capsys.readouterr().out
    assert "Setup complete." in out
    assert (vaults / "welcome" / ".claude-obsidian.json").is_file()
    assert (seeded_home.root / "claude-obsidian" / "scripts" / "claude-obsidian.py").is_file()
    atlas = seeded_home.root / "atlas"
    assert (atlas / ".obsidian" / "app.json").is_file()
    assert "[[#welcome|welcome]]" in (atlas / "Atlas.md").read_text()
    node = json.loads((atlas / "tree" / "welcome" / "node.json").read_text())
    assert node["vault"] == str(vaults / "welcome")
    config = json.loads(seeded_home.config_path.read_text())
    assert config["vaults_dir"] == str(vaults)

    # A second run changes nothing and creates no second vault.
    assert run("--home", str(seeded_home.root), "-y", "setup", "--no-plugin", "--no-open") == 0
    assert "keep       1 registered" in capsys.readouterr().out


def test_vault_new_add_list_and_doctor(seeded_home, tmp_path: Path, capsys):
    vaults = tmp_path / "Vaults"
    assert run("--home", str(seeded_home.root), "-y", "setup", "--no-plugin", "--no-open",
               "--vaults-dir", str(vaults)) == 0
    capsys.readouterr()

    assert run("--home", str(seeded_home.root), "-y", "vault", "new", "triage",
               "--parent", "work", "--purpose", "Sort sensors.", "--no-open") == 0
    out = capsys.readouterr().out
    assert "registered  node work/triage" in out
    assert (vaults / "triage" / "wiki" / "hot.md").is_file()

    assert run("--home", str(seeded_home.root), "vault", "add", str(vaults / "triage")) == 1
    assert "already registered as work/triage" in capsys.readouterr().err

    assert run("--home", str(seeded_home.root), "vault", "add", str(tmp_path)) == 1
    assert "not a claude-obsidian vault" in capsys.readouterr().err

    assert run("--home", str(seeded_home.root), "vault", "list") == 0
    out = capsys.readouterr().out
    assert "work/triage" in out and "welcome" in out and "(cluster)" in out

    assert run("--home", str(seeded_home.root), "doctor") == 0
    out = capsys.readouterr().out
    assert "2 registered" in out and "verified" in out


def test_commands_need_setup_first(tmp_path: Path, capsys):
    assert run("--home", str(tmp_path / "none"), "refresh") == 1
    assert "run `claude-atlas setup` first" in capsys.readouterr().err
