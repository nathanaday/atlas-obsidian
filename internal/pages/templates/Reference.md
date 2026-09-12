
> [!info] Generated page
> `claude-atlas` writes this page on every refresh so it matches the installed version (%VERSION%). Each command sits in its own block so one click copies it.

## Managing Vaults Quickstart

>[!tip] You can also use the Obsidian file explorer in this vault
>Use the `tree/` directory to organize your projects. 
>- Every markdown file in that directory becomes a project by that name
>- Nest named folders to categorize them
>- Call the `refresh` command when you're done to commit it

**Create a vault**

> Walks you through it: name, category (pick one from your tree or type a new one), purpose, confirm.

```bash
claude-atlas new-vault
```

> The same without prompts. It lands in the vaults directory; missing category folders are created.

```bash
claude-atlas new-vault my-project --category university/cs566
```

**Refresh [[Overview]]**

> Run it after editing anything under `tree/`.

```bash
claude-atlas refresh
```

**Open a vault in Obsidian**

> Opens the atlas with no argument, or a project's vault by name. If Obsidian does not know the folder yet, it offers to register it; Obsidian quits and relaunches so it sees the new entry.

```bash
claude-atlas open-vault
```

```bash
claude-atlas open-vault my-project
```

**View the atlas**

> Navigate your projects as a tree, three category layers at a time; deeper categories open on Enter. Enter shows everything the atlas knows about a project; `o` opens its vault in Obsidian; `c` starts Claude Code in it.

```bash
claude-atlas view
```

**Manage vaults**

> Browse every project by category. Open one to rename it, edit its purpose, move it to another category, change its priority or state, repoint or move its vault, or remove it from the atlas (the vault stays on disk).

```bash
claude-atlas manage-vaults
```

**List projects**

```bash
claude-atlas list
```

**Info**

> Show relevant file paths, version, etc.

```bash
claude-atlas info
```

## Linking repos and material

> Link a folder to a project: a git repository (detected by its `.git`) or a folder of static material such as slides, PDFs, and images. Nothing is copied; the atlas remembers the path and reports on it at every refresh.

```bash
claude-atlas link my-project ~/code/my-project
```

```bash
claude-atlas link my-project ~/Documents/lectures --kind materials
```

> Show a project's links and what the last refresh found in them.

```bash
claude-atlas links my-project
```

> Remove a link. The folder is untouched.

```bash
claude-atlas unlink my-project ~/code/my-project
```

## Working with Claude Code

> Start Claude Code inside a project's vault. claude-obsidian's session hook hands Claude the vault's recent context (`wiki/hot.md`) at the start, and its skills are on the slash menu.

```bash
claude-atlas open-claude my-project
```

> The skills, once inside:

```text
/claude-obsidian:wiki-ingest
```

```text
/claude-obsidian:wiki-query
```

```text
/claude-obsidian:wiki-lint
```

> To invoke a skill on every launch, set `claude_code.prompt` in config.json, for example to `/claude-obsidian:wiki`. Set `claude_code.session_context` to `false` to keep the hook silent.


## Setup and health

Install claude-obsidian into Claude Code, create the atlas, and create a first vault. Safe to run again; finished steps are skipped.

```bash
claude-atlas setup
```

Check the installation and reach every vault.

```bash
claude-atlas doctor
```

```bash
claude-atlas version
```

## Advanced

Set purpose and priority when creating a vault.

```bash
claude-atlas new-vault sensor-triage --purpose "Sort field sensor faults." --priority high
```

Create a vault at an explicit path instead of the vaults directory.

```bash
claude-atlas new-vault ~/Desktop/scratch-vault
```

Register an existing vault with a display name, a category, and a priority.

```bash
claude-atlas new-vault --from ~/Documents/OldVault --name "Old Vault" --category archive --priority someday
```

Move a project by moving its page.

```bash
mv %ATLAS%/tree/capstone.md %ATLAS%/tree/university/cs566/
```

Register a claude-obsidian vault that already exists.

```bash
claude-atlas new-vault --from ~/Documents/OldVault
```


Run setup with every location chosen up front.

```bash
claude-atlas setup --atlas-vault %ATLAS% --vaults-dir %VAULTS% --first-vault research
```

Run setup without touching Claude Code plugins.

```bash
claude-atlas setup --no-plugin
```

Answer yes to every prompt, for scripts.

```bash
claude-atlas -y setup
```

Use a different home directory for one command.

```bash
claude-atlas --home ~/other-atlas info
```

| Flag or setting | Effect |
|:--|:--|
| `--home DIR` | Use a different home instead of `~/.claude-atlas`. |
| `-y`, `--yes` | Answer yes to every prompt. |
| `CLAUDE_ATLAS_HOME` | Same as `--home`, as an environment variable. |
| `claude_obsidian.path` in config.json | Use a claude-obsidian checkout instead of the installed plugin. |
