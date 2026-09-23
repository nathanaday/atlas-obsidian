---
type: plan
thread: thr-20260922-ef9b
title: "Lenses on the project map"
created: 2026-09-22
---

> [!plan] Lenses on the project map
> [Stub](<../stubs/Lenses on the project map.md>) → Spec → **Plan** → Receipt
> `thr-20260922-ef9b` · [Thread](<../Lenses on the project map.md>) · filed 2026-09-22

## Approach

A lens changes how the map shows each node and which card Enter opens. It does not change the graph, the layout, or the navigation. `l` cycles Details → Threads → Version control. The header names the lens (`Atlas · threads`). The summary line, the footer hints, and the keys panel follow the lens.

Enter keeps its current rule in every lens: on a hub in the overview, Enter opens the cluster; anywhere else, Enter toggles the card of the current lens. The selected node stays filled blue in every lens, and a problem entry stays red with `✗`.

### Details lens (default)

- The colors do not change.
- The card keeps Path, Description, Wiki, and Touched. It also keeps Mirrors, Mirrored by, Described, Last operation, and Inbox, because the stub does not remove them.
- The card loses Git, Threads, Phases, Open, and Signal.
- The signals list goes away. The blocked and stale threads move to the Threads lens. The Described row already reports a missing or outdated description page. A pending recovery moves onto the Wiki row, in red.

### Threads lens

- A node with open threads is yellow. Other nodes are gray.
- Size: a yellow braille disk drawn under the node. Its radius grows with the square root of the open count, so 1 thread gives a small disk and 9 threads a disk about three times as wide. The label stays at the node.
- The card shows:
  - the open count;
  - a histogram with one row per open stage (stub, spec, plan), where each bar is scaled to the largest count;
  - one line for blocked, stale, and waiting notes;
  - the phases;
  - a scrolling list of the open threads: `[stage] title` on one line, then a dim preview line.
- While the card is open, ↑ and ↓ move a cursor through the list, and the list scrolls inside the card. ← and → still step between projects, and the cursor goes back to the top.
- `c` with the card open and a thread under the cursor starts the agent on that thread and asks what to do. Without the card, `c` works as before.
- `p` with the card open starts the agent and asks the user to describe the thread to plant.
- `n` (the one-line stub prompt) does not change.
- A project with threads off shows a card that says so and names `atlas-obsidian edit NAME --threads on`. `p` refuses with the same message.

### Version control lens

- Red: the work has uncommitted changes. Yellow: the branch is ahead of or behind its upstream. Gray: in sync, no upstream, or no git.
- Red takes precedence over yellow.
- The card shows:
  - Branch;
  - Head (short sha and subject);
  - Upstream, with the ahead and behind counts and when the last fetch ran;
  - Remote (the origin URL);
  - Changes (the uncommitted count);
  - Last commit.
  A folder with no git says "not a git repository".
- `c` with the card open starts the agent on the project's git state.

## Data

- `links.Link` gains `Head`, `Subject`, `Upstream`, `Ahead`, `Behind`, `Remote`, and `Fetched`. `inspectRepo` reads them with local git commands.
- Refresh does not fetch. Ahead and behind are counted against the local upstream ref, so they are only as recent as the last fetch. The card shows the fetch date, taken from the modification time of `FETCH_HEAD`, so the user can see how old the counts are.
- `registry.ThreadLine` gains `Summary`: the first prose line of the thread's stub, which holds the user's own words. `threads.Summary` reads it and skips the frontmatter, the callouts, and the headings.
- Every new field has `omitempty`, so there is no schema bump. An older `registry.json` still reads, and the new rows appear after the next `R`.

## Launch prompts: the CLI first

The three-layer rule requires every new key to have a command.

- `open-agent`, `open-claude`, and `open-codex` take one of `--thread ID [--ask]`, `--plant`, or `--git`. These choices are mutually exclusive.
- `--thread ID` alone continues the thread, as it does now; `thread-work` and the docs depend on that. `--ask` makes it read the thread and ask.
- `claudecode` owns every prompt, and `claudecode.Prompt(harness, Intent)` builds it. It also takes over the Codex `/atlas-obsidian:` → `$` rewrite from `cli.go`.
  - Ask: `/atlas-obsidian:thread <id>`, then the text: read this thread end to end, then ask what I want to do with it; offer to move it to its next stage (name the stage and its skill) or to kill it, with or without a reason.
  - Plant: `/atlas-obsidian:thread-stub`, then: ask me to describe the thread I want to plant, then open it from my words.
  - Git: plain text with no skill: check this project's git state; if the folder is not a repository, offer to initialize one; otherwise summarize `git status` and offer to stage, commit, push, pull, or anything else; do nothing until I choose.
- `tui.Opener.Agent` becomes `func(harness, path string, in claudecode.Intent) error`. The CLI wires it through the same `Prompt` function the flags use.

## Order

1. The git facts in `links`, with tests over a temporary repository cloned from a bare one, to cover upstream, ahead, and behind.
2. `threads.Summary` and `ThreadLine.Summary`, with tests.
3. `claudecode.Intent` and `Prompt`, the CLI flags and their exclusivity, and the tests.
4. The lens framework in the view: the `l` key, the header, the hints, the help, the lens-aware `nodeStyle`, and the trimmed details card.
5. The Threads lens: the color, the disk (the canvas gains a third color layer), the card with the histogram and the list, and the `↑↓`, `c`, and `p` keys.
6. The Version control lens: the color, the card, and `c`.
7. Docs: the key and command table in `docs/usage.md`, the view section of the README, the `internal/tui` line in CLAUDE.md, and the flags in `docs/threads-design.md`.
8. `make test`, then run the view in a real terminal (tmux) over this machine's projects and look at each lens at a narrow and a wide width.

## Tests

- `view_test.go` drives the view with `tea.KeyMsg`:
  - `l` cycles the lenses and wraps;
  - `nodeStyle` gives the right color per lens and state;
  - the details card has no Git or Threads row;
  - the histogram rows match the counts;
  - ↑ and ↓ move the list cursor and clamp at both ends;
  - ← and → reset the cursor;
  - a fake `Opener` records the `Intent` for `c` and `p` in each lens;
  - `p` on a project with threads off gives an error, and the opener is not called.
- The canvas tests cover the third layer.
- The CLI tests cover each flag, the exclusive combinations, and `--ask` without `--thread`.

## Risks

- The disk can clutter a dense cluster. Fallback: a marker glyph that grows (`·` `•` `●`) plus the count after the name.
- ↑ and ↓ move the node selection today. Inside the thread card they move the list cursor instead, and the footer hints must say so.
- Ahead and behind can be out of date. The fetch date on the card makes that visible, and the git prompt tells the agent to fetch first.
- The yellow of the thread nodes is close to the label color `#FFC600` used on cards. They appear in different places, so this is acceptable.

## Progress

- 2026-09-22: Slices 1 and 2 done (438450a). `links.Link` now has head, subject, remote, upstream, ahead, behind, and fetched, all from local refs. `threads.Summary` returns the first paragraph of the stub and returns "" when that paragraph only repeats the title (an empty stub).
- 2026-09-22: Slices 3 to 7 done. `claudecode.Intent` and `Prompt` own every first message and the Codex `$` rewrite. `open-agent`, `open-claude`, and `open-codex` take `--thread ID [--ask] | --plant | --git`. `--ask` refuses a closed thread, like `--thread`, and `--plant` refuses when threads are off. The view has the three lenses, the disk layer on the canvas, the threads card with its histogram and scrolling list, and the version control card. Docs updated.
- Changes from the plan:
  - Details card: kept "not described" as a Described row and "unreachable" as a State row, so no signal is lost.
  - The details summary line no longer counts threads; the threads lens does.
  - The footer hint is `l lens`; the full hint list fits at width 160.
  - The summary line puts the description last and clips it (`fitJoin`). Before, a long description pushed every fact off the line in every lens. This bug was already there before this thread.
  - The histogram bars stop at 30 cells, and a preview line ends in an ellipsis.
- Checked by rendering the real atlas (20 projects) through each lens at 130×38 and 70×20; tmux is not installed. The disk reads as a small yellow blob beside the name. The glyph fallback is not needed.
- 2026-09-22: The cursor is clamped when a refresh shortens the list. `make install` done (5.6.0).
- Next: the user tries `c` and `p` on the threads card and `c` on the version control card in a real terminal. Those launches cannot run under `go test`. Then `thread-receipt`.
