# digitalgarden-backend

A platform where users log in, publish an Obsidian note (markdown, possibly
with plugin content like Excalidraw drawings) to a public web page, and a
homepage shows a feed of notes published by all users. Sibling frontend
repo: `../digitalgarden-frontend`.

## Core architectural decision: Go owns the markdown parsing, not just CRUD

The user is strong in Go, weak in frontend/Next.js. Every piece of this
project that *can* reasonably live in Go, does — including markdown
parsing, which in most stacks would live in the JS ecosystem
(remark/rehype). Don't default to "parsing belongs in the frontend" just
because that's the common pattern elsewhere; it was deliberately decided
otherwise here.

- Parse markdown with **goldmark** (`github.com/yuin/goldmark` — the same
  parser Hugo uses), which is extensible and has existing community
  extensions for Obsidian-flavored syntax:
  `go.abhg.dev/goldmark/wikilink` (or similar) for `[[wikilinks]]`, and
  `goldmark-obsidian` for footnotes/tags/properties. Check current
  state of these packages before assuming exact APIs — this was decided
  in a design conversation, not yet implemented or verified against real
  package versions.
- Go is responsible for turning raw markdown into a structured tree
  (typed nodes: heading, paragraph, list, wikilink, embedded-plugin-block,
  etc.), resolving wikilinks against other published notes, and producing
  the final JSON tree the frontend renders from.
- Go is **not** responsible for the final *visual* rendering of certain
  plugin content types (Excalidraw drawings, Tikz/LaTeX diagrams, Mermaid
  diagrams). Those plugins' actual renderers are browser-only libraries
  (Excalidraw ships a React component; Tikz support in Obsidian typically
  runs via a WASM LaTeX build in-browser) with no Go equivalent — no
  matter what parses the markdown, turning those specific node types into
  pixels is unavoidably a frontend job. Go's job for these node types
  stops at extracting their structured data (e.g. the Excalidraw JSON
  payload) and tagging the node with its type; rendering it is out of
  scope here.

## Data storage

- **Supabase Postgres**: source of truth for everything text-shaped —
  raw markdown, the parsed JSON tree, user/note metadata (title, slug,
  author, published_at, tags). A note's raw markdown and its parsed tree
  are two different stored fields; the parsed tree is a derived/cached
  copy computed at publish time, not recomputed on every read (this
  project has a public feed, so reads vastly outnumber writes — same
  reasoning as the search-index tradeoff in the sibling `onlineshopping-
  backend` project).
- **Supabase Storage** (object storage, not Postgres): binary attachments
  a note embeds (images, screenshots) that aren't representable as text
  data like Excalidraw's inline JSON is. Postgres only stores a URL/
  reference to these, mirroring how `onlineshopping-backend` stores
  `image_url` rather than image bytes.
- Remember: if the parsed-tree cache's generation logic changes later
  (e.g. a new plugin adapter added), previously-published notes' cached
  trees go stale and need reprocessing — there's no automatic
  invalidation. `onlineshopping-backend` hit exactly this class of bug
  with its Elasticsearch search index (fields added to the doc shape
  after the fact required a manual backfill re-run) — expect the same
  operational discipline to be needed here.

## Upload scope (v1) — deliberately narrow

A user publishes **one note at a time**: its raw markdown text, plus any
locally-referenced attachment files (images) that note embeds. Explicitly
out of scope for now: uploading a whole vault/zip, Git-based vault sync,
an Obsidian plugin that pushes directly. Don't build toward these unless
asked — they're a different, bigger feature (bulk import) than "publish
one note."

## API surface the frontend needs

The frontend should never parse markdown itself — it receives the
already-parsed JSON tree from a Go API endpoint (e.g. `GET
/api/notes/:id`), not raw markdown text. If you're adding an endpoint
and find yourself sending raw markdown to the frontend, that's very
likely a design mistake given the architecture decision above — check
first.

## Style / structure precedent

`../onlineshopping-backend` (sibling learning project, same user) is a
working example of the coding conventions and layered architecture this
user likes for Go backends: domain (pure business objects + sentinel
errors) → infra (postgres/redis/etc, implements interfaces implicitly) →
service (defines Repository/port interfaces, dependency inversion) →
handler (gin HTTP adapters, defines its own Service interface). It also
has a working Google OAuth + JWT auth pattern
(`internal/infra/googleauth`, `internal/infra/authtoken`,
`internal/handler/auth`) worth reusing here rather than re-deriving.
When unsure how to structure something in this repo, look there first.
