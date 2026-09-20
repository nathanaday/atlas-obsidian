---
type: receipt
thread: thr-20260917-5ac2
title: "Replace tasks with threads"
outcome: completed
created: 2026-09-20
---

> [!receipt] Replace tasks with threads · completed
> [Stub](<../stubs/Replace tasks with threads.md>) → [Spec](<../specs/Replace tasks with threads.md>) → [Plan](<../plans/Replace tasks with threads.md>) → **Receipt**
> `thr-20260917-5ac2` · [Thread](<../archive/Replace tasks with threads.md>) · filed 2026-09-20

Completed, and shipped as 3.0.0 in commit c38ae98: a thread's stage is the
furthest document that exists, `threads` and `thread` replaced `plant`, `tasks`,
and `task` everywhere, each stage has its own skill, the cards and the board are
generated, and `claude-atlas upgrade` turns 2.x task pages into threads.

Verified then by `make test` and by working this repository's own threads through
the stages. Verified again in v4, where the stage folders moved under `threads/`:
`threads.Migrate` still turns a 2.x task page into a thread (one test per case in
`internal/manage`), and this repository's own board survived the move with its
documents and their callouts intact.

Filed late: the work shipped on 2026-09-17 and the receipt is from 2026-09-20,
because the thread stayed at `plan` while v4 was built on top of it. The lesson is
the one the thread itself is about: the document is the state, so a thread that
shipped without its receipt looks unfinished to every later session, and did.
