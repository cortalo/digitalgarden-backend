package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/Cortalo/digitalgarden-backend/internal/domain/note"
	"github.com/Cortalo/digitalgarden-backend/internal/markdown"
)

// noteRow is the persistence object (PO): the exact shape of a
// digitalgarden_note row. Only this file knows about it; everything else
// deals in note.Note. ParsedTree is read as raw jsonb bytes and unmarshaled
// in toDomain — pgx has no way to scan straight into markdown.Node.
type noteRow struct {
	ID           int64     `db:"note_id"`
	Title        string    `db:"title"`
	Slug         string    `db:"slug"`
	AuthorUserID int64     `db:"author_user_id"`
	AuthorName   string    `db:"author_name"`
	RawMarkdown  string    `db:"raw_markdown"`
	ParsedTree   []byte    `db:"parsed_tree"`
	Excerpt      string    `db:"excerpt"`
	Tags         []string  `db:"tags"`
	PublishedAt  time.Time `db:"published_at"`
}

func (r noteRow) toDomain() (note.Note, error) {
	var tree markdown.Node
	if err := json.Unmarshal(r.ParsedTree, &tree); err != nil {
		return note.Note{}, fmt.Errorf("unmarshal parsed_tree: %w", err)
	}

	return note.Note{
		ID:           r.ID,
		Title:        r.Title,
		Slug:         r.Slug,
		AuthorUserID: r.AuthorUserID,
		AuthorName:   r.AuthorName,
		RawMarkdown:  r.RawMarkdown,
		ParsedTree:   tree,
		Excerpt:      r.Excerpt,
		Tags:         r.Tags,
		PublishedAt:  r.PublishedAt,
	}, nil
}

const noteColumns = "note_id, title, slug, author_user_id, author_name, raw_markdown, parsed_tree, excerpt, tags, published_at"

// GetNoteBySlug returns a single digitalgarden_note row by its slug.
func (s *Store) GetNoteBySlug(ctx context.Context, slug string) (note.Note, error) {
	rows, err := s.pool.Query(ctx,
		"select "+noteColumns+" from digitalgarden.digitalgarden_note where slug = $1",
		slug,
	)
	if err != nil {
		return note.Note{}, fmt.Errorf("get note: %w", err)
	}

	row, err := pgx.CollectOneRow(rows, pgx.RowToStructByName[noteRow])
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return note.Note{}, note.ErrNotFound
		}
		return note.Note{}, fmt.Errorf("get note: %w", err)
	}

	return row.toDomain()
}

// maxNoteListLimit is a hard ceiling on ListNotes, independent of what the
// caller passes, so a bad request can't force an unbounded table scan.
const maxNoteListLimit = 100

// ListNotes returns up to limit digitalgarden_note rows, newest first —
// the shape the public feed reads. limit must be positive and is clamped
// to maxNoteListLimit.
func (s *Store) ListNotes(ctx context.Context, limit int32) ([]note.Note, error) {
	if limit <= 0 {
		return nil, fmt.Errorf("list notes: limit must be positive")
	}
	if limit > maxNoteListLimit {
		limit = maxNoteListLimit
	}

	rows, err := s.pool.Query(ctx,
		"select "+noteColumns+" from digitalgarden.digitalgarden_note order by published_at desc limit $1",
		limit,
	)
	if err != nil {
		return nil, fmt.Errorf("list notes: %w", err)
	}

	found, err := pgx.CollectRows(rows, pgx.RowToStructByName[noteRow])
	if err != nil {
		return nil, fmt.Errorf("list notes: %w", err)
	}

	result := make([]note.Note, len(found))
	for i, row := range found {
		n, err := row.toDomain()
		if err != nil {
			return nil, fmt.Errorf("list notes: %w", err)
		}
		result[i] = n
	}
	return result, nil
}
