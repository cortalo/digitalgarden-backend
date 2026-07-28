package note

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"

	"github.com/Cortalo/digitalgarden-backend/internal/domain/note"
	"github.com/Cortalo/digitalgarden-backend/internal/markdown"
	userservice "github.com/Cortalo/digitalgarden-backend/internal/service/user"
)

// Repository is the port this service needs from persistence. infra/
// postgres.Store satisfies it implicitly — nothing there imports this
// package.
type Repository interface {
	GetNoteBySlug(ctx context.Context, slug string) (note.Note, error)
	ListNotes(ctx context.Context, limit int32) ([]note.Note, error)
	CreateNote(ctx context.Context, n note.Note) (note.Note, error)
	UpdateNote(ctx context.Context, n note.Note) (note.Note, error)
	DeleteNote(ctx context.Context, id int64) error
}

// AuthorFinder resolves the display name to snapshot onto a note at
// publish time — see note.Note.AuthorName. *userservice.Service satisfies
// this implicitly.
type AuthorFinder interface {
	Get(ctx context.Context, id int64) (userservice.User, error)
}

// SearchIndex is a second, independent port — a keyword-searchable index
// that mirrors a subset of what's in Postgres (title, author name,
// excerpt, raw content). Postgres remains the sole source of truth
// (see Repository); this index only exists to answer "find by keyword"
// queries. Kept in sync from Publish (see IndexNote's call site below),
// not treated as authoritative — a failed IndexNote never fails a
// publish.
type SearchIndex interface {
	IndexNote(ctx context.Context, n note.Note) error
	DeleteNote(ctx context.Context, id int64) error
	Search(ctx context.Context, keyword string, limit int32) ([]note.SearchHit, error)
}

type Service struct {
	repo    Repository
	authors AuthorFinder
	search  SearchIndex
}

func NewService(repo Repository, authors AuthorFinder, search SearchIndex) *Service {
	return &Service{repo: repo, authors: authors, search: search}
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

// maxSlugAttempts bounds how many suffixed candidates Publish tries
// before giving up — a handful of published notes sharing the same title
// is expected; hundreds sharing one is almost certainly a caller bug, not
// a case worth looping over indefinitely for.
const maxSlugAttempts = 20

// Publish parses markdownSource into a tree, snapshots the author's
// current display name, and stores the result. v1 scope is text-only —
// no attachment handling (see CLAUDE.md's Upload scope).
//
// slugOverride and excerptOverride are the caller's (optional) explicit
// choices — an empty string means "derive it" for each independently:
// slug falls back to Slugify(title), excerpt to ExcerptFrom(tree). A
// non-empty slugOverride is still re-run through Slugify (never trust
// client input to already be URL-safe) and still goes through the same
// collision-retry loop as a derived slug — a user-chosen slug that
// collides gets the same "-2" suffix treatment, not a hard error.
func (s *Service) Publish(ctx context.Context, authorUserID int64, title, markdownSource, slugOverride, excerptOverride string, tags []string) (note.Note, error) {
	author, err := s.authors.Get(ctx, authorUserID)
	if err != nil {
		return note.Note{}, fmt.Errorf("get author: %w", err)
	}

	tree := markdown.Parse([]byte(markdownSource))
	if tags == nil {
		tags = []string{}
	}

	excerpt := strings.TrimSpace(excerptOverride)
	if excerpt == "" {
		excerpt = note.ExcerptFrom(tree)
	}

	n := note.Note{
		Title:        title,
		AuthorUserID: authorUserID,
		AuthorName:   author.Name,
		RawMarkdown:  markdownSource,
		ParsedTree:   tree,
		Excerpt:      excerpt,
		Tags:         tags,
	}

	base := note.Slugify(title)
	if override := strings.TrimSpace(slugOverride); override != "" {
		base = note.Slugify(override)
	}
	for attempt := 0; attempt < maxSlugAttempts; attempt++ {
		n.Slug = base
		if attempt > 0 {
			n.Slug = fmt.Sprintf("%s-%d", base, attempt+1)
		}

		created, err := s.repo.CreateNote(ctx, n)
		if err == nil {
			// Best-effort: Postgres is already the source of truth for
			// this note, so a search-index hiccup must not fail a
			// publish that otherwise succeeded — the index is a
			// derived, rebuildable cache (see CLAUDE.md).
			if err := s.search.IndexNote(ctx, created); err != nil {
				log.Printf("note: index note %d: %v", created.ID, err)
			}
			return created, nil
		}
		if !errors.Is(err, note.ErrSlugTaken) {
			return note.Note{}, err
		}
	}

	return note.Note{}, fmt.Errorf("publish: no unique slug found for %q after %d attempts", title, maxSlugAttempts)
}

// Update overwrites an existing note in place — no version history (see
// CLAUDE.md's scope for this feature). currentSlug identifies which note
// to edit; only its author (authorUserID) may do so, checked via
// IsOwnedBy before any write.
//
// Unlike Publish, slugOverride only changes the slug if explicitly
// non-empty — omitting it keeps the existing slug, so editing a note's
// title never silently breaks a link to it. A non-empty slugOverride is
// still re-run through Slugify, but unlike Publish, a collision is
// surfaced immediately as note.ErrSlugTaken rather than retried with a
// suffix: this slug is an explicit caller choice, not an auto-derived
// one, so silently renaming it out from under the caller would be
// surprising.
func (s *Service) Update(ctx context.Context, authorUserID int64, currentSlug, title, markdownSource, slugOverride, excerptOverride string, tags []string) (note.Note, error) {
	existing, err := s.repo.GetNoteBySlug(ctx, currentSlug)
	if err != nil {
		return note.Note{}, err
	}
	if !existing.IsOwnedBy(authorUserID) {
		return note.Note{}, note.ErrForbidden
	}

	tree := markdown.Parse([]byte(markdownSource))
	if tags == nil {
		tags = []string{}
	}

	excerpt := strings.TrimSpace(excerptOverride)
	if excerpt == "" {
		excerpt = note.ExcerptFrom(tree)
	}

	slug := existing.Slug
	if override := strings.TrimSpace(slugOverride); override != "" {
		slug = note.Slugify(override)
	}

	n := note.Note{
		ID:           existing.ID,
		Title:        title,
		Slug:         slug,
		AuthorUserID: existing.AuthorUserID,
		AuthorName:   existing.AuthorName,
		RawMarkdown:  markdownSource,
		ParsedTree:   tree,
		Excerpt:      excerpt,
		Tags:         tags,
		PublishedAt:  existing.PublishedAt,
	}

	updated, err := s.repo.UpdateNote(ctx, n)
	if err != nil {
		return note.Note{}, err
	}

	// Best-effort, same rationale as Publish's IndexNote call.
	if err := s.search.IndexNote(ctx, updated); err != nil {
		log.Printf("note: index note %d: %v", updated.ID, err)
	}

	return updated, nil
}

// Delete removes a note outright — no soft-delete flag (see CLAUDE.md's
// scope). Only the note's author may delete it, checked the same way as
// Update.
func (s *Service) Delete(ctx context.Context, authorUserID int64, slug string) error {
	existing, err := s.repo.GetNoteBySlug(ctx, slug)
	if err != nil {
		return err
	}
	if !existing.IsOwnedBy(authorUserID) {
		return note.ErrForbidden
	}

	if err := s.repo.DeleteNote(ctx, existing.ID); err != nil {
		return err
	}

	// Best-effort, same rationale as Publish's IndexNote call.
	if err := s.search.DeleteNote(ctx, existing.ID); err != nil {
		log.Printf("note: delete note %d from index: %v", existing.ID, err)
	}

	return nil
}

// defaultSearchLimit is how many search hits are returned when the caller
// doesn't ask for a specific amount — mirrors List's defaultFeedLimit.
const defaultSearchLimit = 20

// Search returns up to limit notes matching keyword, most relevant first,
// each with the snippets showing where it matched.
func (s *Service) Search(ctx context.Context, keyword string, limit int32) ([]note.SearchHit, error) {
	if limit <= 0 {
		limit = defaultSearchLimit
	}
	return s.search.Search(ctx, keyword, limit)
}
