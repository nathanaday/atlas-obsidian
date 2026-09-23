---
type: stub
thread: thr-20260923-e4e6
title: "Cross-project plan and rename commit bugs"
created: 2026-09-23
---

> [!stub] Cross-project plan and rename commit bugs
> **Stub** → Spec → Plan → Receipt
> `thr-20260923-e4e6` · [Thread](<../Cross-project plan and rename commit bugs.md>) · filed 2026-09-23

Two bugs found on 2026-09-23 while moving pages across the ITL wikis from a session in ~/projects/itl, which is not a project. (1) plan with project fails with 'not in a atlas-obsidian project: none at or above ~/projects/itl'. plan calls s.where() at internal/mcpserver/server.go:730 before projectOf reads the argument. The thread tools at lines 468, 555, and 647 have the same order, and undo acts only on the session's project, so an operation made from elsewhere cannot be undone from there. apply already works, because it opens the plan's folder. (2) project edit --name renamed atlas/itl-ecosystem to atlas/itl-product and committed the new files (5afb925 in ~/kbs/itl-ecosystem), but left the deletions of the old paths unstaged and the new .gitignore, .obsidian/, and threads/ untracked. A second commit (08aab1d) was needed to finish the rename.
