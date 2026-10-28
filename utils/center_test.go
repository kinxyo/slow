package utils

import (
	"strings"
	"testing"
)

func TestCenterBlock(t *testing.T) {
	cases := []struct {
		name  string
		in    string
		width int
		want  string
	}{
		{"centers on the widest line", "abcd\nab\n\nabcd", 10, "   abcd\n   ab\n\n   abcd"},
		{"odd space rounds down", "abc", 8, "  abc"},
		{"full width is untouched", "abcd", 4, "abcd"},
		{"wider than the viewport is untouched", "abcdef", 4, "abcdef"},
		{"unknown viewport is untouched", "abc", 0, "abc"},
		{"escape sequences and trailing spaces do not count", "\x1b[31mab\x1b[0m   \n\x1b]66;s=2;hi\x07", 10, "    \x1b[31mab\x1b[0m   \n    \x1b]66;s=2;hi\x07"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := CenterBlock(c.in, c.width); got != c.want {
				t.Errorf("CenterBlock(%q, %d) = %q, want %q", c.in, c.width, got, c.want)
			}
		})
	}
}

func TestBlockWidth(t *testing.T) {
	if got := BlockWidth("ab\n\x1b[31mabcd\x1b[0m   \nabc"); got != 4 {
		t.Errorf("BlockWidth = %d, want 4", got)
	}
	if got := BlockWidth(""); got != 0 {
		t.Errorf("BlockWidth of nothing = %d, want 0", got)
	}
}

func TestCenterPaddingAndPadBlock(t *testing.T) {
	cases := []struct{ block, width, want int }{
		{4, 10, 3}, {4, 4, 0}, {6, 4, 0}, {4, 0, 0}, {0, 7, 3},
	}
	for _, c := range cases {
		if got := CenterPadding(c.block, c.width); got != c.want {
			t.Errorf("CenterPadding(%d, %d) = %d, want %d", c.block, c.width, got, c.want)
		}
	}
	if got := PadBlock("ab\n\ncd", 2); got != "  ab\n\n  cd" {
		t.Errorf("PadBlock = %q", got)
	}
	if got := PadBlock("ab\n", 2); got != "  ab\n" {
		t.Errorf("PadBlock keeps a final newline, got %q", got)
	}
	if got := PadBlock("ab", 0); got != "ab" {
		t.Errorf("PadBlock with no padding = %q", got)
	}
}

func TestBlockWidthSequences(t *testing.T) {
	cases := map[string]int{
		"plain":                              5,
		"\x1b[38;5;252mhi\x1b[m   ":          2,
		"a\x1b]8;;http://x\x07b\x1b]8;;\x07": 2,
		"\x1b]66;s=2;hi\x1b\\":               0,
		"日本":                                 4,
		"a\x1b":                              1, // an escape cut short
		"\x1b[31":                            0,
		"   ":                                0,
	}
	for in, want := range cases {
		if got := BlockWidth(in); got != want {
			t.Errorf("BlockWidth(%q) = %d, want %d", in, got, want)
		}
	}
}

func BenchmarkBlockWidth(b *testing.B) {
	line := "\x1b[38;5;252m" + strings.Repeat("\x1b[38;5;252m \x1b[m", 80) + "\n"
	s := strings.Repeat(line, 2000)
	for b.Loop() {
		BlockWidth(s)
	}
}
