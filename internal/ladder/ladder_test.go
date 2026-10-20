package ladder

import (
	"slices"
	"strings"
	"testing"

	"github.com/laeioun/pr-ladder/internal/config"
	"github.com/laeioun/pr-ladder/internal/github"
)

func TestResolve(t *testing.T) {
	a := New(config.Config{Home: t.TempDir()})
	if err := a.saveLastFind([]github.Repo{{FullName: "sxyazi/yazi"}}); err != nil {
		t.Fatal(err)
	}
	ok := map[string]string{
		"sxyazi/yazi":                       "sxyazi/yazi",
		"https://github.com/sxyazi/yazi":    "sxyazi/yazi",
		"github.com/sxyazi/yazi.git":        "sxyazi/yazi",
		"https://github.com/a-b/c.d/pull/1": "a-b/c.d",
		"1":                                 "sxyazi/yazi",
	}
	for in, want := range ok {
		if got, err := a.Resolve(in); err != nil || got != want {
			t.Errorf("Resolve(%q) = %q, %v; want %q", in, got, err, want)
		}
	}
	for _, in := range []string{"", "2", "yazi", "../etc", "a/..", "-x/y", `a\b`} {
		if got, err := a.Resolve(in); err == nil {
			t.Errorf("Resolve(%q) = %q; want error", in, got)
		}
	}
}

func TestNotes(t *testing.T) {
	a := New(config.Config{Home: t.TempDir()})
	for _, n := range []string{"add a --json flag", "faster startup"} {
		if err := a.AddNote("o/r", n); err != nil {
			t.Fatal(err)
		}
	}
	notes, _ := a.Notes("o/r")
	if !strings.Contains(notes, "- add a --json flag\n- faster startup\n") {
		t.Fatalf("notes = %q", notes)
	}
	if err := a.AddNote("o/r", "  "); err == nil {
		t.Fatal("empty note accepted")
	}
}

func TestKeywords(t *testing.T) {
	got := keywords("I want a terminal file manager with image previews")
	want := []string{"terminal", "file", "manager", "image", "previews"}
	if !slices.Equal(got, want) {
		t.Fatalf("got %v", got)
	}
}

func TestGuidelineFiles(t *testing.T) {
	files := []string{
		"CONTRIBUTING.md", "docs/HACKING.md", ".github/pull_request_template.md",
		".github/PULL_REQUEST_TEMPLATE/feature.md", "src/contributing.rs", "README.md",
	}
	got := guidelineFiles(files)
	want := []string{"CONTRIBUTING.md", "docs/HACKING.md", ".github/pull_request_template.md", ".github/PULL_REQUEST_TEMPLATE/feature.md"}
	if !slices.Equal(got, want) {
		t.Fatalf("got %v", got)
	}
}
