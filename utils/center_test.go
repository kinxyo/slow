package utils

import "testing"

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
