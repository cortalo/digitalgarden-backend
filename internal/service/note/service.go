package note

import (
	"context"

	"github.com/Cortalo/digitalgarden-backend/internal/domain/note"
)

// Repository is the port this service needs from persistence. infra/
// postgres.Store satisfies it implicitly — nothing there imports this
// package.
type Repository interface {
	GetNoteBySlug(ctx context.Context, slug string) (note.Note, error)
	ListNotes(ctx context.Context, limit int32) ([]note.Note, error)
}

type Service struct {
	repo Repository
}

func NewService(repo Repository) *Service {
	return &Service{repo: repo}
}

// Get returns a single published note by slug.
func (s *Service) Get(ctx context.Context, slug string) (note.Note, error) {
	return s.repo.GetNoteBySlug(ctx, slug)
}

// defaultFeedLimit is how many notes the public feed returns when the
// caller doesn't ask for a specific amount.
const defaultFeedLimit = 20

// List returns the most recently published notes for the public feed. A
// non-positive limit falls back to defaultFeedLimit rather than being
// treated as an error — unlike the repository's own limit check, "no
// preference" is a normal, expected caller state here (e.g. an
// unparsed/absent query param), not a bug.
func (s *Service) List(ctx context.Context, limit int32) ([]note.Note, error) {
	if limit <= 0 {
		limit = defaultFeedLimit
	}
	return s.repo.ListNotes(ctx, limit)
}
