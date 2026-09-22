---
name: thread-receipt
description: "Verify and close a thread with its receipt: before completed, run the checks and, for a change to code, have a fresh reviewer read the diff against the spec; then file what was delivered and how it was verified, or, killed, the reason; then offer the wiki what the work taught. Use for finish, done, close this thread, complete, ship it, review the work, kill, cancel, abandon, drop this thread, wrap up."
---

# Close a thread

The receipt is the record of how the thread ended. A thread may close from
any stage: a stub that was never worth doing gets a receipt as much as a
feature that shipped.

Tools: `status`, `threads`, `thread`. Reads
[threads.md](../thread/references/threads.md). Sends the `thread-review`
agent.

## Decide the outcome

Call `threads` with the `id` and read the documents. `completed` means the
spec's definition of done holds, or the plan's when there is no spec.
`killed` means the work stops for good. Ask when it is unclear.

## Verify before completed

1. **Run the checks.** The tests and every check the plan names, now, and
   read the result. A thread does not complete on tests that fail or were
   not run.
2. **Review the work.** When the thread changed code, send the
   `thread-review` agent: a fresh reader with the spec, the plan, and the
   range of commits the work made (`git log` since the plan was filed, or the
   branch). It reports what the work misses from the spec, what it breaks,
   and what it leaves untested, each with the file and line. In a host
   without workers, do the same read yourself, as a reviewer who did not
   write the code.
3. **Act on it.** A finding that is right and small is fixed now, in its own
   commit, and the checks run again. A finding that is right and large goes
   back to `thread-run`, and the thread stays open. A finding you reject gets
   a reason in the receipt.

When the work sits on a branch, it merges after the checks pass.

## Write the receipt

Call `thread` with `id`, `stage: receipt`, `outcome`, and the receipt as
`text`:

- **Completed:** what was delivered, in the user's terms; where it is
  (commits, files, pages); how it was verified, with the commands, the
  result, and what the review found and what became of it; where the work
  left the spec or the plan, and why; what is left undone.
- **Killed:** the reason, and what to keep from the work so far.

Keep it short and exact. The earlier documents stay as they are: they are
the record of how the thread went.

The card moves to `threads/archive/`. Report the receipt's path. When the
thread's phase is now finished, say so.

## What is left

Work left undone is a new thread: offer a stub for each item, in the same
phase, and open the ones the user picks.

## The wiki learns from completed work

Only for a `completed` thread, offer the wiki what the work taught, in one
line each, and wait for a yes. Declining costs nothing.

- **The page that describes the work**, when `status` says it is behind or
  missing: `wiki-describe` brings it up to date.
- **A durable fact** the work found that outlives it (a cause, a mitigation
  that works, a measurement): `wiki-save` keeps it on a concept page.

## Hand off

`wiki-describe` or `wiki-save` for what the wiki should learn; `thread-stub`
for the work left undone; `thread` for the board.
