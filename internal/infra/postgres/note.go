package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/Cortalo/digitalgarden-backend/internal/domain/note"
	"github.com/Cortalo/digitalgarden-backend/internal/markdown"
)

// pgUniqueViolation is the standard SQLSTATE code for a unique constraint
// violation — not Postgres-specific, part of the SQL standard.
const pgUniqueViolation = "23505"

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

// CreateNote inserts a new digitalgarden_note row. published_at and
// note_id are left to their column defaults (now() / identity) rather
// than set by the caller. A slug collision comes back as
// note.ErrSlugTaken so the service can retry with a suffixed candidate.
func (s *Store) CreateNote(ctx context.Context, n note.Note) (note.Note, error) {
	tree, err := json.Marshal(n.ParsedTree)
	if err != nil {
		return note.Note{}, fmt.Errorf("marshal parsed_tree: %w", err)
	}

	rows, err := s.pool.Query(ctx,
		"insert into digitalgarden.digitalgarden_note "+
			"(title, slug, author_user_id, author_name, raw_markdown, parsed_tree, excerpt, tags) "+
			"values ($1, $2, $3, $4, $5, $6, $7, $8) returning "+noteColumns,
		// tree is passed as string, not []byte: under the simple query
		// protocol (required for Supabase's transaction-mode pooler, see
		// New()), pgx encodes a []byte parameter as a bytea literal, which
		// Postgres then refuses to accept as jsonb ("invalid input syntax
		// for type json") — a string parameter round-trips as text, which
		// Postgres can parse as JSON directly.
		n.Title, n.Slug, n.AuthorUserID, n.AuthorName, n.RawMarkdown, string(tree), n.Excerpt, n.Tags,
	)
	if err != nil {
		return note.Note{}, wrapNoteWriteErr(err)
	}

	row, err := pgx.CollectOneRow(rows, pgx.RowToStructByName[noteRow])
	if err != nil {
		return note.Note{}, wrapNoteWriteErr(err)
	}

	return row.toDomain()
}

// wrapNoteWriteErr maps a slug unique-constraint violation to
// note.ErrSlugTaken and wraps anything else as-is. Shared by CreateNote
// and UpdateNote — both write slug, both hit the same constraint.
// Depending on how the driver executes the query, the violation can
// surface from either the initial Query call or from reading its result
// rows — both call sites need this same check, hence the shared helper (a
// first version only checked one of the two, and the collision path was
// silently never taken).
func wrapNoteWriteErr(err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == pgUniqueViolation {
		return note.ErrSlugTaken
	}
	return fmt.Errorf("write note: %w", err)
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

// UpdateNote overwrites an existing digitalgarden_note row in place — no
// version history, matching the domain's "just update the row" scope.
// n.ID identifies which row; published_at is left untouched (only the
// column default sets it, at insert time). A slug collision comes back as
// note.ErrSlugTaken — unlike CreateNote, the service does not retry this
// with a suffix, since an update's slug is an explicit caller choice.
func (s *Store) UpdateNote(ctx context.Context, n note.Note) (note.Note, error) {
	tree, err := json.Marshal(n.ParsedTree)
	if err != nil {
		return note.Note{}, fmt.Errorf("marshal parsed_tree: %w", err)
	}

	rows, err := s.pool.Query(ctx,
		"update digitalgarden.digitalgarden_note set "+
			"title = $1, slug = $2, raw_markdown = $3, parsed_tree = $4, excerpt = $5, tags = $6 "+
			"where note_id = $7 returning "+noteColumns,
		// tree as string, not []byte — see CreateNote's comment on the
		// same encoding quirk.
		n.Title, n.Slug, n.RawMarkdown, string(tree), n.Excerpt, n.Tags, n.ID,
	)
	if err != nil {
		return note.Note{}, wrapNoteWriteErr(err)
	}

	row, err := pgx.CollectOneRow(rows, pgx.RowToStructByName[noteRow])
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return note.Note{}, note.ErrNotFound
		}
		return note.Note{}, wrapNoteWriteErr(err)
	}

	return row.toDomain()
}

// DeleteNote removes a digitalgarden_note row outright — no soft-delete
// flag, matching the domain's scope. Returns note.ErrNotFound if no row
// matched, though the service will already have confirmed the note exists
// via GetNoteBySlug before calling this (defense in depth against a race,
// not the primary existence check).
func (s *Store) DeleteNote(ctx context.Context, id int64) error {
	tag, err := s.pool.Exec(ctx,
		"delete from digitalgarden.digitalgarden_note where note_id = $1",
		id,
	)
	if err != nil {
		return fmt.Errorf("delete note: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return note.ErrNotFound
	}
	return nil
}
