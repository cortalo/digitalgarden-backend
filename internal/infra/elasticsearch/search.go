package elasticsearch

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/Cortalo/digitalgarden-backend/internal/domain/note"
)

// searchFields is where a keyword is matched, in relevance order — title
// counts for the most (a keyword in the title is a stronger signal than
// one buried in the body), excerpt somewhat more than a single body
// occurrence, author_name unweighted (still searchable, but shouldn't
// outrank an actual content match). ParsedTree is intentionally absent:
// see index.go's noteDoc.
var searchFields = []string{"title^3", "author_name", "excerpt^1.5", "content"}

// highlightFieldOrder is the fixed order Snippets are assembled in,
// regardless of the order OpenSearch's highlight map (unordered JSON
// object keys) happens to return them.
var highlightFieldOrder = []string{"title", "author_name", "excerpt", "content"}

// searchRequest is the subset of OpenSearch's _search body this project
// needs: a relevance-ranked multi-field match, plus highlighting so the
// caller can see where each hit actually matched.
type searchRequest struct {
	Size  int32 `json:"size"`
	Query struct {
		MultiMatch struct {
			Query  string   `json:"query"`
			Fields []string `json:"fields"`
		} `json:"multi_match"`
	} `json:"query"`
	Highlight struct {
		Fields map[string]highlightFieldConfig `json:"fields"`
	} `json:"highlight"`
}

// highlightFieldConfig: number_of_fragments 0 returns the whole field as
// a single highlighted fragment, which is what we want for the short
// fields (title/author_name/excerpt) — content is the only field long
// enough to need splitting into a few fragments instead.
type highlightFieldConfig struct {
	NumberOfFragments int `json:"number_of_fragments"`
	FragmentSize      int `json:"fragment_size,omitempty"`
}

type searchResponse struct {
	Hits struct {
		Hits []struct {
			Source    noteDoc             `json:"_source"`
			Highlight map[string][]string `json:"highlight"`
		} `json:"hits"`
	} `json:"hits"`
}

// Search satisfies noteservice.SearchIndex: a keyword match across each
// indexed note's title, author name, excerpt, and content, most relevant
// first.
func (c *Client) Search(ctx context.Context, keyword string, limit int32) ([]note.SearchHit, error) {
	var reqBody searchRequest
	reqBody.Size = limit
	reqBody.Query.MultiMatch.Query = keyword
	reqBody.Query.MultiMatch.Fields = searchFields
	reqBody.Highlight.Fields = map[string]highlightFieldConfig{
		"title":       {NumberOfFragments: 0},
		"author_name": {NumberOfFragments: 0},
		"excerpt":     {NumberOfFragments: 0},
		// 50 is a practical stand-in for "all matches" at personal-note
		// scale (a real cap OpenSearch requires some number for) — a
		// keyword common enough to blow past it is on the user to
		// narrow down, not something worth engineering around.
		"content": {NumberOfFragments: 50, FragmentSize: 150},
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("elasticsearch: encode search request: %w", err)
	}

	url := c.baseURL + "/" + noteIndex + "/_search"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("elasticsearch: build search request: %w", err)
	}
	req.SetBasicAuth(c.accessKey, c.accessSecret)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("elasticsearch: search: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("elasticsearch: read search response: %w", err)
	}

	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("elasticsearch: search: status %d: %s", resp.StatusCode, respBody)
	}

	var parsed searchResponse
	if err := json.Unmarshal(respBody, &parsed); err != nil {
		return nil, fmt.Errorf("elasticsearch: decode search response: %w", err)
	}

	results := make([]note.SearchHit, len(parsed.Hits.Hits))
	for i, hit := range parsed.Hits.Hits {
		results[i] = note.SearchHit{
			Note: note.Note{
				ID:          hit.Source.NoteID,
				Title:       hit.Source.Title,
				Slug:        hit.Source.Slug,
				AuthorName:  hit.Source.AuthorName,
				Excerpt:     hit.Source.Excerpt,
				Tags:        hit.Source.Tags,
				PublishedAt: hit.Source.PublishedAt,
			},
			Snippets: flattenSnippets(hit.Highlight),
		}
	}
	return results, nil
}

// flattenSnippets assembles a single, deterministically ordered list of
// snippets from OpenSearch's per-field highlight map.
func flattenSnippets(highlight map[string][]string) []string {
	var snippets []string
	for _, field := range highlightFieldOrder {
		snippets = append(snippets, highlight[field]...)
	}
	return snippets
}
