package maintenance

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)


type GitHubClient struct {
	token string
	org   string
	http  *http.Client
}

func NewGitHubClient(token, org string) *GitHubClient {
	return &GitHubClient{
		token: token,
		org:   org,
		http:  &http.Client{Timeout: 15 * time.Second},
	}
}

type Issue struct {
	Number  int    `json:"number"`
	HTMLURL string `json:"html_url"`
	Title   string `json:"title"`
	State   string `json:"state"`
}

type createIssueBody struct {
	Title  string   `json:"title"`
	Body   string   `json:"body"`
	Labels []string `json:"labels"`
}


func (g *GitHubClient) CreateIssue(ctx context.Context, repo, title, body string, labels []string) (*Issue, error) {
	payload, _ := json.Marshal(createIssueBody{Title: title, Body: body, Labels: labels})
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/issues", g.org, repo)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+g.token)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	req.Header.Set("Content-Type", "application/json")

	resp, err := g.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("github request: %w", err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusCreated {
		return nil, fmt.Errorf("github create issue failed (%d): %s", resp.StatusCode, string(raw))
	}
	var issue Issue
	if err := json.Unmarshal(raw, &issue); err != nil {
		return nil, fmt.Errorf("decode github issue: %w", err)
	}
	return &issue, nil
}

func (g *GitHubClient) ListIssues(ctx context.Context, repos []string, label string, perRepo int) ([]Issue, error) {
	out := make([]Issue, 0)
	for _, repo := range repos {
		url := fmt.Sprintf(
			"https://api.github.com/repos/%s/%s/issues?state=all&labels=%s&per_page=%d&sort=created&direction=desc",
			g.org, repo, label, perRepo,
		)
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("Authorization", "Bearer "+g.token)
		req.Header.Set("Accept", "application/vnd.github+json")
		req.Header.Set("X-GitHub-Api-Version", "2022-11-28")

		resp, err := g.http.Do(req)
		if err != nil {
			return nil, fmt.Errorf("github list (%s): %w", repo, err)
		}
		raw, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("github list %s failed (%d): %s", repo, resp.StatusCode, string(raw))
		}
		var issues []Issue
		if err := json.Unmarshal(raw, &issues); err != nil {
			return nil, fmt.Errorf("decode github list: %w", err)
		}
		out = append(out, issues...)
	}
	return out, nil
}
