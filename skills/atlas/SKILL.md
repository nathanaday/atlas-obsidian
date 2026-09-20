---
name: atlas
description: "Orient in the whole atlas and route: every project on this machine, what is wrong, the settings. Use for /atlas, my projects, list projects, what projects do I have, my vaults, refresh the atlas, atlas settings, set new-days, what is wrong with the atlas."
---

# The atlas

Tools: `atlas` and `settings` on the atlas MCP server. Both work from any
session: inside a project, or in a folder the atlas does not know.

## See what exists

1. Call `atlas`. It returns every project with its path, description, mode,
   wiki page count, inbox counts, open thread counts, the page that describes
   its work, and its heat; the folders the atlas cannot read, with a reason;
   and the settings.
2. Show a compact picture: one line per project, warmest first. Name each
   problem entry and its reason. Keep it short; the user asked for
   orientation, not a dump.
3. Say what needs attention: a project no page describes, a registered folder
   that is gone, a folder waiting for `atlas-obsidian upgrade`, a blocked or
   stale thread.

## Route

| The user wants | Skill |
|---|---|
| Make this folder a project; rename it, change its description or mode, forget it | `atlas-project` |
| Work inside a project: its wiki or its threads | `wiki` |
| See or change threads | `thread` |

## Settings and refresh

- `settings` with `new_days` sets it and returns all; with no arguments it
  reads. Say what the value means before changing it: how long a project
  counts as new in the view.
- `atlas` with `refresh: true` reads everything again and rewrites the
  registry. Run it after the user moved a folder by hand; a project heals its
  own path when a session starts in it, so refresh is for the view, not for
  the config.

## What stays in the terminal

`atlas-obsidian doctor` (the installation check), `upgrade`, `recover`, `setup`,
`open-vault`, and `open-claude` are commands, not tools. Name the command; do
not run it through Bash unless the user asks. `upgrade` moves the user's
files, so it is always theirs to run.

Every write here is reversible or leaves the folder alone, so no plan preview
exists; the skill that writes states the change in one line and waits for yes.
Never edit `atlas/<name>/project.json`, `~/.atlas-obsidian/config.json`, or
`registry.json` with Write or Edit.
