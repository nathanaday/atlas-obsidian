---
name: wiki
description: "Orient in a claude-atlas vault and route work to the right skill. Use for /wiki, set up wiki, vault status, what is in this vault, which skill should I use, adopt this vault, Obsidian vault, second brain, persistent wiki, knowledge base setup."
---

# Wiki orientation

A claude-atlas vault is a plain Obsidian vault with a fixed layout: `inbox/` for
sources you have not processed, `.raw/captured/` for immutable copies of the ones
you have, and `wiki/` for the pages. Every change to `wiki/` is one reviewed
operation and one git commit. The atlas MCP server (tools named
`mcp__plugin_claude-atlas_atlas__<tool>`, called `status`, `plan`, `apply`, and so
on below) is the only write path.

## Find the vault

Call `status` first. It resolves the vault from `CLAUDE_ATLAS_VAULT`, then from the
nearest `.claude-atlas.json` above the project directory, and reports the mode,
page count, files waiting in the inbox, git state, and warnings.

If `status` fails because no vault is selected, stop and tell the user to run one
of these in a terminal, then start a session inside the vault:

```bash
claude-atlas new-vault                 # create a vault, step by step
claude-atlas adopt /path/to/vault      # an existing Obsidian or claude-obsidian vault
```

Do not create vault files yourself. If `status` warns that an operation was
interrupted, tell the user to run `claude-atlas recover` before anything else.

## Never write wiki pages directly

Write, Edit, and MultiEdit are refused under `wiki/` by a hook. Read pages with
Read, Grep, and Glob as usual; change them only through `plan` and `apply`. The
core writes `wiki/log.md` and the source ledger itself; a plan that names either
is rejected.

## Route the request

| Intent | Skill |
|---|---|
| Process files in the inbox, or supplied text, into pages | `wiki-ingest` |
| Answer from what the vault already holds | `wiki-query` |
| Keep a specific answer, decision, or insight | `save` |
| Check vault health | `wiki-lint` |
| Read or change the filing mode | `wiki-mode` |
| Roll up log entries | `wiki-fold` |
| Work with an Obsidian Canvas | `canvas` |
| Author a Bases `.base` view | `obsidian-bases` |
| Obsidian syntax questions | `obsidian-markdown` |
| Reason carefully before a consequential change | `think` |

Query is read-only. Keeping an answer is a separate `save` operation the user
asks for. Never update the hot cache merely because a session ended.

## The operation contract

Read [operations.md](references/operations.md) before any change. In short:

1. Read every page you will change and keep its `sha256` from the plan preview or
   compute it; a page that changed since you read it makes apply fail closed.
2. Call `plan` with the kind, a one-line summary, and every write as complete
   file content. The core validates paths, frontmatter, JSON, and links, and
   returns a `plan_id`, a preview, and warnings.
3. Show the user the preview: created, updated, and removed paths, and every
   warning. Ask before apply when the change removes or replaces pages.
4. Call `apply` with the `plan_id`. Report the operation id and changed paths.

Every new canonical page joins `wiki/index.md` (generic mode) or a MOC (lyt mode)
in the same plan. Update `wiki/overview.md` only when the stable high-level
picture changed. Keep `wiki/hot.md` under 500 words.

An operation can be undone with the `undo` tool or `claude-atlas undo`; say so when
a user hesitates rather than skipping a review.

## Conditional references

Read only what the request needs:

- [operations.md](references/operations.md) for the plan and apply contract;
- [provenance.md](references/provenance.md) when a source enters or a claim needs
  support;
- [frontmatter.md](references/frontmatter.md) when defining or adopting page
  properties;
- [modes.md](references/modes.md) for domain scaffolds on top of the mode;
- [css-snippets.md](references/css-snippets.md) for requested visual changes;
- [plugins.md](references/plugins.md) when evaluating optional Obsidian plugins.

## Think, verify, grow

Before applying, pause once: observe the vault's current state, verify the
evidence, then choose the smallest reversible operation that satisfies the
request. Afterward, report uncertainty and the next useful improvement without
doing it unasked.
