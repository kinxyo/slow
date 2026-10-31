package utils

import (
	"strings"
	"unicode/utf8"

	"github.com/mattn/go-runewidth"
)

// lineWidth is the width in columns of a line as the terminal shows it:
// escape sequences (colors, cursor, text sizing, hyperlinks) take no room and
// trailing spaces are not counted.
func lineWidth(l string) int {
	w, end := 0, 0 // end is the width up to the last character that is not a space
	for i := 0; i < len(l); {
		c := l[i]
		if c == 0x1b {
			i = skipEscape(l, i)
			continue
		}
		r, size := rune(c), 1
		if c >= utf8.RuneSelf {
			r, size = utf8.DecodeRuneInString(l[i:])
		}
		i += size
		w += runewidth.RuneWidth(r)
		if r != ' ' {
			end = w
		}
	}
	return end
}

// skipEscape returns the index after the escape sequence that starts at l[i].
func skipEscape(l string, i int) int {
	i++ // ESC
	if i >= len(l) {
		return i
	}
	switch l[i] {
	case '[': // CSI: parameters, then a final byte from 0x40 to 0x7e
		for i++; i < len(l); i++ {
			if l[i] >= 0x40 && l[i] <= 0x7e {
				return i + 1
			}
		}
	case ']': // OSC: up to BEL or ST (ESC \)
		for i++; i < len(l); i++ {
			if l[i] == 0x07 {
				return i + 1
			}
			if l[i] == 0x1b && i+1 < len(l) && l[i+1] == '\\' {
				return i + 2
			}
		}
	default: // two-character sequence
		return i + 1
	}
	return i
}

// BlockWidth is the width in columns of the widest line of s, ignoring escape
// sequences and trailing spaces.
func BlockWidth(s string) int {
	widest := 0
	for len(s) > 0 {
		line := s
		if i := strings.IndexByte(s, '\n'); i >= 0 {
			line, s = s[:i], s[i+1:]
		} else {
			s = ""
		}
		widest = max(widest, lineWidth(line))
	}
	return widest
}
