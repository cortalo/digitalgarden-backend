// Package notehandler is the HTTP adapter for the note domain. It knows
// about JSON and gin; it only depends on the Service interface defined
// here, never on the concrete service/postgres implementations.
package notehandler

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/Cortalo/digitalgarden-backend/internal/domain/note"
	authhandler "github.com/Cortalo/digitalgarden-backend/internal/handler/auth"
	"github.com/Cortalo/digitalgarden-backend/internal/markdown"
)

// Service is defined here, by the consumer, so the concrete
// *noteservice.Service satisfies it implicitly.
type Service interface {
	Get(ctx context.Context, slug string) (note.Note, error)
	List(ctx context.Context, limit int32) ([]note.Note, error)
	Publish(ctx context.Context, authorUserID int64, title, markdownSource, slug, excerpt string, tags []string) (note.Note, error)
	Update(ctx context.Context, authorUserID int64, currentSlug, title, markdownSource, slugOverride, excerptOverride string, tags []string) (note.Note, error)
	Delete(ctx context.Context, authorUserID int64, slug string) error
	Search(ctx context.Context, keyword string, limit int32) ([]note.SearchHit, error)
}

type Handler struct {
	svc Service
}

func NewHandler(svc Service) *Handler {
	return &Handler{svc: svc}
}

// noteResponse is the full single-note payload: metadata plus the parsed
// tree. This is the one JSON shape in the whole system that includes raw
// markdown-derived content — see CLAUDE.md's "API surface the frontend
// needs": the frontend renders Tree by walking node.type, never by parsing
// markdown itself.
type noteResponse struct {
	ID           int64         `json:"id"`
	Title        string        `json:"title"`
	Slug         string        `json:"slug"`
	Author       string        `json:"author"`
	AuthorUserID int64         `json:"author_user_id"`
	Excerpt      string        `json:"excerpt"`
	Tags         []string      `json:"tags"`
	PublishedAt  time.Time     `json:"published_at"`
	Tree         markdown.Node `json:"tree"`
}

// summaryResponse is what the feed lists: enough to render a card and link
// to the note, without shipping every note's full parsed tree over the
// wire on every feed load.
type summaryResponse struct {
	ID          int64     `json:"id"`
	Title       string    `json:"title"`
	Slug        string    `json:"slug"`
	Author      string    `json:"author"`
	Excerpt     string    `json:"excerpt"`
	Tags        []string  `json:"tags"`
	PublishedAt time.Time `json:"published_at"`
}

func toNoteResponse(n note.Note) noteResponse {
	return noteResponse{
		ID:           n.ID,
		Title:        n.Title,
		Slug:         n.Slug,
		Author:       n.AuthorName,
		AuthorUserID: n.AuthorUserID,
		Excerpt:      n.Excerpt,
		Tags:         n.Tags,
		PublishedAt:  n.PublishedAt,
		Tree:         n.ParsedTree,
	}
}

func toSummaryResponse(n note.Note) summaryResponse {
	return summaryResponse{
		ID:          n.ID,
		Title:       n.Title,
		Slug:        n.Slug,
		Author:      n.AuthorName,
		Excerpt:     n.Excerpt,
		Tags:        n.Tags,
		PublishedAt: n.PublishedAt,
	}
}

// searchHitResponse is a search result row: the same card-rendering
// fields as summaryResponse, plus the snippets showing where the keyword
// actually matched (title, author name, excerpt, and/or body — see
// noteservice.Search).
type searchHitResponse struct {
	ID          int64     `json:"id"`
	Title       string    `json:"title"`
	Slug        string    `json:"slug"`
	Author      string    `json:"author"`
	Excerpt     string    `json:"excerpt"`
	Tags        []string  `json:"tags"`
	PublishedAt time.Time `json:"published_at"`
	Snippets    []string  `json:"snippets"`
}

func toSearchHitResponse(hit note.SearchHit) searchHitResponse {
	return searchHitResponse{
		ID:          hit.Note.ID,
		Title:       hit.Note.Title,
		Slug:        hit.Note.Slug,
		Author:      hit.Note.AuthorName,
		Excerpt:     hit.Note.Excerpt,
		Tags:        hit.Note.Tags,
		PublishedAt: hit.Note.PublishedAt,
		Snippets:    hit.Snippets,
	}
}

// Get handles GET /api/notes/:slug.
func (h *Handler) Get(c *gin.Context) {
	n, err := h.svc.Get(c.Request.Context(), c.Param("slug"))
	if err != nil {
		respondError(c, err)
		return
	}

	c.JSON(http.StatusOK, toNoteResponse(n))
}

// Download handles GET /api/notes/:slug/download, serving the note's
// original markdown source as a file rather than JSON. Content-
// Disposition: attachment makes a plain <a href> link on the frontend
// trigger a browser download with no client-side JS needed. Public, same
// access level as Get — the note is already published/public content, so
// downloading its source isn't a bigger exposure than reading it.
func (h *Handler) Download(c *gin.Context) {
	n, err := h.svc.Get(c.Request.Context(), c.Param("slug"))
	if err != nil {
		respondError(c, err)
		return
	}

	// n.Slug only ever contains [a-z0-9-] (see note.Slugify), so it's
	// always safe to interpolate directly into this header.
	c.Header("Content-Disposition", `attachment; filename="`+n.Slug+`.md"`)
	c.Data(http.StatusOK, "text/markdown; charset=utf-8", []byte(n.RawMarkdown))
}

type publishRequest struct {
	Title    string   `json:"title" binding:"required"`
	Markdown string   `json:"markdown" binding:"required"`
	Slug     string   `json:"slug"`
	Excerpt  string   `json:"excerpt"`
	Tags     []string `json:"tags"`
}

// Publish handles POST /api/notes, behind authhandler.RequireAuth. v1
// scope is text-only — no attachment upload yet (see CLAUDE.md's Upload
// scope and the note service's own doc comment).
func (h *Handler) Publish(c *gin.Context) {
	userID, ok := authhandler.UserID(c)
	if !ok {
		// Defense in depth — this route should always sit behind
		// RequireAuth, which would already have aborted the request.
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	var req publishRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	n, err := h.svc.Publish(c.Request.Context(), userID, req.Title, req.Markdown, req.Slug, req.Excerpt, req.Tags)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, toNoteResponse(n))
}

// Update handles PUT /api/notes/:slug, behind authhandler.RequireAuth.
// Same body shape as Publish (a full replace, not a partial patch) — see
// the note service's own doc comment on Update for the slug/excerpt
// override semantics.
func (h *Handler) Update(c *gin.Context) {
	userID, ok := authhandler.UserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	var req publishRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	n, err := h.svc.Update(c.Request.Context(), userID, c.Param("slug"), req.Title, req.Markdown, req.Slug, req.Excerpt, req.Tags)
	if err != nil {
		respondError(c, err)
		return
	}

	c.JSON(http.StatusOK, toNoteResponse(n))
}

// Delete handles DELETE /api/notes/:slug, behind authhandler.RequireAuth.
// No soft-delete — the row is gone outright (see CLAUDE.md's scope).
func (h *Handler) Delete(c *gin.Context) {
	userID, ok := authhandler.UserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	if err := h.svc.Delete(c.Request.Context(), userID, c.Param("slug")); err != nil {
		respondError(c, err)
		return
	}

	c.Status(http.StatusNoContent)
}

var errInvalidLimit = errors.New("limit must be a positive integer")

// List handles GET /api/notes. limit is an optional query param; the
// service falls back to its own default, and the repository enforces a
// hard ceiling regardless of what's requested.
func (h *Handler) List(c *gin.Context) {
	limit, err := parseLimit(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	notes, err := h.svc.List(c.Request.Context(), limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	result := make([]summaryResponse, len(notes))
	for i, n := range notes {
		result[i] = toSummaryResponse(n)
	}

	c.JSON(http.StatusOK, result)
}

// Search handles GET /api/notes/search?q=. Public, same access level as
// List — search only surfaces already-published content.
func (h *Handler) Search(c *gin.Context) {
	keyword := c.Query("q")
	if keyword == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "q is required"})
		return
	}

	limit, err := parseLimit(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	hits, err := h.svc.Search(c.Request.Context(), keyword, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	result := make([]searchHitResponse, len(hits))
	for i, hit := range hits {
		result[i] = toSearchHitResponse(hit)
	}

	c.JSON(http.StatusOK, result)
}

// parseLimit reads the optional ?limit= query param. Absent means "use the
// service default"; present-but-invalid is a client error.
func parseLimit(c *gin.Context) (int32, error) {
	raw := c.Query("limit")
	if raw == "" {
		return 0, nil
	}

	limit, err := strconv.ParseInt(raw, 10, 32)
	if err != nil {
		return 0, errInvalidLimit
	}

	return int32(limit), nil
}

func respondError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, note.ErrNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
	case errors.Is(err, note.ErrForbidden):
		c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
	case errors.Is(err, note.ErrSlugTaken):
		c.JSON(http.StatusConflict, gin.H{"error": "slug already taken"})
	default:
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
	}
}
