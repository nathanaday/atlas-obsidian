---
name: thread-plan
description: "Plan a thread: read its spec, explore the code as in plan mode, and file what you found as the plan document: the approach, where the work lands, the order, how it is tested, the risks. Use for plan this thread, how should we build this, approach, break it down, ready this thread."
---

# Plan a thread

The plan is what you find when you look at the code with the spec in hand:
the same thinking as an agent's plan mode, kept as a document. It is not
a script for someone else to follow line by line, and nobody reviews it but
the user.

Tools: `threads`, `thread`. Reads
[threads.md](../thread/references/threads.md).

## Find the approach

1. Call `threads` with the `id`. Read the spec, or the stub when the thread
   has no spec. A stub too loose to plan from goes to `thread-spec` first;
   say why.
2. Explore: the code the work touches, how it is tested, the conventions
   around it, the phase's goal when the thread names a phase. Read only;
   change nothing.
3. Settle the approach. When two approaches are close, pick one and say
   what the other would have cost. Ask the user only when the choice is
   theirs. Trace what the change touches beyond its own files: the callers,
   the consumers, the data it reads and writes, the way back when it fails.
   A plan that is sound locally and breaks a neighbour is not sound.

## What the plan holds

- The approach in a paragraph, and why this one.
- Where the work lands: the packages, files, or documents, and what changes
  in each.
- The order of the work, in slices that each leave things working and can
  each be a commit.
- How it is tested: what gets a test, and the checks that must pass before
  the thread can close.
- Risks and unknowns, and what would change the plan.
- Anything in the spec that the code shows to be wrong or costly. Say it
  here, and fix the spec with Edit when the user agrees.

Write at the level of intent. Whoever does the work, you or a subagent,
reads the spec too and makes the detailed choices with the code in view.

## File it

In plan mode, present the plan the usual way; when the user approves, file
the same text. Otherwise file it directly. Call `thread` with `id`,
`stage: plan`, and the plan as `text`. The thread is now at its plan. Tell
the user the path and the approach in a few lines.

## Hand off

The plan is a gate: the user agrees before the work starts. Then
`thread-run`, or `thread-work` to go on through the stages.
