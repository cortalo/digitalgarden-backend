-- digitalgarden-backend schema + seed data.
--
-- Run this in the Supabase SQL editor. There's no migration tool wired up
-- yet (same as the sibling onlineshopping-backend project — schema changes
-- are applied by hand there too), so this file is the source of truth to
-- copy/paste from, not something anything runs automatically.
--
-- Naming follows onlineshopping-backend's convention: a dedicated schema,
-- with each table prefixed by the schema name
-- (online_shopping.online_shopping_commodity -> here,
-- digitalgarden.digitalgarden_note).

create schema if not exists digitalgarden;

create table digitalgarden.digitalgarden_user (
  user_id    bigint generated always as identity primary key,
  google_sub text not null unique,
  name       text not null,
  email      text not null unique,
  created_at timestamptz not null default now()
);

-- raw_markdown and parsed_tree are deliberately separate columns: parsed_tree
-- is the JSON tree markdown.Parse() produces, computed once at publish time
-- and served as-is on every read. raw_markdown is kept so the note can be
-- re-edited (or the tree regenerated later if the parser's node shapes
-- change) without ever re-deriving it from parsed_tree. See CLAUDE.md.
--
-- author_name and excerpt are both denormalized rather than joined at read
-- time: author_name is a snapshot of the author's display name taken at
-- publish time (same pattern as onlineshopping-backend's
-- Order.CommodityName — renaming a user later doesn't rewrite the byline
-- on notes they already published), and excerpt is a short preview
-- computed once at publish time, not derived from parsed_tree on every
-- feed read.
create table digitalgarden.digitalgarden_note (
  note_id        bigint generated always as identity primary key,
  title          text not null,
  slug           text not null unique,
  author_user_id bigint not null references digitalgarden.digitalgarden_user(user_id),
  author_name    text not null,
  raw_markdown   text not null,
  parsed_tree    jsonb not null,
  excerpt        text not null,
  tags           text[] not null default '{}',
  published_at   timestamptz not null default now()
);

-- Feed reads list notes newest-first; profile/author pages filter by author.
create index if not exists digitalgarden_note_published_at_idx
  on digitalgarden.digitalgarden_note (published_at desc);
create index if not exists digitalgarden_note_author_user_id_idx
  on digitalgarden.digitalgarden_note (author_user_id);

-- Seed data --------------------------------------------------------------

insert into digitalgarden.digitalgarden_user (google_sub, name, email)
values ('seed-google-sub-1', 'Cortalo', 'longheethz@gmail.com')
on conflict (google_sub) do nothing;

-- Note 1: the same content main.go's /api/notes/hello-world hardcodes —
-- heading, paragraph, list, inline + block math, a tikz diagram, and a
-- plain code block. parsed_tree below is real output copied from that
-- endpoint (verified by actually running the parser), not hand-typed.
insert into digitalgarden.digitalgarden_note
  (title, slug, author_user_id, author_name, raw_markdown, parsed_tree, excerpt, tags)
values (
  'Hello World',
  'hello-world',
  (select user_id from digitalgarden.digitalgarden_user where google_sub = 'seed-google-sub-1'),
  'Cortalo',
  $md$# Hello World

This is a paragraph.

- First item
- Second item
- Third item

Mass-energy equivalence: $E = mc^2$.

$$
\int_0^\infty e^{-x^2} \, dx = \frac{\sqrt{\pi}}{2}
$$

```tikz
\usepackage{tikz}
\begin{document}
\begin{tikzpicture}
\draw[fill=gray!30, thick] (0,0) circle (1);
\node at (0,-1.5) {a circle};
\end{tikzpicture}
\end{document}
```

```go
fmt.Println("hi")
```
$md$,
  $json${
    "type": "root",
    "children": [
      {
        "type": "heading",
        "depth": 1,
        "children": [{"type": "text", "text": "Hello World"}]
      },
      {
        "type": "paragraph",
        "children": [{"type": "text", "text": "This is a paragraph."}]
      },
      {
        "type": "list",
        "children": [
          {"type": "listItem", "children": [{"type": "textBlock", "children": [{"type": "text", "text": "First item"}]}]},
          {"type": "listItem", "children": [{"type": "textBlock", "children": [{"type": "text", "text": "Second item"}]}]},
          {"type": "listItem", "children": [{"type": "textBlock", "children": [{"type": "text", "text": "Third item"}]}]}
        ]
      },
      {
        "type": "paragraph",
        "children": [
          {"type": "text", "text": "Mass-energy equivalence: "},
          {"type": "inlineMath", "text": "E = mc^2"},
          {"type": "text", "text": "."}
        ]
      },
      {
        "type": "mathBlock",
        "text": "\\int_0^\\infty e^{-x^2} \\, dx = \\frac{\\sqrt{\\pi}}{2}"
      },
      {
        "type": "tikzBlock",
        "text": "\\usepackage{tikz}\n\\begin{document}\n\\begin{tikzpicture}\n\\draw[fill=gray!30, thick] (0,0) circle (1);\n\\node at (0,-1.5) {a circle};\n\\end{tikzpicture}\n\\end{document}"
      },
      {
        "type": "codeBlock",
        "text": "fmt.Println(\"hi\")",
        "lang": "go"
      }
    ]
  }$json$::jsonb,
  'This is a paragraph.',
  '{physics,tikz}'
)
on conflict (slug) do nothing;

-- Note 2: a plain note with no plugin content, for a feed item that
-- doesn't need any special frontend rendering. Tree matches
-- internal/markdown/tree_test.go's TestParse_HelloWorld fixture exactly.
insert into digitalgarden.digitalgarden_note
  (title, slug, author_user_id, author_name, raw_markdown, parsed_tree, excerpt, tags)
values (
  'A Plain Note',
  'a-plain-note',
  (select user_id from digitalgarden.digitalgarden_user where google_sub = 'seed-google-sub-1'),
  'Cortalo',
  $md$# Hello World

This is a paragraph.

- First item
- Second item
$md$,
  $json${
    "type": "root",
    "children": [
      {
        "type": "heading",
        "depth": 1,
        "children": [{"type": "text", "text": "Hello World"}]
      },
      {
        "type": "paragraph",
        "children": [{"type": "text", "text": "This is a paragraph."}]
      },
      {
        "type": "list",
        "children": [
          {"type": "listItem", "children": [{"type": "textBlock", "children": [{"type": "text", "text": "First item"}]}]},
          {"type": "listItem", "children": [{"type": "textBlock", "children": [{"type": "text", "text": "Second item"}]}]}
        ]
      }
    ]
  }$json$::jsonb,
  'This is a paragraph.',
  '{}'
)
on conflict (slug) do nothing;
