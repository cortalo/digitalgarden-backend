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
