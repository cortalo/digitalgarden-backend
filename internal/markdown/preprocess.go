package markdown

import (
	"bytes"
	"regexp"
)

// preprocess runs a pipeline of source-to-source passes over raw markdown
// before it's handed to goldmark. It's kept as its own pipeline — rather
// than folded directly into Parse — because it's expected to grow more
// passes over time (this is the first one), each addressing a specific,
// documented problem with the real parser, not general-purpose markdown
// rewriting.
func preprocess(source []byte) []byte {
	source = insertBlankLineBetweenAdjacentMathBlocks(source)
	return source
}

// mathBlockDelimLine matches a line that is, on its own, a $$ math block
// delimiter (allowing up to 3 leading spaces, same threshold CommonMark
// uses before a line counts as indented code) — both the opening and
// closing delimiter look identical, a bare run of 2+ dollar signs.
var mathBlockDelimLine = regexp.MustCompile(`^ {0,3}\${2,}\s*$`)

// fenceDelimLine matches a fenced code block delimiter (``` or ~~~),
// used only to know when to suspend the math-delimiter check below —
// content inside a code fence is verbatim example text, not real
// markdown structure, so a literal "$$" in a code sample must not be
// touched.
var fenceDelimLine = regexp.MustCompile("^ {0,3}(```+|~~~+)")

// insertBlankLineBetweenAdjacentMathBlocks works around a real bug in
// github.com/litao91/goldmark-mathjax's block parser: it keeps a math
// block's start-indent in a single shared parser.Context key instead of
// on the node itself. When one $$ block's closing line is immediately
// followed by another $$ block's opening line with no blank line between
// them, goldmark opens the second block before it gets around to
// (belatedly) closing the first one — confirmed by tracing the actual
// call order — and the first block's deferred Close() wipes the shared
// key right after the second block just set it, so the second block's
// own Continue() reads nil and panics.
//
// Forcing a blank line between the two sidesteps the bug by construction:
// with a blank line there, the first block's Close() always runs before
// the second block's Open() is even attempted, so the two lifecycles
// never overlap in time. See internal/markdown/preprocess_test.go for the
// crash this specifically covers (bisected from a real note, CS229M.md,
// that 500'd the publish endpoint).
func insertBlankLineBetweenAdjacentMathBlocks(source []byte) []byte {
	lines := bytes.Split(source, []byte("\n"))

	out := make([][]byte, 0, len(lines))
	inFence := false
	for i, line := range lines {
		if fenceDelimLine.Match(line) {
			inFence = !inFence
		}

		out = append(out, line)

		if inFence || i == len(lines)-1 {
			continue
		}

		if mathBlockDelimLine.Match(line) && mathBlockDelimLine.Match(lines[i+1]) {
			out = append(out, nil)
		}
	}

	return bytes.Join(out, []byte("\n"))
}
