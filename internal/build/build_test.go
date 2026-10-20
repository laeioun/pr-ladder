package build

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDetect(t *testing.T) {
	cases := []struct {
		files map[string]string
		want  string
	}{
		{map[string]string{"Cargo.toml": "", "Makefile": ""}, "cargo build"},
		{map[string]string{"go.mod": ""}, "go build ./..."},
		{map[string]string{"package.json": `{"scripts":{"build":"vite build"}}`, "pnpm-lock.yaml": ""}, "pnpm install && pnpm run build"},
		{map[string]string{"package.json": `{}`}, "npm install"},
		{map[string]string{"CMakeLists.txt": ""}, "cmake -B build && cmake --build build"},
	}
	for _, c := range cases {
		dir := t.TempDir()
		for name, body := range c.files {
			if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
				t.Fatal(err)
			}
		}
		p, ok := Detect(dir)
		if !ok || p.String() != c.want {
			t.Errorf("Detect(%v) = %q, want %q", c.files, p.String(), c.want)
		}
	}
	if _, ok := Detect(t.TempDir()); ok {
		t.Error("detected a build system in an empty dir")
	}
}
