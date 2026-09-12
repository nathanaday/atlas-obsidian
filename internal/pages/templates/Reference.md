
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

**View the atlas**

> Navigate your projects as a tree, three category layers at a time; deeper categories open on Enter. Open a project to see everything the atlas knows about it.

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

## Working with Claude Code 

> Start Claude Code inside a vault, then use the wiki skills

```bash
cd %VAULTS%/my-project && claude
```

```text
/claude-obsidian:wiki
```


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
