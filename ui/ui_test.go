package ui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNewModelLoadsDocumentBody(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.md")
	if err := os.WriteFile(path, []byte("# Hello\n\nSome text here.\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	m := newModel(Config{Path: path}, "").(model)
	if m.state != stateShowDocument {
		t.Fatalf("expected stateShowDocument, got %s", m.state)
	}
	if !strings.Contains(m.pager.currentDocument.Body, "Some text") {
		t.Errorf("expected document body to be loaded, got %q", m.pager.currentDocument.Body)
	}
}

func TestFindLocalFilesGitignore(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	files := map[string]string{
		".gitignore": "ignored.md\n",
		"a.md":       "# a\n",
		"ignored.md": "# ignored\n",
	}
	for name, body := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	find := func(cfg Config) []string {
		t.Helper()
		cfg.Path = dir
		msg, ok := findLocalFiles(commonModel{cfg: cfg})().(initLocalFileSearchMsg)
		if !ok {
			t.Fatal("expected initLocalFileSearchMsg")
		}
		var got []string
		for res := range msg.ch {
			got = append(got, filepath.Base(res.Path))
		}
		return got
	}

	if got := find(Config{}); len(got) != 2 {
		t.Errorf("by default ignored files are listed too, got %v", got)
	}
	if got := find(Config{ObeyGitignore: true}); len(got) != 1 || got[0] != "a.md" {
		t.Errorf("with ObeyGitignore only a.md is listed, got %v", got)
	}
}
