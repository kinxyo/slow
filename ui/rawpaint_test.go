package ui

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
)

func TestRawPainterPaint(t *testing.T) {
	var buf bytes.Buffer
	p := &rawPainter{w: &buf}

	stale := p.paint("stale frame")
	fresh := p.paint("fresh\nframe")

	fresh()
	stale()

	got := buf.String()
	if strings.Contains(got, "stale") {
		t.Errorf("stale frame was painted over a newer one: %q", got)
	}
	if !strings.Contains(got, "\x1b[?2026h\x1b[H\x1b[2Jfresh\r\nframe\x1b[?2026l") {
		t.Errorf("unexpected frame format: %q", got)
	}
}

func TestPaintModelRendersDocument(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.md")
	if err := os.WriteFile(path, []byte("# Hello\n\nSome text here.\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg := Config{
		GlamourEnabled:  true,
		GlamourStyle:    "dark",
		GlamourMaxWidth: 80,
		Path:            path,
	}

	var buf bytes.Buffer
	painter := &rawPainter{w: &buf}
	var pm tea.Model = paintModel{inner: newModel(cfg, ""), painter: painter}

	msgCh := make(chan tea.Msg, 64)
	var exec func(cmd tea.Cmd)
	exec = func(cmd tea.Cmd) {
		if cmd == nil {
			return
		}
		go func() {
			switch msg := cmd().(type) {
			case tea.BatchMsg:
				for _, c := range msg {
					exec(c)
				}
			case nil:
			default:
				msgCh <- msg
			}
		}()
	}

	exec(pm.Init())
	msgCh <- tea.WindowSizeMsg{Width: 80, Height: 24}

	deadline := time.Now().Add(3 * time.Second)
	for {
		// Frames are painted asynchronously by the paint command, so poll
		// the buffer instead of only checking after processed messages.
		painter.mu.Lock()
		done := strings.Contains(buf.String(), "Some text")
		painter.mu.Unlock()
		if done {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for content; buf=%q", buf.String())
		}

		select {
		case msg := <-msgCh:
			im, cmd := pm.Update(msg)
			pm = im
			exec(cmd)
		case <-time.After(5 * time.Millisecond):
		}
	}
}

func TestPaintModelZeroSizeSubstitution(t *testing.T) {
	var buf bytes.Buffer
	painter := &rawPainter{w: &buf}
	pm := paintModel{inner: newModel(Config{}, "body"), painter: painter}

	im, _ := pm.Update(tea.WindowSizeMsg{Width: 0, Height: 0})
	m := im.(paintModel).inner.(model)
	if m.common.width != 80 || m.common.height != 24 {
		t.Errorf("expected fallback size 80x24, got %dx%d", m.common.width, m.common.height)
	}
}

func TestTrimPadding(t *testing.T) {
	heading := "  \x1b[38;5;39;1m\x1b]66;s=2:n=3:d=4:w=3;Iden\x1b\\\x1b]66;s=2:n=3:d=4:w=3;tify\x1b\\\x1b[m"
	tests := []struct{ name, in, want string }{
		{"plain padding", "abc     ", "abc"},
		{"nothing to trim", "abc", "abc"},
		{"spaces styled with a foreground only", "abc\x1b[38;5;252m \x1b[m\x1b[38;5;252m \x1b[m   ", "abc\x1b[m"},
		{"a blank line of styled spaces", "\x1b[38;5;252m \x1b[m\x1b[38;5;252m \x1b[m", "\x1b[m"},
		{"a scaled heading padded at its unscaled width", heading + strings.Repeat(" ", 108), heading},
		{"spaces under a background stay", "\x1b[48;5;63m Help \x1b[m", "\x1b[48;5;63m Help \x1b[m"},
		{"padding after a background ends goes", "\x1b[48;5;63m Help \x1b[m    ", "\x1b[48;5;63m Help \x1b[m"},
		{"a 256-color foreground of 48 is not a background", "a\x1b[38;5;48m   ", "a\x1b[m"},
		{"reverse video counts as a background", "a\x1b[7m   ", "a\x1b[7m   "},
		{"other sequences leave the line alone", "a\x1b[5X   ", "a\x1b[5X   "},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := trimPadding(tt.in); got != tt.want {
				t.Errorf("trimPadding(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestRawPainterTrimsPadding(t *testing.T) {
	var buf bytes.Buffer
	p := &rawPainter{w: &buf}
	p.paint("one   \ntwo  ")()
	if got := buf.String(); !strings.Contains(got, "one\r\ntwo\x1b[?2026l") {
		t.Errorf("padding was painted: %q", got)
	}
}
