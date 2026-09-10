// Package markdown converts raw markdown source into Node, a JSON-
// serializable tree the frontend renders by walking and dispatching on
// Type — never by parsing markdown syntax itself. This package is the
// only place in the system that understands markdown syntax; see the
// repo's CLAUDE.md.
package markdown

import (
	"regexp"
	"strings"

	mathjax "github.com/litao91/goldmark-mathjax"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/text"
)

// parser adds the mathjax extension to goldmark's parser so that Obsidian/
// KaTeX-style `$inline$` and `$$block$$` math is recognized as its own AST
// node kind instead of falling through as plain text. We only ever use the
// Parser half of this goldmark instance (see Parse below) — its Renderer
// half produces MathJax-flavored HTML, which we don't want: per CLAUDE.md,
// Go's job stops at extracting a node's structured data (here, the raw
// LaTeX source) and tagging its type. Turning that into rendered math is a
// frontend job (KaTeX/MathJax), same as Excalidraw/Tikz/Mermaid.
var parser = goldmark.New(goldmark.WithExtensions(mathjax.MathJax)).Parser()

// Node is our own tree shape, distinct from goldmark's internal AST.
// goldmark's ast.Node is walked once, in convert, and turned into this
// simpler, stable form so the frontend never needs to know anything
// about goldmark itself — only this shape.
type Node struct {
	Type     string `json:"type"`
	Depth    int    `json:"depth,omitempty"`
	Text     string `json:"text,omitempty"`
	Lang     string `json:"lang,omitempty"`
	Href     string `json:"href,omitempty"`
	Ordered  bool   `json:"ordered,omitempty"`
	Children []Node `json:"children,omitempty"`
}

// Parse converts raw markdown source into a Node tree rooted at a
// "root" node. source itself is never mutated or returned — preprocess
// works against its own copy, so a caller storing the original alongside
// the parsed tree (e.g. as raw_markdown) is unaffected by it.
func Parse(source []byte) Node {
	source = preprocess(source)
	doc := parser.Parse(text.NewReader(source))
	return convert(doc, source)
}

// convert walks a single goldmark AST node (and its children) into our
// Node shape. Node kinds this doesn't explicitly recognize yet become
// "unknown" rather than being silently dropped, so an unsupported
// syntax element is visible in the output instead of just vanishing —
// see CLAUDE.md's error-handling stance in the sibling
// onlineshopping-backend project for why silent data loss isn't
// acceptable here either.
func convert(n ast.Node, source []byte) Node {
	switch n.Kind() {
	case ast.KindDocument:
		return Node{Type: "root", Children: convertChildren(n, source)}
	case ast.KindHeading:
		h := n.(*ast.Heading)
		return Node{Type: "heading", Depth: h.Level, Children: convertChildren(n, source)}
	case ast.KindParagraph:
		return Node{Type: "paragraph", Children: convertChildren(n, source)}
	case ast.KindTextBlock:
		// goldmark wraps a tight list item's content in a TextBlock
		// instead of a Paragraph (a TextBlock renders without its own
		// container). It's still just "some inline content," so give it
		// its own type rather than lumping it in with paragraph — the
		// frontend can decide whether that distinction matters visually.
		return Node{Type: "textBlock", Children: convertChildren(n, source)}
	case ast.KindList:
		l := n.(*ast.List)
		return Node{Type: "list", Ordered: l.IsOrdered(), Children: convertChildren(n, source)}
	case ast.KindListItem:
		return Node{Type: "listItem", Children: convertChildren(n, source)}
	case ast.KindText:
		t := n.(*ast.Text)
		return Node{Type: "text", Text: string(t.Segment.Value(source))}
	case ast.KindEmphasis:
		e := n.(*ast.Emphasis)
		typeName := "italic"
		if e.Level >= 2 {
			typeName = "bold"
		}
		return Node{Type: typeName, Children: convertChildren(n, source)}
	case ast.KindCodeSpan:
		return Node{Type: "inlineCode", Text: inlineText(n, source)}
	case ast.KindLink:
		l := n.(*ast.Link)
		return Node{Type: "link", Href: string(l.Destination), Children: convertChildren(n, source)}
	case ast.KindAutoLink:
		al := n.(*ast.AutoLink)
		url := string(al.URL(source))
		return Node{Type: "link", Href: url, Children: []Node{{Type: "text", Text: string(al.Label(source))}}}
	case mathjax.KindInlineMath:
		return Node{Type: "inlineMath", Text: inlineText(n, source)}
	case mathjax.KindMathBlock:
		b := n.(*mathjax.MathBlock)
		return Node{Type: "mathBlock", Text: linesText(b, source)}
	case ast.KindBlockquote:
		// An Obsidian callout is not its own syntax: it's an ordinary
		// blockquote whose first line happens to open with a `[!kind]`
		// marker. goldmark has no notion of it, so the distinction has to
		// be drawn here.
		if kind, body, rest, ok := parseCallout(n, source); ok {
			return Node{Type: "callout-" + kind, Text: body, Children: rest}
		}
		return Node{Type: "blockquote", Children: convertChildren(n, source)}
	case ast.KindFencedCodeBlock:
		fcb := n.(*ast.FencedCodeBlock)
		lang := string(fcb.Language(source))
		// Tikz is a plugin node, same tier as Excalidraw: Go extracts the
		// raw source and tags the type, but doesn't compile it — the
		// browser-side tikzjax WASM engine does that, see CLAUDE.md.
		if lang == "tikz" {
			return Node{Type: "tikzBlock", Text: linesText(fcb, source)}
		}
		// Unlike tikz, an svg fence (Obsidian's SVG Editor plugin) is
		// already final, renderable markup — nothing to compile, the
		// frontend just needs to drop it into the DOM as-is.
		if lang == "svg" {
			return Node{Type: "svgBlock", Text: linesText(fcb, source)}
		}
		return Node{Type: "codeBlock", Lang: lang, Text: linesText(fcb, source)}
	default:
		return Node{Type: "unknown", Children: convertChildren(n, source)}
	}
}

// inlineText concatenates a node's raw ast.Text children into a single
// string — used for node types whose content is always flat text with no
// further inline structure of its own (inlineMath's LaTeX source,
// inlineCode's code), since goldmark can split that text across more than
// one ast.Text child (e.g. inlineMath, if the formula spans a line break).
func inlineText(n ast.Node, source []byte) string {
	var sb strings.Builder
	for c := n.FirstChild(); c != nil; c = c.NextSibling() {
		if t, ok := c.(*ast.Text); ok {
			sb.Write(t.Segment.Value(source))
		}
	}
	return sb.String()
}

// linesText reads the raw source lines a block node spans (goldmark stores
// these as byte-offset segments rather than a materialized string) and
// joins them into a single trimmed string — used for node types whose
// content is opaque source text we pass through as-is (mathBlock,
// codeBlock, tikzBlock) rather than markdown to keep parsing further.
func linesText(n interface{ Lines() *text.Segments }, source []byte) string {
	return strings.TrimRight(string(n.Lines().Value(source)), "\n")
}

// calloutMarker matches the opening marker of an Obsidian callout: a
// blockquote whose first line starts with `[!kind]`, optionally followed
// by a `+`/`-` fold hint. The kind can't contain whitespace or `]`, which
// is what keeps an ordinary quote beginning with a bracketed phrase
// ("[see also] ...") from being mistaken for one.
var calloutMarker = regexp.MustCompile(`^\[!([^\]\s]+)\][-+]?`)

// parseCallout reports whether a blockquote node is an Obsidian callout
// and, if so, returns its lowercased kind plus its body.
//
// Two things make this messier than reading the node's inline children.
// The marker is inline content as far as goldmark is concerned, and its
// link parser shreds `[!todo] Task` into three separate text nodes ("[",
// "!todo", "] Task"). And a callout's title line and its body are folded
// into a *single* paragraph joined by a soft line break, because only a
// blank line ends a paragraph in CommonMark and `> ` lines in a row have
// none. Reading the paragraph's raw source lines instead sidesteps both:
// line 0 is exactly the title line, and dropping it leaves exactly the
// body.
//
// The kind is lowercased but not validated against a list — Obsidian
// accepts arbitrary kinds and styles unknown ones like a plain note, so a
// whitelist here would only make a custom kind disappear. Since the kind
// travels in the node's type name ("callout-todo") rather than in a field
// of its own, the frontend recognizes a kind it has no case for by the
// "callout-" prefix.
//
// The title is parsed and then discarded. It carries nothing the frontend
// needs (it is "Task" in every callout in the vault this was built for),
// and modeling it would mean either a new Node field or splitting the
// title's own inline nodes off the body's — see the design notes in
// tree_test.go for why neither is worth it yet.
//
// The body is kept as opaque text, the same treatment mathBlock and
// codeBlock get. That is only correct while a callout body stays plain
// prose: inline markup inside one (a formula, a link) reaches the
// frontend as unparsed markdown, which is a deliberate v1 narrowing, not
// an oversight — no callout in the vault has any today. Whole blocks
// following the first paragraph (a list, a second paragraph) are still
// converted normally into rest rather than dropped, so nothing silently
// vanishes if one shows up.
func parseCallout(n ast.Node, source []byte) (kind, body string, rest []Node, ok bool) {
	first, isParagraph := n.FirstChild().(*ast.Paragraph)
	if !isParagraph || first.Lines().Len() == 0 {
		return "", "", nil, false
	}

	titleLine := first.Lines().At(0)
	marker := calloutMarker.FindSubmatch(titleLine.Value(source))
	if marker == nil {
		return "", "", nil, false
	}

	var sb strings.Builder
	for i := 1; i < first.Lines().Len(); i++ {
		line := first.Lines().At(i)
		sb.Write(line.Value(source))
	}

	for c := first.NextSibling(); c != nil; c = c.NextSibling() {
		rest = append(rest, convert(c, source))
	}

	return strings.ToLower(string(marker[1])), strings.TrimRight(sb.String(), "\n"), rest, true
}

func convertChildren(n ast.Node, source []byte) []Node {
	var children []Node
	for c := n.FirstChild(); c != nil; c = c.NextSibling() {
		children = append(children, convert(c, source))
	}
	return children
}
