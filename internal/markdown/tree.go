// Package markdown converts raw markdown source into Node, a JSON-
// serializable tree the frontend renders by walking and dispatching on
// Type — never by parsing markdown syntax itself. This package is the
// only place in the system that understands markdown syntax; see the
// repo's CLAUDE.md.
package markdown

import (
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
	case mathjax.KindInlineMath:
		return Node{Type: "inlineMath", Text: inlineText(n, source)}
	case mathjax.KindMathBlock:
		b := n.(*mathjax.MathBlock)
		return Node{Type: "mathBlock", Text: linesText(b, source)}
	case ast.KindFencedCodeBlock:
		fcb := n.(*ast.FencedCodeBlock)
		lang := string(fcb.Language(source))
		// Tikz is a plugin node, same tier as Excalidraw: Go extracts the
		// raw source and tags the type, but doesn't compile it — the
		// browser-side tikzjax WASM engine does that, see CLAUDE.md.
		if lang == "tikz" {
			return Node{Type: "tikzBlock", Text: linesText(fcb, source)}
		}
		return Node{Type: "codeBlock", Lang: lang, Text: linesText(fcb, source)}
	default:
		return Node{Type: "unknown", Children: convertChildren(n, source)}
	}
}

// inlineText concatenates an inline math node's raw text segments (goldmark-
// mathjax stores the LaTeX source as one or more ast.Text children rather
// than a single segment) into the formula's source string, e.g. "E = mc^2".
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

func convertChildren(n ast.Node, source []byte) []Node {
	var children []Node
	for c := n.FirstChild(); c != nil; c = c.NextSibling() {
		children = append(children, convert(c, source))
	}
	return children
}
