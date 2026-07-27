package note_test

import (
	"strings"
	"testing"

	"github.com/Cortalo/digitalgarden-backend/internal/domain/note"
	"github.com/Cortalo/digitalgarden-backend/internal/markdown"
)

func TestSlugify(t *testing.T) {
	cases := map[string]string{
		"Hello World":                  "hello-world",
		"  Leading and trailing  ":     "leading-and-trailing",
		"Mass-Energy Equivalence!!":    "mass-energy-equivalence",
		"Über café":                    "ber-caf", // non-ASCII letters aren't in [a-z0-9], so they're dropped, not transliterated.
		"already-a-slug":               "already-a-slug",
		"multiple   spaces\tand\ttabs": "multiple-spaces-and-tabs",
	}

	for title, want := range cases {
		if got := note.Slugify(title); got != want {
			t.Errorf("Slugify(%q) = %q, want %q", title, got, want)
		}
	}
}

func TestExcerptFrom(t *testing.T) {
	tree := markdown.Node{
		Type: "root",
		Children: []markdown.Node{
			{Type: "heading", Depth: 1, Children: []markdown.Node{{Type: "text", Text: "Title"}}},
			{Type: "paragraph", Children: []markdown.Node{
				{Type: "text", Text: "Mass-energy equivalence: "},
				{Type: "inlineMath", Text: "E = mc^2"},
				{Type: "text", Text: "."},
			}},
			{Type: "paragraph", Children: []markdown.Node{{Type: "text", Text: "A second paragraph, not used."}}},
		},
	}

	got := note.ExcerptFrom(tree)
	want := "Mass-energy equivalence: E = mc^2."
	if got != want {
		t.Errorf("ExcerptFrom() = %q, want %q", got, want)
	}
}

func TestExcerptFrom_NoParagraph(t *testing.T) {
	tree := markdown.Node{
		Type: "root",
		Children: []markdown.Node{
			{Type: "heading", Depth: 1, Children: []markdown.Node{{Type: "text", Text: "Title"}}},
		},
	}

	if got := note.ExcerptFrom(tree); got != "" {
		t.Errorf("ExcerptFrom() = %q, want empty string", got)
	}
}

func TestExcerptFrom_Truncates(t *testing.T) {
	long := strings.Repeat("a", 300)
	tree := markdown.Node{
		Type: "root",
		Children: []markdown.Node{
			{Type: "paragraph", Children: []markdown.Node{{Type: "text", Text: long}}},
		},
	}

	got := note.ExcerptFrom(tree)
	wantRunes := 200
	gotRunes := []rune(got)
	if len(gotRunes) != wantRunes+1 { // +1 for the trailing ellipsis rune.
		t.Fatalf("ExcerptFrom() length = %d runes, want %d", len(gotRunes), wantRunes+1)
	}
	if !strings.HasSuffix(got, "…") {
		t.Errorf("ExcerptFrom() = %q, want it to end with an ellipsis", got)
	}
}
