---
type: stub
thread: thr-20260922-5519
title: "Consolidate overlapping member pages into the parent wiki"
created: 2026-09-22
---

> [!stub] Consolidate overlapping member pages into the parent wiki
> **Stub** → [Spec](<../specs/Consolidate overlapping member pages into the parent wiki.md>) → [Plan](<../plans/Consolidate overlapping member pages into the parent wiki.md>) → Receipt
> `thr-20260922-5519` · [Thread](<../Consolidate overlapping member pages into the parent wiki.md>) · filed 2026-09-22

It's possible that two children projects create very similar entries for very similar items um, because it's some context that they needed at the time. But only when you get to the parent level does it become clear that you can merge those and kind of consolidate them a little bit. So what we would really want to do is have some kind of skill where the parent knowledge base identifies overlapping items and flags them as candidates for um, moving them completely to the parent vault. which would actually just involve removing them from the child vault completely and maintaining them in the parent at that point, because that's only clear once you connect them. We might have to give it some more thought. And like I said, for now, this is just a thread. It's not really anything that we need to design or build at the moment, but I don't want to lose track of this topic since as we start to merge and consolidate more projects like this, I'm sure this will come up more often.

Found while connecting cs513-course and cs513-project to usc-f26 (2026-09-22):
- File name collision: sync mirrors cs513-project's wiki/index.md as wiki/projects/cs513-project/cs513-project.md, which shares a basename with the member's entity page wiki/projects/cs513-project/entities/cs513-project.md. A bare [[cs513-project]] in the parent is ambiguous, and lint makes no finding about mirrored pages.
- Overlap example: cs513-course's "CS513 Course Project" and cs513-project's "cs513-project" entity describe the same group project, and neither links to the other.
