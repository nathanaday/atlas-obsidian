---
name: wiki-base
description: "Draft, explain, and change Obsidian Bases views over the wiki: .base files with filters, formulas, properties, summaries, and table, card, or list views, written as one reviewed operation. Use for a Base, Bases, a database view of the wiki, a dynamic table, a reading list, a tracker, filters, formulas, .base file edits."
---

# Bases views

A Base is a view over the wiki's pages, selected and shaped by their
properties. This skill drafts one, explains one, or changes one. For a field
or function this page does not cover, or one that changes between Obsidian
versions, see the [official Bases syntax](https://help.obsidian.md/bases/syntax).

Tools: `status`, `plan`, `apply`. Reads
[operations.md](../wiki/references/operations.md) and
[frontmatter.md](../wiki/references/frontmatter.md).

Answer design and syntax questions read-only. A `.base` change goes through
one plan of kind `base`, with the file under `wiki/`; never write it directly.

## Workflow

1. Inspect representative page properties and any existing `.base` file.
2. Define the smallest filter that selects the intended pages.
3. Add formulas only for values that must be computed.
4. Choose views and display order. Do not assume a view type or option is
   supported by the user's Obsidian version or plugins.
5. Validate YAML, expression quoting, property names, formula references, and
   null handling.
6. Preview the complete file and the expected result set.
7. Build one plan of kind `base` following
   [operations.md](../wiki/references/operations.md); show the preview; apply
   on approval.
8. Ask the user to render the Base in Obsidian when application behavior cannot
   be verified locally.

## Compact schema

`.base` files are YAML. Common top-level keys are `filters`, `formulas`,
`properties`, `summaries`, and `views`.

```yaml
filters:
  and:
    - file.inFolder("wiki")
    - 'status != "archived"'

formulas:
  age_days: '((now() - file.ctime) / 86400000).round(0)'
  status_label: 'if(status == "mature", "Ready", "Review")'

properties:
  status:
    displayName: "Status"
  formula.age_days:
    displayName: "Age (days)"

views:
  - type: table
    name: "Wiki pages"
    order:
      - file.name
      - type
      - status
      - updated
      - formula.age_days
```

Global filters apply to every view; a view may add its own. Recursive filter
objects use one of `and`, `or`, or `not` at each level.

```yaml
filters:
  or:
    - file.hasTag("concept")
    - and:
        - file.hasTag("source")
        - 'status == "active"'
```

Use page properties by name, file metadata as `file.name`, `file.path`,
`file.folder`, `file.ext`, `file.ctime`, `file.mtime`, or `file.tags`, and
computed properties as `formula.<name>`.

## Formula and YAML rules

- Quote expressions that contain operators, colons, or nested quotes.
- Guard nullable properties with `if()`.
- Subtracting two dates gives milliseconds; divide by `86400000` for days.
- Define every `formula.<name>` before referencing it.
- Do not transplant Dataview keys such as `from` or `where` into a Base.
- Do not invent properties absent from the selected pages without saying the
  column will be empty.

```yaml
formulas:
  days_until: 'if(due_date, ((date(due_date) - today()) / 86400000).round(0), "")'
```

Table, cards, and list views are common:

```yaml
views:
  - type: cards
    name: "Reading list"
    order:
      - file.name
      - author
      - status
  - type: list
    name: "Quick list"
    order:
      - file.name
      - status
```

Embed a Base or one named view in a page:

```markdown
![[Dashboard.base]]
![[Dashboard.base#Wiki pages]]
```

After an applied edit, report the operation id, the changed path, the
validation performed, and anything that still needs rendering in Obsidian.

## Hand off

`wiki-edit` when the pages lack the properties the view needs;
`wiki-canvas` for a spatial map instead of a table.
