package utils

import (
	"fmt"
	"io"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"charm.land/glamour/v2/ansi"
	"github.com/muesli/cancelreader"
	"github.com/rivo/uniseg"
	"golang.org/x/term"
)

const (
	osc66ST         = "\x1b\\"
	osc66MaxPayload = 4000

	markerPrefix = "\x1b]6666;"
	markerClose  = markerPrefix + "0" + osc66ST
)

var textSizingEnabled = sync.OnceValue(detectTextSizing)

// TextSizingEnabled reports whether the terminal supports kitty's text
// sizing protocol (OSC 66), auto-detecting support on first call and caching
// the result. Set SLO_TEXT_SIZING to on/off to override detection.
func TextSizingEnabled() bool {
	return textSizingEnabled()
}

func detectTextSizing() bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv("SLO_TEXT_SIZING"))) {
	case "0", "false", "off", "no":
		return false
	case "1", "true", "on", "yes":
		return true
	}

	if !term.IsTerminal(int(os.Stdout.Fd())) {
		return false
	}
	if termEnv := os.Getenv("TERM"); termEnv == "" || termEnv == "dumb" {
		return false
	}

	if supported, decisive := probeTextSizing(); decisive {
		return supported
	}

	return os.Getenv("KITTY_WINDOW_ID") != "" ||
		os.Getenv("TERM_PROGRAM") == "kitty" ||
		strings.Contains(os.Getenv("TERM"), "kitty")
}

var cprPattern = regexp.MustCompile(`\x1b\[(\d+);(\d+)R`)

func probeTextSizing() (supported, decisive bool) {
	tty, err := os.OpenFile("/dev/tty", os.O_RDWR, 0)
	if err != nil {
		return false, false
	}
	defer func() { _ = tty.Close() }()

	fd := int(tty.Fd())
	oldState, err := term.MakeRaw(fd)
	if err != nil {
		return false, false
	}
	defer func() { _ = term.Restore(fd, oldState) }()

	reader, err := cancelreader.NewReader(tty)
	if err != nil {
		return false, false
	}
	defer func() { _ = reader.Close() }()

	if _, err := io.WriteString(tty, "\r\x1b[6n\x1b]66;w=2; \a\x1b[6n\r"); err != nil {
		return false, false
	}

	timer := time.AfterFunc(800*time.Millisecond, func() { reader.Cancel() })
	defer timer.Stop()

	var buf []byte
	tmp := make([]byte, 128)
	nextCol := func() (int, bool) {
		for {
			if loc := cprPattern.FindSubmatchIndex(buf); loc != nil {
				col, _ := strconv.Atoi(string(buf[loc[4]:loc[5]]))
				buf = buf[loc[1]:]
				return col, true
			}
			n, rerr := reader.Read(tmp)
			if n > 0 {
				buf = append(buf, tmp[:n]...)
			}
			if rerr != nil {
				return 0, false
			}
		}
	}

	before, ok := nextCol()
	if !ok {
		return false, false
	}
	after, ok := nextCol()
	if !ok {
		return false, true
	}
	return after-before == 2, true
}

func markerOpen(level int) string {
	return markerPrefix + strconv.Itoa(level) + osc66ST
}

func markerLevel(s string) int {
	if !strings.HasPrefix(s, markerPrefix) {
		return -1
	}
	rest := s[len(markerPrefix):]
	if len(rest) < 1+len(osc66ST) || rest[0] < '0' || rest[0] > '9' || !strings.HasPrefix(rest[1:], osc66ST) {
		return -1
	}
	return int(rest[0] - '0')
}

// AddHeadingSizeMarkers marks the heading styles of levels 1 to 3 in the
// given style config so that ApplyTextSizing can find the rendered heading
// text and scale it. Headings with a background color are left unmarked:
// the background does not cover the scaled glyphs and renders broken.
// The style config is modified in place; pass a copy if the original is
// shared.
func AddHeadingSizeMarkers(cfg *ansi.StyleConfig) {
	blocks := []struct {
		level int
		block *ansi.StyleBlock
	}{
		{1, &cfg.H1},
		{2, &cfg.H2},
		{3, &cfg.H3},
	}
	for _, b := range blocks {
		if b.block.BackgroundColor != nil {
			continue
		}
		prefix := b.block.Prefix
		if strings.HasPrefix(prefix, "#") {
			prefix = strings.TrimPrefix(strings.TrimLeft(prefix, "#"), " ")
		}
		b.block.Prefix = markerOpen(b.level) + prefix
		b.block.Suffix += markerClose
	}
}

func headingRows(level int) int {
	if level == 1 {
		return 3
	}
	return 2
}

// ApplyTextSizing replaces the heading markers inserted by
// AddHeadingSizeMarkers with kitty text sizing (OSC 66) sequences: level 1
// headings are scaled 3x, level 2 headings 2x, and level 3 headings 1.5x via
// fractional scaling with a tight width so glyphs keep their natural width.
// Output without markers is returned unchanged.
func ApplyTextSizing(s string) string {
	if !strings.Contains(s, markerPrefix) {
		return s
	}

	var b strings.Builder
	b.Grow(len(s))
	for len(s) > 0 {
		i := strings.Index(s, markerPrefix)
		if i < 0 {
			b.WriteString(s)
			break
		}
		b.WriteString(s[:i])
		s = s[i:]

		level := markerLevel(s)
		if level < 0 {
			b.WriteString(markerPrefix)
			s = s[len(markerPrefix):]
			continue
		}
		s = s[len(markerOpen(level)):]
		if level < 1 || level > 3 {
			continue
		}

		end := strings.Index(s, markerClose)
		if end < 0 {
			b.WriteString(s)
			break
		}
		writeSizedSpan(&b, level, s[:end])
		s = s[end+len(markerClose):]
		b.WriteString(strings.Repeat("\n", headingRows(level)-1))
	}
	return b.String()
}

func writeSizedSpan(b *strings.Builder, level int, span string) {
	lines := strings.Split(span, "\n")
	for i, ln := range lines {
		if i > 0 {
			b.WriteString(strings.Repeat("\n", headingRows(level)))
		}
		writeSizedLine(b, level, ln, i > 0)
	}
}

type segment struct {
	escape bool
	value  string
}

func writeSizedLine(b *strings.Builder, level int, ln string, continuation bool) {
	var segs []segment
	for len(ln) > 0 {
		i := strings.IndexByte(ln, '\x1b')
		if i < 0 {
			segs = append(segs, segment{value: ln})
			break
		}
		if i > 0 {
			segs = append(segs, segment{value: ln[:i]})
			ln = ln[i:]
			continue
		}
		seq := scanEscape(ln)
		segs = append(segs, segment{escape: true, value: seq})
		ln = ln[len(seq):]
	}

	firstContent, lastContent := -1, -1
	for i, sg := range segs {
		if !sg.escape && strings.Trim(sg.value, " \t") != "" {
			if firstContent < 0 {
				firstContent = i
			}
			lastContent = i
		}
	}

	if firstContent < 0 {
		b.WriteString(strings.Join(segmentValues(segs), ""))
		return
	}

	// The part of the line that is scaled: its text, and the escapes inside
	// it. Spaces after the last text stay as they are.
	var sized, after []segment
	for i, sg := range segs {
		text := sg.value
		switch {
		case i > lastContent:
			after = append(after, sg)
			continue
		case sg.escape:
			if i < firstContent {
				b.WriteString(text)
			} else {
				sized = append(sized, sg)
			}
			continue
		case i < firstContent && continuation:
			b.WriteString(text)
			continue
		}
		if i == firstContent && continuation {
			trimmed := strings.TrimLeft(text, " \t")
			b.WriteString(text[:len(text)-len(trimmed)])
			text = trimmed
		}
		var trail string
		if i == lastContent {
			trimmed := strings.TrimRight(text, " \t")
			trail, text = text[len(trimmed):], trimmed
		}
		if text != "" {
			sized = append(sized, segment{value: text})
		}
		if trail != "" {
			after = append([]segment{{value: trail}}, after...)
		}
	}
	writeSized(b, level, sized)
	b.WriteString(strings.Join(segmentValues(after), ""))
}

func segmentValues(segs []segment) []string {
	values := make([]string, len(segs))
	for i, sg := range segs {
		values[i] = sg.value
	}
	return values
}

// writeSized writes text scaled for a heading level. Level 3 headings are
// written in pieces that run on across style changes (see tightChunks).
func writeSized(b *strings.Builder, level int, segs []segment) {
	if level == 3 {
		t := tightChunks{b: b}
		for _, sg := range segs {
			if sg.escape {
				t.escape(sg.value)
			} else {
				t.text(printable(sg.value))
			}
		}
		t.flush()
		return
	}
	for _, sg := range segs {
		if sg.escape {
			b.WriteString(sg.value)
		} else {
			b.WriteString(sizedChunk(level, sg.value))
		}
	}
}

func scanEscape(s string) string {
	if len(s) < 2 {
		return s
	}
	switch s[1] {
	case '[':
		for i := 2; i < len(s); i++ {
			if s[i] >= 0x40 && s[i] <= 0x7e {
				return s[:i+1]
			}
		}
		return s
	case ']':
		for i := 2; i < len(s); i++ {
			if s[i] == '\a' {
				return s[:i+1]
			}
			if s[i] == '\x1b' && i+1 < len(s) && s[i+1] == '\\' {
				return s[:i+2]
			}
		}
		return s
	default:
		i := 1
		for i < len(s) && s[i] >= 0x20 && s[i] <= 0x2f {
			i++
		}
		if i < len(s) {
			i++
		}
		return s[:i]
	}
}

func sizedChunk(level int, text string) string {
	text = printable(text)
	if text == "" {
		return ""
	}
	switch level {
	case 1:
		return osc66Chunks("s=3", text)
	case 2:
		return osc66Chunks("s=2", text)
	default:
		return osc66TightChunks(text)
	}
}

// printable drops control characters, which have no place in an OSC 66 payload.
func printable(text string) string {
	return strings.Map(func(r rune) rune {
		if r < 0x20 || (r >= 0x7f && r <= 0x9f) {
			return -1
		}
		return r
	}, text)
}

func osc66Chunks(meta, text string) string {
	var b strings.Builder
	var chunk strings.Builder
	g := uniseg.NewGraphemes(text)
	for g.Next() {
		cluster := g.Str()
		if chunk.Len() > 0 && chunk.Len()+len(cluster) > osc66MaxPayload {
			writeOSC66(&b, meta, chunk.String())
			chunk.Reset()
		}
		chunk.WriteString(cluster)
	}
	if chunk.Len() > 0 {
		writeOSC66(&b, meta, chunk.String())
	}
	return b.String()
}

func osc66TightChunks(text string) string {
	var b strings.Builder
	t := tightChunks{b: &b}
	t.text(text)
	t.flush()
	return b.String()
}

// tightChunks writes text at 1.5x in pieces of 4 columns, the widest that
// scales to a whole number of cells (w=3). A narrower piece is given whole
// cells too and leaves a gap after it, so a piece runs on across style changes
// and only the last one of a line can be narrower. An escape met inside a
// piece is written after it: its first few characters keep the style before.
type tightChunks struct {
	b       *strings.Builder
	chunk   strings.Builder
	width   int
	pending []string
}

func (t *tightChunks) text(s string) {
	g := uniseg.NewGraphemes(s)
	for g.Next() {
		cluster := g.Str()
		cw := uniseg.StringWidth(cluster)
		if t.width > 0 && t.width+cw > 4 {
			t.flush()
		}
		t.chunk.WriteString(cluster)
		t.width += cw
	}
}

func (t *tightChunks) escape(seq string) {
	if t.chunk.Len() == 0 {
		t.b.WriteString(seq)
		return
	}
	t.pending = append(t.pending, seq)
}

func (t *tightChunks) flush() {
	if t.chunk.Len() > 0 {
		w := (t.width*3 + 3) / 4
		writeOSC66(t.b, fmt.Sprintf("s=2:n=3:d=4:w=%d", w), t.chunk.String())
		t.chunk.Reset()
		t.width = 0
	}
	for _, seq := range t.pending {
		t.b.WriteString(seq)
	}
	t.pending = nil
}

func writeOSC66(b *strings.Builder, meta, text string) {
	b.WriteString("\x1b]66;")
	b.WriteString(meta)
	b.WriteByte(';')
	b.WriteString(text)
	b.WriteString(osc66ST)
}
