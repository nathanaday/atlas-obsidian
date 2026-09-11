from datetime import date
from pathlib import Path

from claude_atlas.refresh import (
    active_threads,
    derive_cluster,
    derive_leaf,
    heat_of,
    newest_log_date,
    render_atlas,
    seed_pages,
    signals,
)
from claude_atlas.tree import Node

TODAY = date(2026, 9, 11)


def _vault(tmp_path: Path, *, log: str = "", hot: str = "", pages: dict[str, str] | None = None) -> Path:
    vault = tmp_path / "vault"
    (vault / "wiki").mkdir(parents=True)
    (vault / ".claude-obsidian.json").write_text("{}")
    (vault / "wiki" / "log.md").write_text(log)
    (vault / "wiki" / "hot.md").write_text(hot)
    for name, text in (pages or {}).items():
        (vault / "wiki" / name).write_text(text)
    return vault


def _node(vault: Path | None, **overrides) -> Node:
    fields = dict(dir=Path("/x"), rel="x", id="x", name="x", kind="leaf" if vault else "cluster", vault=vault)
    fields.update(overrides)
    return Node(**fields)


def test_heat_thresholds():
    assert heat_of(None) is None
    assert heat_of(0) == "hot"
    assert heat_of(6) == "hot"
    assert heat_of(7) == "warm"
    assert heat_of(29) == "warm"
    assert heat_of(30) == "cold"


def test_newest_log_date_takes_the_latest_heading(tmp_path: Path):
    vault = _vault(
        tmp_path,
        log="# Wiki Log\n\n## 2026-09-04 — ingest-a\n\ntext\n\n## 2026-09-10 — ingest-b\n\n## not-a-date\n",
    )
    assert newest_log_date(vault) == date(2026, 9, 10)
    assert newest_log_date(tmp_path / "missing") is None


def test_active_threads_joins_continuation_lines(tmp_path: Path):
    vault = _vault(
        tmp_path,
        hot="# Recent\n\n## Recent Changes\n\n- ignored\n\n## Active Threads\n\n- First thread\n  continues here.\n- Second\n\n## Later\n\n- not a thread\n",
    )
    assert active_threads(vault) == ["First thread continues here.", "Second"]


def test_seed_pages_counts_frontmatter_status(tmp_path: Path):
    vault = _vault(
        tmp_path,
        pages={
            "a.md": "---\ntitle: A\nstatus: seed\n---\n# A\n",
            "b.md": "---\nstatus: \"seed\"\n---\n",
            "c.md": "---\nstatus: evergreen\n---\n",
            "d.md": "no frontmatter\nstatus: seed\n",
        },
    )
    assert seed_pages(vault) == 2


def test_derive_leaf_marks_missing_vault(tmp_path: Path):
    state = derive_leaf(None, _node(tmp_path / "nope"), today=TODAY, generated_at="t")
    assert state["vault_ok"] is False
    assert state["vault_error"] == "not found"
    assert state["heat"] is None


def test_derive_leaf_uses_later_of_log_and_mtime(tmp_path: Path):
    vault = _vault(tmp_path, log="## 2026-08-01 — old\n")
    state = derive_leaf(None, _node(vault), today=TODAY, generated_at="t")
    assert state["last_operation"] == "2026-08-01"
    assert state["last_touched"] == date.today().isoformat()
    assert state["vault_ok"] is False
    assert state["vault_error"] == "claude-obsidian is not installed"


def test_derive_cluster_rolls_up_children():
    hot = {
        "vault_ok": True, "last_operation": "2026-09-01", "last_touched": "2026-09-10", "days_idle": 1,
        "pages": 4, "open_threads": ["a"], "unfinished": {"empty_sections": 1, "seed_pages": 0, "dead_links": None},
    }
    cold = {
        "vault_ok": True, "last_operation": None, "last_touched": "2026-07-01", "days_idle": 72,
        "pages": 10, "open_threads": ["b", "c"], "unfinished": {"empty_sections": 2, "seed_pages": 3, "dead_links": None},
        "leaves": 2,
    }
    state = derive_cluster([hot, cold], generated_at="t")
    assert state["vault_ok"] is True
    assert state["heat"] == "hot"
    assert state["days_idle"] == 1
    assert state["last_operation"] == "2026-09-01"
    assert state["last_touched"] == "2026-09-10"
    assert state["pages"] == 14
    assert state["open_threads"] == ["a", "b", "c"]
    assert state["unfinished"] == {"empty_sections": 3, "seed_pages": 3, "dead_links": None}
    assert state["leaves"] == 4


def test_signals_cross_intent_with_derived_state():
    node = _node(Path("/v"), priority="high", state="active", review_after="2026-01-01")
    state = {"vault_ok": True, "vault_error": "", "heat": "cold", "days_idle": 45}
    notes = signals(node, state, today=TODAY)
    assert any("priority high" in n and "45 days" in n for n in notes)
    assert any("review date 2026-01-01" in n for n in notes)
    blocked = _node(Path("/v"), state="blocked", blocked_on="hardware")
    assert signals(blocked, {"vault_ok": True, "vault_error": "", "heat": "hot"}, today=TODAY) == [
        "blocked on: hardware"
    ]


def test_plain_text_strips_wikilinks():
    from claude_atlas.refresh import plain_text

    assert plain_text("See [[Spec]] and [[Long Name|alias]].") == "See Spec and alias."


def test_render_atlas_lists_rows_and_signals():
    node = _node(Path("/Users/me/v"), rel="work/v", purpose="Why.")
    state = {
        "vault_ok": True, "vault_error": "", "last_operation": None, "last_touched": "2026-08-01",
        "days_idle": 41, "heat": "cold", "pages": 3, "open_threads": ["thread one"],
        "unfinished": {"empty_sections": 1, "seed_pages": 2, "dead_links": 0},
    }
    page = render_atlas([(node, state)], generated_at="2026-09-11T20:00:00Z", today=TODAY)
    assert "| cold | [[#work/v|work/v]] | normal | active | 41d | 3 | 1 | 3 |" in page
    assert "obsidian://open?path=%2FUsers%2Fme%2Fv" in page
    assert "## Signals" in page and "cold for 41 days" in page
    assert "## work/v" in page and "Why." in page and "- thread one" in page


def test_refresh_against_a_real_vault(tmp_path: Path, config, core):
    from claude_atlas.refresh import refresh
    from claude_atlas.tree import read_state, walk
    from claude_atlas.vaults import create_vault, register_vault
    from claude_atlas.console import Console

    vault = config.vaults_dir / "fresh"
    create_vault(core, vault, Console(assume_yes=True), confirm=False)
    node = register_vault(config, vault, parent="area")
    page = refresh(config, core)
    state = read_state(node.dir)
    assert state["vault_ok"] is True
    assert state["pages"] == 4
    assert state["heat"] == "hot"
    assert state["open_threads"] == ["Add a source to `inbox/`, then ingest it."]
    assert state["unfinished"] == {"empty_sections": 0, "seed_pages": 0, "dead_links": 0}
    cluster = read_state(config.atlas_vault / "tree" / "area")
    assert cluster["leaves"] == 1 and cluster["pages"] == 4
    assert "[[#area/fresh|area/fresh]]" in page.read_text()
    assert [n.rel for n in walk(config.atlas_vault / "tree")] == ["area", "area/fresh"]
