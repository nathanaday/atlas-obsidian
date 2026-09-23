---
type: stub
thread: thr-20260922-ef9b
title: "Lenses on the project map"
created: 2026-09-22
---

> [!stub] Lenses on the project map
> **Stub** → Spec → [Plan](<../plans/Lenses on the project map.md>) → Receipt
> `thr-20260922-ef9b` · [Thread](<../Lenses on the project map.md>) · filed 2026-09-22

The next feature allows us to use the same node graph to manage many dimensions of atlas obsidian's capabilities.

We should introduce the concept of lenses. The hotkey "l" should cycle through the available lenses. The different lens modes are described below. They show the same set of nodes in the same position, but through the display details and color scheme, they allow the user to visualize new aspects of their project collection.

Lenses on project navigation:

Details view (default)
- Enter toggles show/hide of detail card
- REMOVE threads, open, signals items from details card (they get their own lens)
- REMOVE git (it gets its own lens)
- KEEP path, description, wiki, touched
- KEEP the color system: selected node is blue, connections to parent nodes are highlighted, parent nodes are purple, all other nodes are gray

Threads view
- Project node size is scaled based on number of open threads
- Project nodes with open threads are highlighted yellow
- Enter toggles show/hide of thread detail card
    - The threads detail card shows the number of open threads, then a short break down of how many threads are in each state, where the states (stub, plan, spec, etc.) are an entry, and a bar chart shows the distribution like a histogram
    - The threads detail card shows a scroll box text area where you can navigate through all open threads with the up and down arrow keys while the card is open, a short title and preview description is provided
    - Pressing the agent key (c) opens the agent (claude or codex based on config) with the thread id context. the agent should read the thread and ask the user "what would you like to do with this thread" then suggest the base options: work on it by moving it up a phase (e.g. from a stub to a spec), or kill the thread with or without provided reason. the user then uses the agent session from there. note this leaves the tui view in the same way the "c"
    - Pressing p while the thread card is open lets you plant a new thread stub. this opens an agent session (claude or codex based on config) and prompts the user to describe the thread they want to plant. the description is used to create the thread stub.

Version Control view
- Project nodes with dirty branches are RED
- Project nodes with changes ahead/behind the remote are YELLOW
- Project nodes that are in sync, no pending changes, are default gray
- Project nodes with no git tracking are also default gray
- Enter toggles the show/hide of the git status card
    - This get status card is a pretty standard git status view: ahead/behind remote status, current head, remote origin url
    - Pressing c while the git status card is open stats an agent session with the context that the user wants to handle the git status of this project:
        - If no git is enabled, the agent offers to initialize it
        - If git is enabled, the agent checks the git status and offers to stage changes, make a commit, push, pull, or anything else the user wants
