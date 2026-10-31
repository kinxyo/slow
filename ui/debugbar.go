package ui

import (
	"fmt"
	"strings"
	"time"

	"github.com/dustin/go-humanize"
	"github.com/muesli/reflow/truncate"

	"github.com/kinxyo/slow/utils"
)

// renderStats describes one render of the document, for the debug bar.
type renderStats struct {
	wrap    int           // width handed to glamour, 0 when not wrapping
	lines   int           // lines of rendered text
	widest  int           // width of the widest line
	bytes   int           // size of the rendered text
	took    time.Duration // time the render took, mermaid included
	mermaid utils.MermaidStats
}

// debugInfo is everything the debug bar shows.
type debugInfo struct {
	termW, termH int
	viewW, viewH int
	column       int
	stats        renderStats
	renders      int
	layouts      int           // times the text was only laid out again
	layoutTook   time.Duration // the last of them
	yOffset      int
	totalLines   int
	percent      float64
	textSizing   bool
}

// line formats the bar, cut to width columns (0 for no limit).
func (d debugInfo) line(width int) string {
	parts := []string{
		fmt.Sprintf("term %d×%d", d.termW, d.termH),
		fmt.Sprintf("view %d×%d", d.viewW, d.viewH),
		fmt.Sprintf("col %d wrap %d", d.column, d.stats.wrap),
		fmt.Sprintf("%d lines, widest %d, %s", d.stats.lines, d.stats.widest, humanize.Bytes(uint64(max(0, d.stats.bytes)))),
		fmt.Sprintf("y %d/%d (%d%%)", d.yOffset, d.totalLines, int(d.percent*100)),
		fmt.Sprintf("render %.1fms #%d", float64(d.stats.took.Microseconds())/1000, d.renders),
	}
	if d.layouts > 0 {
		parts = append(parts, fmt.Sprintf("relayout %.1fms #%d", float64(d.layoutTook.Microseconds())/1000, d.layouts))
	}
	if n := d.stats.mermaid; n.Drawn+n.Fallback > 0 {
		parts = append(parts, fmt.Sprintf("mermaid %d drawn, %d plain", n.Drawn, n.Fallback))
	}
	sizing := "off"
	if d.textSizing {
		sizing = "on"
	}
	parts = append(parts, "sizing "+sizing)

	s := " " + strings.Join(parts, " · ")
	if width > 0 {
		s = truncate.StringWithTail(s, uint(width), "…")
	}
	return s
}

// debugInfo gathers the numbers the debug bar shows.
func (m pagerModel) debugInfo() debugInfo {
	return debugInfo{
		termW:      m.common.width,
		termH:      m.common.height,
		viewW:      m.viewport.Width(),
		viewH:      m.viewport.Height(),
		column:     m.column,
		stats:      m.stats,
		renders:    m.renders,
		layouts:    m.layouts,
		layoutTook: m.layoutTook,
		yOffset:    m.viewport.YOffset(),
		totalLines: m.viewport.TotalLineCount(),
		percent:    m.viewport.ScrollPercent(),
		textSizing: utils.TextSizingEnabled(),
	}
}

func (m pagerModel) debugBarView() string {
	return m.common.styles.subtleStyle.Render(m.debugInfo().line(m.common.width))
}
