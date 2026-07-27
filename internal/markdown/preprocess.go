package markdown

import (
	"bytes"
	"encoding/json"
	"regexp"
)

// preprocess runs a pipeline of source-to-source passes over raw markdown
// before it's handed to goldmark. It's kept as its own pipeline — rather
// than folded directly into Parse — because it's expected to grow more
// passes over time, each addressing a specific, documented problem with
// the real parser or a specific plugin's on-disk format, not general-
// purpose markdown rewriting.
func preprocess(source []byte) []byte {
	source = insertBlankLineBetweenAdjacentMathBlocks(source)
	source = unwrapCircuitTikzDesignerBlocks(source)
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

// circuitTikzFenceOpen matches the opening fence of an Obsidian
// CircuiTikZ Designer plugin block. "circuittikz-designer-test" is the
// language tag hardcoded in that plugin's own source
// (DESIGNER_CODE_BLOCK_LANG in main.js) — not a name we chose.
var circuitTikzFenceOpen = regexp.MustCompile("^ {0,3}`{3,}circuittikz-designer-test\\s*$")

// bareFenceLine matches a closing fence: backticks only, no info string —
// distinct from fenceDelimLine above, which also matches an *opening*
// fence (backticks followed by a language tag).
var bareFenceLine = regexp.MustCompile("^ {0,3}`{3,}\\s*$")

// unwrapCircuitTikzDesignerBlocks replaces a CircuiTikZ Designer block
// with a plain ```svg block containing just its "svg" field. The
// plugin's fenced block isn't raw tikz or raw SVG — it's a JSON blob
// describing the diagram editor's internal state (component positions,
// rotations, etc.), which happens to also carry a fully pre-rendered SVG
// string as one field (unlike tikz, nothing needs compiling here). After
// this pass, such a block is indistinguishable from a native Obsidian
// SVG Editor block — see tree.go's "svg" case — so no separate AST-level
// handling for this plugin is needed at all.
//
// The extracted SVG also has every var(--bs-emphasis-color) — a
// Bootstrap custom property the plugin relies on, defined by Obsidian's
// own app shell but by nothing on digitalgarden-frontend's page —
// rewritten to currentColor, which the SVG already uses in a handful of
// places itself. Without this, every stroke/fill using that undefined
// variable resolves to nothing (CSS: an invalid var() reference falls
// back to the property's inherited/initial value, which for stroke/fill
// is effectively invisible), so a real circuit diagram rendered on the
// site showed only its text labels and none of its wires — confirmed by
// counting the actual attribute usages in Circuits.md's payload (54
// stroke, 15 fill referencing the missing variable, vs. 4 each already
// using currentColor and rendering fine).
//
// A block that doesn't parse as JSON, or has no "svg" field, is left
// completely untouched rather than dropped or guessed at: it still shows
// up downstream as an unrecognized codeBlock (lang
// "circuittikz-designer-test") instead of silently vanishing.
func unwrapCircuitTikzDesignerBlocks(source []byte) []byte {
	lines := bytes.Split(source, []byte("\n"))
	out := make([][]byte, 0, len(lines))

	for i := 0; i < len(lines); i++ {
		line := lines[i]
		if !circuitTikzFenceOpen.Match(line) {
			out = append(out, line)
			continue
		}

		end := -1
		for j := i + 1; j < len(lines); j++ {
			if bareFenceLine.Match(lines[j]) {
				end = j
				break
			}
		}
		if end == -1 {
			// No closing fence found — malformed/truncated block, leave
			// it alone rather than guessing where it ends.
			out = append(out, line)
			continue
		}

		var payload struct {
			SVG string `json:"svg"`
		}
		content := bytes.Join(lines[i+1:end], []byte("\n"))
		if err := json.Unmarshal(content, &payload); err != nil || payload.SVG == "" {
			for _, l := range lines[i : end+1] {
				out = append(out, l)
			}
			i = end
			continue
		}

		svg := bytes.ReplaceAll([]byte(payload.SVG), []byte("var(--bs-emphasis-color)"), []byte("currentColor"))
		out = append(out, []byte("```svg"), svg, []byte("```"))
		i = end
	}

	return bytes.Join(out, []byte("\n"))
}
