package utils

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
)

// errMermaidUnsupported is returned for any diagram, syntax or shape the
// renderer does not handle. Callers fall back to showing the source.
var errMermaidUnsupported = errors.New("unsupported mermaid syntax")

func mmUnsupported(format string, args ...any) error {
	return fmt.Errorf("%w: %s", errMermaidUnsupported, fmt.Sprintf(format, args...))
}

type mmShape int

const (
	mmRect mmShape = iota
	mmRound
	mmDiamond
	mmStart
	mmEnd
)

type mmNode struct {
	id    string
	label string
	shape mmShape
	sub   int      // index into mmGraph.subs, -1 when free
	loops []string // labels of self-loop edges
}

type mmEdge struct {
	from, to  int
	label     string
	dotted    bool
	arrowTo   bool
	arrowFrom bool
}

type mmSub struct {
	id, title string
}

type mmGraph struct {
	nodes []*mmNode
	edges []mmEdge
	subs  []*mmSub
	index map[string]int
}

const mmMaxNodes = 80

func newMMGraph() *mmGraph {
	return &mmGraph{index: map[string]int{}}
}

// node returns the index of the node with the given id, creating it when needed.
func (g *mmGraph) node(id string, sub int) int {
	if i, ok := g.index[id]; ok {
		if g.nodes[i].sub < 0 && sub >= 0 {
			g.nodes[i].sub = sub
		}
		return i
	}
	g.nodes = append(g.nodes, &mmNode{id: id, label: id, sub: sub})
	g.index[id] = len(g.nodes) - 1
	return len(g.nodes) - 1
}

func (g *mmGraph) addEdge(e mmEdge) {
	if e.from == e.to {
		g.nodes[e.from].loops = append(g.nodes[e.from].loops, e.label)
		return
	}
	g.edges = append(g.edges, e)
}

var (
	mmTag   = regexp.MustCompile(`</?[a-zA-Z][^>]*>`)
	mmBreak = regexp.MustCompile(`(?i)<br\s*/?>`)
)

// cleanLabel strips quotes and the little HTML mermaid labels may carry.
func cleanLabel(s string) string {
	s = strings.TrimSpace(s)
	if len(s) >= 2 && s[0] == '"' && s[len(s)-1] == '"' {
		s = s[1 : len(s)-1]
	}
	s = mmBreak.ReplaceAllString(s, "\n")
	s = mmTag.ReplaceAllString(s, "")
	return strings.TrimSpace(s)
}

// parseMermaid parses the source of one mermaid block.
func parseMermaid(src string) (*mmGraph, error) {
	var lines []string
	for _, l := range strings.Split(strings.ReplaceAll(src, "\r\n", "\n"), "\n") {
		t := strings.TrimSpace(l)
		if t == "" || strings.HasPrefix(t, "%%") {
			continue
		}
		lines = append(lines, t)
	}
	if len(lines) == 0 {
		return nil, mmUnsupported("empty diagram")
	}

	header := strings.Fields(lines[0])
	var (
		g   *mmGraph
		err error
	)
	switch header[0] {
	case "flowchart", "graph":
		if len(header) > 1 && header[1] != "TB" && header[1] != "TD" {
			return nil, mmUnsupported("direction %s", header[1])
		}
		g, err = parseFlowchart(lines[1:])
	case "stateDiagram-v2", "stateDiagram":
		g, err = parseStateDiagram(lines[1:])
	default:
		return nil, mmUnsupported("diagram type %s", header[0])
	}
	if err != nil {
		return nil, err
	}
	if len(g.nodes) == 0 {
		return nil, mmUnsupported("no nodes")
	}
	if len(g.nodes) > mmMaxNodes {
		return nil, mmUnsupported("more than %d nodes", mmMaxNodes)
	}
	return g, nil
}

// splitStatements splits a flowchart line on ';' outside of double quotes.
func splitStatements(line string) []string {
	var out []string
	inQuote := false
	start := 0
	for i := 0; i < len(line); i++ {
		switch line[i] {
		case '"':
			inQuote = !inQuote
		case ';':
			if !inQuote {
				out = append(out, line[start:i])
				start = i + 1
			}
		}
	}
	return append(out, line[start:])
}

var (
	mmSubgraphDecl = regexp.MustCompile(`^([A-Za-z0-9_]+)\s*\[(.*)\]$`)
	mmIgnored      = []string{"classDef ", "class ", "style ", "linkStyle ", "click ", "direction "}
)

func parseFlowchart(lines []string) (*mmGraph, error) {
	g := newMMGraph()
	cur := -1
	for _, line := range lines {
		for _, stmt := range splitStatements(line) {
			stmt = strings.TrimSpace(stmt)
			if stmt == "" {
				continue
			}
			switch {
			case stmt == "end":
				if cur < 0 {
					return nil, mmUnsupported("stray end")
				}
				cur = -1
			case strings.HasPrefix(stmt, "subgraph "):
				if cur >= 0 {
					return nil, mmUnsupported("nested subgraphs")
				}
				rest := strings.TrimSpace(strings.TrimPrefix(stmt, "subgraph "))
				sub := &mmSub{id: cleanLabel(rest), title: cleanLabel(rest)}
				if m := mmSubgraphDecl.FindStringSubmatch(rest); m != nil {
					sub.id, sub.title = m[1], cleanLabel(m[2])
				}
				g.subs = append(g.subs, sub)
				cur = len(g.subs) - 1
			case hasAnyPrefix(stmt, mmIgnored):
				// styling only
			default:
				if err := g.parseChain(stmt, cur); err != nil {
					return nil, err
				}
			}
		}
	}
	if cur >= 0 {
		return nil, mmUnsupported("unterminated subgraph")
	}
	return g, nil
}

func hasAnyPrefix(s string, prefixes []string) bool {
	for _, p := range prefixes {
		if strings.HasPrefix(s, p) {
			return true
		}
	}
	return false
}

type mmScanner struct {
	s string
	i int
}

func (p *mmScanner) eof() bool    { return p.i >= len(p.s) }
func (p *mmScanner) rest() string { return p.s[p.i:] }

func (p *mmScanner) skipSpace() {
	for !p.eof() && (p.s[p.i] == ' ' || p.s[p.i] == '\t') {
		p.i++
	}
}

func isIDChar(c byte) bool {
	return c == '_' || (c >= '0' && c <= '9') || (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')
}

// parseChain parses `A --> B & C -.->|x| D`.
func (g *mmGraph) parseChain(stmt string, sub int) error {
	p := &mmScanner{s: stmt}
	prev, err := p.nodeList(g, sub)
	if err != nil {
		return err
	}
	for {
		p.skipSpace()
		if p.eof() {
			return nil
		}
		op, err := p.edgeOp()
		if err != nil {
			return err
		}
		next, err := p.nodeList(g, sub)
		if err != nil {
			return err
		}
		for _, a := range prev {
			for _, b := range next {
				e := op
				e.from, e.to = a, b
				g.addEdge(e)
			}
		}
		prev = next
	}
}

func (p *mmScanner) nodeList(g *mmGraph, sub int) ([]int, error) {
	var out []int
	for {
		p.skipSpace()
		n, err := p.nodeExpr(g, sub)
		if err != nil {
			return nil, err
		}
		out = append(out, n)
		p.skipSpace()
		if !p.eof() && p.s[p.i] == '&' {
			p.i++
			continue
		}
		return out, nil
	}
}

var shapeOpeners = []struct {
	open, close string
	shape       mmShape
}{
	{"((", "))", mmRound},
	{"([", "])", mmRound},
	{"[[", "]]", mmRect},
	{"[", "]", mmRect},
	{"(", ")", mmRound},
	{"{", "}", mmDiamond},
}

func (p *mmScanner) nodeExpr(g *mmGraph, sub int) (int, error) {
	start := p.i
	for !p.eof() && isIDChar(p.s[p.i]) {
		p.i++
	}
	if p.i == start {
		return 0, mmUnsupported("expected a node at %q", p.rest())
	}
	id := p.s[start:p.i]

	label, shape, hasShape := "", mmRect, false
	rest := p.rest()
	for _, so := range shapeOpeners {
		if !strings.HasPrefix(rest, so.open) {
			continue
		}
		p.i += len(so.open)
		p.skipSpace()
		var text string
		if !p.eof() && p.s[p.i] == '"' {
			end := strings.IndexByte(p.s[p.i+1:], '"')
			if end < 0 {
				return 0, mmUnsupported("unterminated label")
			}
			text = p.s[p.i : p.i+end+2]
			p.i += end + 2
			p.skipSpace()
			if !strings.HasPrefix(p.rest(), so.close) {
				return 0, mmUnsupported("expected %q", so.close)
			}
			p.i += len(so.close)
		} else {
			end := strings.Index(p.rest(), so.close)
			if end < 0 {
				return 0, mmUnsupported("expected %q", so.close)
			}
			text = p.s[p.i : p.i+end]
			p.i += end + len(so.close)
		}
		label, shape, hasShape = cleanLabel(text), so.shape, true
		break
	}
	if !hasShape && !p.eof() && strings.ContainsRune("{[(>", rune(p.s[p.i])) {
		return 0, mmUnsupported("node shape at %q", p.rest())
	}

	// ":::className" styling suffix
	if strings.HasPrefix(p.rest(), ":::") {
		p.i += 3
		for !p.eof() && isIDChar(p.s[p.i]) {
			p.i++
		}
	}

	n := g.node(id, sub)
	if hasShape {
		g.nodes[n].label, g.nodes[n].shape = label, shape
	}
	return n, nil
}

var (
	mmLabeledOp = regexp.MustCompile(`^(<?)(--|==|-\.)\s+(.+?)\s+(-{2,}>?|={2,}>?|\.+-+>?)(\s|$)`)
	mmPlainOp   = regexp.MustCompile(`^(<?)(-\.+-|-{2,}|={2,})(>?)`)
)

// edgeOp parses an edge operator and its optional label.
func (p *mmScanner) edgeOp() (mmEdge, error) {
	rest := p.rest()
	var e mmEdge
	if m := mmLabeledOp.FindStringSubmatchIndex(rest); m != nil {
		e.arrowFrom = rest[m[2]:m[3]] == "<"
		e.dotted = rest[m[4]:m[5]] == "-."
		e.label = cleanLabel(rest[m[6]:m[7]])
		closer := rest[m[8]:m[9]]
		e.arrowTo = strings.HasSuffix(closer, ">")
		p.i += m[9]
		return e, nil
	}
	m := mmPlainOp.FindStringSubmatch(rest)
	if m == nil {
		return e, mmUnsupported("edge at %q", rest)
	}
	e.arrowFrom = m[1] == "<"
	e.dotted = strings.HasPrefix(m[2], "-.")
	e.arrowTo = m[3] == ">"
	p.i += len(m[0])
	p.skipSpace()
	if !p.eof() && p.s[p.i] == '|' {
		end := strings.IndexByte(p.s[p.i+1:], '|')
		if end < 0 {
			return e, mmUnsupported("unterminated edge label")
		}
		e.label = cleanLabel(p.s[p.i+1 : p.i+1+end])
		p.i += end + 2
	}
	return e, nil
}

var (
	mmTransition = regexp.MustCompile(`^(\[\*\]|[A-Za-z0-9_]+)\s*-->\s*(\[\*\]|[A-Za-z0-9_]+)\s*(?::\s*(.*))?$`)
	mmStateAlias = regexp.MustCompile(`^state\s+"([^"]*)"\s+as\s+([A-Za-z0-9_]+)$`)
	mmStateKind  = regexp.MustCompile(`^state\s+([A-Za-z0-9_]+)\s+<<(\w+)>>$`)
	mmStateDesc  = regexp.MustCompile(`^([A-Za-z0-9_]+)\s*:\s*(.+)$`)
)

func parseStateDiagram(lines []string) (*mmGraph, error) {
	g := newMMGraph()

	state := func(id string, isTarget bool) int {
		if id != "[*]" {
			n := g.node(id, -1)
			g.nodes[n].shape = mmRound
			return n
		}
		key, shape := "[*]start", mmStart
		if isTarget {
			key, shape = "[*]end", mmEnd
		}
		n := g.node(key, -1)
		g.nodes[n].shape, g.nodes[n].label = shape, ""
		return n
	}

	described := map[int]bool{}
	describe := func(n int, text string) {
		if described[n] {
			g.nodes[n].label += "\n" + text
		} else {
			g.nodes[n].label = text
			described[n] = true
		}
	}

	for _, line := range lines {
		m := mmTransition.FindStringSubmatch(line)
		switch {
		case hasAnyPrefix(line, []string{"classDef ", "class ", "style ", "click "}):
			// styling only
		case strings.HasPrefix(line, "direction "):
			if f := strings.Fields(line); len(f) > 1 && f[1] != "TB" && f[1] != "TD" {
				return nil, mmUnsupported("direction %s", f[1])
			}
		case line == "}" || line == "--" || strings.HasPrefix(line, "note "):
			return nil, mmUnsupported("state syntax %q", line)
		case m != nil:
			from := state(m[1], false)
			to := state(m[2], true)
			g.addEdge(mmEdge{from: from, to: to, label: cleanLabel(m[3]), arrowTo: true})
		case mmStateAlias.MatchString(line):
			m := mmStateAlias.FindStringSubmatch(line)
			n := state(m[2], false)
			g.nodes[n].label = cleanLabel(m[1])
			described[n] = true
		case mmStateKind.MatchString(line):
			m := mmStateKind.FindStringSubmatch(line)
			if m[2] != "choice" {
				return nil, mmUnsupported("state kind <<%s>>", m[2])
			}
			g.nodes[state(m[1], false)].shape = mmDiamond
		case mmStateDesc.MatchString(line):
			m := mmStateDesc.FindStringSubmatch(line)
			describe(state(m[1], false), cleanLabel(m[2]))
		case mmSimpleID.MatchString(line):
			state(line, false)
		default:
			return nil, mmUnsupported("state syntax %q", line)
		}
	}
	return g, nil
}

var mmSimpleID = regexp.MustCompile(`^[A-Za-z0-9_]+$`)
