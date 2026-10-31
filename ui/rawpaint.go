package ui

import (
	"io"
	"os"
	"strconv"
	"strings"
	"sync"
	"unicode/utf8"

	tea "charm.land/bubbletea/v2"
	"golang.org/x/term"
)

// rawPainter paints frames directly to the terminal. It is used when kitty
// text sizing support is detected: bubbletea's default renderer parses each
// frame into a cell buffer which discards OSC 66 sequences along with their
// payload text, so the program runs with [tea.WithoutRenderer] and this
// painter drives the terminal instead.
type rawPainter struct {
	w     io.Writer
	tty   *os.File
	state *term.State
	mouse bool
	done  chan struct{}

	mu      sync.Mutex
	gen     int
	painted int
}

func newRawPainter(w io.Writer, tty *os.File, state *term.State, mouse bool) *rawPainter {
	return &rawPainter{w: w, tty: tty, state: state, mouse: mouse, done: make(chan struct{})}
}

func (p *rawPainter) windowSize() tea.WindowSizeMsg {
	if p.tty != nil {
		if w, h, err := term.GetSize(int(p.tty.Fd())); err == nil && w > 0 && h > 0 {
			return tea.WindowSizeMsg{Width: w, Height: h}
		}
	}
	return tea.WindowSizeMsg{Width: 80, Height: 24}
}

func (p *rawPainter) start() {
	p.mu.Lock()
	defer p.mu.Unlock()
	_, _ = io.WriteString(p.w, "\x1b[?1049h\x1b[?25l")
	if p.mouse {
		_, _ = io.WriteString(p.w, "\x1b[?1002h\x1b[?1006h")
	}
}

func (p *rawPainter) stop() {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.mouse {
		_, _ = io.WriteString(p.w, "\x1b[?1002l\x1b[?1006l")
	}
	_, _ = io.WriteString(p.w, "\x1b[?25h\x1b[?1049l")
	if p.tty != nil {
		if p.state != nil {
			_ = term.Restore(int(p.tty.Fd()), p.state)
		}
		_ = p.tty.Close()
	}
	select {
	case <-p.done:
	default:
		close(p.done)
	}
}

// watchResize sends the initial window size to the program and reports
// terminal resizes. Without a renderer bubbletea never initializes the tty,
// so its own size reporting never happens.
func (p *rawPainter) watchResize(prog *tea.Program) {
	ch := make(chan os.Signal, 1)
	winchNotify(ch)
	go func() {
		prog.Send(p.windowSize())
		for {
			select {
			case <-p.done:
				winchStop(ch)
				return
			case <-ch:
				prog.Send(p.windowSize())
			}
		}
	}()
}

func (p *rawPainter) paint(frame string) tea.Cmd {
	p.mu.Lock()
	p.gen++
	gen := p.gen
	p.mu.Unlock()

	return func() tea.Msg {
		p.mu.Lock()
		defer p.mu.Unlock()
		if gen <= p.painted {
			return nil
		}
		p.painted = gen

		var b strings.Builder
		b.WriteString("\x1b[?2026h\x1b[H\x1b[2J")
		for i, line := range strings.Split(frame, "\n") {
			if i > 0 {
				b.WriteString("\r\n")
			}
			b.WriteString(trimPadding(line))
		}
		b.WriteString("\x1b[?2026l")
		_, _ = io.WriteString(p.w, b.String())
		return nil
	}
}

// trimPadding drops the spaces that pad a line out to the width of the
// screen, and the colors that only style those spaces. The screen is cleared
// before every frame, so they show nothing, and they do harm next to scaled
// heading text: counted at the unscaled width they spill onto the next row,
// and on the row below a heading they are written over its lower half.
// Spaces under a background color are kept.
func trimPadding(line string) string {
	cut := 0         // the line is kept up to here
	escapes := false // whether what follows cut holds escapes
	background := false
	for i := 0; i < len(line); {
		switch {
		case line[i] == ' ' && !background:
			i++
		case strings.HasPrefix(line[i:], "\x1b["):
			end := i + 2
			for end < len(line) && (line[end] < 0x40 || line[end] > 0x7e) {
				end++
			}
			if end == len(line) || line[end] != 'm' {
				return line // not a style: leave the line alone
			}
			background = sgrBackground(line[i+2:end], background)
			escapes = true
			i = end + 1
		default:
			_, size := utf8.DecodeRuneInString(line[i:])
			i += size
			cut, escapes = i, false
		}
	}
	if cut == len(line) {
		return line
	}
	if escapes {
		return line[:cut] + "\x1b[m"
	}
	return line[:cut]
}

// sgrBackground reports whether a background (a color or reverse video) is
// on after an SGR sequence with these parameters, given whether it was on
// before.
func sgrBackground(params string, on bool) bool {
	ps := strings.Split(params, ";")
	for i := 0; i < len(ps); i++ {
		n, _ := strconv.Atoi(ps[i]) // an empty parameter is 0
		switch {
		case n == 0 || n == 27 || n == 49:
			on = false
		case n == 38 || n == 58: // extended foreground or underline color
			if i+1 < len(ps) && ps[i+1] == "2" {
				i += 4
			} else {
				i += 2
			}
		case n == 7 || n == 48 || (n >= 40 && n <= 47) || (n >= 100 && n <= 107):
			on = true
			if n == 48 {
				if i+1 < len(ps) && ps[i+1] == "2" {
					i += 4
				} else {
					i += 2
				}
			}
		}
	}
	return on
}

// paintModel wraps the root model, painting a frame after every processed
// message. Bubbletea's renderer never calls View when it is disabled, so the
// wrapper drives painting itself.
type paintModel struct {
	inner   tea.Model
	painter *rawPainter
}

func (m paintModel) Init() tea.Cmd {
	m.painter.start()
	return tea.Batch(m.inner.Init(), m.painter.paint(m.inner.View().Content))
}

func (m paintModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if ws, ok := msg.(tea.WindowSizeMsg); ok && (ws.Width <= 0 || ws.Height <= 0) {
		msg = m.painter.windowSize()
	}
	inner, cmd := m.inner.Update(msg)
	m.inner = inner
	return m, tea.Batch(cmd, m.painter.paint(inner.View().Content))
}

func (m paintModel) View() tea.View {
	return m.inner.View()
}
