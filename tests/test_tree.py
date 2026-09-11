import json
from pathlib import Path

import pytest

from claude_atlas.errors import AtlasError
from claude_atlas.tree import (
    NODE_SCHEMA,
    create_leaf,
    find_by_rel,
    find_by_vault,
    leaves,
    load_node,
    slugify,
    walk,
)


def test_slugify_lowercases_and_joins_with_dashes():
    assert slugify("Sensor Triage (2026)") == "sensor-triage-2026"
    with pytest.raises(AtlasError):
        slugify("!!!")


def test_create_leaf_under_parent_creates_cluster_nodes(tmp_path: Path):
    root = tmp_path / "tree"
    root.mkdir()
    node_dir = create_leaf(
        root, node_id="triage", name="Triage", vault=tmp_path / "v", parent="technical/work"
    )
    assert node_dir == root / "technical" / "work" / "triage"
    assert (node_dir / "outputs").is_dir()
    nodes = walk(root)
    assert [n.rel for n in nodes] == ["technical", "technical/work", "technical/work/triage"]
    assert [n.kind for n in nodes] == ["cluster", "cluster", "leaf"]
    assert leaves(nodes)[0].vault == tmp_path / "v"


def test_walk_lists_parents_before_children_regardless_of_names(tmp_path: Path):
    create_leaf(tmp_path, node_id="alpha", name="alpha", vault=tmp_path / "v1", parent="area")
    create_leaf(tmp_path, node_id="zeta", name="zeta", vault=tmp_path / "v2", parent="area/deep")
    assert [n.rel for n in walk(tmp_path)] == ["area", "area/alpha", "area/deep", "area/deep/zeta"]


def test_create_leaf_refuses_duplicate_node(tmp_path: Path):
    create_leaf(tmp_path, node_id="a", name="a", vault=tmp_path / "v1")
    with pytest.raises(AtlasError, match="already exists"):
        create_leaf(tmp_path, node_id="a", name="a", vault=tmp_path / "v2")


def test_walk_rejects_two_leaves_on_one_vault(tmp_path: Path):
    create_leaf(tmp_path, node_id="a", name="a", vault=tmp_path / "v")
    create_leaf(tmp_path, node_id="b", name="b", vault=tmp_path / "v")
    with pytest.raises(AtlasError, match="same vault"):
        walk(tmp_path)


def test_find_helpers(tmp_path: Path):
    create_leaf(tmp_path, node_id="a", name="a", vault=tmp_path / "v", parent="x")
    nodes = walk(tmp_path)
    assert find_by_vault(nodes, tmp_path / "v").rel == "x/a"
    assert find_by_rel(nodes, "x/a").id == "a"
    assert find_by_rel(nodes, "a").rel == "x/a"
    assert find_by_rel(nodes, "missing") is None


@pytest.mark.parametrize(
    "document, message",
    [
        ({"schema": "nope", "kind": "leaf", "vault": "/v"}, "unsupported schema"),
        ({"schema": NODE_SCHEMA, "kind": "leaf"}, "must name a vault"),
        ({"schema": NODE_SCHEMA, "kind": "cluster", "vault": "/v"}, "must not name"),
        ({"schema": NODE_SCHEMA, "kind": "leaf", "vault": "/v", "priority": "urgent"}, "priority"),
        ({"schema": NODE_SCHEMA, "kind": "leaf", "vault": "/v", "state": "done"}, "state"),
    ],
)
def test_load_node_validates(tmp_path: Path, document, message):
    node_dir = tmp_path / "n"
    node_dir.mkdir()
    (node_dir / "node.json").write_text(json.dumps(document))
    with pytest.raises(AtlasError, match=message):
        load_node(node_dir, tmp_path)
