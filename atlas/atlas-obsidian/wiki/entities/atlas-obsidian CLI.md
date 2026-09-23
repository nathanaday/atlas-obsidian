---
type: entity
title: "atlas-obsidian CLI"
status: developing
created: 2026-09-22
updated: 2026-09-22
aliases:
  - atlas-obsidian command line
  - atlas-obsidian binary
tags:
  - entity
  - cli
---

# atlas-obsidian CLI

## Overview

The `atlas-obsidian` binary is the command line of atlas-obsidian. Every
atlas action is one subcommand; the terminal view and the MCP tools are
subsets of it. At version 5.6.0 it groups its commands in six categories
([[atlas-obsidian CLI reference 5.6.0]]):

| Category | Commands |
|---|---|
| Getting started | `setup`, `init` |
| Projects | `list`, `show`, `edit`, `sync`, `describe`, `forget`, `open-vault`, `open-ide`, `open-terminal`, `open-agent`, `open-claude`, `open-codex` |
| Threads | `threads`, `thread`, `phase` |
| The wiki | `ingest`, `lint`, `overlap`, `stub`, `history`, `undo`, `recover`, `apply` |
| Across the atlas | `view`, `refresh`, `config`, `info`, `doctor` |
| Plugin | `mcp`, `hook`, `version` |

With no command in a terminal, the binary opens `view`. The global options
are `--home DIR` and `-y`/`--yes`. Exit code 2 means wrong arguments; 1 means
failure or cancel ([[atlas-obsidian CLI reference 5.6.0]]).

## Relationships

- The full reference, with every flag: [[atlas-obsidian CLI reference 5.6.0]].

## Sources

- [[atlas-obsidian CLI reference 5.6.0]]
