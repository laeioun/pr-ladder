package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"runtime"
	"strconv"
	"strings"

	"github.com/laeioun/pr-ladder/internal/config"
	"github.com/laeioun/pr-ladder/internal/ladder"
	"github.com/laeioun/pr-ladder/internal/mcp"
)

var version = "dev"

const usage = `PR Ladder: find a project, try it, write down what you'd change, get a plan to contribute.

Usage:
  pr-ladder find <problem...> [-n 10]   search GitHub for projects that solve a problem
  pr-ladder try <repo> [-no-build]      clone and build a project
  pr-ladder note <repo> [text...]       add a note, or open your notes in $EDITOR
  pr-ladder guide <repo> [-facts]       contribution plan from the repo's rules and your notes
  pr-ladder ls                          list cloned projects
  pr-ladder path <repo>                 print a project's local directory
  pr-ladder mcp                         run as an MCP server over stdio

<repo> is owner/repo, a GitHub URL, or a number from the last find.

Environment:
  GITHUB_TOKEN        GitHub token (falls back to "gh auth token")
  OPENROUTER_API_KEY  use OpenRouter
  PR_LADDER_LLM       openrouter, ollama, or any OpenAI-compatible base URL
  PR_LADDER_LLM_KEY   API key for PR_LADDER_LLM
  PR_LADDER_MODEL     model name
  PR_LADDER_HOME      data directory (default ~/.pr-ladder)
`

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	if err := run(ctx, os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, args []string) error {
	if len(args) == 0 {
		fmt.Print(usage)
		return nil
	}
	cmd, rest := args[0], args[1:]
	flags, pos := split(rest)
	app := ladder.New(config.Load())

	switch cmd {
	case "find":
		if len(pos) == 0 {
			return errors.New("usage: pr-ladder find <problem...>")
		}
		limit, _ := strconv.Atoi(flags["n"])
		res, err := app.Find(ctx, strings.Join(pos, " "), limit)
		if err != nil {
			return err
		}
		fmt.Fprintf(os.Stderr, "search: %s\n\n", strings.Join(res.Queries, " | "))
		if len(res.Repos) == 0 {
			return errors.New("nothing found, try different words")
		}
		for i, r := range res.Repos {
			fmt.Println(ladder.FormatRepo(i+1, r))
		}
		fmt.Fprintln(os.Stderr, "\nnext: pr-ladder try <number>")
		return nil

	case "try":
		name, err := repoArg(app, pos)
		if err != nil {
			return err
		}
		dir, err := app.Clone(ctx, name, os.Stderr)
		if err != nil {
			return err
		}
		if _, skip := flags["no-build"]; !skip {
			if err := app.Build(ctx, name, os.Stderr); err != nil {
				return err
			}
		}
		fmt.Fprintf(os.Stderr, "\nready: %s\nnext: try it out, then pr-ladder note %s \"what you'd change\"\n", dir, name)
		return nil

	case "note":
		name, err := repoArg(app, rest)
		if err != nil {
			return err
		}
		if len(rest) > 1 {
			return app.AddNote(name, strings.Join(rest[1:], " "))
		}
		path, err := app.EnsureNotes(name)
		if err != nil {
			return err
		}
		return edit(path)

	case "guide":
		name, err := repoArg(app, pos)
		if err != nil {
			return err
		}
		if _, facts := flags["facts"]; facts || app.LLM() == nil {
			out, err := app.Context(ctx, name)
			if err != nil {
				return err
			}
			fmt.Print(out)
			if !facts {
				fmt.Fprintln(os.Stderr, "\nno LLM configured: showing collected facts only (set OPENROUTER_API_KEY or PR_LADDER_LLM)")
			}
			return nil
		}
		fmt.Fprintf(os.Stderr, "model: %s\n\n", app.LLM().Model())
		if err := app.Guide(ctx, name, os.Stdout); err != nil {
			return err
		}
		fmt.Println()
		return nil

	case "ls":
		projects, err := app.Projects()
		if err != nil {
			return err
		}
		for _, p := range projects {
			fmt.Printf("%-40s %d notes\n", p.Name, p.Notes)
		}
		return nil

	case "path":
		name, err := repoArg(app, pos)
		if err != nil {
			return err
		}
		fmt.Println(app.RepoDir(name))
		return nil

	case "mcp":
		return mcp.New(app, version).Serve(ctx, os.Stdin, os.Stdout)

	case "version", "-v", "--version":
		fmt.Println(version)
		return nil

	case "help", "-h", "--help":
		fmt.Print(usage)
		return nil
	}
	return fmt.Errorf("unknown command %q, see pr-ladder help", cmd)
}

func split(args []string) (map[string]string, []string) {
	flags := map[string]string{}
	var pos []string
	for i := 0; i < len(args); i++ {
		a := args[i]
		if !strings.HasPrefix(a, "-") || a == "-" {
			pos = append(pos, a)
			continue
		}
		key, val, ok := strings.Cut(strings.TrimLeft(a, "-"), "=")
		if !ok && key == "n" && i+1 < len(args) {
			i++
			val = args[i]
		}
		flags[key] = val
	}
	return flags, pos
}

func repoArg(app *ladder.App, pos []string) (string, error) {
	if len(pos) == 0 {
		return "", errors.New("missing <repo>")
	}
	return app.Resolve(pos[0])
}

func edit(path string) error {
	editor := strings.Fields(os.Getenv("VISUAL"))
	if len(editor) == 0 {
		editor = strings.Fields(os.Getenv("EDITOR"))
	}
	if len(editor) == 0 {
		editor = []string{"vi"}
		if runtime.GOOS == "windows" {
			editor = []string{"notepad"}
		}
	}
	cmd := exec.Command(editor[0], append(editor[1:], path)...)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	return cmd.Run()
}
