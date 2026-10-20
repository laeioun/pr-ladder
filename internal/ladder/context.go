package ladder

import (
	"context"
	"fmt"
	"maps"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"
)

var guidelinePrefixes = []string{
	"contributing", "hacking", "development", "pull_request_template",
	"commitlint.config", ".commitlintrc", "dco", "cla", ".pre-commit-config",
}

var conventional = regexp.MustCompile(`^(feat|fix|docs|chore|refactor|test|tests|perf|build|ci|style|revert)(\([^)]*\))?!?: `)

func (a *App) Context(ctx context.Context, name string) (string, error) {
	if !a.Cloned(name) {
		return "", fmt.Errorf("%s is not cloned yet, run: pr-ladder try %s", name, name)
	}
	dir := a.RepoDir(name)
	var b strings.Builder
	fmt.Fprintf(&b, "# %s\n\n", name)

	if r, err := a.gh().Repo(ctx, name); err == nil {
		fmt.Fprintf(&b, "%s\n\nStars %d · forks %d · open issues %d · license %s · last push %s\n\n",
			r.Description, r.Stars, r.Forks, r.OpenIssues, r.LicenseID(), ago(r.PushedAt))
	}

	if s, err := a.gh().PRStats(ctx, name); err == nil && s.Sampled > 0 {
		b.WriteString("## Maintainer activity\n\n")
		fmt.Fprintf(&b, "Last %d closed PRs: %d merged (%d%%).\n", s.Sampled, s.Merged, pct(s.Merged, s.Sampled))
		if s.External > 0 {
			fmt.Fprintf(&b, "From outside contributors: %d of %d merged (%d%%).\n", s.ExternalMerged, s.External, pct(s.ExternalMerged, s.External))
		}
		if s.Merged > 0 {
			fmt.Fprintf(&b, "Median time to merge: %s.\n", human(s.MedianTimeToMerge))
		}
		b.WriteString("\n")
	}

	if subjects := lines(gitOutput(ctx, dir, "log", "-n", "30", "--no-merges", "--format=%s")); len(subjects) > 0 {
		conv := 0
		for _, s := range subjects {
			if conventional.MatchString(s) {
				conv++
			}
		}
		signed := len(lines(gitOutput(ctx, dir, "log", "-n", "30", "--no-merges", "--grep=Signed-off-by:", "--format=%h")))
		b.WriteString("## Commit history\n\n")
		fmt.Fprintf(&b, "Conventional Commits in %d of the last %d commits. Signed-off-by in %d.\n\n", conv, len(subjects), signed)
		for _, s := range subjects[:min(15, len(subjects))] {
			fmt.Fprintf(&b, "    %s\n", s)
		}
		b.WriteString("\n")
	}

	files := lines(gitOutput(ctx, dir, "ls-files"))

	var workflows []string
	for _, f := range files {
		if strings.HasPrefix(f, ".github/workflows/") {
			workflows = append(workflows, path.Base(f))
		}
	}
	if len(workflows) > 0 {
		fmt.Fprintf(&b, "## CI workflows\n\n%s\n\n", strings.Join(workflows, ", "))
	}

	for _, f := range guidelineFiles(files) {
		data, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(f)))
		if err != nil {
			continue
		}
		fmt.Fprintf(&b, "## %s\n\n%s\n\n", f, truncate(string(data), 8000))
	}

	fmt.Fprintf(&b, "## Layout\n\n%s\n\n", layout(files))

	for _, f := range files {
		if !strings.Contains(f, "/") && strings.HasPrefix(strings.ToLower(f), "readme") {
			if data, err := os.ReadFile(filepath.Join(dir, f)); err == nil {
				fmt.Fprintf(&b, "## %s (excerpt)\n\n%s\n\n", f, truncate(string(data), 3000))
			}
			break
		}
	}

	notes, _ := a.Notes(name)
	if strings.TrimSpace(notes) == "" {
		notes = "(no notes yet)"
	}
	fmt.Fprintf(&b, "## My notes\n\n%s\n", notes)
	return b.String(), nil
}

func guidelineFiles(files []string) []string {
	var out []string
	for _, f := range files {
		dir, base := path.Split(f)
		if dir != "" && dir != ".github/" && dir != "docs/" && dir != ".github/PULL_REQUEST_TEMPLATE/" {
			continue
		}
		lower := strings.ToLower(base)
		for _, p := range guidelinePrefixes {
			if strings.HasPrefix(lower, p) || dir == ".github/PULL_REQUEST_TEMPLATE/" {
				out = append(out, f)
				break
			}
		}
	}
	return out
}

func layout(files []string) string {
	children := map[string]map[string]bool{}
	var top []string
	for _, f := range files {
		first, rest, nested := strings.Cut(f, "/")
		if !nested {
			top = append(top, first)
			continue
		}
		if children[first] == nil {
			children[first] = map[string]bool{}
			top = append(top, first+"/")
		}
		child, _, deeper := strings.Cut(rest, "/")
		if deeper {
			child += "/"
		}
		children[first][child] = true
	}
	slices.Sort(top)
	var b strings.Builder
	for _, t := range top {
		fmt.Fprintf(&b, "    %s", t)
		if kids := children[strings.TrimSuffix(t, "/")]; kids != nil {
			names := slices.Sorted(maps.Keys(kids))
			if len(names) > 12 {
				fmt.Fprintf(&b, " (%d entries)", len(names))
			} else {
				fmt.Fprintf(&b, " %s", strings.Join(names, " "))
			}
		}
		b.WriteString("\n")
	}
	return strings.TrimRight(b.String(), "\n")
}

func lines(s string) []string {
	if s == "" {
		return nil
	}
	return strings.Split(strings.ReplaceAll(s, "\r\n", "\n"), "\n")
}

func truncate(s string, n int) string {
	s = strings.TrimSpace(s)
	if len(s) <= n {
		return s
	}
	return s[:n] + "\n…(truncated)"
}

func pct(a, b int) int {
	if b == 0 {
		return 0
	}
	return a * 100 / b
}

func ago(t time.Time) string {
	if t.IsZero() {
		return "unknown"
	}
	return human(time.Since(t)) + " ago"
}

func human(d time.Duration) string {
	switch {
	case d < time.Hour:
		return fmt.Sprintf("%d min", int(d.Minutes()))
	case d < 48*time.Hour:
		return fmt.Sprintf("%d h", int(d.Hours()))
	case d < 60*24*time.Hour:
		return fmt.Sprintf("%d days", int(d.Hours()/24))
	case d < 730*24*time.Hour:
		return fmt.Sprintf("%d mo", int(d.Hours()/24/30))
	default:
		return fmt.Sprintf("%d y", int(d.Hours()/24/365))
	}
}
