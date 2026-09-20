---
name: atlas-project
description: "Make the current folder a project, with its wiki and its threads, or change one: name, description, filing mode, describe, forget. Use for new project, init here, make this a project, project for this repo, set up a wiki here, rename the project, change the mode, forget this project, adopt this vault."
---

# Make or change a project

Tools: `atlas`, `project` on the atlas MCP server. A project is a folder
`atlas/<name>/` inside the user's work, a repository or a folder of documents.
The folder takes the project's name, and it holds both halves of what the
project knows: the wiki under `wiki/`, with its inbox and its raw store, and
the threads under `threads/`. The work's repository tracks it like any other
folder; the wiki's operations commit into that repository, scoped to the
wiki's own paths.

Call `atlas` first, to see the projects that exist.

## Make one

Ask one question at a time. Offer the default; accept a yes.

1. **Where.** The current folder, when the session is in the work. Otherwise
   the path the user names. Never a folder inside another project.
2. **Name.** Default: the folder's name.
3. **Description.** One to three sentences saying what the work is and what
   its wiki should remember. `wiki-ingest` and `wiki-query` read it to judge
   what belongs. Draft it from the folder's README or CLAUDE.md when there is
   one, and read it back.
4. **Mode.** `generic` files a new page by type, which suits most work;
   `lyt` keeps atomic notes in `wiki/notes/` and navigates them through Maps
   of Content. Offer generic unless the user asks otherwise.

State the whole change in one line. When the folder is in no git repository,
init makes it one; say so in the line:

> Make `~/code/webapp` the project `webapp` (generic mode), "The customer-facing web application for the fire-detection product; its wiki holds the alarm pipeline and the field tests"?

On yes: `project` with `action: init`, `work`, `name`, `description`, and
`mode`; add `no_git` only when the user refuses a repository, and say that the
wiki then has no history and no operation can run. It writes `atlas/<name>/`
with both halves, commits them, and lists the folder in the atlas config.

Report the result, then offer `describe`, which writes the page in the wiki
that says what the work is, and say how to work: a session anywhere inside the
work is the project's session, and `claude-atlas open-vault NAME` opens
`atlas/<name>/` in Obsidian.

If the tool refuses, say why in the tool's words and ask again for that one
answer; do not retry with a guess.

## Change one

- Rename, describe, or set the mode: `project` with `action: edit`, and
  `name`, `description`, or `mode`. A new name moves `atlas/<name>/` to match;
  say so in the one-line statement. The work folder does not move; it is the
  user's. A new mode routes future pages only and moves nothing.
- Forget: `project` with `action: forget`. The work folder and its
  `atlas/<name>/` stay. Deleting `atlas/<name>/` is how a project ends, and
  that is the user's to do by hand.
- A folder of an earlier version is refused with the command to run:
  `claude-atlas upgrade PATH`. It absorbs a 3.x knowledge base into the
  project, moves the thread folders under `threads/`, and turns the task pages
  of 2.x into threads. It moves the user's files, so give it to the user to
  run rather than running it.

`work` names another project by name; this session's project is implied. Every
question comes before the tool call, and the tool call comes after a yes. Never
make the folder or its files yourself.
