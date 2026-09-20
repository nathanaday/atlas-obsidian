---
type: stub
thread: thr-20260919-26da
title: "Consolidate the project and the knowledge base into one entity"
created: 2026-09-19
---

> [!stub] Consolidate the project and the knowledge base into one entity
> **Stub** → [Spec](<../specs/Consolidate the project and the knowledge base into one entity.md>) → [Plan](<../plans/Consolidate the project and the knowledge base into one entity.md>) → Receipt
> `thr-20260919-26da` · [Thread](<../Consolidate the project and the knowledge base into one entity.md>) · filed 2026-09-19

Using the tool for a week found the split between a project and a knowledge base to be the heaviest thing left in the design. Two Obsidian vaults per project, often not in the same parent folder, and a decision before every session about which one holds what I want. Two inboxes. A link to maintain: init --knowledge, link, unlink, an id inside project.json, a knowledge base that is not on this machine. The reason for the split was one knowledge base serving many projects, and in practice each project's knowledge was about that project.

Consolidate the two into one entity. A project is atlas/<name>/ in the work and it holds both halves: the wiki with its engine, inbox, ideas, and raw store, and the threads with their stages. No linking, one inbox, one session kind, one list in the view. Also move the thread folders (stubs, specs, plans, receipts, phases) under threads/, so opening the vault shows four folders and not nine.

Sharing knowledge between projects stays the mission and is tabled as the next phase, to be built over projects that each hold their own wiki.
