package ladder

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/laeioun/pr-ladder/internal/github"
	"github.com/laeioun/pr-ladder/internal/llm"
)

type FindResult struct {
	Queries []string
	Repos   []github.Repo
}

const queryPrompt = `You turn a developer's problem into GitHub repository searches.
Return a JSON array of 1 to 3 search queries and nothing else.
Each query is 1 to 4 plain keywords that would appear in a matching repository's name, description or topics.
Add a qualifier such as language:rust only when the user asks for it.
Example: ["terminal file manager", "tui file browser"]`

func (a *App) Find(ctx context.Context, problem string, limit int) (FindResult, error) {
	queries := []string{problem}
	if a.llm != nil {
		if qs := a.searchQueries(ctx, problem); len(qs) > 0 {
			queries = qs
		}
	} else if kw := keywords(problem); len(kw) > 0 {
		queries = []string{strings.Join(kw[:min(3, len(kw))], " ")}
		if len(kw) > 3 {
			queries = append(queries, strings.Join(kw, " "))
		}
	}
	return a.FindQueries(ctx, queries, limit)
}

func (a *App) FindQueries(ctx context.Context, queries []string, limit int) (FindResult, error) {
	if limit <= 0 {
		limit = 10
	}
	var lists [][]github.Repo
	for _, q := range queries {
		search := q
		if !strings.Contains(q, "stars:") {
			search += " stars:>=10"
		}
		repos, err := a.gh().Search(ctx, search, limit)
		if err == nil && len(repos) == 0 && search != q {
			repos, err = a.gh().Search(ctx, q, limit)
		}
		if err != nil {
			return FindResult{}, err
		}
		lists = append(lists, repos)
	}

	seen := map[string]bool{}
	var merged []github.Repo
	for i := 0; len(merged) < limit; i++ {
		more := false
		for _, list := range lists {
			if i >= len(list) {
				continue
			}
			more = true
			r := list[i]
			if r.Fork || seen[r.FullName] || len(merged) >= limit {
				continue
			}
			seen[r.FullName] = true
			merged = append(merged, r)
		}
		if !more {
			break
		}
	}
	return FindResult{Queries: queries, Repos: merged}, a.saveLastFind(merged)
}

func (a *App) searchQueries(ctx context.Context, problem string) []string {
	out, err := a.llm.Complete(ctx, []llm.Message{
		{Role: "system", Content: queryPrompt},
		{Role: "user", Content: problem},
	})
	if err != nil {
		return nil
	}
	start, end := strings.Index(out, "["), strings.LastIndex(out, "]")
	if start < 0 || end <= start {
		return nil
	}
	var qs []string
	if json.Unmarshal([]byte(out[start:end+1]), &qs) != nil {
		return nil
	}
	if len(qs) > 3 {
		qs = qs[:3]
	}
	return qs
}

var stopwords = map[string]bool{
	"a": true, "an": true, "the": true, "i": true, "my": true, "me": true, "to": true, "for": true,
	"of": true, "in": true, "on": true, "and": true, "or": true, "with": true, "that": true, "which": true,
	"want": true, "need": true, "something": true, "tool": true, "app": true, "way": true, "can": true,
	"is": true, "it": true, "be": true, "from": true, "into": true, "like": true, "faster": true, "easier": true,
}

func keywords(s string) []string {
	var out []string
	for _, w := range strings.Fields(strings.ToLower(s)) {
		w = strings.Trim(w, ".,!?;:\"'()")
		if w != "" && !stopwords[w] {
			out = append(out, w)
		}
	}
	if len(out) > 5 {
		out = out[:5]
	}
	return out
}
