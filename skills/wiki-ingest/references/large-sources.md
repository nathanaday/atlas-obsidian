# Large sources: map, reduce, write

A source past about 40 PDF pages or 2500 lines does not fit one careful read.
The pipeline splits it into ranges, sends a read-only worker to each range in
parallel, reduces what they return into one set of claims per subject, and
writes the pages from that set as one operation. The orchestrator (you)
never holds the whole source; it holds the workers' packets, a few hundred
words each.

## 1. Plan the ranges

From the measure `capture` returned:

- **PDF**: ranges of 20 pages, the most Read takes in one call: 1-20,
  21-40, and so on. When `pages` is 0 (the file did not say), read pages 1-5
  first: a table of contents gives the ranges, and Read's error on a page
  past the end gives the count.
- **Markdown**: follow the `outline`. Join consecutive sections into ranges
  of at most 800 lines; split a longer section at its subheadings, or at 800
  lines when it has none. A range starts at a heading, so each worker sees a
  whole section.
- **Text**: ranges of 800 lines, cut at a blank line near the boundary.

Tell the user the plan in one line: the source, its size, the ranges, and
the workers. Ask first only when the source is past about 600 pages; then
offer the chapters or ranges that matter most.

## 2. Map

Send one `wiki-ingest` worker per range, in parallel, in waves of at most
eight. Give each:

- the project's folder, the captured path, the source id, and the range
  (`pages 41-60`, or `lines 1201-1980`, with the section titles);
- `mode: extract`;
- the titles and aliases of the wiki's pages the source may touch, from
  `wiki/index.md` and a Grep, so the worker names an existing page by its
  existing name;
- the project's description, so the worker knows what belongs.

A worker reads only its range and returns an extract packet: a summary of
the range, its claims with locators, the entities and concepts it names, and
its open questions. It drafts no page. In a host without workers, read the
ranges yourself, one at a time, and write the same packet for each before
moving on.

Check each packet as it arrives: its range matches the one sent, and it is
`complete`. Send a `partial` range again, narrower.

## 3. Reduce

1. Put the packets in range order.
2. Group the claims by subject: every entity and concept the packets name,
   with names joined when they are one thing (the same name, an alias, an
   abbreviation the source defines). Map each subject to a page that exists
   when `route` finds one.
3. In each group, drop repeated claims and keep every locator. Keep claims
   that disagree side by side, with both locators.
4. Apply the gates of the skill: an entity page for each nameable subject; a
   concept page only for durable synthesis. Past 25 new pages, keep the ones
   the most claims support, and list the rest for a later run.
5. The source page is the spine: the document's own structure, one short
   section per range or chapter from the range summaries, linking the pages
   that grew from it.

Past about 40 ranges, reduce in two levels: a worker merges the packets of
each chapter into one, then you merge the chapters.

## 4. Write

Draft each page from its group only. Return to the source for one claim at
its locator when the packets conflict or a claim carries the page. Then the
skill's steps 4 and 5: one plan of kind `ingest`, the preview, the apply.

In the preview, say the coverage: every range read, any range partial, the
pages created and replaced, and the subjects left for a later run.
