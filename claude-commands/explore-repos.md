Present the user with this numbered list of repositories and ask them to select which ones to explore (they can pick multiple, e.g. "1,3,5" or "all"):

1. **MaaS** (models-as-a-service) — https://github.com/opendatahub-io/models-as-a-service
2. **Kuadrant** — https://github.com/Kuadrant/kuadrant-operator
3. **Limitador** — https://github.com/Kuadrant/limitador
4. **Authorino** — https://github.com/Kuadrant/authorino
5. **Gateway API Inference Extension** — https://github.com/kubernetes-sigs/gateway-api-inference-extension
6. **AI Gateway Payload Processing** (BBR plugins) — https://github.com/opendatahub-io/ai-gateway-payload-processing
7. **Tools** (noyitz) — https://github.com/noyitz/tools

After the user selects repos, ask them: **"Run exploration in the background (you can keep working) or hold the session until results are ready?"** Options: `background` or `hold` (default: hold).

For EACH selected repo do the following deep-dive (use the Agent tool to parallelize across repos). Set `run_in_background: true` on each Agent call if the user chose "background", or `run_in_background: false` (default) if they chose "hold":

### For each repo:
1. **Clone/fetch info via GitHub API**: Use `gh` CLI to get repo description, languages, and directory structure
2. **Architecture & Structure**: Use `gh api` to explore the repo tree, identify key directories, entry points, config files, and understand the project layout
3. **README & Docs**: Fetch and read the README and any docs/ directory to understand purpose, setup, and architecture
4. **Recent PRs**: Use `gh pr list --repo <repo> --state all --limit 10` to see recent PRs, then read the most interesting/active ones to understand current development focus
5. **Open Issues**: Use `gh issue list --repo <repo> --state open --limit 10` to see what's being worked on and what problems exist
6. **Key Code Patterns**: Identify the main language, framework, and coding patterns used

### After exploring all selected repos, provide:
- A summary table comparing the repos (purpose, language, activity level)
- Key architectural patterns and how the repos relate to each other
- Notable recent changes or active development areas
- Any integration points between the repos

Keep the analysis concise but thorough. Focus on information that would help someone contribute to or integrate with these projects.
