package utils

import (
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mattn/go-runewidth"
)

var updateGolden = flag.Bool("update", false, "rewrite the mermaid golden files")

// TestMermaidFixtures draws every fixture and compares it with its golden
// file. Run `go test ./utils -run Fixtures -update` after changing the
// renderer on purpose, and read the diff.
func TestMermaidFixtures(t *testing.T) {
	files, err := filepath.Glob("testdata/mermaid/*.mmd")
	if err != nil || len(files) == 0 {
		t.Fatalf("no fixtures: %v", err)
	}
	for _, f := range files {
		t.Run(filepath.Base(f), func(t *testing.T) {
			src, err := os.ReadFile(f)
			if err != nil {
				t.Fatal(err)
			}
			got, err := renderMermaidBlock(string(src), 80)
			if err != nil {
				t.Fatalf("render: %v", err)
			}
			for _, line := range strings.Split(got, "\n") {
				if w := runewidth.StringWidth(line); w > 74 {
					t.Errorf("line is %d columns wide: %q", w, line)
				}
			}
			golden := strings.TrimSuffix(f, ".mmd") + ".golden"
			if *updateGolden {
				if err := os.WriteFile(golden, []byte(got+"\n"), 0o644); err != nil {
					t.Fatal(err)
				}
				return
			}
			want, err := os.ReadFile(golden)
			if err != nil {
				t.Fatalf("missing golden file, run with -update: %v", err)
			}
			if got+"\n" != string(want) {
				t.Errorf("output differs from %s:\n%s", golden, got)
			}
		})
	}
}

func TestRenderMermaidReplacesBlocks(t *testing.T) {
	md := "# Title\n\n```mermaid\nflowchart TD\n  A[\"one\"] --> B[\"two\"]\n```\n\ntext\n"
	got := RenderMermaid(md, 80)
	if strings.Contains(got, "flowchart") || strings.Contains(got, "-->") {
		t.Errorf("source still present:\n%s", got)
	}
	for _, want := range []string{"# Title", "```text", "│ one │", "│ two │", "▼", "text"} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in:\n%s", want, got)
		}
	}
}

func TestRenderMermaidFallsBack(t *testing.T) {
	cases := map[string]string{
		"unsupported type":      "```mermaid\nsequenceDiagram\n  A->>B: hi\n```",
		"left to right":         "```mermaid\nflowchart LR\n  A --> B\n```",
		"nested subgraph":       "```mermaid\nflowchart TD\n  subgraph a\n  subgraph b\n  X\n  end\n  end\n```",
		"composite state":       "```mermaid\nstateDiagram-v2\n  state X {\n  [*] --> Y\n  }\n```",
		"broken edge":           "```mermaid\nflowchart TD\n  A --> \n```",
		"empty":                 "```mermaid\n```",
		"unterminated fence":    "```mermaid\nflowchart TD\n  A --> B\n",
		"other language":        "```go\nflowchart TD\n  A --> B\n```",
		"mermaid in plain text": "talking about mermaid and A --> B",
	}
	for name, md := range cases {
		t.Run(name, func(t *testing.T) {
			if got := RenderMermaid(md, 80); got != md {
				t.Errorf("expected the input back, got:\n%s", got)
			}
		})
	}
}

func TestRenderMermaidSkipsOtherFences(t *testing.T) {
	md := "````md\n```mermaid\nflowchart TD\n  A --> B\n```\n````\n"
	if got := RenderMermaid(md, 80); got != md {
		t.Errorf("a mermaid block inside another fence must be left alone, got:\n%s", got)
	}
}

func TestRenderMermaidTooNarrow(t *testing.T) {
	md := "```mermaid\nflowchart TD\n  A --> B & C & D & E & F & G & H\n```"
	if got := RenderMermaid(md, 30); got != md {
		t.Errorf("a diagram that cannot fit must fall back to its source, got:\n%s", got)
	}
}

func TestParseFlowchart(t *testing.T) {
	g, err := parseMermaid(`flowchart TD
    A["Start; here"] -->|"go"| B{Choice} & C(Round)
    B -.-> D[[Sub]]
    C -- text --> D
    D --- A
    E --> E
    %% a comment
    style A fill:#fff
    subgraph S["Group"]
        F --> G
    end`)
	if err != nil {
		t.Fatal(err)
	}
	if len(g.nodes) != 7 || len(g.subs) != 1 {
		t.Fatalf("nodes=%d subs=%d", len(g.nodes), len(g.subs))
	}
	a := g.nodes[g.index["A"]]
	if a.label != "Start; here" || g.nodes[g.index["B"]].shape != mmDiamond || g.nodes[g.index["C"]].shape != mmRound {
		t.Errorf("labels or shapes wrong: %+v", a)
	}
	if got := g.nodes[g.index["E"]].loops; len(got) != 1 {
		t.Errorf("self loop not recorded: %v", got)
	}
	if g.nodes[g.index["F"]].sub != 0 || g.nodes[g.index["A"]].sub != -1 {
		t.Error("subgraph membership wrong")
	}
	var dotted, plain, labeled int
	for _, e := range g.edges {
		switch {
		case e.dotted:
			dotted++
		case !e.arrowTo:
			plain++
		}
		if e.label != "" {
			labeled++
		}
	}
	if len(g.edges) != 6 || dotted != 1 || plain != 1 || labeled != 3 {
		t.Errorf("edges=%d dotted=%d plain=%d labeled=%d", len(g.edges), dotted, plain, labeled)
	}
}

func TestParseStateDiagram(t *testing.T) {
	g, err := parseMermaid(`stateDiagram-v2
    [*] --> A
    A --> B: go
    B --> B: again
    state "Long name" as C
    B --> C
    C --> [*]`)
	if err != nil {
		t.Fatal(err)
	}
	if g.nodes[g.index["[*]start"]].shape != mmStart || g.nodes[g.index["[*]end"]].shape != mmEnd {
		t.Error("start and end nodes missing")
	}
	if g.nodes[g.index["C"]].label != "Long name" {
		t.Errorf("alias label = %q", g.nodes[g.index["C"]].label)
	}
	if len(g.nodes[g.index["B"]].loops) != 1 {
		t.Error("self loop not recorded")
	}
}

func TestMermaidDrawingProperties(t *testing.T) {
	// every edge of a loop must stay visible: arrows both ways
	out, err := renderMermaidBlock("flowchart TD\n  A --> B\n  B --> C\n  C --> A", 0)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "▼") || !strings.Contains(out, "▲") {
		t.Errorf("expected a down and an up arrow:\n%s", out)
	}
	for _, id := range []string{"A", "B", "C"} {
		if !strings.Contains(out, "│ "+id+" │") {
			t.Errorf("node %s missing:\n%s", id, out)
		}
	}
}

func TestWrapText(t *testing.T) {
	cases := []struct {
		in    string
		width int
		want  []string
	}{
		{"one two three", 7, []string{"one two", "three"}},
		{"abcdefghij", 4, []string{"abcd", "efgh", "ij"}},
		{"a\nb", 10, []string{"a", "b"}},
		{"", 5, []string{""}},
	}
	for _, c := range cases {
		got := wrapText(c.in, c.width)
		if strings.Join(got, "|") != strings.Join(c.want, "|") {
			t.Errorf("wrapText(%q, %d) = %q, want %q", c.in, c.width, got, c.want)
		}
	}
}
