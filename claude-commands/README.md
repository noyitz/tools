# Claude Code Custom Commands

Custom slash commands for [Claude Code](https://docs.anthropic.com/en/docs/claude-code) that help with multi-repo development in the AI Gateway ecosystem.

## Available Commands

### `/explore-repos`

Parallel deep-dive into all repos in the AI Gateway ecosystem. Launches one agent per repo to pull architecture, recent PRs, open issues, and code patterns via the GitHub CLI, then synthesizes a cross-repo summary with integration points.

**Repos covered:** MaaS, Kuadrant, Limitador, Authorino, Gateway API Inference Extension, AI Gateway Payload Processing, Tools

**What it does:**
- Fetches repo structure, README, and docs
- Reviews the 10 most recent PRs and open issues
- Identifies key code patterns and frameworks
- Produces a comparison table and architecture diagram showing how repos relate
- Runs all repos in parallel (one agent each)

**Options:**
- Select specific repos (e.g. "1,3,5") or "all"
- Run in `background` (keep working) or `hold` (wait for results)

**Best with:** Claude Opus + 1M context window, so the full exploration fits in a single session.

## Installation

Copy the command file into your Claude Code commands directory:

```bash
# Project-level (scoped to a specific repo)
mkdir -p .claude/commands
cp explore-repos.md .claude/commands/

# User-level (available in all projects)
mkdir -p ~/.claude/commands
cp explore-repos.md ~/.claude/commands/
```

Then start Claude Code and run `/explore-repos`.

## Prerequisites

- [Claude Code](https://docs.anthropic.com/en/docs/claude-code) CLI installed
- [GitHub CLI](https://cli.github.com/) (`gh`) installed and authenticated
- Recommended: Claude Opus model with 1M context window

## Customization

The repo list is defined at the top of `explore-repos.md`. Edit it to add or remove repos for your own workflow. The exploration steps and output format can also be adjusted to focus on what matters most to your team.
