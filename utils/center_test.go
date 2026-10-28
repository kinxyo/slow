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

func TestBlockWidth(t *testing.T) {
	if got := BlockWidth("ab\n\x1b[31mabcd\x1b[0m   \nabc"); got != 4 {
		t.Errorf("BlockWidth = %d, want 4", got)
	}
	if got := BlockWidth(""); got != 0 {
		t.Errorf("BlockWidth of nothing = %d, want 0", got)
	}
}

func TestCenterBlockPad(t *testing.T) {
	out, pad := CenterBlockPad("abcd", 10)
	if pad != 3 || out != "   abcd" {
		t.Errorf("got %q, %d", out, pad)
	}
	if out, pad := CenterBlockPad("abcd", 4); pad != 0 || out != "abcd" {
		t.Errorf("a full-width block is untouched, got %q, %d", out, pad)
	}
}
