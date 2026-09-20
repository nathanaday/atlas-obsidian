# Working on atlas-obsidian

Read README.md and CLAUDE.md before changing this repository. CLAUDE.md holds
the shared architecture, invariants, and build instructions for both agents.

The plugin shares its skills, MCP server, and hooks between Claude Code and
Codex. Keep both manifests compatible and test hook payloads from both hosts.
Codex patches arrive as `tool_name: "apply_patch"` with patch text in
`tool_input.command`; Claude edits carry `tool_input.file_path`.
