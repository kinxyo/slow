package ui

import (
	"fmt"
	"math"
	"path/filepath"
	"strings"
	"sync/atomic"
	"time"

	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/glamour/v2"
	"charm.land/lipgloss/v2"
	"github.com/atotto/clipboard"
	"github.com/charmbracelet/log"
	"github.com/fsnotify/fsnotify"
	"github.com/kinxyo/slow/utils"
	runewidth "github.com/mattn/go-runewidth"
	"github.com/muesli/reflow/ansi"
	"github.com/muesli/reflow/truncate"
	"github.com/muesli/termenv"
)

const (
	statusBarHeight = 1
	lineNumberWidth = 4
)

var pagerHelpHeight int

type (
	contentRenderedMsg struct {
		body  string // as shown
		text  string // before centering, kept to center again without glamour
		src   string // the markdown it was rendered from
		wrap  int    // the width it was wrapped at
		gen   int64  // which render this is; 0 when it is not tracked
		stats renderStats
	}
	reloadMsg struct{}
)

type pagerState int

const (
	pagerStateBrowse pagerState = iota
	pagerStateStatusMessage
)

type pagerModel struct {
	common   *commonModel
	viewport viewport.Model
	state    pagerState
	showHelp bool

	// Width of the centered text column. It starts at the configured width and
	// can be changed while reading; 0 means no wrapping.
	column int

	// Debug bar: shown on request, with the numbers of the last render.
	showDebug  bool
	stats      renderStats
	renders    int
	layouts    int
	layoutTook time.Duration

	// The last render, so that a change that does not alter the wrapping
	// (a wider terminal, for one) only has to center it again.
	text     string
	textSrc  string
	textWrap int

	// Set when the next render should keep the reader at the same place in the
	// document (after a resize or a column change) instead of the same line.
	keepPlace bool
	place     float64

	statusMessage      string
	statusMessageTimer *time.Timer

	// Current document being rendered, sans-glamour rendering. We cache
	// it here so we can re-render it on resize.
	currentDocument markdown

	watcher *fsnotify.Watcher
}

func newPagerModel(common *commonModel) pagerModel {
	// Init viewport
	vp := viewport.New()

	m := pagerModel{
		common:   common,
		state:    pagerStateBrowse,
		viewport: vp,
		column:   int(common.cfg.GlamourMaxWidth),

		showDebug: common.cfg.ShowDebugBar,
	}
	m.initWatcher()
	return m
}

func (m *pagerModel) setSize(w, h int) {
	m.viewport.SetWidth(w)
	chrome := statusBarHeight
	if m.showDebug {
		chrome++
	}
	m.viewport.SetHeight(h - chrome)

	if m.showHelp {
		if pagerHelpHeight == 0 {
			pagerHelpHeight = strings.Count(m.helpView(), "\n")
		}
		m.viewport.SetHeight(m.viewport.Height() - (statusBarHeight + pagerHelpHeight))
	}
}

const (
	columnStep = 10
	columnMin  = 40
	columnMax  = 400
)

// adjustColumn moves the text column by delta columns, within its limits.
func adjustColumn(column, delta int) int {
	return min(max(column+delta, columnMin), columnMax)
}

// columnMessage tells the reader the column width, and when the terminal is
// narrower than the column, the width actually used.
func (m pagerModel) columnMessage() string {
	if w := m.viewport.Width(); w > 0 && m.column > w {
		return fmt.Sprintf("Column %d (terminal %d)", m.column, w)
	}
	return fmt.Sprintf("Column %d", m.column)
}

// rememberPlace notes where in the document the reader is, so that the next
// render can put them back at the same place when the lines reflow.
func (m *pagerModel) rememberPlace() {
	place := 0.0
	// a document that fits the screen has no place to keep; asking the
	// viewport would say "the bottom"
	if m.viewport.TotalLineCount() > m.viewport.Height() {
		place = m.viewport.ScrollPercent()
	}
	m.keepPlace, m.place = true, place
}

func (m *pagerModel) setContent(s string) {
	m.viewport.SetContent(s)
}

func (m *pagerModel) toggleHelp() {
	m.showHelp = !m.showHelp
	m.setSize(m.common.width, m.common.height)
	if m.viewport.PastBottom() {
		m.viewport.GotoBottom()
	}
}

type pagerStatusMessage struct {
	message string
	isError bool
}

// Perform stuff that needs to happen after a successful markdown stash. Note
// that the returned command should be sent back the through the pager
// update function.
func (m *pagerModel) showStatusMessage(msg pagerStatusMessage) tea.Cmd {
	// Show a success message to the user
	m.state = pagerStateStatusMessage
	m.statusMessage = msg.message
	if m.statusMessageTimer != nil {
		m.statusMessageTimer.Stop()
	}
	m.statusMessageTimer = time.NewTimer(statusMessageTimeout)

	return waitForStatusMessageTimeout(pagerContext, m.statusMessageTimer)
}

func (m *pagerModel) unload() {
	log.Debug("unload")
	if m.showHelp {
		m.toggleHelp()
	}
	if m.statusMessageTimer != nil {
		m.statusMessageTimer.Stop()
	}
	m.state = pagerStateBrowse
	m.text, m.textSrc = "", ""
	m.viewport.SetContent("")
	m.viewport.SetYOffset(0)
	m.unwatchFile()
}

func (m pagerModel) update(msg tea.Msg) (pagerModel, tea.Cmd) {
	var (
		cmd  tea.Cmd
		cmds []tea.Cmd
	)

	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch msg.String() {
		case "q", keyEsc:
			if m.state != pagerStateBrowse {
				m.state = pagerStateBrowse
				return m, nil
			}
		case "home", "g":
			m.viewport.GotoTop()
		case "end", "G":
			m.viewport.GotoBottom()

		case "d":
			m.viewport.HalfPageDown()

		case "u":
			m.viewport.HalfPageUp()

		case "e":
			lineno := int(math.RoundToEven(float64(m.viewport.TotalLineCount()) * m.viewport.ScrollPercent()))
			if m.viewport.AtTop() {
				lineno = 0
			}
			log.Info(
				"opening editor",
				"file", m.currentDocument.localPath,
				"line", fmt.Sprintf("%d/%d", lineno, m.viewport.TotalLineCount()),
			)
			return m, openEditor(m.currentDocument.localPath, lineno)

		case "c":
			// Copy using OSC 52
			termenv.Copy(m.currentDocument.Body)
			// Copy using native system clipboard
			_ = clipboard.WriteAll(m.currentDocument.Body)
			cmds = append(cmds, m.showStatusMessage(pagerStatusMessage{"Copied contents", false}))

		case "r":
			return m, loadLocalMarkdown(&m.currentDocument)

		case "+", "=", "-", "_", "0":
			if m.column == 0 {
				cmds = append(cmds, m.showStatusMessage(pagerStatusMessage{"Wrapping is off (--width 0)", false}))
				break
			}
			switch msg.String() {
			case "+", "=":
				m.column = adjustColumn(m.column, columnStep)
			case "-", "_":
				m.column = adjustColumn(m.column, -columnStep)
			default:
				m.column = int(m.common.cfg.GlamourMaxWidth)
			}
			cmds = append(cmds,
				m.relayout(),
				m.showStatusMessage(pagerStatusMessage{m.columnMessage(), false}),
			)

		case "D":
			m.showDebug = !m.showDebug
			m.setSize(m.common.width, m.common.height)
			if m.viewport.PastBottom() {
				m.viewport.GotoBottom()
			}

		case "?":
			m.toggleHelp()
		}

	// slow has rendered the content
	case contentRenderedMsg:
		log.Info("content rendered", "state", m.state)

		if msg.gen != 0 && msg.gen < renderGen.Load() {
			return m, nil // a newer render is on its way
		}
		m.stats, m.renders = msg.stats, m.renders+1
		m.text, m.textSrc, m.textWrap = msg.text, msg.src, msg.wrap
		m.setContent(msg.body)
		if m.keepPlace {
			m.keepPlace = false
			m.viewport.SetYOffset(int(math.Round(m.place * float64(max(0, m.viewport.TotalLineCount()-m.viewport.Height())))))
		}
		cmds = append(cmds, m.watchFile)

	// The file was changed on disk and we're reloading it
	case reloadMsg:
		return m, loadLocalMarkdown(&m.currentDocument)

	// We've finished editing the document, potentially making changes. Let's
	// retrieve the latest version of the document so that we display
	// up-to-date contents.
	case editorFinishedMsg:
		return m, loadLocalMarkdown(&m.currentDocument)

	// We've received terminal dimensions, either for the first time or
	// after a resize
	case tea.WindowSizeMsg:
		return m, m.relayout()

	case statusMessageTimeoutMsg:
		m.state = pagerStateBrowse
	}

	m.viewport, cmd = m.viewport.Update(msg)
	cmds = append(cmds, cmd)

	return m, tea.Batch(cmds...)
}

func (m pagerModel) View() string {
	var b strings.Builder
	fmt.Fprint(&b, m.viewport.View()+"\n")
	if m.showDebug {
		fmt.Fprint(&b, m.debugBarView()+"\n")
	}

	// Footer
	m.statusBarView(&b)

	if m.showHelp {
		fmt.Fprint(&b, "\n"+m.helpView())
	}

	return b.String()
}

func (m pagerModel) statusBarView(b *strings.Builder) {
	const (
		minPercent               float64 = 0.0
		maxPercent               float64 = 1.0
		percentToStringMagnitude float64 = 100.0
	)

	showStatusMessage := m.state == pagerStateStatusMessage
	styles := m.common.styles

	// Logo
	logo := logoView(m.common.styles)

	// Scroll percent
	percent := math.Max(minPercent, math.Min(maxPercent, m.viewport.ScrollPercent()))
	scrollPercent := fmt.Sprintf(" %3.f%% ", percent*percentToStringMagnitude)
	if showStatusMessage {
		scrollPercent = styles.statusBarMessageScrollPosStyle(scrollPercent)
	} else {
		scrollPercent = styles.statusBarScrollPosStyle(scrollPercent)
	}

	// "Help" note
	var helpNote string
	if showStatusMessage {
		helpNote = styles.statusBarMessageHelpStyle(" ? Help ")
	} else {
		helpNote = styles.statusBarHelpStyle(" ? Help ")
	}

	// Note
	var note string
	if showStatusMessage {
		note = m.statusMessage
	} else {
		note = m.currentDocument.Note
	}
	note = truncate.StringWithTail(" "+note+" ", uint(max(0, //nolint:gosec
		m.common.width-
			ansi.PrintableRuneWidth(logo)-
			ansi.PrintableRuneWidth(scrollPercent)-
			ansi.PrintableRuneWidth(helpNote),
	)), ellipsis)
	if showStatusMessage {
		note = styles.statusBarMessageStyle(note)
	} else {
		note = styles.statusBarNoteStyle(note)
	}

	// Empty space
	padding := max(0,
		m.common.width-
			ansi.PrintableRuneWidth(logo)-
			ansi.PrintableRuneWidth(note)-
			ansi.PrintableRuneWidth(scrollPercent)-
			ansi.PrintableRuneWidth(helpNote),
	)
	emptySpace := strings.Repeat(" ", padding)
	if showStatusMessage {
		emptySpace = styles.statusBarMessageStyle(emptySpace)
	} else {
		emptySpace = styles.statusBarNoteStyle(emptySpace)
	}

	fmt.Fprintf(b, "%s%s%s%s%s",
		logo,
		note,
		emptySpace,
		scrollPercent,
		helpNote,
	)
}

func (m pagerModel) helpView() (s string) {
	col1 := []string{
		"g/home  go to top",
		"G/end   go to bottom",
		"c       copy contents",
		"e       edit this document",
		"r       reload this document",
		"esc     back to files",
		"q       quit",
	}

	s += "\n"
	s += "k/↑      up                  " + col1[0] + "\n"
	s += "j/↓      down                " + col1[1] + "\n"
	s += "b/pgup   page up             " + col1[2] + "\n"
	s += "f/pgdn   page down           " + col1[3] + "\n"
	s += "u        ½ page up           " + col1[4] + "\n"
	s += "d        ½ page down         "

	if len(col1) > 5 {
		s += col1[5]
	}
	s += "\n+/-      column width        0       reset column width"
	s += "\nD        debug bar           " + col1[6]

	s = indent(s, 2)

	// Fill up empty cells with spaces for background coloring
	if m.common.width > 0 {
		lines := strings.Split(s, "\n")
		for i := 0; i < len(lines); i++ {
			l := runewidth.StringWidth(lines[i])
			n := max(m.common.width-l, 0)
			lines[i] += strings.Repeat(" ", n)
		}

		s = strings.Join(lines, "\n")
	}

	return m.common.styles.helpViewStyle(s)
}

// COMMANDS

// renderGen numbers the renders that were asked for. Renders run in the
// background and can finish out of order, so a finished one is dropped when a
// newer one has been asked for since.
var renderGen atomic.Int64

func renderWithGlamour(m pagerModel, md string) tea.Cmd {
	gen := renderGen.Add(1)
	return func() tea.Msg {
		r, err := renderDocument(m, md)
		if err != nil {
			log.Error("error rendering with Glamour", "error", err)
			return errMsg{err}
		}
		return contentRenderedMsg{body: r.body, text: r.text, src: md, wrap: r.stats.wrap, gen: gen, stats: r.stats}
	}
}

// rendered is a document ready to show, with what is needed to show it again
// at another width without asking glamour a second time.
type rendered struct {
	body  string // as shown: centered, with line numbers when asked for
	text  string // as glamour made it, before centering
	stats renderStats
}

func (m pagerModel) isCode() bool {
	return !utils.IsMarkdownFile(m.currentDocument.Note)
}

// wrapWidth is the width glamour wraps the text at.
func (m pagerModel) wrapWidth() int {
	if m.isCode() {
		return 0
	}
	return max(0, min(m.column, m.viewport.Width()))
}

// renderDocument renders a document for the viewer.
func renderDocument(m pagerModel, markdown string) (rendered, error) {
	start := time.Now()
	if !m.common.cfg.GlamourEnabled {
		return rendered{body: markdown}, nil
	}
	text, stats, err := renderText(m, markdown)
	if err != nil {
		return rendered{}, err
	}
	body, pad, lines := layoutText(m, text, stats.widest)
	stats.pad, stats.lines, stats.bytes = pad, lines, len(body)
	stats.took = time.Since(start)
	return rendered{body: body, text: text, stats: stats}, nil
}

// renderText is the expensive part: markdown to styled text, wrapped at the
// column. This is where the magic happens.
func renderText(m pagerModel, markdown string) (string, renderStats, error) {
	var stats renderStats
	isCode := m.isCode()
	width := m.wrapWidth()

	options := []glamour.TermRendererOption{
		utils.GlamourStyle(m.common.cfg.GlamourStyle, isCode),
		glamour.WithWordWrap(width),
	}

	if m.common.cfg.PreserveNewLines {
		options = append(options, glamour.WithPreservedNewLines())
	}
	r, err := glamour.NewTermRenderer(options...)
	if err != nil {
		return "", stats, fmt.Errorf("error creating glamour renderer: %w", err)
	}

	if isCode {
		markdown = utils.WrapCodeBlock(markdown, filepath.Ext(m.currentDocument.Note))
	} else {
		markdown, stats.mermaid = utils.RenderMermaidStats(markdown, width)
	}

	out, err := r.Render(markdown)
	if err != nil {
		return "", stats, fmt.Errorf("error rendering markdown: %w", err)
	}

	if isCode {
		out = strings.TrimSpace(out)
	} else {
		out = utils.ApplyTextSizing(out)
	}
	stats.wrap = width
	stats.widest = utils.BlockWidth(out)
	return out, stats, nil
}

// layoutText is the cheap part: centering and line numbers. widest is the
// width of the text, as renderText measured it.
func layoutText(m pagerModel, text string, widest int) (body string, pad, lines int) {
	isCode := m.isCode()
	if !isCode {
		pad = utils.CenterPadding(widest, m.viewport.Width())
		text = utils.PadBlock(text, pad)
	}
	lines = strings.Count(text, "\n") + 1
	if !isCode && !m.common.cfg.ShowLineNumbers {
		return text, pad, lines
	}

	trunc := lipgloss.NewStyle().MaxWidth(m.viewport.Width() - lineNumberWidth).Render
	var content strings.Builder
	for i, s := range strings.Split(text, "\n") {
		content.WriteString(m.common.styles.lineNumberStyle(fmt.Sprintf("%"+fmt.Sprint(lineNumberWidth)+"d", i+1)))
		content.WriteString(trunc(s))

		// don't add an artificial newline after the last split
		if i+1 < lines {
			content.WriteRune('\n')
		}
	}
	return content.String(), pad, lines
}

// relayout shows the document again after the terminal or the column changed.
// When glamour would wrap at the same width as before, only the centering
// changes and that is done on the spot; otherwise it returns the command that
// renders the document again, and the reader keeps their place in it.
func (m *pagerModel) relayout() tea.Cmd {
	if m.text != "" && m.wrapWidth() == m.textWrap && !m.isCode() && !m.common.cfg.ShowLineNumbers {
		start := time.Now()
		body, pad, lines := layoutText(*m, m.text, m.stats.widest)
		m.setContent(body)
		m.stats.pad, m.stats.lines, m.stats.bytes = pad, lines, len(body)
		m.layouts++
		m.layoutTook = time.Since(start)
		return nil
	}

	m.rememberPlace()
	src := m.textSrc
	if src == "" {
		src = m.currentDocument.Body
	}
	return renderWithGlamour(*m, src)
}

func (m *pagerModel) initWatcher() {
	var err error
	m.watcher, err = fsnotify.NewWatcher()
	if err != nil {
		log.Error("error creating fsnotify watcher", "error", err)
	}
}

func (m *pagerModel) watchFile() tea.Msg {
	dir := m.localDir()

	if err := m.watcher.Add(dir); err != nil {
		log.Error("error adding dir to fsnotify watcher", "error", err)
		return nil
	}

	log.Info("fsnotify watching dir", "dir", dir)

	for {
		select {
		case event, ok := <-m.watcher.Events:
			if !ok || event.Name != m.currentDocument.localPath {
				continue
			}

			if !event.Has(fsnotify.Write) && !event.Has(fsnotify.Create) {
				continue
			}

			log.Debug("fsnotify event", "file", event.Name, "event", event.Op)
			return reloadMsg{}
		case err, ok := <-m.watcher.Errors:
			if !ok {
				continue
			}
			log.Debug("fsnotify error", "dir", dir, "error", err)
		}
	}
}

func (m *pagerModel) unwatchFile() {
	dir := m.localDir()

	err := m.watcher.Remove(dir)
	if err == nil {
		log.Debug("fsnotify dir unwatched", "dir", dir)
	} else {
		log.Error("fsnotify fail to unwatch dir", "dir", dir, "error", err)
	}
}

func (m *pagerModel) localDir() string {
	return filepath.Dir(m.currentDocument.localPath)
}
