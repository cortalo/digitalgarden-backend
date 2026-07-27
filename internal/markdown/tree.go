// Package markdown converts raw markdown source into Node, a JSON-
// serializable tree the frontend renders by walking and dispatching on
// Type — never by parsing markdown syntax itself. This package is the
// only place in the system that understands markdown syntax; see the
// repo's CLAUDE.md.
package markdown

import (
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/text"
)

// Node is our own tree shape, distinct from goldmark's internal AST.
// goldmark's ast.Node is walked once, in convert, and turned into this
// simpler, stable form so the frontend never needs to know anything
// about goldmark itself — only this shape.
type Node struct {
	Type     string `json:"type"`
	Depth    int    `json:"depth,omitempty"`
	Text     string `json:"text,omitempty"`
	Ordered  bool   `json:"ordered,omitempty"`
	Children []Node `json:"children,omitempty"`
}

// Parse converts raw markdown source into a Node tree rooted at a
// "root" node.
func Parse(source []byte) Node {
	doc := goldmark.DefaultParser().Parse(text.NewReader(source))
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
	default:
		return Node{Type: "unknown", Children: convertChildren(n, source)}
	}
}

func convertChildren(n ast.Node, source []byte) []Node {
	var children []Node
	for c := n.FirstChild(); c != nil; c = c.NextSibling() {
		children = append(children, convert(c, source))
	}
	return children
}
