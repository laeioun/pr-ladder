package ladder

import (
	"context"
	"errors"
	"io"

	"github.com/laeioun/pr-ladder/internal/llm"
)

const GuidePrompt = `You are a senior open-source maintainer coaching a contributor.
You get facts about a repository (maintainer activity, commit history, CI, contribution guidelines, layout, README) and the contributor's notes on what they want to change.

Write a concise Markdown guide with these sections:

## Verdict
For each note: "PR upstream", "open an issue first", or "fork / build your own". Small, focused fixes go upstream. Large features, redesigns or rewrites go to an issue first or a fork, weighted by the outside-contributor merge rate and how active the maintainers are. One line of reasoning each.

## How to make each change
For each note marked PR or issue: which files or directories to start in (only paths from the layout), the approach, and how to test it.

## House rules
A checklist drawn only from the provided facts: commit message format, sign-off, CLA, branch naming, PR template fields, required tests, lint and format commands, CI checks. Say "not specified" instead of guessing.

## PR draft
A title and description for the first PR that follow the repository's template and conventions.

Be specific and brief. No filler, no generic open-source advice.`

var ErrNoLLM = errors.New("no LLM configured")

func (a *App) Guide(ctx context.Context, name string, w io.Writer) error {
	if a.llm == nil {
		return ErrNoLLM
	}
	facts, err := a.Context(ctx, name)
	if err != nil {
		return err
	}
	return a.llm.Stream(ctx, []llm.Message{
		{Role: "system", Content: GuidePrompt},
		{Role: "user", Content: facts},
	}, w)
}
