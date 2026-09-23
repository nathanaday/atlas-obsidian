# atlas-obsidian

A wiki and the state of the work, in every project.

[![License: MIT](https://img.shields.io/badge/license-MIT-2563eb.svg)](LICENSE)
[![Go](https://img.shields.io/badge/Go-1.24+-00ADD8.svg?logo=go&logoColor=white)](go.mod)
[![Claude Code plugin](https://img.shields.io/badge/Claude%20Code-plugin-7c3aed.svg)](.claude-plugin/plugin.json)
[![Codex plugin](https://img.shields.io/badge/Codex-plugin-10a37f.svg)](.codex-plugin/plugin.json)
[![Version](https://img.shields.io/badge/version-5.2.0-d97745.svg)](.claude-plugin/plugin.json)

## What it is

A new agent session needs context from the last one. atlas-obsidian
keeps what a project knows in the project itself, as Markdown files that git
tracks and Obsidian opens.

It comes in two pieces that install separately: a Go binary, which makes a
folder of work into a project and serves its MCP tools, and a plugin for
Claude Code and Codex, which shares the same skills and hooks between them.
Either agent can pick up the same wiki and threads without conversion.

A project is one folder, `atlas/<name>/`, sitting beside your code. It holds
two things.

**The wiki** is Markdown pages that cite where they came from. Put a PDF, an
article, or a note in `inbox/` and the agent reads it, keeps an unmodified copy
under `.raw/captured/`, and writes pages that cite that copy. Ask the wiki a
question later and the answer names the page and the source behind it.

**The threads** are the work in progress. A thread is one bug, feature, or
chore, and it moves through four documents: a stub in your own words, then a
spec, a plan, and a receipt. Nothing records which stage a thread is in. The
furthest document that exists is the stage, so every change of state is a page
you can open and read.

Start either agent anywhere inside the repository and a hook hands the session the
project's description, the wiki's recent context, which threads are open, and
what is waiting in the inbox.

## Getting started

You need [Claude Code](https://claude.com/claude-code) or
[Codex CLI](https://learn.chatgpt.com/docs/cli), with `claude` or `codex` on your
PATH, git, and Go 1.24 or newer to build the binary. For Codex, use a release
with `codex plugin` and `/hooks` support. macOS and Linux are
supported. [Obsidian](https://obsidian.md) is optional — the files are plain
Markdown and read fine anywhere, but the folder is laid out for it, with a CSS
snippet that colors each kind of folder.

Build and install the binary:

```bash
git clone https://github.com/nathanaday/atlas-obsidian.git
cd atlas-obsidian
make install
```

That puts `atlas-obsidian` in `$(go env GOPATH)/bin`, usually `~/go/bin`; add
that directory to your PATH if it is not there already. Use `make install`
rather than `go install` so the binary carries its version number, which lets
`atlas-obsidian doctor` tell you when the binary and the plugin have drifted
apart.

Then install the plugin for your agent from this checkout:

```bash
# Claude Code
atlas-obsidian setup --agent claude --plugin-source "$PWD"

# Codex
atlas-obsidian setup --agent codex --plugin-source "$PWD"
```

Setup prints what it intends to do and waits for you to agree. It creates
`~/.atlas-obsidian/` and installs through the selected agent's plugin CLI.
Run both commands if you use both agents. Without `--agent`, setup uses Claude
Code; without `--plugin-source`, it uses your saved source (GitHub by default).

For Codex, start a new session, open `/hooks`, review and trust the Atlas hooks,
then restart the session so the startup context is loaded. Codex skips hooks
until they are trusted. Check `/mcp` for the `atlas` tools and use the `$` skill
picker to find the bundled skills. See the official
[plugin](https://learn.chatgpt.com/docs/plugins) and
[hook](https://learn.chatgpt.com/docs/hooks) documentation.

Check your installation with `atlas-obsidian doctor --agent codex` or
`atlas-obsidian doctor --agent claude`.

The equivalent manual Codex installation, from the checkout, is:

```bash
codex plugin marketplace add "$PWD"
codex plugin add atlas-obsidian@nathanaday-atlas-obsidian
```

## Making a project

Start a session in the folder and run `/atlas-obsidian:atlas-onboard`
(`$atlas-onboard` in Codex). It makes the project, and for a folder with
history it also describes the work in the wiki, proposes a structure for the
wiki, and turns the TODO and FIXME lines, roadmap notes, and open issues it
finds into threads you pick. Or from the terminal:

```bash
cd ~/code/webapp
atlas-obsidian init --description "The customer-facing web app."
```

This writes `atlas/webapp/` beside your code, commits it, and records the
folder in the atlas config. If the folder is not in a git repository yet, init
creates one, since the wiki needs a history to commit into. Pass `--no-git` if
you would rather it did not.

Open a thread and start a session:

```bash
atlas-obsidian thread webapp new "Filter vehicle false alarms" --priority high
claude
# or: codex
```

The thread command writes a card and a stub in the words you gave it. The
session that follows opens with that thread in view.

From there, choose the skill in your agent. Every skill is a verb on one of
three nouns, so the name says where it belongs: `atlas-…` for projects,
`wiki-…` for what a project knows, `thread-…` for what it does. In Codex,
type `$` and pick the skill.

| Claude Code | Codex | What it does |
|---|---|---|
| `/atlas-obsidian:atlas` | `$atlas` | every project, and the map of every skill |
| `/atlas-obsidian:thread-work` | `$thread-work` | moves a thread to its next stage, and on: spec, plan, the work, the receipt |
| `/atlas-obsidian:wiki-ingest` | `$wiki-ingest` | turns the sources in the inbox into cited pages |
| `/atlas-obsidian:wiki-query` | `$wiki-query` | answers from the wiki |
| `/atlas-obsidian:wiki-review` | `$wiki-review` | checks the wiki, quick or deep |

For example, `/atlas-obsidian:thread-work Filter vehicle false alarms` writes
the spec for that thread and stops for your yes, then the plan, then does the
work. All 21 skills and how they fit: [docs/skills.md](docs/skills.md).

To teach the wiki something, drop the source in the inbox and start a session:

```bash
cp ~/Downloads/paper.pdf ~/code/webapp/atlas/webapp/inbox/
claude
# or: codex
```

`/atlas-obsidian:wiki-ingest` reads whatever is waiting and writes pages that
cite it. `/atlas-obsidian:wiki-query` answers questions from those pages
without changing anything. A short note dropped in the same inbox becomes a
thread instead, through `/atlas-obsidian:thread-stub`.

In Codex the same workflows are `$wiki-ingest`, `$wiki-query`, and `$thread-stub`.

Every command and its options: [docs/usage.md](docs/usage.md).

## One wiki over several projects

A project may list other projects as members. Its wiki then mirrors each
member's wiki under `wiki/projects/<name>/`, and its own pages link into those
mirrors with ordinary wikilinks, so Obsidian shows one graph over the whole
ecosystem and a session there reads one wiki.

```bash
atlas-obsidian edit platform --add-member svc-a --add-member svc-b
atlas-obsidian sync platform
```

The mirrors are derived: sync rewrites them as one operation, the guard refuses
edits there, and a member never knows it is listed. The members' threads come
along too, under `threads/projects/<name>/`, and the hub's board shows every
open thread in the ecosystem; a change to a member's thread, made from the hub,
lands in the member. When two members wrote about one thing, `overlap` finds
the pair and `/atlas-obsidian:atlas-merge` proposes the page that replaces
both, or a bridge between them. The design and its reasons:
[docs/members-design.md](docs/members-design.md).

## Seeing every project

```bash
atlas-obsidian
```

With no arguments the binary opens a map of every project it knows. Each
project is a node and each member link is an edge, laid out by a live force
simulation, so the projects that share knowledge sit together and the map
settles like a mobile. `→` and `←` walk the clusters, one per project that no
other project mirrors. Enter on a hub opens its cluster onto the whole screen,
where the arrows walk every project in it, and Esc steps back out. `/` finds any
project by name as you type, and Shift+arrows nudge a node while the rest
answer. The selected project is filled, what it mirrors and what mirrors it are
lit, and one line under the map says how it connects. Enter on a project opens
its card with only the facts that exist.

`l` changes the lens: the same map colored by one aspect. The threads lens
shows which projects have open threads, with a disk that grows with the count,
and its card lists them; the version control lens shows uncommitted work in red
and a branch ahead of or behind its upstream in yellow.

From the map: `o` opens `atlas/<name>/` in Obsidian, `c` starts your preferred
harness in the work folder, `i` opens it in VS Code, `t` opens a terminal
window there, `n` opens a new thread, `R` re-reads everything, `,` opens the
settings, and `?` shows the keys.

The settings hold the harness (Claude Code or Codex) and the IDE, saved
globally. `atlas-obsidian config preferred-harness codex` sets the same thing.
`open-agent webapp` launches the preferred harness; `open-claude` and
`open-codex` select one; add `--thread "Filter vehicle"` to continue a thread,
with `--ask` to be asked what to do with it; `--plant` plants a new thread and
`--git` opens a session on the work's git state.
`open-ide webapp` and `open-terminal webapp` open the work folder in VS Code
and in a terminal window.

## How writes reach the wiki

The shared guard refuses Claude's Write/Edit tools and Codex's `apply_patch`
anywhere under `wiki/`. The agent prepares a plan instead. You see
which files the plan creates, replaces, or deletes, and approving it makes one
git commit. `atlas-obsidian undo` takes a commit back by restoring the files it
touched from its parent, and refuses if you have changed one of those files in
the meantime.

Anything you typed in Obsidian is committed first, as its own operation, so an
undo can never reach your own edits. Every git command an operation runs is
limited to `wiki/`, `.raw/`, `inbox/`, and `project.json`, so an apply or an
undo never stages your source code.

These hooks must be enabled (and trusted in Codex). They cover the hosts' file
edit tools, not arbitrary shell commands; the skills also prohibit shell writes
that bypass the operation workflow.

Some pages belong to the tool rather than to the agent: the wiki's log, the ledger
of sources, the thread cards, the board that lists them, and the opening
callout of each thread document. The binary derives all of those from the files
around them, and the agent writes the prose pages.

## What the folder holds

A new project looks like this:

```
webapp/                          your repository
├── ...                          your work, untouched
└── atlas/
    └── webapp/                  named after the project; a rename moves it
        ├── project.json         name, description, filing mode, threads on or off
        ├── wiki/                index.md, log.md, hot.md, overview.md
        ├── threads/             stubs/ specs/ plans/ receipts/ phases/ archive/; optional
        ├── inbox/               what you drop in: sources, and notes
        ├── ideas/               your scratch notes, which nothing reads
        └── .obsidian/           vault settings and the CSS snippet
```

The rest arrives as it is used. Opening a thread adds its card directly under
`threads/`, alongside `threads.md`, the board that lists them all; filing wiki
pages creates `wiki/sources/`, `wiki/entities/`, and `wiki/concepts/`; and
ingesting a source creates `.raw/captured/`.

Two filing modes decide where a new wiki page lands. `generic`, the default,
files by type into the folders above. `lyt` keeps atomic notes in `wiki/notes/`
and navigates them through Maps of Content in `wiki/mocs/`. Switch with
`atlas-obsidian edit webapp --mode lyt`; existing pages stay where they are.

## Where the atlas keeps its own state

```
~/.atlas-obsidian/
├── config.json             every project's work folder, the plugin, settings
└── state/registry.json     derived; `atlas-obsidian refresh` rebuilds it
```

A project's `project.json` holds its name, description, and id, but no path.
The paths live only in `config.json`, and the atlas never searches your disk
for projects — it knows about one because `init` added its folder to that list.
So a project can live anywhere. If you move the folder, the next session you
start inside it repairs the entry.

## Documentation

- Every command and its options: [docs/usage.md](docs/usage.md)
- How the engine works and why: [docs/core-design.md](docs/core-design.md)
- The layout of a project: [docs/v4-design.md](docs/v4-design.md)
- How threads work: [docs/threads-design.md](docs/threads-design.md)
- The skills, how they fit, and what each owns: [docs/skills.md](docs/skills.md)

External references: [Obsidian](https://obsidian.md),
[Claude Code plugins](https://code.claude.com/docs/en/plugins),
[Codex plugin packaging](https://developers.openai.com/plugins/build/plugins),
[MCP Go SDK](https://github.com/modelcontextprotocol/go-sdk).

## Credits

The vault layout, the inbox workflow, and the skills derive from
[claude-obsidian](https://github.com/AgriciDaniel/claude-obsidian) by
AgriciDaniel (MIT), which follows
[Andrej Karpathy's LLM Wiki pattern](https://gist.github.com/karpathy/442a6bf555914893e9891c11519de94f).
Obsidian syntax references draw on
[kepano/obsidian-skills](https://github.com/kepano/obsidian-skills).
atlas-obsidian is released under the [MIT License](LICENSE).
