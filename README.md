<p align="center"><img src="docs/logo.svg" width="96" alt="PR Ladder logo"></p>

# PR Ladder

Turn "this tool annoys me" into a merged pull request.

Describe a problem. PR Ladder finds GitHub projects that already solve it, clones and builds them for you, collects your notes on what you'd improve, then reads each repo's contribution rules and maintainer history to tell you how to land the change, or whether to fork it or build your own instead.

One small Go binary. No dependencies, no server, no Docker.

```
pr-ladder find "terminal file manager with image previews"
pr-ladder try 2
pr-ladder note 2 "keybindings can't be remapped"
pr-ladder guide 2
```

## The ladder

1. **find**: searches GitHub for projects that match your problem. With an LLM configured, it first turns your description into good search queries.
2. **try**: clones the project (partial clone, so it's fast) and builds it with whatever it uses: Cargo, Go, npm/pnpm/yarn/bun, uv, Zig, Gradle, Maven, Meson, CMake, Make or .NET.
3. **note**: you use the project and write down what you'd change, one line at a time or in `$EDITOR`.
4. **guide**: collects the repo's `CONTRIBUTING`, PR templates, commit conventions, sign-off and CLA rules, CI workflows and layout, plus how often outside PRs actually get merged and how fast. An LLM then writes:
   - a verdict per note: PR upstream, open an issue first, or fork / build your own
   - where in the code to start and how to test it
   - a checklist of the repo's house rules
   - a PR title and description that follow its template

Small fixes usually get merged. Big changes get a lot of scrutiny from maintainers, so the guide tells you when a fork or your own rewrite is the better move.

## Install

```bash
go install github.com/laeioun/pr-ladder/cmd/pr-ladder@latest
```

Or from source:

```bash
git clone https://github.com/laeioun/pr-ladder
cd pr-ladder
go build ./cmd/pr-ladder
```

You need `git`, plus the toolchains of whatever projects you want to build.

## Setup

**GitHub.** If you're logged in with the [GitHub CLI](https://cli.github.com), nothing to do. Otherwise set `GITHUB_TOKEN`. It works without a token too, but GitHub's anonymous rate limit is low.

**LLM (optional).** Any OpenAI-compatible API works.

```bash
# OpenRouter
export OPENROUTER_API_KEY=sk-or-...
export PR_LADDER_MODEL=...         # optional, defaults to openrouter/auto

# Ollama
export PR_LADDER_LLM=ollama
export PR_LADDER_MODEL=...         # optional, defaults to llama3.2

# Anything else (LM Studio, vLLM, OpenAI, ...)
export PR_LADDER_LLM=http://localhost:1234/v1
export PR_LADDER_LLM_KEY=...
export PR_LADDER_MODEL=...
```

Without an LLM, `find` falls back to keyword search and `guide` prints the collected facts so you can read them yourself or paste them into any chat.

## Commands

| Command | |
| --- | --- |
| `find <problem...> [-n 10]` | Search GitHub for projects that solve a problem |
| `try <repo> [-no-build]` | Clone or update a project and build it |
| `note <repo> [text...]` | Add a note, or open all notes in `$EDITOR` |
| `guide <repo> [-facts]` | Write a contribution plan. `-facts` skips the LLM |
| `ls` | List cloned projects |
| `path <repo>` | Print a project's directory, e.g. `cd $(pr-ladder path 2)` |
| `mcp` | Run as an MCP server over stdio |

`<repo>` can be `owner/repo`, a GitHub URL, or a result number from the last `find`.

Everything lives in `~/.pr-ladder` (or `PR_LADDER_HOME`): clones in `repos/`, notes as plain Markdown in `notes/`.

## Use it from your coding agent

`pr-ladder mcp` lets Claude Code, Cursor or any MCP client run the whole loop. The agent writes the search queries and the guide itself, so you don't need a separate LLM key.

```bash
claude mcp add pr-ladder -- pr-ladder mcp
```

Other clients:

```json
{
  "mcpServers": {
    "pr-ladder": { "command": "pr-ladder", "args": ["mcp"] }
  }
}
```

Tools: `find_projects`, `try_project`, `add_note`, `read_notes`, `contribution_context`, `list_projects`.

## Safety

`try` runs the project's own build: `build.rs`, npm install scripts, Makefiles. That is arbitrary code from the internet. Use `-no-build` (or leave `build` off over MCP) for anything you don't trust, and build it in a container or VM instead.

## Contributing

Issues and PRs are welcome. Keep changes small and focused, keep the binary dependency-free, and run:

```bash
go vet ./...
go test ./...
```

## License

[MIT](LICENSE)
