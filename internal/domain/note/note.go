package note

import (
	"errors"
	"regexp"
	"strings"
	"time"

	"github.com/Cortalo/digitalgarden-backend/internal/markdown"
)

// ErrNotFound is returned by the service/repository layer when no row
// matches.
var ErrNotFound = errors.New("note: not found")

// ErrSlugTaken is returned by the repository when an insert collides with
// an existing slug's unique constraint, so the service can retry with a
// suffixed candidate rather than failing the publish outright.
var ErrSlugTaken = errors.New("note: slug already taken")

// ErrForbidden means the note exists but the caller isn't its author —
// returned by Update/Delete when the authenticated user doesn't match
// AuthorUserID.
var ErrForbidden = errors.New("note: forbidden")

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

// SearchHit is one search result: a note (partially populated — a
// search-derived Note carries no ParsedTree, since the index only stores
// what a result card needs, not the full parsed tree structure) plus the
// highlighted snippets showing where the keyword matched, across whichever
// of title/author_name/excerpt/content actually matched.
type SearchHit struct {
	Note     Note
	Snippets []string
}

// IsOwnedBy reports whether userID is this note's author.
func (n Note) IsOwnedBy(userID int64) bool {
	return n.AuthorUserID == userID
}

var slugNonAlphanumeric = regexp.MustCompile(`[^a-z0-9]+`)

// Slugify turns a title into a URL-safe slug: lowercase, non-alphanumeric
// runs collapsed to a single hyphen, leading/trailing hyphens trimmed.
// It's a pure function of the title alone — uniqueness against existing
// slugs is the repository's job (see ErrSlugTaken).
func Slugify(title string) string {
	return strings.Trim(slugNonAlphanumeric.ReplaceAllString(strings.ToLower(title), "-"), "-")
}

const maxExcerptRunes = 200

// ExcerptFrom derives a short preview from a parsed tree: the text of its
// first paragraph, truncated. Returns "" if the tree has no paragraph
// (e.g. a note that's just a heading and a diagram).
func ExcerptFrom(tree markdown.Node) string {
	for _, child := range tree.Children {
		if child.Type != "paragraph" {
			continue
		}

		var sb strings.Builder
		collectText(child, &sb)
		return truncateRunes(sb.String(), maxExcerptRunes)
	}
	return ""
}

func collectText(n markdown.Node, sb *strings.Builder) {
	if n.Text != "" {
		sb.WriteString(n.Text)
	}
	for _, child := range n.Children {
		collectText(child, sb)
	}
}

func truncateRunes(s string, max int) string {
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	return string(r[:max]) + "…"
}
