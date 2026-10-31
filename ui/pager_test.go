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

	r, err := renderDocument(m, "# Hello\n\n## World\n")
	out := r.body
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

	outr, err := renderDocument(m, doc)
	out := outr.body
	if err != nil {
		t.Fatal(err)
	}
	pad := leadingSpaces(out)
	wide := widestLine(out) - pad
	if wide > 100 || wide < 80 {
		t.Errorf("a 100 column text should be about 100 wide, got %d", wide)
	}
	if pad > 2 {
		t.Errorf("the text should start at the left edge: padding %d", pad)
	}

	for range 3 {
		m, _ = m.update(key("-"))
	}
	narrowr, err := renderDocument(m, doc)
	narrow := narrowr.body
	if err != nil {
		t.Fatal(err)
	}
	npad := leadingSpaces(narrow)
	nw := widestLine(narrow) - npad
	if nw > 70 || nw < 55 || nw >= wide {
		t.Errorf("after narrowing to 70 the text should be about 70 wide, got %d (was %d)", nw, wide)
	}
	if npad != pad {
		t.Errorf("a narrower column keeps the text at the left edge: padding %d after, %d before", npad, pad)
	}
}

func TestColumnNeverExceedsTheTerminal(t *testing.T) {
	m := newTestPager(100, 60, 40)
	r, err := renderDocument(m, strings.Repeat("word ", 200)+"\n")
	if err != nil {
		t.Fatal(err)
	}
	out := r.body
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
	m, _ = m.update(contentRenderedMsg{body: strings.TrimSuffix(strings.Repeat("line\n", 400), "\n")})
	h := float64(m.viewport.Height())
	want := int(math.Round(90 / (200 - h) * (400 - h)))
	if got := m.viewport.YOffset(); got != want {
		t.Errorf("offset = %d, want %d (the same place in 400 lines)", got, want)
	}
	if m.keepPlace {
		t.Error("the place should be used up once applied")
	}
}

const relayoutDoc = "# Title\n\nSome words in a paragraph that is long enough to wrap, " +
	"repeated. Some words in a paragraph that is long enough to wrap, repeated. " +
	"Some words in a paragraph that is long enough to wrap, repeated.\n\n" +
	"- one\n- two\n\n```go\nfmt.Println(\"hi\")\n```\n\n```mermaid\nflowchart TD\n  A --> B\n```\n"

// show applies a render the way the program does when its message arrives.
func show(t *testing.T, m pagerModel, doc string) pagerModel {
	t.Helper()
	msg, ok := renderWithGlamour(m, doc)().(contentRenderedMsg)
	if !ok {
		t.Fatal("expected contentRenderedMsg")
	}
	m, _ = m.update(msg)
	return m
}

func TestRelayoutSkipsGlamourWhenTheWrapIsUnchanged(t *testing.T) {
	t.Setenv("SLO_TEXT_SIZING", "on") // detected once per process, see TestDebugBarToggle
	m := show(t, newTestPager(100, 200, 40), relayoutDoc)
	renders := m.renders

	// a wider terminal: the column is still 100, so glamour would wrap the same
	m.viewport.SetWidth(260)
	m.common.width = 260
	if cmd := m.relayout(); cmd != nil {
		t.Fatal("a wider terminal should not need another render")
	}
	if m.renders != renders || m.layouts != 1 {
		t.Errorf("renders %d→%d, layouts %d: only the layout should have run", renders, m.renders, m.layouts)
	}

	// and the result is what a full render at that width gives
	want, err := renderDocument(m, relayoutDoc)
	if err != nil {
		t.Fatal(err)
	}
	if got := m.viewport.View(); strings.TrimSpace(ansiSeq.ReplaceAllString(got, "")) == "" {
		t.Fatal("nothing on screen")
	}
	if m.stats.lines != want.stats.lines || m.stats.bytes != want.stats.bytes {
		t.Errorf("lines/bytes = %d/%d after relayout, %d/%d from a full render",
			m.stats.lines, m.stats.bytes, want.stats.lines, want.stats.bytes)
	}
	if m.viewport.TotalLineCount() != want.stats.lines {
		t.Errorf("the viewport holds %d lines, a full render has %d", m.viewport.TotalLineCount(), want.stats.lines)
	}
}

func TestRelayoutRendersAgainWhenTheWrapChanges(t *testing.T) {
	t.Setenv("SLO_TEXT_SIZING", "on")
	m := show(t, newTestPager(100, 80, 40), relayoutDoc) // the terminal is the limit: wrap 80
	if m.textWrap != 80 {
		t.Fatalf("textWrap = %d, want 80", m.textWrap)
	}

	m.viewport.SetWidth(90)
	m.common.width = 90
	if cmd := m.relayout(); cmd == nil {
		t.Fatal("a different wrap width needs a new render")
	}
	if !m.keepPlace {
		t.Error("the reader should keep their place across the new render")
	}
}

func TestRelayoutNeedsAnotherRenderForLineNumbers(t *testing.T) {
	t.Setenv("SLO_TEXT_SIZING", "on")
	m := show(t, newTestPager(100, 200, 40), relayoutDoc)
	m.common.cfg.ShowLineNumbers = true
	if cmd := m.relayout(); cmd == nil {
		t.Error("line numbers are not handled by the quick path")
	}
}

func TestRelayoutUsesTheTextThatWasRendered(t *testing.T) {
	t.Setenv("SLO_TEXT_SIZING", "on")
	m := newTestPager(100, 80, 40)
	m.currentDocument.Body = "---\ntitle: x\n---\n\nfrontmatter was removed before rendering\n"
	m = show(t, m, "frontmatter was removed before rendering\n")

	m.viewport.SetWidth(70)
	m.common.width = 70
	cmd := m.relayout() // the wrap changes, so this renders again
	if cmd == nil {
		t.Fatal("expected a render")
	}
	msg := cmd().(contentRenderedMsg)
	if strings.Contains(msg.body, "title: x") || msg.src != "frontmatter was removed before rendering\n" {
		t.Errorf("a resize should render the text that was rendered before, not the raw document: %q", msg.src)
	}
}

func TestStaleRenderIsDropped(t *testing.T) {
	t.Setenv("SLO_TEXT_SIZING", "on")
	m := newTestPager(100, 200, 40)
	older := renderWithGlamour(m, "old text\n")()
	newer := renderWithGlamour(m, "new text\n")()

	// the newer render finishes first, then the older one
	m, _ = m.update(newer)
	m, _ = m.update(older)
	if got := ansiSeq.ReplaceAllString(m.viewport.View(), ""); !strings.Contains(got, "new text") || strings.Contains(got, "old text") {
		t.Errorf("the older render replaced the newer one:\n%s", got)
	}
	if m.renders != 1 {
		t.Errorf("renders = %d, want 1: the stale one is not counted", m.renders)
	}
}

func TestUnloadForgetsTheRender(t *testing.T) {
	t.Setenv("SLO_TEXT_SIZING", "on")
	m := show(t, newTestPager(100, 200, 40), relayoutDoc)
	m.unload()
	if m.text != "" || m.textSrc != "" {
		t.Error("leaving the document should drop the text kept for laying out again")
	}
}

func TestAFreshDocumentOpensAtTheTop(t *testing.T) {
	t.Setenv("SLO_TEXT_SIZING", "on")
	long := strings.Repeat("a paragraph\n\n", 200)

	// the size arrives while nothing is on screen yet, as it does at start-up
	m := newTestPager(100, 120, 20)
	m, cmd := m.update(tea.WindowSizeMsg{Width: 120, Height: 20})
	if cmd == nil {
		t.Fatal("expected a render")
	}
	m = show(t, m, long)
	if m.viewport.YOffset() != 0 {
		t.Errorf("the document opened at line %d of %d, want the top", m.viewport.YOffset(), m.viewport.TotalLineCount())
	}
}
