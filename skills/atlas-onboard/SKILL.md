---
name: atlas-onboard
description: "Make a folder a project in one guided flow: a blank folder gets its wiki, its threads, and a git repository; a long-lived repository or folder of documents also gets the page that describes it, a wiki structure the user approves, and threads for the work already in flight (TODO and FIXME markers, roadmap documents, open issues). Use for onboard this, onboard this repo, new project, init here, make this a project, set up a wiki here, set up atlas, adopt this codebase, start a project."
---

# Onboard a folder

A folder becomes a project: `atlas/<name>/` inside it, with the wiki and the
threads, listed in the atlas. A blank folder needs only that. A folder with
history also needs the wiki to know the work and the board to hold the work
already under way, so a first session there starts with a map and a list,
not a blank page. Every step shows what it will do and waits for a yes.

Tools: `atlas`, `project`, `status`, `threads`, `thread`. Reads
[modes.md](../wiki/references/modes.md) for the structure. Hands the
describing to `wiki-describe`.

## 1. Look before asking

Call `atlas`, to see the projects that exist. Then look at the folder:

- inside a project already, or inside another project's folder: stop, say
  which, and hand to `atlas-project`;
- blank: no files, or only a README;
- long-lived: code, documents, a git history.

Read the README, AGENTS.md, and CLAUDE.md when there are any; they answer
most of the questions below.

## 2. Make the project

Ask one question at a time, each with a default the user can take with a yes:

1. **Where.** The session's folder, or the path the user names.
2. **Name.** The folder's name.
3. **Description.** One to three sentences: what the work is and what its
   wiki should remember. `wiki-ingest` and `wiki-query` read it to judge what
   belongs. Draft it from what you read.
4. **Mode.** `generic` unless the user wants `lyt` ([modes.md](../wiki/references/modes.md)).
5. **Threads.** On unless the user wants a wiki only.

State the whole change in one line, and say when the folder becomes a git
repository:

> Make `~/code/webapp` the project `webapp` (generic mode, threads on), "The customer-facing web application; its wiki holds the alarm pipeline and the field tests". The folder is in no repository, so this runs `git init` there.

On yes: `project` with `action: init`, `work`, `name`, `description`,
`mode`, and `threads`. Pass `no_git` only when the user refuses a
repository, and say that the wiki then has no history and no operation can
run until there is one. On a refusal, say why in the tool's words and ask
again for that one answer.

A blank folder is done here: go to step 6.

## 3. Describe the work

Hand to `wiki-describe`. It stages a snapshot of the work, reads it and the
code, and writes the page that says what the project is, how it is built and
laid out, and what it has delivered, as one operation the user approves. Its
snapshot also lists the TODO, FIXME, XXX, and HACK lines of the work, which
step 5 reads.

## 4. Propose the structure

The mode files pages by type; a profile in [modes.md](../wiki/references/modes.md)
adds the folders and page types one kind of work needs. From the snapshot
and the page `wiki-describe` wrote:

1. Pick the profile that fits (a software repository, research, a course,
   a business project), or none.
2. Name the pages the work already calls for: the components, modules, or
   documents a reader will look up, as entities; the ideas the work
   introduces, as concepts. At most twelve, each with a one-line reason.
   A page the describe operation wrote is not proposed again.
3. Show the structure as one list. On a yes, `wiki-edit` seeds the pages the
   user picked, as one commit through the `stub` tool. A seed costs little,
   and lint lists it as a stub until an ingest or a save fills it.

An empty folder tree is not a structure. Propose folders only for pages that
exist or are seeded now.

## 5. Gather the work in flight

When threads are on, find the work already under way:

- the snapshot's markers: group the TODO and FIXME lines by what they are
  about; one thread per piece of work, not per line;
- roadmap, TODO, or plan documents in the work (the snapshot lists the
  headings of every document under `docs/`);
- open issues, when the work has a GitHub remote and `gh` is signed in:
  `gh issue list --state open --limit 50`. Ask before the first call; it
  reaches the network.

Show the candidates as one numbered list: a title, where it came from, and a
line of the text. The user picks. Each pick becomes a thread with
`thread-stub`'s rule: the words as found, with the file and line or the issue
number, and nothing added. A pile of forty stubs helps nobody; say when the
list is long and offer the ten that matter most.

## 6. Report

Say what exists now: the project's folder, the describe operation, the
seeded pages, the threads opened, and what was skipped. Then say how to
work: a session anywhere inside the work is the project's session;
`atlas-obsidian open-vault NAME` opens it in Obsidian; the inbox takes
sources for `wiki-ingest` and notes for `thread-stub`.

## Hand off

`thread-work` on a thread the user picks, `wiki-ingest` for sources waiting
in the inbox, `atlas-project` to change what was set here, and to make this
project a member of a hub, or a hub of others.
