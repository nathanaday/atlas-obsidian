# atlas-obsidian CLI reference, version 5.6.0

This reference lists every command of the `atlas-obsidian` binary at version
5.6.0. The sources are `atlas-obsidian help`, the flag definitions and usage
errors in `internal/cli/cli.go`, and `docs/usage.md`, all read at the same
commit.

## Invocation

```
atlas-obsidian                                open the view: every project on one screen
atlas-obsidian [--home DIR] [-y] <command> [options]
atlas-obsidian help | --help | -h             print the short usage
```

With no command in a terminal, the binary opens `view`. When stdout is not a
terminal, it prints the usage. When the atlas home does not exist, it prints
the usage and tells you to run `setup`.

Flags may come before or after the positional arguments of a command.

### Global options

| Option | Effect |
|---|---|
| `--home DIR` | Use DIR as the atlas home. The default is `~/.atlas-obsidian`, or `$ATLAS_OBSIDIAN_HOME` when it is set. |
| `-y`, `--yes` | Answer yes to every prompt. |

Global options go before the command.

### How a command names a project

| Argument | Meaning |
|---|---|
| `PROJECT` | A name, an id or an id prefix of eight or more characters, or a path. `.` or an omitted optional `PROJECT` means the project that holds the current directory. |
| `NAME` | A project's name or id. Some commands also take a path. |
| `ID` | A thread's id (such as `thr-20260917-3f2a`), its title, or the start of its title. |

A value that holds `/` or `\`, or starts with `~`, is a path. The command finds
the nearest `atlas/<name>/project.json` at or above that path. When two
projects have the same name, the command asks for the path or the id.

Every command that acts on a project reads the config and scans afresh first.

### Exit codes

| Code | Meaning |
|---|---|
| 0 | Success. |
| 1 | The command failed, the user cancelled a prompt, or `lint --strict` found problems. |
| 2 | Wrong arguments or flags, or an unknown command. The command prints its usage line. |

`open-agent`, `open-claude`, and `open-codex` return the exit code of the
harness they start.

## Getting started

### setup

```
atlas-obsidian setup [--agent claude|codex] [--plugin-source SRC] [--no-plugin]
```

Creates the atlas home and installs the plugin in the selected host. Setup
shows its plan and asks before it acts. It creates no project.

| Flag | Effect |
|---|---|
| `--agent claude\|codex` | The plugin host. The default is `claude`. |
| `--plugin-source SRC` | Install the plugin from this marketplace source, such as a local checkout. |
| `--no-plugin` | Do not install a plugin. |

After a Codex install, start a new Codex session, review and trust the plugin
hooks in `/hooks`, then restart the session.

### init

```
atlas-obsidian init [PATH] [--name N] [--description TEXT] [--mode generic|lyt] [--no-git] [--no-threads]
```

Makes the current folder, or PATH, a project. It writes `atlas/<name>/` with
`project.json`, `wiki/`, `threads/`, `inbox/`, `ideas/`, and `.obsidian/`,
commits them as one `setup` operation, and adds the work folder to the config.

| Flag | Effect |
|---|---|
| `--name N` | The project's name. The default is the folder's name. |
| `--description TEXT` | One to three sentences: what the work is and what its wiki must remember. |
| `--mode generic\|lyt` | The filing mode for new wiki pages. The default is `generic`. |
| `--no-git` | Do not run `git init` in a folder that is in no repository. No operation can run until the work is a repository. |
| `--no-threads` | Keep threads off. The thread tools refuse in this project. |

In a terminal with no flags, `init` asks for the name and the description.
It refuses a folder that is a project already, a folder inside another
project, a folder the repository ignores, and an `atlas/<name>/` that holds
files.

## Projects

### list

```
atlas-obsidian list
```

Prints every project with its heat, name, and path, then the folders the atlas
cannot read.

### show

```
atlas-obsidian show NAME
```

Prints all the atlas knows about one project, and the signals that need
attention: open threads by stage, blocked threads, and stale threads (a plan
with no update for 14 days).

### edit

```
atlas-obsidian edit NAME [--name N] [--description TEXT] [--mode generic|lyt]
                         [--threads on|off] [--add-member P]... [--remove-member P]...
```

Changes the identity file and commits it as a `setup` operation.

| Flag | Effect |
|---|---|
| `--name N` | A new name. The command also moves `atlas/<name>/` to the cleaned name. A taken folder refuses the whole edit. |
| `--description TEXT` | A new description. `""` clears it. |
| `--mode generic\|lyt` | The filing mode. It changes only the pages filed after the edit. |
| `--threads on\|off` | Turn threads on or off. Off keeps the folder; the tools refuse until on. |
| `--add-member P` | Mirror the wiki of project P, by name, id, or path. Repeatable. |
| `--remove-member P` | Drop member P. The next `sync` removes its mirror. Repeatable. |

`--add-member` refuses the project itself, a project the atlas does not list,
a cycle, and more than 128 members.

### sync

```
atlas-obsidian sync [PROJECT]
```

Mirrors the wikis of the transitive closure of the members under
`wiki/projects/<name>/` as one `sync` operation. It also copies each member's
threads under `threads/projects/<name>/` as plain files, when both projects
track threads. A sync with nothing new commits nothing.

### describe

```
atlas-obsidian describe [PROJECT] [--no-claude]
```

Stages a snapshot of the work into `inbox/`. Then it offers to start Claude
Code with the `wiki-describe` skill, which writes the page that describes the
work.

| Flag | Effect |
|---|---|
| `--no-claude` | Stage the snapshot and stop. |

The short usage shows `describe PROJECT`; the command also accepts no
argument inside a project.

### forget

```
atlas-obsidian forget PROJECT
```

Removes the project from the config. The `atlas/<name>/` folder stays. A
session started in the folder adds the project again.

### open-vault

```
atlas-obsidian open-vault [PROJECT | PATH]
```

Opens the project's `atlas/<name>/` folder in Obsidian. When Obsidian does not
know the folder, the command offers to register it; Obsidian quits and starts
again to read the new entry.

### open-ide

```
atlas-obsidian open-ide NAME
```

Opens the work folder in a new window of the preferred IDE. VS Code (`code` on
PATH) is the only supported IDE.

### open-terminal

```
atlas-obsidian open-terminal NAME
```

Opens a new terminal window at the work folder. On macOS it uses the terminal
in `TERM_PROGRAM` (Terminal, iTerm, WezTerm, Ghostty, kitty, Alacritty, Warp),
or Terminal. On Linux it uses `$TERMINAL`, or the first known emulator on PATH.

### open-agent, open-claude, open-codex

```
atlas-obsidian open-agent  NAME [--thread ID [--ask] | --plant | --git]
atlas-obsidian open-claude NAME [--thread ID [--ask] | --plant | --git]
atlas-obsidian open-codex  NAME [--thread ID [--ask] | --plant | --git]
```

Starts a harness session in the work folder. `open-agent` uses the
`preferred-harness` setting. `open-claude` always starts Claude Code, and
`open-codex` always starts Codex.

| Flag | Effect |
|---|---|
| `--thread ID` | Continue a thread. The first message is the skill for the thread's next stage. |
| `--ask` | With `--thread` only. The session reads the thread and asks what to do with it. |
| `--plant` | The session asks you to describe a new thread, then opens it. |
| `--git` | The session handles the git state of the work: it offers `git init`, or it fetches, summarizes, and offers to stage, commit, push, or pull. |

Use one of `--thread`, `--plant`, and `--git` at a time.

## Threads

A thread moves through four stages: stub, spec, plan, receipt. The stage is
the furthest document that exists. A command never sets it.

### threads

```
atlas-obsidian threads [PROJECT] [--all] [--stage S] [--json]
```

Lists the open threads by stage. With no PROJECT outside a project, it lists
every project. On a hub, it lists the member boards after the hub's own.

| Flag | Effect |
|---|---|
| `--all` | Include the closed threads. |
| `--stage S` | Only threads at stage S: `stub`, `spec`, `plan`, or `receipt`. |
| `--json` | Print the board as JSON. |

### thread

```
atlas-obsidian thread PROJECT new TEXT... [--title T] [--priority P] [--phase NAME]
atlas-obsidian thread PROJECT show ID [--json]
atlas-obsidian thread PROJECT file ID STAGE [--text T | --file PATH] [--outcome completed|killed]
atlas-obsidian thread PROJECT close ID TEXT... [--killed]
atlas-obsidian thread PROJECT set ID [--title T] [--priority P] [--phase NAME] [--blocked TEXT]
atlas-obsidian thread PROJECT reopen ID
```

| Action | Effect |
|---|---|
| `new` | Creates a card and a stub from TEXT. |
| `show` | Prints the thread's state and the path of each document. |
| `file` | Files the `spec`, `plan`, or `receipt` document, which moves the thread to that stage. A document that exists is never filed again; edit the page. |
| `close` | Files the receipt with TEXT. The outcome is `completed`, or `killed` with `--killed`. |
| `set` | Changes the card's title, priority, phase, or blocker. A new title renames the card and every document. |
| `reopen` | Deletes the receipt. |

| Flag | Actions | Effect |
|---|---|---|
| `--title T` | `new`, `set` | The thread's title. On `new`, the default comes from the text. |
| `--priority P` | `new`, `set` | `high`, `normal`, `low`, or `someday`. |
| `--phase NAME` | `new`, `set` | The phase the thread belongs to. `""` clears it. The phase page must exist. |
| `--blocked TEXT` | `set` | What the thread waits on. `""` unblocks it. |
| `--text T` | `file` | The document's text. |
| `--file PATH` | `file` | Read the text from PATH. `-` is stdin. With neither flag, the text comes from stdin. |
| `--outcome completed\|killed` | `file ID receipt` | The receipt's outcome. |
| `--killed` | `close` | The thread was killed, not completed. |
| `--json` | `show` | Print the thread as JSON. |

On a hub, a thread named by id or title may belong to a member. `file`, `set`,
`close`, and `reopen` make the change in the member that owns it and update
the mirror.

### phase

```
atlas-obsidian phase PROJECT create TITLE [--goal TEXT] [--order N]
atlas-obsidian phase PROJECT rename TITLE --to NEW
atlas-obsidian phase PROJECT reorder TITLE --order N
atlas-obsidian phase PROJECT remove TITLE
```

A phase is a page under `threads/phases/` with a goal and an order. Threads
name their phase; a phase does not list its threads.

| Flag | Effect |
|---|---|
| `--goal TEXT` | What the phase delivers. |
| `--order N` | Where the phase sits in the timeline. |
| `--to NEW` | The new title, for `rename`. The command rewrites every card that names the phase. |

`remove` refuses while a thread names the phase.

## The wiki

Each command in this section that changes `wiki/` makes one operation and one
git commit, scoped to the project's engine paths.

### ingest

```
atlas-obsidian ingest PROJECT [PATH...] [--dry-run] [--no-claude]
```

Copies new files from PATH into `inbox/`. It skips a file whose bytes the
project already holds. With no PATH, it stages the new files in every folder
the project staged from before. Then it offers to start Claude Code with the
`wiki-ingest` skill. Nothing enters the wiki until that session runs the
skill.

| Flag | Effect |
|---|---|
| `--dry-run` | Show what the command would stage, and stop. |
| `--no-claude` | Stage the files and stop. |

PROJECT is required; use `.` for the current project.

### lint

```
atlas-obsidian lint [PROJECT] [--json] [--strict]
```

Runs the read-only health check over `wiki/`. It also lists stubs, wanted
pages, and pages that cite no source. These are signals, not findings.

| Flag | Effect |
|---|---|
| `--json` | Print the report as JSON. |
| `--strict` | Exit 1 when there are findings. Signals do not count. |

### overlap

```
atlas-obsidian overlap [PROJECT] [--member NAME] [--within] [-n N] [--json]
```

Reports what the hub's pages and the mirrors of its members hold in common:
page pairs scored by name, content, and links, as `duplicate` or `related`,
and the link names and tags two origins share. The `atlas-merge` skill reads
this report.

| Flag | Effect |
|---|---|
| `--member NAME` | Only the pairs, names, and tags that involve this origin. |
| `--within` | Also pair the pages of one origin with each other (near-duplicates in one wiki). |
| `-n N` | At most N pairs, N names, and N tags. The default is 30. |
| `--json` | Print the report as JSON. |

### stub

```
atlas-obsidian stub PROJECT [TITLE...] [--type T]
```

Creates seed pages: frontmatter and the usual headings of the type. With no
TITLE, it seeds every wanted page and every empty page that a link points to.

| Flag | Effect |
|---|---|
| `--type T` | The type of a page whose title names none: `concept` or `entity`, and in `lyt` mode also `note` or `moc`. The default is `concept`, or `note` in `lyt` mode. |

PROJECT is required; use `.` for the current project.

### history

```
atlas-obsidian history [PROJECT] [-n N]
```

Lists the operations, newest first.

| Flag | Effect |
|---|---|
| `-n N` | Show N operations. The default is 20. |

### undo

```
atlas-obsidian undo PROJECT OPERATION
```

Restores every page the operation wrote, as a new commit. It refuses when one
of those pages changed after the operation. It does not use `git revert` and
does not touch the code or the thread documents.

### recover

```
atlas-obsidian recover [PROJECT]
```

Restores the wiki after an interrupted operation. The session start says when
a project needs it.

### apply

```
atlas-obsidian apply PROJECT PLAN.json
```

Applies a plan file without a session, for scripts. The file holds the
arguments of the `plan` tool: `kind`, `summary`, `writes`, and `sources`. A
write may give `content_file`, a path relative to the plan file, in place of
`content`. The command prints the preview and the warnings, asks, and then
applies.

## Across the atlas

### view

```
atlas-obsidian view
atlas-obsidian
```

The interactive map of every project. It needs a terminal. The view is for
seeing and launching; the other commands create and change things.

| Key | Effect |
|---|---|
| `→` `↓` Tab | The next top project on the overview; the next project inside a cluster. |
| `←` `↑` Shift+Tab | The previous one. |
| Shift+arrows | Move the selected project; the map answers. |
| Enter | Open the selected hub's cluster, or the lens's card. Again to close the card. |
| `l` | The next lens: details, threads, version control. |
| `↑` `↓` on the threads card | Move through the open threads. |
| `/` | Find a project by name. Enter keeps the match; Esc goes back. |
| `o` | Open `atlas/<name>/` in Obsidian (`open-vault`). |
| `c` | Start the preferred harness in the work (`open-agent`). On the threads card, on the thread under the cursor (`--thread ID --ask`). On the version control card, on the git state (`--git`). |
| `p` on the threads card | Plant a thread in a session (`open-agent --plant`). |
| `i` | Open the work folder in the preferred IDE (`open-ide`). |
| `t` | Open a terminal at the work folder (`open-terminal`). |
| `n` | Open a new thread (`thread PROJECT new`). |
| `R` | Refresh in the background (`refresh`). |
| `,` | Settings: the harness and the IDE (`config`). |
| `?` | Show every key. Again to hide them. |
| Esc | Close the card, the settings, or the keys; else leave the cluster; on the overview, quit. |
| `q` | Quit. |

When the view changed something, the command refreshes the registry after it
closes.

### refresh

```
atlas-obsidian refresh
```

Scans every listed project and rewrites
`~/.atlas-obsidian/state/registry.json`. Everything in that file is derived.
Refresh reads git without the network, so ahead and behind counts are as of
the last fetch.

### config

```
atlas-obsidian config
atlas-obsidian config KEY VALUE
```

With no arguments, prints the settings: `preferred-harness`, `preferred-ide`,
`new-days`, the count of projects, the claude command, the plugin source, and
the config file's path.

| Key | Values | Effect |
|---|---|---|
| `new-days` | a number of days | How long a project shows as new after its creation. The default is 7; 0 turns it off. The command then refreshes. |
| `preferred-harness` | `claude`, `codex` | The harness for `open-agent` and the view's `c` key. The default is `claude`. |
| `preferred-ide` | `vscode` | The IDE for `open-ide` and the view's `i` key. `vscode` is the only value. |

Other settings are edited in `~/.atlas-obsidian/config.json`: `projects`,
`claude_code.command`, `claude_code.prompt`, `claude_code.args`,
`claude_code.session_context`, `plugin.id`, and `plugin.source`.

### info

```
atlas-obsidian info
```

Prints the version, the binary's path, the home, the config, the registry and
state paths, the installed plugin and its source, the Claude Code config
folder, and every project the scan finds.

### doctor

```
atlas-obsidian doctor [--agent claude|codex]
```

Checks the home, git, the selected host's plugin and its version against the
binary's, and every listed project. It names a project whose folder is gone or
whose wiki needs `recover`.

| Flag | Effect |
|---|---|
| `--agent claude\|codex` | The plugin host to check. The default is `claude`. |

## Plugin

These commands serve the plugin. The plugin calls them through
`scripts/atlas`; a person rarely does.

### mcp

```
atlas-obsidian mcp
```

Serves the atlas MCP tools over stdio. The host starts one process per
session.

### hook

```
atlas-obsidian hook session-start|guard|touched|stop
```

| Event | Effect |
|---|---|
| `session-start` | Prints the session's context: the project, its wiki, the page that describes the work, the open threads, the inbox, and `hot.md`. |
| `guard` | The PreToolUse guard. Refuses Write and Edit in `wiki/`, the thread cards and board, `project.json`, the mirrors, and a new file in a stage folder. |
| `touched` | The PostToolUse hook. An edit to a stage document marks its thread as updated today. |
| `stop` | Warns at the end of a session. |

### version

```
atlas-obsidian version
```

Prints the version: `5.6.0`.

## Environment variables

| Variable | Effect |
|---|---|
| `ATLAS_OBSIDIAN_HOME` | The atlas home, when `--home` is not given. |
| `ATLAS_OBSIDIAN_BIN` | The binary that `scripts/atlas` runs first. Otherwise it looks on PATH, in `~/go/bin`, and in the Homebrew prefixes. |
| `ATLAS_OBSIDIAN_PROJECT` | The project a session uses, before the search above the working directory. The `open-*` commands set it. |
| `ATLAS_OBSIDIAN_VAULT` | Set with `ATLAS_OBSIDIAN_PROJECT` by the `open-*` commands. |
| `ATLAS_OBSIDIAN_SESSION_CONTEXT` | Set by the `open-*` commands; controls whether the session-start hook includes `hot.md`. |

## Commands, view keys, and tools

| Command | View key | MCP tool |
|---|---|---|
| `show NAME` | Enter | `atlas`, `status` |
| `open-vault` | `o` | — |
| `open-ide` | `i` | — |
| `open-terminal` | `t` | — |
| `open-agent` | `c`, `p` | — |
| `thread PROJECT new` | `n` | `thread` |
| `refresh` | `R` | `atlas` with `refresh` |
| `config` | `,` | `settings` (for `new-days`) |
| `init`, `edit`, `forget`, `sync` | — | `project` |
| `describe` | — | `stage` with `snapshot` |
| `ingest` | — | `stage`, then `capture`, `plan`, `apply` |
| `threads`, `thread`, `phase` | — | `threads`, `thread`, `phase` |
| `stub` | — | `stub` |
| `lint`, `history`, `undo` | — | `lint`, `history`, `undo` |
| `overlap` | — | `overlap` |
| `recover`, `apply`, `setup`, `doctor`, `info`, `version` | — | — |
