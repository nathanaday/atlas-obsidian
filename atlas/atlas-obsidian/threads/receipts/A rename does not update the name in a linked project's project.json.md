---
type: receipt
thread: thr-20260918-45e6
title: "A rename does not update the name in a linked project's project.json"
outcome: killed
created: 2026-09-20
---

> [!killed] A rename does not update the name in a linked project's project.json · killed
> [Stub](<../stubs/A rename does not update the name in a linked project's project.json.md>) → Spec → Plan → **Receipt**
> `thr-20260918-45e6` · [Thread](<../archive/A rename does not update the name in a linked project's project.json.md>) · filed 2026-09-20

Killed: the feature is gone. v4 merged the knowledge base into the project, so
`project.json` carries no `knowledge` field and there is no `link` to run again.
A stale name cannot happen where nothing records a second entity's name.

What to keep from it: the shape of the bug was one fact recorded twice, the id
and the name, with only the id kept current. `describe.Page` still matches a page
by the project's id or its name, and `registry.Entry` carries both. If knowledge
that crosses projects returns as a citation between wikis, it cites by id and
carries a name for people only, and something must refresh that name or accept
that it goes stale.
