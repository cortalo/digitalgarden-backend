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
