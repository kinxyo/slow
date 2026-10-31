package ui

import (
	"strings"
	"testing"
	"time"

	"github.com/kinxyo/slow/utils"
)

func TestDebugInfoLine(t *testing.T) {
	d := debugInfo{
		termW: 200, termH: 50,
		viewW: 200, viewH: 48,
		column: 100,
		stats: renderStats{
			wrap: 100, lines: 1770, widest: 100, bytes: 118000,
			took: 12500 * time.Microsecond, mermaid: utils.MermaidStats{Drawn: 18},
		},
		renders: 3, yOffset: 120, totalLines: 1770, percent: 0.067,
	}
	got := d.line(0)
	for _, want := range []string{
		"term 200×50", "view 200×48", "col 100 wrap 100",
		"1770 lines, widest 100", "118 kB", "y 120/1770 (6%)",
		"render 12.5ms #3", "mermaid 18 drawn, 0 plain", "sizing off",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in %q", want, got)
		}
	}

	d.stats.mermaid = utils.MermaidStats{}
	if strings.Contains(d.line(0), "mermaid") {
		t.Error("a document without mermaid blocks should not mention them")
	}

	cut := d.line(40)
	if w := utils.BlockWidth(cut); w > 40 || !strings.HasSuffix(cut, "…") {
		t.Errorf("the bar should be cut to 40 columns with an ellipsis, got %d wide: %q", w, cut)
	}
}

func TestGlamourRenderStats(t *testing.T) {
	t.Setenv("SLO_TEXT_SIZING", "on") // detected once per process, see TestDebugBarToggle
	m := newTestPager(100, 200, 40)
	doc := strings.Repeat("lorem ipsum dolor sit amet consectetur ", 80) +
		"\n\n```mermaid\nflowchart TD\n  A --> B\n```\n\n```mermaid\nsequenceDiagram\n  A->>B: hi\n```\n"

	r, err := renderDocument(m, doc)
	if err != nil {
		t.Fatal(err)
	}
	out, stats := r.body, r.stats
	if stats.wrap != 100 {
		t.Errorf("wrap = %d, want 100", stats.wrap)
	}
	if stats.widest < 80 || stats.widest > 100 {
		t.Errorf("widest = %d, want about 100", stats.widest)
	}
	if stats.lines != strings.Count(out, "\n")+1 || stats.bytes != len(out) {
		t.Errorf("lines/bytes = %d/%d, output has %d/%d", stats.lines, stats.bytes, strings.Count(out, "\n")+1, len(out))
	}
	if stats.mermaid.Drawn != 1 || stats.mermaid.Fallback != 1 {
		t.Errorf("mermaid = %+v, want 1 drawn and 1 plain", stats.mermaid)
	}
	if stats.took <= 0 {
		t.Error("the render time should be measured")
	}

	// wrap follows the terminal when it is narrower than the column
	narrow, err := renderDocument(newTestPager(100, 60, 40), doc)
	if err != nil {
		t.Fatal(err)
	}
	if narrow.stats.wrap != 60 {
		t.Errorf("wrap on a 60 column terminal = %d, want 60", narrow.stats.wrap)
	}
}

func TestDebugBarToggle(t *testing.T) {
	// the bar asks whether text sizing is on, and that answer is detected once
	// per process; set it the way the other pager tests expect to find it
	t.Setenv("SLO_TEXT_SIZING", "on")
	m := newTestPager(100, 200, 40)
	m.setContent(strings.TrimSuffix(strings.Repeat("line\n", 100), "\n"))
	base := m.viewport.Height()

	if strings.Contains(m.View(), "term 200×40") {
		t.Fatal("the debug bar should start hidden")
	}

	m, _ = m.update(key("D"))
	if !m.showDebug || m.viewport.Height() != base-1 {
		t.Fatalf("showDebug=%v height=%d, want true and %d", m.showDebug, m.viewport.Height(), base-1)
	}
	if !strings.Contains(m.View(), "term 200×40") {
		t.Errorf("the bar should be drawn above the status bar:\n%s", m.View())
	}

	m, _ = m.update(key("D"))
	if m.showDebug || m.viewport.Height() != base {
		t.Errorf("toggling again should restore the height: showDebug=%v height=%d", m.showDebug, m.viewport.Height())
	}
}

func TestDebugBarFlagStartsVisible(t *testing.T) {
	common := &commonModel{cfg: Config{ShowDebugBar: true, GlamourMaxWidth: 100}, styles: newStyles(true)}
	if m := newPagerModel(common); !m.showDebug {
		t.Error("--debug-bar should show the bar from the start")
	}
}
