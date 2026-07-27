package markdown

import "testing"

func TestInsertBlankLineBetweenAdjacentMathBlocks(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "adjacent blocks get a blank line inserted",
			in:   "$$\na\n$$\n$$\nb\n$$\n",
			want: "$$\na\n$$\n\n$$\nb\n$$\n",
		},
		{
			name: "already-separated blocks are untouched",
			in:   "$$\na\n$$\n\n$$\nb\n$$\n",
			want: "$$\na\n$$\n\n$$\nb\n$$\n",
		},
		{
			name: "a single math block is untouched",
			in:   "$$\na\n$$\n",
			want: "$$\na\n$$\n",
		},
		{
			name: "no math blocks at all is untouched",
			in:   "# Heading\n\nA paragraph.\n",
			want: "# Heading\n\nA paragraph.\n",
		},
		{
			name: "literal $$ lines inside a fenced code block are left alone",
			in:   "```text\n$$\n$$\n```\n",
			want: "```text\n$$\n$$\n```\n",
		},
		{
			name: "a real adjacent pair after a code fence is still fixed",
			in:   "```text\n$$\n$$\n```\n$$\na\n$$\n$$\nb\n$$\n",
			want: "```text\n$$\n$$\n```\n$$\na\n$$\n\n$$\nb\n$$\n",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := string(insertBlankLineBetweenAdjacentMathBlocks([]byte(c.in)))
			if got != c.want {
				t.Errorf("got:\n%q\nwant:\n%q", got, c.want)
			}
		})
	}
}

func TestUnwrapCircuitTikzDesignerBlocks(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "extracts the svg field into a plain svg fence",
			in:   "before\n```circuittikz-designer-test\n{\"svg\": \"<svg>ok</svg>\", \"components\": []}\n```\nafter\n",
			want: "before\n```svg\n<svg>ok</svg>\n```\nafter\n",
		},
		{
			name: "no circuittikz block at all is untouched",
			in:   "# Heading\n\nA paragraph.\n",
			want: "# Heading\n\nA paragraph.\n",
		},
		{
			name: "invalid JSON is left completely untouched",
			in:   "```circuittikz-designer-test\nnot json\n```\n",
			want: "```circuittikz-designer-test\nnot json\n```\n",
		},
		{
			name: "valid JSON with no svg field is left completely untouched",
			in:   "```circuittikz-designer-test\n{\"components\": []}\n```\n",
			want: "```circuittikz-designer-test\n{\"components\": []}\n```\n",
		},
		{
			name: "unclosed block is left completely untouched",
			in:   "```circuittikz-designer-test\n{\"svg\": \"<svg>ok</svg>\"}\n",
			want: "```circuittikz-designer-test\n{\"svg\": \"<svg>ok</svg>\"}\n",
		},
		{
			name: "a multi-line svg value comes through with real newlines",
			in:   "```circuittikz-designer-test\n{\"svg\": \"<svg>\\n<circle/>\\n</svg>\"}\n```\n",
			want: "```svg\n<svg>\n<circle/>\n</svg>\n```\n",
		},
		{
			name: "var(--bs-emphasis-color) is rewritten to currentColor",
			in:   "```circuittikz-designer-test\n{\"svg\": \"<path stroke=\\\"var(--bs-emphasis-color)\\\" fill=\\\"var(--bs-emphasis-color)\\\"/>\"}\n```\n",
			want: "```svg\n<path stroke=\"currentColor\" fill=\"currentColor\"/>\n```\n",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := string(unwrapCircuitTikzDesignerBlocks([]byte(c.in)))
			if got != c.want {
				t.Errorf("got:\n%q\nwant:\n%q", got, c.want)
			}
		})
	}
}
