# Claude Code Custom Commands

Custom slash commands for [Claude Code](https://docs.anthropic.com/en/docs/claude-code) that help with multi-repo development in the AI Gateway ecosystem.

## Available Commands

### `/explore-repos`

Parallel deep-dive into all repos in the AI Gateway ecosystem. Launches one agent per repo to pull everything relevant via the GitHub CLI, then synthesizes a cross-repo summary.

**Repos covered:** MaaS, Kuadrant, Limitador, Authorino, Gateway API Inference Extension, AI Gateway Payload Processing, Tools

**What each agent collects per repo:**
- Repo structure, README, docs, and AI/contributor guidance (CLAUDE.md, CONTRIBUTING.md)
- CRDs, API types, and key interfaces/extension points
- Cross-repo dependency graph (from go.mod / Cargo.toml)
- Last 20 PRs with review comments on the most active ones
- Open issues (15 per repo)
- Latest releases and branch strategy
- Build & test targets (Makefile / CI workflows)
- Key code patterns and frameworks

**Cross-repo synthesis:**
- Summary table (purpose, language, activity, latest release)
- Dependency graph showing which repos import from which
- CRDs and API types that serve as integration contracts
- Cross-cutting PRs/issues that reference or affect other repos

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
