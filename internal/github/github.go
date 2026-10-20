package github

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"time"
)

type Client struct {
	token string
	http  *http.Client
}

type Repo struct {
	FullName    string    `json:"full_name"`
	Description string    `json:"description"`
	Language    string    `json:"language"`
	Stars       int       `json:"stargazers_count"`
	Forks       int       `json:"forks_count"`
	OpenIssues  int       `json:"open_issues_count"`
	PushedAt    time.Time `json:"pushed_at"`
	Fork        bool      `json:"fork"`
	License     *struct {
		SPDX string `json:"spdx_id"`
	} `json:"license"`
}

func (r Repo) LicenseID() string {
	if r.License == nil || r.License.SPDX == "" || r.License.SPDX == "NOASSERTION" {
		return "none"
	}
	return r.License.SPDX
}

type PRStats struct {
	Sampled           int
	Merged            int
	External          int
	ExternalMerged    int
	MedianTimeToMerge time.Duration
}

func New(token string) *Client {
	return &Client{token: token, http: &http.Client{Timeout: 30 * time.Second}}
}

func (c *Client) Search(ctx context.Context, query string, limit int) ([]Repo, error) {
	if !strings.Contains(query, "archived:") {
		query += " archived:false"
	}
	q := url.Values{"q": {query}, "per_page": {fmt.Sprint(limit)}}
	var res struct {
		Items []Repo `json:"items"`
	}
	if err := c.get(ctx, "/search/repositories?"+q.Encode(), &res); err != nil {
		return nil, err
	}
	return res.Items, nil
}

func (c *Client) Repo(ctx context.Context, fullName string) (Repo, error) {
	var r Repo
	err := c.get(ctx, "/repos/"+fullName, &r)
	return r, err
}

func (c *Client) PRStats(ctx context.Context, fullName string) (PRStats, error) {
	var prs []struct {
		CreatedAt   time.Time  `json:"created_at"`
		MergedAt    *time.Time `json:"merged_at"`
		Association string     `json:"author_association"`
	}
	if err := c.get(ctx, "/repos/"+fullName+"/pulls?state=closed&sort=updated&direction=desc&per_page=50", &prs); err != nil {
		return PRStats{}, err
	}
	var s PRStats
	var durations []time.Duration
	for _, pr := range prs {
		s.Sampled++
		external := !slices.Contains([]string{"OWNER", "MEMBER", "COLLABORATOR"}, pr.Association)
		if external {
			s.External++
		}
		if pr.MergedAt == nil {
			continue
		}
		s.Merged++
		if external {
			s.ExternalMerged++
		}
		durations = append(durations, pr.MergedAt.Sub(pr.CreatedAt))
	}
	if len(durations) > 0 {
		slices.Sort(durations)
		s.MedianTimeToMerge = durations[len(durations)/2]
	}
	return s, nil
}

func (c *Client) get(ctx context.Context, path string, v any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://api.github.com"+path, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	req.Header.Set("User-Agent", "pr-ladder")
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	res, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		var body struct {
			Message string `json:"message"`
		}
		_ = json.NewDecoder(res.Body).Decode(&body)
		if res.StatusCode == http.StatusForbidden || res.StatusCode == http.StatusTooManyRequests {
			return fmt.Errorf("github: %s (rate limited? set GITHUB_TOKEN or run `gh auth login`)", res.Status)
		}
		return fmt.Errorf("github: %s %s", res.Status, body.Message)
	}
	return json.NewDecoder(res.Body).Decode(v)
}
