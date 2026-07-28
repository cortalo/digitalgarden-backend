package elasticsearch

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/Cortalo/digitalgarden-backend/internal/domain/note"
)

// noteDoc is what actually gets indexed. Content holds the raw markdown —
// the simplest v1 content field, no separate plain-text extraction from
// ParsedTree — and Content is deliberately the only field not returned
// directly to a caller of Search (see search.go's use of highlight
// fragments instead of the raw field). ParsedTree itself is never
// indexed: it's a rendering structure, not something search relevance or
// highlighting should ever match against.
type noteDoc struct {
	NoteID      int64     `json:"note_id"`
	Title       string    `json:"title"`
	Slug        string    `json:"slug"`
	AuthorName  string    `json:"author_name"`
	Excerpt     string    `json:"excerpt"`
	Content     string    `json:"content"`
	Tags        []string  `json:"tags"`
	PublishedAt time.Time `json:"published_at"`
}

// IndexNote upserts n into the search index, keyed by its ID so
// re-indexing the same note overwrites rather than duplicates. Called
// from noteservice.Service.Publish right after a successful Postgres
// insert — best-effort: a failure here is logged by the caller and must
// never fail the publish itself, since Postgres is the source of truth
// and this index is a derived, rebuildable cache (see CLAUDE.md).
func (c *Client) IndexNote(ctx context.Context, n note.Note) error {
	doc := noteDoc{
		NoteID:      n.ID,
		Title:       n.Title,
		Slug:        n.Slug,
		AuthorName:  n.AuthorName,
		Excerpt:     n.Excerpt,
		Content:     n.RawMarkdown,
		Tags:        n.Tags,
		PublishedAt: n.PublishedAt,
	}

	body, err := json.Marshal(doc)
	if err != nil {
		return fmt.Errorf("elasticsearch: encode note %d: %w", n.ID, err)
	}

	url := c.baseURL + "/" + noteIndex + "/_doc/" + strconv.FormatInt(n.ID, 10)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("elasticsearch: build index request: %w", err)
	}
	req.SetBasicAuth(c.accessKey, c.accessSecret)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("elasticsearch: index note %d: %w", n.ID, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("elasticsearch: index note %d: status %d: %s", n.ID, resp.StatusCode, respBody)
	}
	return nil
}

// DeleteNote removes a note from the search index, called from
// noteservice.Service.Delete right after a successful Postgres delete —
// best-effort, same as IndexNote: a failure here is logged by the caller
// and must never fail the delete itself. A 404 (the doc was already
// absent — e.g. the original IndexNote call had failed) is treated as
// success, not an error: the end state either way is "not in the index."
func (c *Client) DeleteNote(ctx context.Context, id int64) error {
	url := c.baseURL + "/" + noteIndex + "/_doc/" + strconv.FormatInt(id, 10)
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, url, nil)
	if err != nil {
		return fmt.Errorf("elasticsearch: build delete request: %w", err)
	}
	req.SetBasicAuth(c.accessKey, c.accessSecret)

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("elasticsearch: delete note %d: %w", id, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 && resp.StatusCode != http.StatusNotFound {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("elasticsearch: delete note %d: status %d: %s", id, resp.StatusCode, respBody)
	}
	return nil
}
