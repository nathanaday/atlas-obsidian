# atlas-obsidian

A wiki and the state of the work, in every project.

[![License: MIT](https://img.shields.io/badge/license-MIT-2563eb.svg)](LICENSE)
[![Go](https://img.shields.io/badge/Go-1.24+-00ADD8.svg?logo=go&logoColor=white)](go.mod)
[![Claude Code plugin](https://img.shields.io/badge/Claude%20Code-plugin-7c3aed.svg)](.claude-plugin/plugin.json)
[![Version](https://img.shields.io/badge/version-5.1.0-d97745.svg)](.claude-plugin/plugin.json)

## What it is

A Claude Code session begins with no memory of the last one. atlas-obsidian
keeps what a project knows in the project itself, as Markdown files that git
tracks and Obsidian opens.

It comes in two pieces that install separately: a Go binary, which makes a
folder of work into a project and serves the tools Claude uses inside it, and
a Claude Code plugin, which carries the skills and the hooks.

A project is one folder, `atlas/<name>/`, sitting beside your code. It holds
two things.

**The wiki** is Markdown pages that cite where they came from. Put a PDF, an
article, or a note in `inbox/` and Claude reads it, keeps an unmodified copy
under `.raw/captured/`, and writes pages that cite that copy. Ask the wiki a
question later and the answer names the page and the source behind it.

**The threads** are the work in progress. A thread is one bug, feature, or
chore, and it moves through four documents: a stub in your own words, then a
spec, a plan, and a receipt. Nothing records which stage a thread is in. The
furthest document that exists is the stage, so every change of state is a page
you can open and read.

Start Claude anywhere inside the repository and a hook hands the session the
project's description, the wiki's recent context, which threads are open, and
what is waiting in the inbox.

## Getting started

You need [Claude Code](https://claude.com/claude-code) with `claude` on your
PATH, git, and Go 1.24 or newer to build the binary. macOS and Linux are
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

Then install the Claude Code plugin:

```bash
atlas-obsidian setup
```

Setup prints what it intends to do and waits for you to agree. It creates
`~/.atlas-obsidian/` and installs the plugin through `claude plugin`. Running
it again is harmless; anything already done is skipped.

## Making a project

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
```

The thread command writes a card and a stub in the words you gave it. The
Claude session that follows opens with that thread in view.

From there you ask Claude for the skill you want:

| Ask for | What it does |
|---|---|
| `/atlas-obsidian:describe` | writes the wiki page that says what this codebase is |
| `/atlas-obsidian:thread-spec` | turns a stub into a spec: what will be true when it is done |
| `/atlas-obsidian:thread-plan` | reads the code and files the approach |
| `/atlas-obsidian:thread-run` | does the work, keeping the plan document current as it goes |
| `/atlas-obsidian:thread-receipt` | closes the thread and offers the wiki what the work taught |

To teach the wiki something, drop the source in the inbox and start a session:

```bash
cp ~/Downloads/paper.pdf ~/code/webapp/atlas/webapp/inbox/
claude
```

`/atlas-obsidian:wiki-ingest` reads whatever is waiting and writes pages that
cite it. `/atlas-obsidian:wiki-query` answers questions from those pages
without changing anything. A short note dropped in the same inbox becomes a
thread instead, through `/atlas-obsidian:thread-stub`.

Every command and its options: [docs/usage.md](docs/usage.md).

## Seeing every project

```bash
atlas-obsidian
```

With no arguments the binary opens a terminal view of every project it knows,
listed by name, each with its recent activity and what is open. Arrow keys
move, Enter expands an entry in place, `o` opens it in Obsidian, `c` starts
Claude Code in the work folder, `n` opens a new thread, `R` re-reads
everything, and `h` shows the keys.

## How writes reach the wiki

Claude cannot write a wiki page with its ordinary file tools; a hook refuses
Write and Edit anywhere under `wiki/`. It prepares a plan instead. You see
which files the plan creates, replaces, or deletes, and approving it makes one
git commit. `atlas-obsidian undo` takes a commit back by restoring the files it
touched from its parent, and refuses if you have changed one of those files in
the meantime.

Anything you typed in Obsidian is committed first, as its own operation, so an
undo can never reach your own edits. Every git command an operation runs is
limited to `wiki/`, `.raw/`, `inbox/`, and `project.json`, so an apply or an
undo never stages your source code.

Some pages belong to the tool rather than to Claude: the wiki's log, the ledger
of sources, the thread cards, the board that lists them, and the opening
callout of each thread document. The binary derives all of those from the files
around them, and Claude is left to write the prose pages.

## What the folder holds

A new project looks like this:

```
webapp/                          your repository
├── ...                          your work, untouched
└── atlas/
    └── webapp/                  named after the project; a rename moves it
        ├── project.json         name, description, filing mode
        ├── wiki/                index.md, log.md, hot.md, overview.md
        ├── threads/             stubs/ specs/ plans/ receipts/ phases/ archive/
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
- What each skill promises to do: [skills/](skills/), one `SKILL.md` per directory

External references: [Obsidian](https://obsidian.md),
[Claude Code plugins](https://code.claude.com/docs/en/plugins),
[MCP Go SDK](https://github.com/modelcontextprotocol/go-sdk).

## Credits

The vault layout, the inbox workflow, and the skills derive from
[claude-obsidian](https://github.com/AgriciDaniel/claude-obsidian) by
AgriciDaniel (MIT), which follows
[Andrej Karpathy's LLM Wiki pattern](https://gist.github.com/karpathy/442a6bf555914893e9891c11519de94f).
Obsidian syntax references draw on
[kepano/obsidian-skills](https://github.com/kepano/obsidian-skills).
atlas-obsidian is released under the [MIT License](LICENSE).
