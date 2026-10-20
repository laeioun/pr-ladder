package ladder

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/laeioun/pr-ladder/internal/config"
	"github.com/laeioun/pr-ladder/internal/github"
	"github.com/laeioun/pr-ladder/internal/llm"
)

type App struct {
	cfg    config.Config
	llm    *llm.Client
	ghOnce sync.Once
	ghc    *github.Client
}

func New(cfg config.Config) *App {
	a := &App{cfg: cfg}
	if cfg.HasLLM() {
		a.llm = llm.New(cfg.LLMURL, cfg.LLMKey, cfg.Model)
	}
	return a
}

func (a *App) gh() *github.Client {
	a.ghOnce.Do(func() { a.ghc = github.New(a.cfg.GitHubTokenOrGH()) })
	return a.ghc
}

func (a *App) LLM() *llm.Client { return a.llm }

var namePattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9-]*/[A-Za-z0-9._-]+$`)

func (a *App) Resolve(arg string) (string, error) {
	s := strings.TrimSpace(arg)
	if n, err := strconv.Atoi(s); err == nil {
		last, err := a.lastFind()
		if err != nil || n < 1 || n > len(last) {
			return "", fmt.Errorf("no result #%d from the last find", n)
		}
		return last[n-1].FullName, nil
	}
	for _, p := range []string{"https://", "http://", "www.", "github.com/"} {
		s = strings.TrimPrefix(s, p)
	}
	s = strings.TrimSuffix(strings.TrimSuffix(s, "/"), ".git")
	if parts := strings.Split(s, "/"); len(parts) > 2 {
		s = parts[0] + "/" + parts[1]
	}
	if !namePattern.MatchString(s) || strings.HasSuffix(s, "/.") || strings.HasSuffix(s, "/..") {
		return "", fmt.Errorf("%q is not owner/repo, a GitHub URL, or a result number", arg)
	}
	return s, nil
}

func (a *App) RepoDir(name string) string {
	return filepath.Join(a.cfg.Home, "repos", filepath.FromSlash(name))
}

func (a *App) NotesPath(name string) string {
	return filepath.Join(a.cfg.Home, "notes", filepath.FromSlash(name)+".md")
}

func (a *App) AddNote(name, text string) error {
	text = strings.TrimSpace(text)
	if text == "" {
		return errors.New("empty note")
	}
	path, err := a.EnsureNotes(name)
	if err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = fmt.Fprintf(f, "- %s\n", text)
	return err
}

func (a *App) EnsureNotes(name string) (string, error) {
	path := a.NotesPath(name)
	if _, err := os.Stat(path); err == nil {
		return path, nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return "", err
	}
	header := fmt.Sprintf("# %s\n\nWhat would you change or improve? One line per idea.\n\n", name)
	return path, os.WriteFile(path, []byte(header), 0o644)
}

func (a *App) Notes(name string) (string, error) {
	data, err := os.ReadFile(a.NotesPath(name))
	if errors.Is(err, fs.ErrNotExist) {
		return "", nil
	}
	return string(data), err
}

type Project struct {
	Name     string
	Notes    int
	Modified time.Time
}

func (a *App) Projects() ([]Project, error) {
	dirs, err := filepath.Glob(filepath.Join(a.cfg.Home, "repos", "*", "*"))
	if err != nil {
		return nil, err
	}
	var out []Project
	for _, d := range dirs {
		info, err := os.Stat(d)
		if err != nil || !info.IsDir() {
			continue
		}
		name := filepath.Base(filepath.Dir(d)) + "/" + filepath.Base(d)
		p := Project{Name: name, Modified: info.ModTime()}
		notes, _ := a.Notes(name)
		for _, line := range strings.Split(notes, "\n") {
			if strings.HasPrefix(strings.TrimSpace(line), "- ") {
				p.Notes++
			}
		}
		out = append(out, p)
	}
	slices.SortFunc(out, func(x, y Project) int { return y.Modified.Compare(x.Modified) })
	return out, nil
}

func (a *App) lastFind() ([]github.Repo, error) {
	data, err := os.ReadFile(filepath.Join(a.cfg.Home, "last-find.json"))
	if err != nil {
		return nil, err
	}
	var repos []github.Repo
	return repos, json.Unmarshal(data, &repos)
}

func (a *App) saveLastFind(repos []github.Repo) error {
	if err := os.MkdirAll(a.cfg.Home, 0o755); err != nil {
		return err
	}
	data, err := json.Marshal(repos)
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(a.cfg.Home, "last-find.json"), data, 0o644)
}
