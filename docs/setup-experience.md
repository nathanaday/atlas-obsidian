
This repo should serve as a single setup point for the full workflow.
- assumes you already have Claude Code and Obsidian

The goal is to easily set up the capabilities using just a simple CLI interface. The setup and init process of claude-obsdian is a little painful (frankly) with long json outputs and copy/pasting CLI args between inputs and outputs.

The security mechanisms in claude-obsidian (in terms of hash verification and plan vs. execute) is really useful, but only when data exists. This project will not support migrating from existing repositories (yet) so all init just creates a fresh setup, and this can be done deterministically through setup tools. Since no LLM is involved, I don't think we need that level of security exchange for the init process.

The project's init helper sets up:
- `~/.atlas-obsidian` for data files
- clones the `claude-obsidian` project using a pinned version; the most recent right now is `https://github.com/AgriciDaniel/claude-obsidian/releases/tag/v2.2.0`
- installs this repo's skill files (host on some skill sever? prompt user for skill directory and move manually?) TBD
- creates a welcome repository and single-node atlas project to verify setup
- welcomes you to the project and shows you where to open your new view

