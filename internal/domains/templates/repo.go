package templates

import "context"

type Repo interface {
	Get(ctx context.Context, slug string) (Template, error)
	List(ctx context.Context) ([]Template, error)
}
