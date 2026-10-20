package ladder

import (
	"fmt"
	"strings"

	"github.com/laeioun/pr-ladder/internal/github"
)

func FormatRepo(i int, r github.Repo) string {
	lang := r.Language
	if lang == "" {
		lang = "-"
	}
	desc := strings.Join(strings.Fields(r.Description), " ")
	if r := []rune(desc); len(r) > 90 {
		desc = string(r[:87]) + "..."
	}
	return fmt.Sprintf("%2d  %-40s %6s  %-10s %-12s %s", i, r.FullName, "★"+stars(r.Stars), lang, ago(r.PushedAt), desc)
}

func stars(n int) string {
	if n >= 1000 {
		return fmt.Sprintf("%.1fk", float64(n)/1000)
	}
	return fmt.Sprint(n)
}
