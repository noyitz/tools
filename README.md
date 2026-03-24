# Tools

A collection of developer tools for the AI Gateway ecosystem.

## Available Tools

### [bbr-sandbox](bbr-sandbox/)

Interactive web-based sandbox for testing BBR (Body-Based Router) plugins. Drag-and-drop plugin pipeline builder, sample request library, and color-coded mutation diffs.

```bash
cd bbr-sandbox
go run .
```

### [claude-commands](claude-commands/)

Custom slash commands for Claude Code that bootstrap multi-repo context at the start of a session. The `/explore-repos` command launches parallel agents to deep-dive all ecosystem repos (MaaS, Kuadrant, Limitador, Authorino, Gateway API Inference Extension, AI Gateway Payload Processing) and loads architecture, recent PRs, open issues, and integration points into context.

```bash
# Install globally (available in all projects)
cp claude-commands/explore-repos.md ~/.claude/commands/
```

See [claude-commands/README.md](claude-commands/README.md) for full setup and customization instructions.
