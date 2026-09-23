---
type: stub
thread: thr-20260922-9ee5
title: "A staged file is committed as a hand edit"
created: 2026-09-22
---

> [!stub] A staged file is committed as a hand edit
> **Stub** → Spec → Plan → Receipt
> `thr-20260922-9ee5` · [Thread](<../A staged file is committed as a hand edit.md>) · filed 2026-09-22

The describe snapshot is committed as a hand edit. `stage` with `snapshot` writes the file into inbox/, and the next operation (the capture) first commits it as `manual: 1 file changed by hand`, which says the user edited a file when a tool wrote it. Found by the atlas-onboard end-to-end run on 2026-09-22 (commit 45b4ffc in the scratch shop project). The same happens to any file `stage` copies into the inbox. The fix is probably for `stage` to commit what it writes, or for the capture to take the inbox files it captures into its own commit.
