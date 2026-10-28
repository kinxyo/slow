package utils

import (
	"regexp"
	"strings"

	"github.com/mattn/go-runewidth"
)

// escapeSeq matches CSI sequences (colors, cursor) and OSC sequences (text
// sizing, hyperlinks) so that they do not count towards a line's width.
var escapeSeq = regexp.MustCompile(`\x1b(\[[0-9;?:]*[ -/]*[@-~]|\][^\x07\x1b]*(\x07|\x1b\\))`)

// CenterBlock pads the left of every line so that the block sits in the
// middle of a viewport width columns wide. The block's width is that of its
// widest line, trailing spaces excluded. A block as wide as the viewport, or
// an unknown viewport width (0), leaves s as it is.
func CenterBlock(s string, width int) string {
	lines := strings.Split(s, "\n")
	widest := 0
	for _, l := range lines {
		visible := strings.TrimRight(escapeSeq.ReplaceAllString(l, ""), " ")
		widest = max(widest, runewidth.StringWidth(visible))
	}
	pad := (width - widest) / 2
	if width <= 0 || pad <= 0 {
		return s
	}
	prefix := strings.Repeat(" ", pad)
	for i, l := range lines {
		if l != "" {
			lines[i] = prefix + l
		}
	}
	return strings.Join(lines, "\n")
}
