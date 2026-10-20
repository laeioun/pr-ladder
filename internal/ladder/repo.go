package ladder

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/laeioun/pr-ladder/internal/build"
)

func (a *App) Clone(ctx context.Context, name string, out io.Writer) (string, error) {
	dir := a.RepoDir(name)
	if _, err := os.Stat(filepath.Join(dir, ".git")); err == nil {
		fmt.Fprintf(out, "Updating %s\n", dir)
		return dir, git(ctx, out, dir, "pull", "--ff-only")
	}
	if err := os.MkdirAll(filepath.Dir(dir), 0o755); err != nil {
		return "", err
	}
	fmt.Fprintf(out, "Cloning %s into %s\n", name, dir)
	return dir, git(ctx, out, "", "clone", "--filter=blob:none", "https://github.com/"+name+".git", dir)
}

func (a *App) Build(ctx context.Context, name string, out io.Writer) error {
	dir := a.RepoDir(name)
	plan, ok := build.Detect(dir)
	if !ok {
		return fmt.Errorf("no known build system in %s, build it manually", dir)
	}
	fmt.Fprintf(out, "Detected %s\n", plan.System)
	return build.Run(ctx, dir, plan, out)
}

func (a *App) Cloned(name string) bool {
	_, err := os.Stat(filepath.Join(a.RepoDir(name), ".git"))
	return err == nil
}

func git(ctx context.Context, out io.Writer, dir string, args ...string) error {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
	cmd.Stdout = out
	cmd.Stderr = out
	return cmd.Run()
}

func gitOutput(ctx context.Context, dir string, args ...string) string {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = dir
	out, _ := cmd.Output()
	return strings.TrimSpace(string(out))
}
