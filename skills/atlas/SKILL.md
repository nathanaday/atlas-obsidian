---
name: atlas
description: "The front door: see every project on this machine and what is wrong with them, change the atlas settings, and find the right skill for any request. Use for /atlas, my projects, list projects, what projects do I have, which skill should I use, what can atlas do, refresh the atlas, atlas settings, set new-days, what is wrong with the atlas."
---

# The atlas

The atlas is every project on this machine. A project knows things (its
**wiki**) and does things (its **threads**); the atlas holds the projects and
the links between them. Every skill is a verb on one of those three nouns.
This skill shows the projects, changes the settings, and routes a request to
the skill that owns it.

Tools: `atlas`, `settings`. Both work from any session: inside a project, or
in a folder the atlas does not know.

## See what exists

1. Call `atlas`. It returns every project with its path, description, mode,
   wiki page count, inbox counts, open thread counts, the page that describes
   its work, and how recently it moved; the folders the atlas cannot read,
   with a reason; and the settings.
2. Show one line per project, the most recent first. Name each folder it
   cannot read and the reason. Keep it short: the user asked for orientation.
3. Say what needs attention: a project no page describes, a folder that is
   gone, a blocked or stale thread, a hub whose members are not mirrored.

## Route

| The user wants | Skill |
|---|---|
| **The atlas** | |
| Make a folder a project, blank or long-lived | `atlas-onboard` |
| Change a project: name, description, mode, threads on or off, members, sync, forget | `atlas-project` |
| Find what a hub's members hold in common; upgrade pages, build bridges | `atlas-merge` |
| **The wiki** (home: `wiki`) | |
| Turn a source into pages, of any size | `wiki-ingest` |
| Answer from the wiki | `wiki-query` |
| Keep an answer, a decision, or an insight | `wiki-save` |
| Change pages that exist: rewrite, rename, move, split, combine, fix | `wiki-edit` |
| Describe the work, or bring that page up to date | `wiki-describe` |
| Check the wiki's health, quick or deep | `wiki-review` |
| Roll up the log | `wiki-fold` |
| A canvas board | `wiki-canvas` |
| A Bases view | `wiki-base` |
| **The threads** (home: `thread`) | |
| Do this, fix this, move a thread on | `thread-work` |
| Note an idea, a bug, a chore | `thread-stub` |
| Define what done means | `thread-spec` |
| Decide how to do it | `thread-plan` |
| Do the planned work | `thread-run` |
| Close a thread | `thread-receipt` |
| See the board, change a card, a phase | `thread` |

`docs/skills.md` in the plugin holds the same map with what each skill owns.

## Settings and refresh

- `settings` with `new_days` sets how long a project counts as new in the
  view, and returns every setting; with no arguments it only reads. Say what
  the value means before changing it.
- `atlas` with `refresh: true` reads everything again and rewrites the
  registry. Run it after the user moved a folder by hand. A project heals its
  own path when a session starts in it, so refresh is for the view.

## What stays in the terminal

`atlas-obsidian doctor`, `recover`, `setup`, `open-vault`, `open-agent`,
`open-claude`, and `open-codex` are commands, not tools. Name the command; do
not run it through Bash unless the user asks.

Never edit `atlas/<name>/project.json`, `~/.atlas-obsidian/config.json`, or
`registry.json` with Write or Edit.

## Hand off

To the skill the route table names. Inside a project, `wiki` and `thread`
orient in each half.
