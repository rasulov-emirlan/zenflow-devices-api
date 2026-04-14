package templates

import "context"

type Service struct {
	repo Repo
}

func NewService(repo Repo) *Service { return &Service{repo: repo} }

func (s *Service) Get(ctx context.Context, slug string) (Template, error) {
	return s.repo.Get(ctx, slug)
}

func (s *Service) List(ctx context.Context) ([]Template, error) {
	return s.repo.List(ctx)
}
