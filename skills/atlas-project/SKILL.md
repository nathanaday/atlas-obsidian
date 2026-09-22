---
name: atlas-project
description: "Change a project that exists: its name, description, filing mode (generic or lyt), threads on or off, and members, which make it a hub that mirrors other projects' wikis; sync the members; or forget the project. Use for rename the project, change the description, what is my mode, switch to lyt, use generic, turn threads off, add a member, link projects, make a hub, mirror another project, sync the members, forget this project. To make a new project, atlas-onboard."
---

# Change a project

A project's identity is `atlas/<name>/project.json`: its id, name,
description, mode, whether it tracks threads, and its members. This skill
changes it, one field at a time, and syncs the members' mirrors. Making a
project is `atlas-onboard`'s.

Tools: `atlas`, `status`, `route`, `project`. Reads
[modes.md](../wiki/references/modes.md) for the mode.

Every change is one line to the user and a yes before the call. `work`
names another project by name, id, or path; this session's project is
implied. Never edit `project.json` or the atlas config with Write or Edit.
When the tool refuses, say why in its words and ask again for that one
answer; do not retry with a guess.

## Name and description

`project` with `action: edit` and `name` or `description`. A new name moves
`atlas/<name>/` to match: say so in the line. The work folder does not move;
it is the user's. The description is what `wiki-ingest` and `wiki-query`
read to judge what belongs; one to three sentences.

## Mode

`status` gives the mode; `route` with a type and title shows where a new page
would go. `project` with `action: edit` and `mode` (`generic` or `lyt`)
changes it as one commit. Say the old mode and the new one, and that only
future pages follow: nothing moves, no link changes. When the user wants the
pages that exist reorganized too, that is a separate `wiki-edit` operation
with a complete move map.

## Threads on or off

`project` with `action: edit` and `threads`. Off, the thread tools refuse
and the session hook lists no threads; the `threads/` folder stays as it is.
On again, everything in it comes back.

## Members: one wiki over several projects

A member is a project whose wiki (and threads, when both track them) this one
mirrors under `wiki/projects/<name>/`. The member never knows, and a project
may be a member of any number of hubs.

- **Add or remove**: `project` with `action: edit` and `add_members` or
  `remove_members`, each a list of projects by name, id, or path. The tool
  refuses this project itself, a project the atlas does not list, and a
  cycle. State the change as "Make svc-a a member of platform; the next sync
  mirrors its wiki".
- **Sync**: `project` with `action: sync`. It mirrors the transitive closure
  of the members, flat, as one operation, and reports each project with its
  page count, and the counts and the commit when something changed. Nothing
  new means no commit. Run it after adding or removing a member; the
  session-start hook runs it too. A mirrored page changes in its own project,
  never here.

After the first sync of a new member, offer `atlas-merge`: it finds what the
members' wikis hold in common and proposes what to upgrade into this wiki.

## Forget

`project` with `action: forget` drops the project from the atlas. The work
folder and its `atlas/<name>/` stay. Deleting `atlas/<name>/` is how a
project ends, and that is the user's to do by hand.

## Hand off

`atlas-merge` after a member's first sync; `wiki-edit` to move pages after a
mode change; `atlas` to see every project.
