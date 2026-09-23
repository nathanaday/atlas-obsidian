# atlas-obsidian

A wiki and the state of the work, in every project.

[![License: MIT](https://img.shields.io/badge/license-MIT-2563eb.svg)](LICENSE)
[![Go](https://img.shields.io/badge/Go-1.24+-00ADD8.svg?logo=go&logoColor=white)](go.mod)
[![Claude Code plugin](https://img.shields.io/badge/Claude%20Code-plugin-7c3aed.svg)](.claude-plugin/plugin.json)
[![Codex plugin](https://img.shields.io/badge/Codex-plugin-10a37f.svg)](.codex-plugin/plugin.json)
[![Version](https://img.shields.io/badge/version-5.6.0-d97745.svg)](.claude-plugin/plugin.json)

## About

`atlas-obsidian` keeps a markdown wiki inside the project. A hook runs when you
start Claude Code or Codex anywhere in the repository. It hands the session the
project's description, the wiki's recent context, the open threads, and the files
waiting in the inbox.

![Every project on one screen](docs/examples/TUI-project-nav.png)

*`atlas-obsidian` TUI. Evoke the TUI anywhere and browse your atlas-powered projects*

# Features

### Ingest huge documents into the wiki

![Ingesting a 351-page slide deck](docs/examples/ingesting-huge-slide-deck.png)

The skills use parallel agents to shard and ingest content accurarately. Once hundreds of pages have been ingested, you can make quick, cheap queries to the knowledge base using the Go+MCP core.

### View your knowledge base in Obsidian

Ingestion writes ordinary wikilinks, so Obsidian's graph view and its backlinks
work on the result. A source page links to the concepts it covers, and each
concept page lists the sources that cite it.

![The graph around a paper](docs/examples/huge-kb-focus-1.png) 

![The graph around a concept](docs/examples/huge-kb-focus-2.png)

### Connect projects into a high level knowledge base

![A hub wiki over four member projects](docs/examples/huge-kb-high-level.png)

A project may list other projects as members. Its wiki then mirrors each member's wiki using the built in sync system. Updating the children knowledge base propagates the changes upward to the parent hub.

Design and reasons:
[docs/members-design.md](docs/members-design.md).

## Quickstart

### Prerequisites

- [Claude Code](https://claude.com/claude-code) or
  [Codex CLI](https://learn.chatgpt.com/docs/cli), with `claude` or `codex` on
  your PATH. Codex needs a release with `codex plugin` and `/hooks` support.
- git, and Go 1.24 or newer to build the binary.
- macOS or Linux.
- [Obsidian](https://obsidian.md) is optional. The files are plain Markdown and
  read fine anywhere, but the folder is laid out for it and ships a CSS snippet
  that colors each kind of folder.

### Install

```bash
git clone https://github.com/nathanaday/atlas-obsidian.git
cd atlas-obsidian
make install

atlas-obsidian setup --agent claude --plugin-source "$PWD"   # or --agent codex
```

`make install` puts `atlas-obsidian` in `$(go env GOPATH)/bin`, usually
`~/go/bin`; add that to your PATH if it is not there. 

Use `make install` rather than `go install` so the binary is stamped with its version, which
`atlas-obsidian doctor` compares against the plugin's.


## Patterns and conventions

Full reasoning in [docs/core-design.md](docs/core-design.md).

- **One operation, one commit.** A hook refuses the agent's file-edit tools
  anywhere under `wiki/`. The agent prepares a plan instead; you see what it
  creates, replaces, and deletes, and approving it makes one git commit.
  `atlas-obsidian undo` takes that commit back by restoring the files it touched,
  and refuses if you have changed one of them since.
- **The folder is yours.** Anything you typed in Obsidian is committed first, as
  its own operation, so an undo can never reach your edits. Every git command an
  operation runs is limited to `wiki/`, `.raw/`, `inbox/`, and `project.json`, so
  an apply never stages your source code.
- **Code owns what code can derive.** The wiki's log, the ledger of sources, the
  thread cards, the board that lists them, and the opening callout of each thread
  document are all written by the binary. The agent writes the prose.
- **Ids travel; paths stay.** A project's `project.json` holds no path. The paths
  live in `~/.atlas-obsidian/config.json`, which `init` appends to. The atlas
  never scans your disk, so a project can live anywhere. Move the folder, and the
  next session started inside it repairs the entry.

## Documentation

- Every command and its options — [docs/usage.md](docs/usage.md)
- How the engine works and why — [docs/core-design.md](docs/core-design.md)
- The layout of a project — [docs/v4-design.md](docs/v4-design.md)
- How threads work — [docs/threads-design.md](docs/threads-design.md)
- The skills, how they fit, and what each owns — [docs/skills.md](docs/skills.md)
- One wiki over several projects — [docs/members-design.md](docs/members-design.md)

External: [Obsidian](https://obsidian.md) ·
[Claude Code plugins](https://code.claude.com/docs/en/plugins) ·
[Codex plugin packaging](https://developers.openai.com/plugins/build/plugins) ·
[MCP Go SDK](https://github.com/modelcontextprotocol/go-sdk)

## Credits

The vault layout, the inbox workflow, and the skills derive from
[claude-obsidian](https://github.com/AgriciDaniel/claude-obsidian) by
AgriciDaniel (MIT), which follows
[Andrej Karpathy's LLM Wiki pattern](https://gist.github.com/karpathy/442a6bf555914893e9891c11519de94f).
Obsidian syntax references draw on
[kepano/obsidian-skills](https://github.com/kepano/obsidian-skills).
atlas-obsidian is released under the [MIT License](LICENSE).
