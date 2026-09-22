# Obsidian itself

A project's folder is an Obsidian vault. It needs Obsidian's core features
only: no community plugin, theme, or downloaded program. Read this when the
user asks about Obsidian itself: how the vault looks, a plugin, or a clipper.

## Open the vault

`atlas-obsidian open-vault NAME` opens `atlas/<name>/` in Obsidian, and adds
it to Obsidian's list of vaults when it is not there. Install Obsidian from
[Obsidian Help](https://help.obsidian.md/). The core Properties, Backlinks,
Outline, Graph, and Bases views are the ones the skills write for.

## The vault's colors and callouts

Every project carries `.obsidian/snippets/atlas-obsidian.css`, turned on in
`.obsidian/appearance.json`. It gives each kind of place one color in the
file explorer (the wiki and its folders, `inbox/`, `ideas/`, the thread
stages) and defines the callouts in [syntax.md](syntax.md). Change a color in
the variables at the top of that file; do not add a second snippet for the
same folders. `atlas-obsidian` rewrites the snippet on a refresh of the
project and keeps the user's choice to turn it off.

When the explorer shows no colors: Settings, Appearance, CSS snippets, turn
`atlas-obsidian` on, or reload Obsidian.

## Graph groups

Obsidian stores graph groups in its own settings; the user sets them in the
graph view's settings. A set that matches the snippet:

| Query | Color |
|---|---|
| `path:wiki/sources` | `#ce9178` |
| `path:wiki/entities` | `#c586c0` |
| `path:wiki/concepts` | `#dcdcaa` |
| `path:wiki/canvases` | `#b5cea8` |
| `path:wiki/projects` | gray: the mirrors of members |
| `path:threads` | `#c586c0` at lower opacity, or hidden |

## Community plugins

Community plugins are third-party code. Before recommending one:

1. Confirm the need, and get the user's yes for the install.
2. Check the publisher, the source repository, the permissions, and how it is
   maintained.
3. Install through Obsidian's own interface; never copy a `main.js` in.
4. When the plugin can rewrite notes or properties, test it on a copy of the
   vault first.

A git plugin that commits in the background is a convenience for syncing,
not the safety net: every atlas operation is already one commit, and a
background commit can race with an apply. Turn its automatic commits off while
an operation runs.

## Web Clipper

A clipped page is a source like any other: save it into `inbox/`, then
`wiki-ingest` captures it and cites it. A clip is not evidence that its
content is true.
