package maintenance

import (
	"context"
	"fmt"
	"strings"
	"time"
)

const (
	Label = "maintenance"
	completedRetention = 24 * time.Hour
)

type Repos struct {
	Frontend string
	Backend  string
	Infra    string
}

func (r Repos) slug(t Target) string {
	switch t {
	case TargetBackend:
		return r.Backend
	case TargetInfra:
		return r.Infra
	default:
		return r.Frontend
	}
}

func (r Repos) all() []string {
	return []string{r.Frontend, r.Backend, r.Infra}
}

type GitHub interface {
	CreateIssue(ctx context.Context, repo, title, body string, labels []string) (*Issue, error)
	ListIssues(ctx context.Context, repos []string, label string, perRepo int) ([]Issue, error)
}

type Service struct {
	gh    GitHub
	repos Repos
}

func NewService(gh GitHub, repos Repos) *Service {
	return &Service{gh: gh, repos: repos}
}

type CreateResult struct {
	Issue *Issue `json:"issue"`
	Repo  string `json:"repo"`
	Area  Target `json:"area"`
}

func (s *Service) Create(ctx context.Context, title, description string) (*CreateResult, error) {
	title = strings.TrimSpace(title)
	description = strings.TrimSpace(description)
	if title == "" {
		return nil, fmt.Errorf("title is required")
	}

	area := Classify(title, description)
	repo := s.repos.slug(area)

	body := fmt.Sprintf(
		"%s\n\n---\n_Filed from the ISF admin maintenance form. Auto-routed to **%s** (`%s`)._",
		description, area, repo,
	)

	issue, err := s.gh.CreateIssue(ctx, repo, title, body, []string{Label})
	if err != nil {
		return nil, err
	}
	return &CreateResult{Issue: issue, Repo: repo, Area: area}, nil
}

func (s *Service) List(ctx context.Context, perRepo int) ([]Issue, error) {
	if perRepo <= 0 || perRepo > 50 {
		perRepo = 20
	}
	issues, err := s.gh.ListIssues(ctx, s.repos.all(), Label, perRepo)
	if err != nil {
		return nil, err
	}
	cutoff := time.Now().Add(-completedRetention)
	out := make([]Issue, 0, len(issues))
	for _, it := range issues {
		if it.State == "closed" && it.ClosedAt != nil && it.ClosedAt.Before(cutoff) {
			continue // completed more than a day ago → drop it
		}
		out = append(out, it)
	}
	return out, nil
}
