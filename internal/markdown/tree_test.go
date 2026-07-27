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
