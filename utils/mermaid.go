package utils

import (
	"fmt"
	"strings"
)

// mermaidWraps are the label widths tried, widest first, until the diagram
// fits the available width.
var mermaidWraps = []int{28, 26, 24, 22, 20, 18, 16, 14, 12}

// RenderMermaid replaces every ```mermaid block of the markdown with a text
// drawing of the diagram. width is the number of columns available, 0 for
// unlimited. A block that cannot be drawn is left as it is.
func RenderMermaid(md string, width int) string {
	out, _ := RenderMermaidStats(md, width)
	return out
}

// MermaidStats counts what RenderMermaidStats did with the mermaid blocks.
type MermaidStats struct {
	Drawn    int // replaced by a drawing
	Fallback int // left as source
}

// RenderMermaidStats is RenderMermaid that also reports how many blocks were
// drawn and how many were left as they were.
func RenderMermaidStats(md string, width int) (string, MermaidStats) {
	var stats MermaidStats
	if !strings.Contains(md, "mermaid") {
		return md, stats
	}

	lines := strings.Split(md, "\n")
	var out []string
	for i := 0; i < len(lines); i++ {
		fence, info, ok := openingFence(lines[i])
		if !ok {
			out = append(out, lines[i])
			continue
		}
		end := closingFence(lines, i+1, fence)
		block := lines[i:min(end+1, len(lines))]
		if end < len(lines) && isMermaidInfo(info) {
			if art, err := renderMermaidBlock(strings.Join(lines[i+1:end], "\n"), width); err == nil {
				block = fencedText(art, lines[i])
				stats.Drawn++
			} else {
				stats.Fallback++
			}
		}
		out = append(out, block...)
		i = end
	}
	return strings.Join(out, "\n"), stats
}

// openingFence recognizes a code fence line and returns its marker.
func openingFence(line string) (marker, info string, ok bool) {
	trimmed := strings.TrimLeft(line, " ")
	if len(line)-len(trimmed) > 3 || len(trimmed) < 3 {
		return "", "", false
	}
	c := trimmed[0]
	if c != '`' && c != '~' {
		return "", "", false
	}
	n := 0
	for n < len(trimmed) && trimmed[n] == c {
		n++
	}
	if n < 3 {
		return "", "", false
	}
	info = strings.TrimSpace(trimmed[n:])
	if c == '`' && strings.Contains(info, "`") {
		return "", "", false
	}
	return trimmed[:n], info, true
}

// closingFence returns the index of the line closing a fence, or len(lines).
func closingFence(lines []string, from int, marker string) int {
	for i := from; i < len(lines); i++ {
		t := strings.TrimSpace(lines[i])
		if strings.HasPrefix(t, marker) && strings.Trim(t, marker[:1]) == "" {
			return i
		}
	}
	return len(lines)
}

func isMermaidInfo(info string) bool {
	f := strings.Fields(info)
	return len(f) > 0 && strings.EqualFold(f[0], "mermaid")
}

// fencedText wraps a drawing in a plain text fence, indented like the
// fence it replaces.
func fencedText(art, opening string) []string {
	indent := opening[:len(opening)-len(strings.TrimLeft(opening, " "))]
	marker := "```"
	for strings.Contains(art, marker) {
		marker += "`"
	}
	out := []string{indent + marker + "text"}
	for _, l := range strings.Split(art, "\n") {
		out = append(out, indent+l)
	}
	return append(out, indent+marker)
}

func renderMermaidBlock(src string, width int) (string, error) {
	g, err := parseMermaid(src)
	if err != nil {
		return "", err
	}
	// glamour indents code blocks, so keep a few columns in reserve
	limit := 0
	if width > 0 {
		limit = max(width-6, 20)
	}
	var lastErr error
	for _, wrap := range mermaidWraps {
		l, err := layoutMermaid(g, wrap)
		if err != nil {
			lastErr = err
			continue
		}
		if limit == 0 || l.width <= limit {
			return l.draw(), nil
		}
		lastErr = fmt.Errorf("diagram is %d columns wide, %d available", l.width, limit)
	}
	return "", lastErr
}
