package note

import (
	"errors"
	"time"

	"github.com/Cortalo/digitalgarden-backend/internal/markdown"
)

// ErrNotFound is returned by the service/repository layer when no row
// matches.
var ErrNotFound = errors.New("note: not found")

// Note is the business object (BO): the core entity, with no knowledge of
// HTTP or the database. ParsedTree reuses markdown.Node directly rather
// than a duplicate type — Node is already a plain, JSON-serializable data
// shape (see its own doc comment), not something that pulls goldmark or
// parsing logic into this package.
type Note struct {
	ID           int64
	Title        string
	Slug         string
	AuthorUserID int64
	// AuthorName is a snapshot of the author's display name taken at
	// publish time, not a live join against the user's current profile —
	// see db/schema.sql for why.
	AuthorName  string
	RawMarkdown string
	ParsedTree  markdown.Node
	// Excerpt is a short preview computed once at publish time, not
	// derived from ParsedTree on every read.
	Excerpt     string
	Tags        []string
	PublishedAt time.Time
}
