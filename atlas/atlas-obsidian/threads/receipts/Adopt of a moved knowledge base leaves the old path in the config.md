---
type: receipt
thread: thr-20260918-1652
title: "Adopt of a moved knowledge base leaves the old path in the config"
outcome: killed
created: 2026-09-20
---

> [!killed] Adopt of a moved knowledge base leaves the old path in the config · killed
> [Stub](<../stubs/Adopt of a moved knowledge base leaves the old path in the config.md>) → Spec → Plan → **Receipt**
> `thr-20260918-1652` · [Thread](<../archive/Adopt of a moved knowledge base leaves the old path in the config.md>) · filed 2026-09-20

Killed: the feature is gone. v4 has no `adopt` of a knowledge base and no
`knowledge` list in the config to hold a second path. `upgrade` makes a moved
vault's folder a project and lists that one work folder; `manage.RegisterProject`
heals a path that moved, by id, and drops the entry it replaced.

What to keep from it: the failure was an add without a matching remove. That is
now one function, `healPaths`, with a test for each of its three cases: the same
id at another path, the only listed path that is gone, and neither.
