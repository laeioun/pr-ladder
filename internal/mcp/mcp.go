package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"slices"
	"strings"

	"github.com/laeioun/pr-ladder/internal/ladder"
)

type Server struct {
	app     *ladder.App
	version string
}

func New(app *ladder.App, version string) *Server {
	return &Server{app: app, version: version}
}

type request struct {
	ID     json.RawMessage `json:"id,omitempty"`
	Method string          `json:"method"`
	Params json.RawMessage `json:"params,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

var protocols = []string{"2024-11-05", "2025-03-26", "2025-06-18"}

func (s *Server) Serve(ctx context.Context, in io.Reader, out io.Writer) error {
	sc := bufio.NewScanner(in)
	sc.Buffer(make([]byte, 64*1024), 16*1024*1024)
	enc := json.NewEncoder(out)
	for sc.Scan() {
		var req request
		if err := json.Unmarshal(sc.Bytes(), &req); err != nil {
			continue
		}
		if len(req.ID) == 0 {
			continue
		}
		result, rerr := s.dispatch(ctx, req)
		resp := map[string]any{"jsonrpc": "2.0", "id": req.ID}
		if rerr != nil {
			resp["error"] = rerr
		} else {
			resp["result"] = result
		}
		if err := enc.Encode(resp); err != nil {
			return err
		}
	}
	return sc.Err()
}

func (s *Server) dispatch(ctx context.Context, req request) (any, *rpcError) {
	switch req.Method {
	case "initialize":
		var p struct {
			ProtocolVersion string `json:"protocolVersion"`
		}
		_ = json.Unmarshal(req.Params, &p)
		version := protocols[len(protocols)-1]
		if slices.Contains(protocols, p.ProtocolVersion) {
			version = p.ProtocolVersion
		}
		return map[string]any{
			"protocolVersion": version,
			"capabilities":    map[string]any{"tools": map[string]any{}},
			"serverInfo":      map[string]string{"name": "pr-ladder", "version": s.version},
		}, nil
	case "ping":
		return map[string]any{}, nil
	case "tools/list":
		return map[string]any{"tools": tools}, nil
	case "tools/call":
		var p struct {
			Name      string          `json:"name"`
			Arguments json.RawMessage `json:"arguments"`
		}
		if err := json.Unmarshal(req.Params, &p); err != nil {
			return nil, &rpcError{-32602, err.Error()}
		}
		text, err := s.call(ctx, p.Name, p.Arguments)
		if err != nil {
			return toolResult(err.Error(), true), nil
		}
		return toolResult(text, false), nil
	default:
		return nil, &rpcError{-32601, "method not found: " + req.Method}
	}
}

type args struct {
	Problem string   `json:"problem"`
	Queries []string `json:"queries"`
	Limit   int      `json:"limit"`
	Repo    string   `json:"repo"`
	Build   bool     `json:"build"`
	Note    string   `json:"note"`
}

func (s *Server) call(ctx context.Context, name string, raw json.RawMessage) (string, error) {
	var a args
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &a); err != nil {
			return "", err
		}
	}
	var repo string
	if a.Repo != "" {
		r, err := s.app.Resolve(a.Repo)
		if err != nil {
			return "", err
		}
		repo = r
	}
	needRepo := func() error {
		if repo == "" {
			return fmt.Errorf("repo is required")
		}
		return nil
	}

	switch name {
	case "find_projects":
		var res ladder.FindResult
		var err error
		if len(a.Queries) > 0 {
			res, err = s.app.FindQueries(ctx, a.Queries, a.Limit)
		} else if strings.TrimSpace(a.Problem) != "" {
			res, err = s.app.Find(ctx, a.Problem, a.Limit)
		} else {
			return "", fmt.Errorf("problem or queries is required")
		}
		if err != nil {
			return "", err
		}
		var b strings.Builder
		fmt.Fprintf(&b, "Queries: %s\n\n", strings.Join(res.Queries, " | "))
		for i, r := range res.Repos {
			fmt.Fprintf(&b, "%s\n", ladder.FormatRepo(i+1, r))
		}
		if len(res.Repos) == 0 {
			b.WriteString("No repositories found. Try broader queries.\n")
		}
		return b.String(), nil
	case "try_project":
		if err := needRepo(); err != nil {
			return "", err
		}
		var log strings.Builder
		dir, err := s.app.Clone(ctx, repo, &log)
		if err == nil && a.Build {
			err = s.app.Build(ctx, repo, &log)
		}
		out := tail(log.String(), 6000)
		if err != nil {
			return "", fmt.Errorf("%v\n\n%s", err, out)
		}
		return fmt.Sprintf("Ready in %s\n\n%s", dir, out), nil
	case "add_note":
		if err := needRepo(); err != nil {
			return "", err
		}
		if err := s.app.AddNote(repo, a.Note); err != nil {
			return "", err
		}
		return "Saved to " + s.app.NotesPath(repo), nil
	case "read_notes":
		if err := needRepo(); err != nil {
			return "", err
		}
		notes, err := s.app.Notes(repo)
		if notes == "" && err == nil {
			notes = "No notes yet."
		}
		return notes, err
	case "contribution_context":
		if err := needRepo(); err != nil {
			return "", err
		}
		facts, err := s.app.Context(ctx, repo)
		if err != nil {
			return "", err
		}
		return "Instructions:\n\n" + ladder.GuidePrompt + "\n\n---\n\n" + facts, nil
	case "list_projects":
		projects, err := s.app.Projects()
		if err != nil {
			return "", err
		}
		if len(projects) == 0 {
			return "No projects cloned yet.", nil
		}
		var b strings.Builder
		for _, p := range projects {
			fmt.Fprintf(&b, "%s (%d notes)\n", p.Name, p.Notes)
		}
		return b.String(), nil
	}
	return "", fmt.Errorf("unknown tool %q", name)
}

func toolResult(text string, isError bool) map[string]any {
	return map[string]any{
		"content": []map[string]string{{"type": "text", "text": text}},
		"isError": isError,
	}
}

func tail(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return "…" + s[len(s)-n:]
}

func schema(props map[string]any, required ...string) map[string]any {
	if required == nil {
		required = []string{}
	}
	return map[string]any{"type": "object", "properties": props, "required": required}
}

var (
	str     = map[string]any{"type": "string"}
	repoArg = map[string]any{"type": "string", "description": "owner/repo, GitHub URL, or a result number from the last find"}
)

var tools = []map[string]any{
	{
		"name":        "find_projects",
		"description": "Search GitHub for projects that solve a problem. Pass your own GitHub search queries, or a plain-language problem.",
		"inputSchema": schema(map[string]any{
			"problem": str,
			"queries": map[string]any{"type": "array", "items": str, "description": "GitHub repository search queries, 1-4 keywords each"},
			"limit":   map[string]any{"type": "integer", "minimum": 1, "maximum": 30},
		}),
	},
	{
		"name":        "try_project",
		"description": "Clone or update a project locally. Set build to compile it with its detected build system. Building runs the project's own build scripts.",
		"inputSchema": schema(map[string]any{"repo": repoArg, "build": map[string]any{"type": "boolean"}}, "repo"),
	},
	{
		"name":        "add_note",
		"description": "Record something the user would change or improve in a project.",
		"inputSchema": schema(map[string]any{"repo": repoArg, "note": str}, "repo", "note"),
	},
	{
		"name":        "read_notes",
		"description": "Read the user's improvement notes for a project.",
		"inputSchema": schema(map[string]any{"repo": repoArg}, "repo"),
	},
	{
		"name":        "contribution_context",
		"description": "Collect a cloned project's contribution rules, commit conventions, maintainer merge activity, layout and the user's notes, with instructions for writing a contribution guide.",
		"inputSchema": schema(map[string]any{"repo": repoArg}, "repo"),
	},
	{
		"name":        "list_projects",
		"description": "List cloned projects and how many notes each has.",
		"inputSchema": schema(map[string]any{}),
	},
}
