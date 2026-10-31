package utils

import (
	"testing"
)

func TestBlockWidth(t *testing.T) {
	if got := BlockWidth("ab\n\x1b[31mabcd\x1b[0m   \nabc"); got != 4 {
		t.Errorf("BlockWidth = %d, want 4", got)
	}
	if got := BlockWidth("\x1b]66;s=2;hi\x07\n\x1b]8;;http://x\x1b\\abc\x1b]8;;\x1b\\"); got != 3 {
		t.Errorf("BlockWidth with OSC sequences = %d, want 3", got)
	}
	if got := BlockWidth(""); got != 0 {
		t.Errorf("BlockWidth of nothing = %d, want 0", got)
	}
}
