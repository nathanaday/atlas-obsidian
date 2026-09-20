---
name: wiki
description: "Orient in a atlas-obsidian session and route work to the right skill. Use for /wiki, set up wiki, project status, what is in this wiki, which skill should I use, make this a project, Obsidian vault, second brain, persistent wiki, wiki setup."
---

# Orientation

Atlas has one thing. A **project** is a folder `atlas/<name>/` inside the
user's work, a repository or a folder of documents, and it holds both halves of
what the project knows:

- The **wiki**, under `wiki/`: sources, entities, and concepts, with `inbox/`
  for what the user drops in, `ideas/` for their own scratch notes, and
  `.raw/captured/` for immutable copies of ingested sources. Every change to
  `wiki/` is one reviewed operation and one git commit.
- The **threads**, under `threads/`: one line of work each, with a document per
  stage in `threads/stubs/`, `threads/specs/`, `threads/plans/`, and
  `threads/receipts/`, the cards and the board at the top of `threads/`, and
  `threads/phases/` for the timeline.

The folder is an Obsidian vault the user opens, and `project.json` says what it
is. The wiki commits into the repository that holds the work, scoped to the
wiki's own paths, so an operation never touches the code.

The atlas MCP server (tools named `mcp__plugin_atlas-obsidian_atlas__<tool>`,
called `status`, `plan`, `apply`, and so on below) is the only write path
into the wiki.

## Find the place

Call `status` first. It reports the project's id, name, description, and path,
its mode, what git says about the work, the page that describes the work, its
thread counts by stage, its wiki's page count and git state, what waits in the
inbox, and warnings. The session hook's first line already names the place:
`atlas-obsidian: project …`.

If `status` fails because the session is in no project, hand off:
`atlas-project` makes the current folder a project, and `atlas` shows what
exists.

Do not create wiki files yourself. If `status` warns that an operation was
interrupted, tell the user to run `atlas-obsidian recover` before anything else.

## Never write wiki pages directly

Write, Edit, MultiEdit, and NotebookEdit are refused under `wiki/` by a hook.
Read pages with Read, Grep, and Glob as usual; change them only through `plan`
and `apply`. The core writes `wiki/log.md` and the source ledger itself; a plan
that names either is rejected.

The stage documents and the phase pages are different: their prose is the
model's to write with Edit. The hook refuses the cards and the board (the pages
directly under `threads/`), `project.json`, and a new file written straight into
a stage folder; the `thread`, `phase`, and `project` tools make those changes.

## Route the request

| Intent | Skill |
|---|---|
| Turn the sources waiting in `inbox/`, or supplied text, into pages | `wiki-ingest` |
| Answer from what the wiki already holds | `wiki-query` |
| Keep a specific answer, decision, or insight | `save` |
| Check the wiki's health | `wiki-lint` |
| Read or change the filing mode | `wiki-mode` |
| Roll up log entries | `wiki-fold` |
| Describe the work in the project's own wiki, or bring its page up to date | `describe` |
| Make a change now: do this, implement, fix this | `work` |
| See, change, review, or route threads; create or change a phase | `thread` |
| Note an idea as a thread, or open threads from the notes in `inbox/` | `thread-stub` |
| Define what done means for a thread | `thread-spec` |
| Decide how to do a thread | `thread-plan` |
| Work on a thread, or resume one | `thread-run` |
| Close a thread as completed or killed | `thread-receipt` |
| Work with an Obsidian Canvas | `canvas` |
| Author a Bases `.base` view | `obsidian-bases` |
| Obsidian syntax questions | `obsidian-markdown` |
| Reason carefully before a consequential change | `think` |
| See every project on the machine, refresh, or change a setting | `atlas` |
| Make this folder a project; rename it, change its description or mode, or forget it | `atlas-project` |

Query is read-only. Keeping an answer is a separate `save` operation the user
asks for. Never update the hot cache merely because a session ended.

Every tool acts on this session's project. The thread tools take `project` to
reach another project the atlas lists; the `atlas` tool names them.

## The operation contract

Read [operations.md](references/operations.md) before any change to the wiki.
In short:

1. Read every page you will change and keep its `sha256` from the plan preview
   or compute it; a page that changed since you read it makes apply fail
   closed.
2. Call `plan` with the kind, a one-line summary, and every write as complete
   file content. The core validates paths, frontmatter, JSON, and links, and
   returns a `plan_id`, a preview, and warnings.
3. Show the user the preview: created, updated, and removed paths, and every
   warning. Ask before apply when the change removes or replaces pages.
4. Call `apply` with the `plan_id`. Report the operation id and changed paths.

Every new canonical page joins `wiki/index.md` (generic mode) or a MOC (lyt
mode) in the same plan. Update `wiki/overview.md` only when the stable
high-level picture changed. Keep `wiki/hot.md` under 500 words.

An operation can be undone with the `undo` tool or `atlas-obsidian undo`; say so
when a user hesitates rather than skipping a review.

## Conditional references

Read only what the request needs:

- [operations.md](references/operations.md) for the plan and apply contract;
- [threads.md](references/threads.md) for the threads and the phase pages;
- [provenance.md](references/provenance.md) when a source enters or a claim
  needs support;
- [frontmatter.md](references/frontmatter.md) when defining or adopting page
  properties;
- [modes.md](references/modes.md) for domain scaffolds on top of the mode;
- [css-snippets.md](references/css-snippets.md) for requested visual changes;
- [plugins.md](references/plugins.md) when evaluating optional Obsidian
  plugins.

## Think, verify, grow

Before applying, pause once: observe the current state, verify the evidence,
then choose the smallest reversible operation that satisfies the request.
Afterward, report uncertainty and the next useful improvement without doing
it unasked.
