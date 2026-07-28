// Package elasticsearch is a thin adapter over the same Bonsai-hosted
// OpenSearch/Elasticsearch cluster onlineshopping-backend uses, isolated
// into its own index (see noteIndex) rather than a separate cluster —
// same reasoning as sharing one Supabase Postgres instance across both
// projects via a separate schema. HTTP + Basic Auth, no client SDK,
// scoped to the one index this project needs.
package elasticsearch

import (
	"net/http"
	"net/url"
	"strings"
)

// noteIndex is the only index this project uses. Not configurable: there's
// no second use case yet to justify it, and hardcoding avoids a footgun
// where index writes and searches silently disagree.
const noteIndex = "note"

// Client wraps everything needed to index and search note documents.
// Consumers depend on a small interface they define themselves (see
// noteservice.SearchIndex), not on this concrete type.
type Client struct {
	baseURL      string
	accessKey    string
	accessSecret string
	http         *http.Client
}

// New builds a Client. baseURL is the cluster's Access URL — Bonsai's
// dashboard shows it with the credentials embedded
// (https://key:secret@host); that userinfo is stripped here since
// accessKey/accessSecret are sent explicitly as Basic Auth on every
// request instead, so either form (with or without embedded credentials)
// works.
func New(baseURL, accessKey, accessSecret string) *Client {
	if u, err := url.Parse(baseURL); err == nil {
		u.User = nil
		baseURL = u.String()
	}

	return &Client{
		baseURL:      strings.TrimSuffix(baseURL, "/"),
		accessKey:    accessKey,
		accessSecret: accessSecret,
		http:         &http.Client{},
	}
}
