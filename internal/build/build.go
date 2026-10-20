package build

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

type Plan struct {
	System string
	Steps  [][]string
}

func (p Plan) String() string {
	cmds := make([]string, len(p.Steps))
	for i, s := range p.Steps {
		cmds[i] = strings.Join(s, " ")
	}
	return strings.Join(cmds, " && ")
}

func Detect(dir string) (Plan, bool) {
	has := func(name string) bool {
		_, err := os.Stat(filepath.Join(dir, name))
		return err == nil
	}
	switch {
	case has("Cargo.toml"):
		return Plan{"cargo", [][]string{{"cargo", "build"}}}, true
	case has("go.mod"):
		return Plan{"go", [][]string{{"go", "build", "./..."}}}, true
	case has("package.json"):
		return node(dir, has), true
	case has("build.zig"):
		return Plan{"zig", [][]string{{"zig", "build"}}}, true
	case has("pyproject.toml") || has("setup.py"):
		return Plan{"python", [][]string{{"uv", "sync"}}}, true
	case has("gradlew"):
		gradlew := "./gradlew"
		if runtime.GOOS == "windows" {
			gradlew = `.\gradlew.bat`
		}
		return Plan{"gradle", [][]string{{gradlew, "build", "-x", "test"}}}, true
	case has("pom.xml"):
		return Plan{"maven", [][]string{{"mvn", "-q", "package", "-DskipTests"}}}, true
	case has("meson.build"):
		return Plan{"meson", [][]string{{"meson", "setup", "build"}, {"meson", "compile", "-C", "build"}}}, true
	case has("CMakeLists.txt"):
		return Plan{"cmake", [][]string{{"cmake", "-B", "build"}, {"cmake", "--build", "build"}}}, true
	case has("Makefile") || has("makefile"):
		return Plan{"make", [][]string{{"make"}}}, true
	}
	if m, _ := filepath.Glob(filepath.Join(dir, "*.sln")); len(m) > 0 {
		return Plan{"dotnet", [][]string{{"dotnet", "build"}}}, true
	}
	return Plan{}, false
}

func node(dir string, has func(string) bool) Plan {
	pm := "npm"
	switch {
	case has("bun.lock") || has("bun.lockb"):
		pm = "bun"
	case has("pnpm-lock.yaml"):
		pm = "pnpm"
	case has("yarn.lock"):
		pm = "yarn"
	}
	steps := [][]string{{pm, "install"}}
	var pkg struct {
		Scripts map[string]string `json:"scripts"`
	}
	if data, err := os.ReadFile(filepath.Join(dir, "package.json")); err == nil {
		_ = json.Unmarshal(data, &pkg)
	}
	if _, ok := pkg.Scripts["build"]; ok {
		steps = append(steps, []string{pm, "run", "build"})
	}
	return Plan{pm, steps}
}

func Run(ctx context.Context, dir string, p Plan, out io.Writer) error {
	for _, step := range p.Steps {
		if _, err := exec.LookPath(step[0]); err != nil && !strings.HasPrefix(step[0], ".") {
			return fmt.Errorf("%s not found on PATH", step[0])
		}
		fmt.Fprintf(out, "$ %s\n", strings.Join(step, " "))
		cmd := exec.CommandContext(ctx, step[0], step[1:]...)
		cmd.Dir = dir
		cmd.Stdout = out
		cmd.Stderr = out
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("%s: %w", strings.Join(step, " "), err)
		}
	}
	return nil
}
