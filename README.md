# digitalgarden-backend

A public markdown publishing platform: users log in, publish an Obsidian
note (with plugin content like Excalidraw-style drawings, TikZ diagrams,
math, and SVGs), and a homepage shows a feed of notes published by
everyone. This repo is the Go backend; the Next.js frontend lives in the
sibling repo [`digitalgarden-frontend`](../digitalgarden-frontend).

## Core design decision

Unlike most stacks (where markdown parsing lives in the JS ecosystem via
remark/rehype), **Go owns the markdown parsing here**, using
[goldmark](https://github.com/yuin/goldmark) (the same parser Hugo uses).
The frontend never parses markdown itself — every API response ships an
already-parsed JSON tree that the frontend renders by dispatching on each
node's `type`. Go's job stops at extracting structured data and tagging a
node's type; turning certain plugin content into pixels (Excalidraw,
TikZ, Mermaid) is unavoidably a browser job, since those renderers are
JS/WASM libraries with no Go equivalent.

## Supported markdown & plugin syntax

- Headings, paragraphs, lists (ordered/unordered), bold, italic, inline
  code, links (including bare autolinks)
- Inline (`$...$`) and block (`$$...$$`) math
- Fenced code blocks, tagged with their language
- **TikZ** diagrams (` ```tikz `): Go extracts the raw source; the
  frontend compiles it client-side via a WASM LaTeX engine
  ([tikzjax](https://github.com/kisonecat/tikzjax)) — chosen over
  server-side compilation for security (no LaTeX toolchain running on the
  backend)
- **SVG** blocks (` ```svg `, from Obsidian's SVG Editor plugin): passed
  through as-is, ready to drop into the DOM
- Obsidian's **CircuiTikZ Designer** plugin blocks: a preprocessing pass
  unwraps its JSON payload into a plain SVG block before parsing, so it's
  indistinguishable from a native SVG block downstream

A general preprocessing pipeline (`internal/markdown/preprocess.go`) runs
before goldmark parses, for working around real parser quirks and
normalizing plugin-specific formats — see its doc comments for the
specific bugs/formats it handles.

## Features

- Google OAuth login, backend-issued JWT sessions
- Publish a note (title + markdown, optional slug/excerpt/tags) — v1 is
  text-only, no attachment upload yet
- Public feed (`GET /api/notes`) and single-note read (`GET
  /api/notes/:slug`), plus a raw-markdown download endpoint
- Edit and delete your own notes (author-gated; a straight update/delete,
  no version history)
- Keyword search across title, author, excerpt, and body
  (`GET /api/notes/search?q=`), returning **where** each result matched
  (highlighted snippets), not just that it matched

## Architecture

Layered like the sibling learning project `onlineshopping-backend`:
domain (pure business objects + sentinel errors) → infra (Postgres,
OpenSearch, Google/JWT auth — implements interfaces implicitly) → service
(defines the repository/port interfaces it needs) → handler (Gin HTTP
adapters). See `CLAUDE.md` for the full set of conventions this repo
follows.

## Deployment — fully on free tiers

| Piece | Where | Notes |
|---|---|---|
| Backend (this repo) | [Vercel](https://vercel.com) | Go serverless runtime, reads `PORT` dynamically |
| Frontend | [Vercel](https://vercel.com) | Next.js, sibling repo |
| Database | [Supabase](https://supabase.com) Postgres | Shared instance with `onlineshopping-backend`, isolated via its own `digitalgarden` schema |
| Search index | [Bonsai](https://bonsai.io) (OpenSearch/Elasticsearch) | Shared cluster with `onlineshopping-backend`, isolated via its own `note` index |
| Auth | Google OAuth + self-issued JWT (HS256) | No paid identity provider; shares one Google Cloud OAuth Client with `onlineshopping-backend`, independent JWT secret |

Nothing in this stack requires a paid tier at the project's current scale.

## API

| Method | Path | Auth | Description |
|---|---|---|---|
| POST | `/api/auth/google` | — | Exchange a Google ID token for a session JWT |
| GET | `/api/notes` | — | Public feed (`?limit=`) |
| GET | `/api/notes/search` | — | Keyword search (`?q=`, `?limit=`) with matched snippets |
| GET | `/api/notes/:slug` | — | Single note, full parsed tree |
| GET | `/api/notes/:slug/download` | — | Raw markdown source as a file |
| POST | `/api/notes` | required | Publish a note |
| PUT | `/api/notes/:slug` | required, author-only | Edit a note |
| DELETE | `/api/notes/:slug` | required, author-only | Delete a note |

## Local development

```sh
cp .env.example .env.local   # fill in real values
go run .
```

Required environment variables are documented in `.env.example`:
Postgres connection string, Google OAuth client ID, JWT secret, and Bonsai
cluster credentials.

```sh
go build ./...
go vet ./...
go test ./...
```
