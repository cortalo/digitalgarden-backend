package markdown

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const helloWorldMarkdown = `# Hello World

This is a paragraph.

- First item
- Second item
`

func TestParse_HelloWorld(t *testing.T) {
	got := Parse([]byte(helloWorldMarkdown))

	require.Equal(t, "root", got.Type)
	require.Len(t, got.Children, 3)

	heading := got.Children[0]
	assert.Equal(t, "heading", heading.Type)
	assert.Equal(t, 1, heading.Depth)
	require.Len(t, heading.Children, 1)
	assert.Equal(t, "Hello World", heading.Children[0].Text)

	paragraph := got.Children[1]
	assert.Equal(t, "paragraph", paragraph.Type)
	require.Len(t, paragraph.Children, 1)
	assert.Equal(t, "This is a paragraph.", paragraph.Children[0].Text)

	list := got.Children[2]
	assert.Equal(t, "list", list.Type)
	assert.False(t, list.Ordered)
	require.Len(t, list.Children, 2)
	assert.Equal(t, "listItem", list.Children[0].Type)
	require.Len(t, list.Children[0].Children, 1)
	assert.Equal(t, "textBlock", list.Children[0].Children[0].Type)

	// Print the JSON shape so it can be eyeballed directly — this is the
	// actual thing under validation: not "does goldmark parse markdown"
	// (already proven elsewhere, e.g. Hugo), but "is this JSON shape
	// something a frontend can sanely walk and render."
	out, err := json.MarshalIndent(got, "", "  ")
	require.NoError(t, err)
	t.Logf("parsed tree:\n%s", out)
}

const mathMarkdown = "Mass-energy equivalence: $E = mc^2$.\n\n$$\n\\frac{\\sqrt{\\pi}}{2}\n$$\n"

func TestParse_Math(t *testing.T) {
	got := Parse([]byte(mathMarkdown))

	require.Equal(t, "root", got.Type)
	require.Len(t, got.Children, 2)

	paragraph := got.Children[0]
	assert.Equal(t, "paragraph", paragraph.Type)
	require.Len(t, paragraph.Children, 3)
	assert.Equal(t, "text", paragraph.Children[0].Type)
	assert.Equal(t, "inlineMath", paragraph.Children[1].Type)
	assert.Equal(t, "E = mc^2", paragraph.Children[1].Text)
	assert.Equal(t, "text", paragraph.Children[2].Type)

	mathBlock := got.Children[1]
	assert.Equal(t, "mathBlock", mathBlock.Type)
	assert.Equal(t, `\frac{\sqrt{\pi}}{2}`, mathBlock.Text)

	out, err := json.MarshalIndent(got, "", "  ")
	require.NoError(t, err)
	t.Logf("parsed tree:\n%s", out)
}

// adjacentMathBlocksMarkdown is a minimal reproduction (bisected from a
// real note, CS229M.md, that crashed the live publish endpoint with a 500)
// of a bug in goldmark-mathjax: two `$$...$$` blocks back to back with no
// blank line between them. Its block parser keeps state in a single
// shared parser.Context key rather than per-node, and Close() nils that
// key out — when a second math block's Continue() runs right after the
// first one's Close(), it reads that now-nil value and panics on the type
// assertion (block.go: `pc.Get(mathBlockInfoKey).(*mathBlockData)`).
const adjacentMathBlocksMarkdown = "$$\na\n$$\n$$\nb\n$$\n"

func TestParse_AdjacentMathBlocks(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("Parse panicked on adjacent math blocks: %v", r)
		}
	}()

	got := Parse([]byte(adjacentMathBlocksMarkdown))

	require.Equal(t, "root", got.Type)
	require.Len(t, got.Children, 2)
	assert.Equal(t, "mathBlock", got.Children[0].Type)
	assert.Equal(t, "a", got.Children[0].Text)
	assert.Equal(t, "mathBlock", got.Children[1].Type)
	assert.Equal(t, "b", got.Children[1].Text)
}

const tikzMarkdown = "```tikz\n\\begin{tikzpicture}\n\\draw (0,0) circle (1);\n\\end{tikzpicture}\n```\n"

func TestParse_Tikz(t *testing.T) {
	got := Parse([]byte(tikzMarkdown))

	require.Equal(t, "root", got.Type)
	require.Len(t, got.Children, 1)

	tikz := got.Children[0]
	assert.Equal(t, "tikzBlock", tikz.Type)
	assert.Empty(t, tikz.Lang)
	assert.Equal(t, "\\begin{tikzpicture}\n\\draw (0,0) circle (1);\n\\end{tikzpicture}", tikz.Text)

	out, err := json.MarshalIndent(got, "", "  ")
	require.NoError(t, err)
	t.Logf("parsed tree:\n%s", out)
}

const svgMarkdown = "```svg\n<svg xmlns=\"http://www.w3.org/2000/svg\" viewBox=\"0 0 100 100\">\n  <circle cx=\"50\" cy=\"50\" r=\"40\" fill=\"red\" />\n</svg>\n```\n"

func TestParse_Svg(t *testing.T) {
	got := Parse([]byte(svgMarkdown))

	require.Equal(t, "root", got.Type)
	require.Len(t, got.Children, 1)

	svg := got.Children[0]
	assert.Equal(t, "svgBlock", svg.Type)
	assert.Empty(t, svg.Lang)
	assert.Equal(t,
		"<svg xmlns=\"http://www.w3.org/2000/svg\" viewBox=\"0 0 100 100\">\n  <circle cx=\"50\" cy=\"50\" r=\"40\" fill=\"red\" />\n</svg>",
		svg.Text,
	)

	out, err := json.MarshalIndent(got, "", "  ")
	require.NoError(t, err)
	t.Logf("parsed tree:\n%s", out)
}

const codeMarkdown = "```go\nfmt.Println(\"hi\")\n```\n\n```\nno language here\n```\n"

func TestParse_CodeBlock(t *testing.T) {
	got := Parse([]byte(codeMarkdown))

	require.Equal(t, "root", got.Type)
	require.Len(t, got.Children, 2)

	withLang := got.Children[0]
	assert.Equal(t, "codeBlock", withLang.Type)
	assert.Equal(t, "go", withLang.Lang)
	assert.Equal(t, `fmt.Println("hi")`, withLang.Text)

	withoutLang := got.Children[1]
	assert.Equal(t, "codeBlock", withoutLang.Type)
	assert.Empty(t, withoutLang.Lang)
	assert.Equal(t, "no language here", withoutLang.Text)

	out, err := json.MarshalIndent(got, "", "  ")
	require.NoError(t, err)
	t.Logf("parsed tree:\n%s", out)
}

const emphasisMarkdown = "**bold**, *italic*, and `inline code`, plus **bold with *nested italic* inside**.\n"

func TestParse_Emphasis(t *testing.T) {
	got := Parse([]byte(emphasisMarkdown))

	require.Equal(t, "root", got.Type)
	require.Len(t, got.Children, 1)

	paragraph := got.Children[0]
	assert.Equal(t, "paragraph", paragraph.Type)
	require.Len(t, paragraph.Children, 8)

	bold := paragraph.Children[0]
	assert.Equal(t, "bold", bold.Type)
	require.Len(t, bold.Children, 1)
	assert.Equal(t, "text", bold.Children[0].Type)
	assert.Equal(t, "bold", bold.Children[0].Text)

	assert.Equal(t, "text", paragraph.Children[1].Type)
	assert.Equal(t, ", ", paragraph.Children[1].Text)

	italic := paragraph.Children[2]
	assert.Equal(t, "italic", italic.Type)
	require.Len(t, italic.Children, 1)
	assert.Equal(t, "italic", italic.Children[0].Text)

	assert.Equal(t, "text", paragraph.Children[3].Type)
	assert.Equal(t, ", and ", paragraph.Children[3].Text)

	code := paragraph.Children[4]
	assert.Equal(t, "inlineCode", code.Type)
	assert.Equal(t, "inline code", code.Text)

	assert.Equal(t, "text", paragraph.Children[5].Type)
	assert.Equal(t, ", plus ", paragraph.Children[5].Text)

	nestedBold := paragraph.Children[6]
	assert.Equal(t, "bold", nestedBold.Type)
	require.Len(t, nestedBold.Children, 3)
	assert.Equal(t, "text", nestedBold.Children[0].Type)
	assert.Equal(t, "bold with ", nestedBold.Children[0].Text)
	assert.Equal(t, "italic", nestedBold.Children[1].Type)
	assert.Equal(t, "nested italic", nestedBold.Children[1].Children[0].Text)
	assert.Equal(t, "text", nestedBold.Children[2].Type)
	assert.Equal(t, " inside", nestedBold.Children[2].Text)

	assert.Equal(t, "text", paragraph.Children[7].Type)
	assert.Equal(t, ".", paragraph.Children[7].Text)

	out, err := json.MarshalIndent(got, "", "  ")
	require.NoError(t, err)
	t.Logf("parsed tree:\n%s", out)
}

const linkMarkdown = "[a plain link](https://example.com), a link with **bold** text: [**bold link**](https://bold.example.com), and a bare <https://auto.example.com> autolink.\n"

func TestParse_Link(t *testing.T) {
	got := Parse([]byte(linkMarkdown))

	require.Equal(t, "root", got.Type)
	require.Len(t, got.Children, 1)

	paragraph := got.Children[0]
	assert.Equal(t, "paragraph", paragraph.Type)

	plain := paragraph.Children[0]
	assert.Equal(t, "link", plain.Type)
	assert.Equal(t, "https://example.com", plain.Href)
	require.Len(t, plain.Children, 1)
	assert.Equal(t, "text", plain.Children[0].Type)
	assert.Equal(t, "a plain link", plain.Children[0].Text)

	var richLink, autolink *Node
	for i := range paragraph.Children {
		c := &paragraph.Children[i]
		if c.Type != "link" {
			continue
		}
		switch c.Href {
		case "https://bold.example.com":
			richLink = c
		case "https://auto.example.com":
			autolink = c
		}
	}

	require.NotNil(t, richLink, "expected a link to https://bold.example.com")
	require.Len(t, richLink.Children, 1)
	assert.Equal(t, "bold", richLink.Children[0].Type)
	assert.Equal(t, "bold link", richLink.Children[0].Children[0].Text)

	require.NotNil(t, autolink, "expected an autolink to https://auto.example.com")
	require.Len(t, autolink.Children, 1)
	assert.Equal(t, "text", autolink.Children[0].Type)
	assert.Equal(t, "https://auto.example.com", autolink.Children[0].Text)

	out, err := json.MarshalIndent(got, "", "  ")
	require.NoError(t, err)
	t.Logf("parsed tree:\n%s", out)
}

const blockquoteMarkdown = `> Just an ordinary quote.

> [not a callout] this only looks like one.
`

func TestParse_Blockquote(t *testing.T) {
	got := Parse([]byte(blockquoteMarkdown))

	require.Equal(t, "root", got.Type)
	require.Len(t, got.Children, 2)

	// A plain quote gets its own type rather than falling through to
	// "unknown" — the frontend can't render it as a quote otherwise.
	quote := got.Children[0]
	assert.Equal(t, "blockquote", quote.Type)
	require.Len(t, quote.Children, 1)
	assert.Equal(t, "paragraph", quote.Children[0].Type)

	// A bracketed word without the "!" isn't a callout marker, and its
	// text must survive intact.
	notCallout := got.Children[1]
	assert.Equal(t, "blockquote", notCallout.Type)
	require.Len(t, notCallout.Children, 1)
	assert.Equal(t, "paragraph", notCallout.Children[0].Type)

	out, err := json.MarshalIndent(got, "", "  ")
	require.NoError(t, err)
	t.Logf("parsed tree:\n%s", out)
}

// calloutMarkdown is copied verbatim from the real note this support was
// added for (VCO Semester Project.md). Obsidian's callout syntax is a
// blockquote whose first line carries a `[!kind]` marker plus an optional
// title. goldmark knows nothing about it and parses that marker into three
// stray inline text nodes ("[", "!todo", "] Task") glued to the body by a
// soft line break, all inside a single paragraph — which is what leaks to
// the frontend today.
//
// The body is read back out of the paragraph's raw source lines and kept
// as opaque text, the same treatment mathBlock/tikzBlock/codeBlock get.
// That holds only as long as a callout body stays plain prose: a formula
// or a list inside one would reach the frontend as unparsed markdown. No
// callout in the vault has either today, and this is a deliberate v1
// narrowing, not an oversight.
const calloutMarkdown = `> [!todo] Task
> Carry out the similar procedure for the active part.
`

func TestParse_Callout(t *testing.T) {
	got := Parse([]byte(calloutMarkdown))

	require.Equal(t, "root", got.Type)
	require.Len(t, got.Children, 1)

	callout := got.Children[0]
	// The kind rides in the type name instead of a dedicated Node field:
	// only one kind is in real use, so four new fields on a struct shared
	// by every node type isn't worth it. The "callout-" prefix leaves the
	// frontend able to recognize a kind it has no specific case for.
	assert.Equal(t, "callout-todo", callout.Type)

	// The marker and the title are both consumed. The title is always
	// "Task" in real use and carries nothing the frontend needs; leaking
	// the marker's three text fragments into the body is the bug this
	// change exists to fix.
	assert.Equal(t, "Carry out the similar procedure for the active part.", callout.Text)
	assert.Empty(t, callout.Children)

	out, err := json.MarshalIndent(got, "", "  ")
	require.NoError(t, err)
	t.Logf("parsed tree:\n%s", out)
}

// calloutMultiLineMarkdown covers a body written across several source
// lines. goldmark folds them into one paragraph and drops the newlines
// between them, so the line structure has to come from the paragraph's
// raw source segments, not from its inline children.
const calloutMultiLineMarkdown = `> [!TODO]- Task
> Build the passive part model.
> Compare with simulations.
`

func TestParse_CalloutMultiLine(t *testing.T) {
	got := Parse([]byte(calloutMultiLineMarkdown))

	require.Equal(t, "root", got.Type)
	require.Len(t, got.Children, 1)

	callout := got.Children[0]
	// Kind is lowercased, and the fold marker ("-"/"+") is consumed
	// rather than becoming part of the kind. Folding isn't modeled yet;
	// what matters here is that the marker doesn't leak into the type.
	assert.Equal(t, "callout-todo", callout.Type)
	assert.Equal(t, "Build the passive part model.\nCompare with simulations.", callout.Text)

	out, err := json.MarshalIndent(got, "", "  ")
	require.NoError(t, err)
	t.Logf("parsed tree:\n%s", out)
}

// calloutTitleOnlyMarkdown covers a callout whose title line is the whole
// callout: once that line is consumed there is nothing left, so Text must
// come back empty rather than holding the title or the marker.
const calloutTitleOnlyMarkdown = `> [!todo] Task
`

func TestParse_CalloutTitleOnly(t *testing.T) {
	got := Parse([]byte(calloutTitleOnlyMarkdown))

	require.Equal(t, "root", got.Type)
	require.Len(t, got.Children, 1)

	callout := got.Children[0]
	assert.Equal(t, "callout-todo", callout.Type)
	assert.Empty(t, callout.Text)
	assert.Empty(t, callout.Children)

	out, err := json.MarshalIndent(got, "", "  ")
	require.NoError(t, err)
	t.Logf("parsed tree:\n%s", out)
}
