package utils

import (
	"regexp"
	"strings"

	"github.com/mattn/go-runewidth"
)

// escapeSeq matches CSI sequences (colors, cursor) and OSC sequences (text
// sizing, hyperlinks) so that they do not count towards a line's width.
var escapeSeq = regexp.MustCompile(`\x1b(\[[0-9;?:]*[ -/]*[@-~]|\][^\x07\x1b]*(\x07|\x1b\\))`)

// BlockWidth is the width in columns of the widest line of s, ignoring escape
// sequences and trailing spaces.
func BlockWidth(s string) int {
	widest := 0
	for _, l := range strings.Split(s, "\n") {
		visible := strings.TrimRight(escapeSeq.ReplaceAllString(l, ""), " ")
		widest = max(widest, runewidth.StringWidth(visible))
	}
	return widest
}

// CenterBlock pads the left of every line so that the block sits in the
// middle of a viewport width columns wide. The block's width is that of its
// widest line. A block as wide as the viewport, or an unknown viewport width
// (0), leaves s as it is.
func CenterBlock(s string, width int) string {
	s, _ = CenterBlockPad(s, width)
	return s
}

// CenterBlockPad is CenterBlock that also reports the padding it added.
func CenterBlockPad(s string, width int) (string, int) {
	pad := (width - BlockWidth(s)) / 2
	if width <= 0 || pad <= 0 {
		return s, 0
	}
	lines := strings.Split(s, "\n")
	prefix := strings.Repeat(" ", pad)
	for i, l := range lines {
		if l != "" {
			lines[i] = prefix + l
		}
	}
	return strings.Join(lines, "\n"), pad
}
