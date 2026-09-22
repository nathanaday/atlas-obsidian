---
name: thread-work
description: "Move work forward: take a thread, or a sentence that becomes one, to its next stage, and on through the stages while the user agrees: stub, spec, plan, run, receipt. It sizes the ceremony to the change, stops for the user's yes at the spec and at the plan, and works in small commits with subagents. Use for do this, make this change, implement, build, fix this, work on this, start on this now, continue this thread, next step, move this thread on, take this to the next stage. To only look at the board, thread."
---

# Work a thread

The conductor of the thread stages. Given a thread, it reads where the thread
stands and runs the next stage's skill; given a sentence, it opens the thread
first. It goes on through the stages while the user agrees, and it holds no
stage procedure of its own: each stage is done as its skill says.

Tools: `status`, `threads`, `thread`. Reads
[threads.md](../thread/references/threads.md).

## 1. Find the thread

1. Call `status` and `threads`.
2. A thread named by id or title, or one the user plainly means: take it.
   In a hub, a member's thread is worked in the member's own session: say so
   and offer `atlas-obsidian open-agent <project> --thread <id>`, because the
   code is there.
3. A sentence that matches no open thread is new work: open it with
   `thread-stub`'s rule, the user's words verbatim, and say so.
4. The change belongs to another project the atlas lists: say so and offer
   the same command for that project.

## 2. Size the ceremony

Decide which stages the change needs, and say so in one line before the
first:

| The change | Stages |
|---|---|
| Its definition of done is in doubt, or it spans sessions | spec, plan, run, receipt |
| Clear, and more than a few edits | plan, run, receipt |
| One obvious edit | run, receipt, with a plan of a few lines |
| Not worth doing | receipt, killed |

A skipped stage may be filed later. The user can always ask for more
ceremony; offer less only when the change is plainly small.

## 3. Run the next stage

| The thread is at | Next | Skill |
|---|---|---|
| stub | define what done means | `thread-spec` |
| spec | decide the approach | `thread-plan` |
| plan, work left | do the work | `thread-run` |
| plan, work done and tests pass | verify and close | `thread-receipt` |

Do the stage as its skill says, in full. Then report the document it filed,
with its path, in a few lines.

## 4. The gates

Two stages end in the user's yes before the next one starts:

- **After the spec**: the user agrees with what done means. A no means a
  revised spec, with Edit.
- **After the plan**: the user agrees with the approach. A no means a new
  plan.

Between the gates, and after the plan's yes, keep going without asking:
`thread-run` works slice by slice, commits each one, and writes progress; it
stops for a block, a surprise that changes the intent, or the end of the
work. When the user said at the start to go all the way, the gates still
show the document, and the work goes on unless the user stops it.

## 5. Stop well

At every stop, the thread's documents say where it stands: the last stage
filed, and under the plan's `## Progress`, what was done and what is next. A
later session, or `atlas-obsidian open-agent <project> --thread <id>`, picks
it up from there with this skill.

## Hand off

The stage skills, one after another. `thread` for the board.
