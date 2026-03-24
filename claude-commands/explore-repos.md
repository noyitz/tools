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
4. **AI/Contributor Guidance**: Fetch CLAUDE.md, AGENTS.md, or CONTRIBUTING.md if they exist — these contain repo-specific conventions, test commands, and architectural constraints
5. **CRDs & API Types**: Find `*_types.go`, `types.go`, or `.proto` files and list all CRD GroupVersionKinds and key API types. For Rust repos, look for protobuf definitions
6. **Key Interfaces & Extension Points**: Identify the main interfaces/traits that plugins or extensions must implement (e.g., BBRPlugin, Translator, AuthConfigEvaluator, CounterStorage). List them with their method signatures
7. **Cross-Repo Dependencies**: Read `go.mod` (or `Cargo.toml`) and identify which of the OTHER repos in this list are imported as dependencies
8. **Recent PRs**: Use `gh pr list --repo <repo> --state all --limit 20` to see recent PRs, then read the 3-4 most interesting/active ones with `gh pr view <number> --repo <repo> --comments` to understand current development focus and review discussions
9. **Open Issues**: Use `gh issue list --repo <repo> --state open --limit 15` to see what's being worked on and what problems exist
10. **Latest Releases**: Run `gh release list --repo <repo> --limit 3` to see release cadence and current version
11. **Active Branches**: Run `gh api repos/<owner>/<repo>/branches --jq '.[].name'` to understand branch strategy (main, stable, release branches)
12. **Build & Test**: Read the Makefile (first 50 lines) or CI workflow files to identify how to build, test, and lint the project. Summarize the key `make` targets
13. **Key Code Patterns**: Identify the main language, framework, and coding patterns used

### After exploring all selected repos, provide:
- A summary table comparing the repos (purpose, language, activity level, latest release)
- A dependency graph showing which repos import from which
- Key CRDs and API types that serve as integration contracts between repos
- Key architectural patterns and how the repos relate to each other
- Notable recent changes or active development areas
- Cross-cutting concerns: PRs or issues in one repo that reference or affect another repo
- Any integration points between the repos

Keep the analysis concise but thorough. Focus on information that would help someone contribute to or integrate with these projects.
