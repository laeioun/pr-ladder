package config

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type Config struct {
	Home        string
	GitHubToken string
	LLMURL      string
	LLMKey      string
	Model       string
}

const (
	openRouterURL = "https://openrouter.ai/api/v1"
	ollamaURL     = "http://localhost:11434/v1"
)

func Load() Config {
	home := os.Getenv("PR_LADDER_HOME")
	if home == "" {
		dir, _ := os.UserHomeDir()
		home = filepath.Join(dir, ".pr-ladder")
	}
	cfg := Config{
		Home:        home,
		GitHubToken: firstEnv("GITHUB_TOKEN", "GH_TOKEN"),
		LLMKey:      os.Getenv("PR_LADDER_LLM_KEY"),
		Model:       os.Getenv("PR_LADDER_MODEL"),
	}

	provider := strings.TrimSpace(os.Getenv("PR_LADDER_LLM"))
	if provider == "" && os.Getenv("OPENROUTER_API_KEY") != "" {
		provider = "openrouter"
	}
	switch strings.ToLower(provider) {
	case "":
	case "openrouter":
		cfg.LLMURL = openRouterURL
		cfg.LLMKey = or(cfg.LLMKey, os.Getenv("OPENROUTER_API_KEY"))
		cfg.Model = or(cfg.Model, "openrouter/auto")
	case "ollama":
		cfg.LLMURL = ollamaURL
		cfg.Model = or(cfg.Model, "llama3.2")
	default:
		cfg.LLMURL = provider
	}
	return cfg
}

func (c Config) HasLLM() bool {
	return c.LLMURL != "" && c.Model != ""
}

func (c Config) GitHubTokenOrGH() string {
	if c.GitHubToken != "" {
		return c.GitHubToken
	}
	out, err := exec.Command("gh", "auth", "token").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func firstEnv(keys ...string) string {
	for _, k := range keys {
		if v := os.Getenv(k); v != "" {
			return v
		}
	}
	return ""
}

func or(a, b string) string {
	if a != "" {
		return a
	}
	return b
}
