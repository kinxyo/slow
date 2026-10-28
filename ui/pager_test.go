package ui

import (
	"math"
	"regexp"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	runewidth "github.com/mattn/go-runewidth"
)

func TestGlamourRenderTextSizing(t *testing.T) {
	t.Setenv("SLO_TEXT_SIZING", "on")

	common := &commonModel{
		cfg: Config{
			GlamourEnabled:  true,
			GlamourStyle:    "dark",
			GlamourMaxWidth: 80,
		},
		styles: newStyles(true),
		width:  80,
		height: 24,
	}
	m := newPagerModel(common)
	m.currentDocument = markdown{Note: "test.md"}
	m.setSize(80, 24)

	out, err := glamourRender(m, "# Hello\n\n## World\n")
	if err != nil {
		t.Fatal(err)
	}
	// The dark style's H1 has a background color, which renders broken when
	// scaled, so it is left at normal size.
	if strings.Contains(out, "\x1b]66;s=3;") {
		t.Errorf("expected H1 with background to not be scaled, got %q", out)
	}
	if !strings.Contains(out, "Hello") {
		t.Errorf("expected H1 text in pager output, got %q", out)
	}
	if !strings.Contains(out, "\x1b]66;s=2;World\x1b\\") {
		t.Errorf("expected H2 at 2x scale in pager output, got %q", out)
	}
	if strings.Contains(out, "\x1b]6666;") {
		t.Errorf("expected no leftover markers in pager output, got %q", out)
	}
}

var ansiSeq = regexp.MustCompile(`\x1b(\[[0-9;?:]*[ -/]*[@-~]|\][^\x07\x1b]*(\x07|\x1b\\))`)

func widestLine(s string) int {
	widest := 0
	for _, l := range strings.Split(ansiSeq.ReplaceAllString(s, ""), "\n") {
		widest = max(widest, runewidth.StringWidth(strings.TrimRight(l, " ")))
	}
	return widest
}

// leadingSpaces is the padding shared by every non-blank line.
func leadingSpaces(s string) int {
	pad := -1
	for _, l := range strings.Split(ansiSeq.ReplaceAllString(s, ""), "\n") {
		if strings.TrimSpace(l) == "" {
			continue
		}
		n := len(l) - len(strings.TrimLeft(l, " "))
		if pad < 0 || n < pad {
			pad = n
		}
	}
	return max(pad, 0)
}

func newTestPager(column, width, height int) pagerModel {
	common := &commonModel{
		cfg: Config{
			GlamourEnabled:  true,
			GlamourStyle:    "dark",
			GlamourMaxWidth: uint(column), //nolint:gosec
		},
		styles: newStyles(true),
		width:  width,
		height: height,
	}
	m := newPagerModel(common)
	m.currentDocument = markdown{Note: "test.md"}
	m.setSize(width, height)
	return m
}

func key(s string) tea.KeyPressMsg {
	return tea.KeyPressMsg{Code: rune(s[0]), Text: s}
}

func TestAdjustColumn(t *testing.T) {
	cases := []struct{ column, delta, want int }{
		{100, columnStep, 110},
		{100, -columnStep, 90},
		{columnMin, -columnStep, columnMin},
		{columnMin + 3, -columnStep, columnMin},
		{columnMax, columnStep, columnMax},
	}
	for _, c := range cases {
		if got := adjustColumn(c.column, c.delta); got != c.want {
			t.Errorf("adjustColumn(%d, %d) = %d, want %d", c.column, c.delta, got, c.want)
		}
	}
}

func TestColumnKeys(t *testing.T) {
	m := newTestPager(100, 200, 40)
	for _, step := range []struct {
		key  string
		want int
	}{{"+", 110}, {"=", 120}, {"-", 110}, {"_", 100}, {"-", 90}, {"0", 100}} {
		m, _ = m.update(key(step.key))
		if m.column != step.want {
			t.Fatalf("after %q column = %d, want %d", step.key, m.column, step.want)
		}
	}

	for range 10 {
		m, _ = m.update(key("-"))
	}
	if m.column != columnMin {
		t.Errorf("column should stop at %d, got %d", columnMin, m.column)
	}
}

func TestColumnKeysIgnoredWhenWrappingIsOff(t *testing.T) {
	m := newTestPager(0, 200, 40)
	m, _ = m.update(key("+"))
	if m.column != 0 {
		t.Errorf("column = %d, want 0 while wrapping is off", m.column)
	}
}

func TestColumnWidthShapesTheText(t *testing.T) {
	doc := strings.Repeat("lorem ipsum dolor sit amet consectetur ", 80) + "\n"
	m := newTestPager(100, 200, 40)

	out, err := glamourRender(m, doc)
	if err != nil {
		t.Fatal(err)
	}
	pad := leadingSpaces(out)
	wide := widestLine(out) - pad
	if wide > 100 || wide < 80 {
		t.Errorf("a 100 column text should be about 100 wide, got %d", wide)
	}
	if want := (200 - wide) / 2; pad < want-1 || pad > want+1 {
		t.Errorf("the text should be centered on 200 columns: padding %d, want about %d", pad, want)
	}

	for range 3 {
		m, _ = m.update(key("-"))
	}
	narrow, err := glamourRender(m, doc)
	if err != nil {
		t.Fatal(err)
	}
	npad := leadingSpaces(narrow)
	nw := widestLine(narrow) - npad
	if nw > 70 || nw < 55 || nw >= wide {
		t.Errorf("after narrowing to 70 the text should be about 70 wide, got %d (was %d)", nw, wide)
	}
	if npad <= pad {
		t.Errorf("a narrower column leaves more room on the sides: padding %d after, %d before", npad, pad)
	}
}

func TestColumnNeverExceedsTheTerminal(t *testing.T) {
	m := newTestPager(100, 60, 40)
	out, err := glamourRender(m, strings.Repeat("word ", 200)+"\n")
	if err != nil {
		t.Fatal(err)
	}
	if w := widestLine(out); w > 60 {
		t.Errorf("text is %d wide on a 60 column terminal", w)
	}
}

func TestPlaceIsKeptWhenTheTextReflows(t *testing.T) {
	m := newTestPager(100, 120, 20)
	m.setContent(strings.TrimSuffix(strings.Repeat("line\n", 200), "\n"))
	m.viewport.SetYOffset(90) // halfway: 90 of 200-20 lines

	m, _ = m.update(key("+"))
	if !m.keepPlace {
		t.Fatal("changing the column should remember the place")
	}
	m, _ = m.update(contentRenderedMsg(strings.TrimSuffix(strings.Repeat("line\n", 400), "\n")))
	h := float64(m.viewport.Height())
	want := int(math.Round(90 / (200 - h) * (400 - h)))
	if got := m.viewport.YOffset(); got != want {
		t.Errorf("offset = %d, want %d (the same place in 400 lines)", got, want)
	}
	if m.keepPlace {
		t.Error("the place should be used up once applied")
	}
}
