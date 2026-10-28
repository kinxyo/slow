package utils

import (
	"math"
	"sort"
	"strings"

	"github.com/mattn/go-runewidth"
)

// The layout is a small layered (Sugiyama style) drawing:
//
//  1. nodes get a layer (longest path, cycles broken by reversing back edges)
//     and rank = 2*layer, so every edge spans at least two ranks;
//  2. each edge becomes a chain of items, one per rank it crosses: a dummy
//     that is just a vertical line, or a label item carrying the edge label;
//  3. items in a rank are ordered by barycenter sweeps, keeping the members
//     of a subgraph together;
//  4. x positions come from repeated neighbor averaging with a minimum
//     separation constraint (isotonic regression);
//  5. rows are assigned per rank and per gap between ranks; the gap holds the
//     horizontal "jogs" of the edges, one track each when they would overlap.

type mmItemKind int

const (
	itemNode mmItemKind = iota
	itemDummy
	itemLabel
)

type mmItem struct {
	kind   mmItemKind
	node   int // graph node index for itemNode
	rank   int
	w, h   int
	anchor int // column, relative to the left edge, where edges attach
	x      int // absolute anchor column
	y      int // top row
	lines  []string
	group  int  // subgraph index, -1 when free
	dotted bool // the item belongs to a dotted edge
}

func (it *mmItem) left() int  { return it.x - it.anchor }
func (it *mmItem) right() int { return it.left() + it.w - 1 }

// isBox reports whether the item is drawn as a framed box.
func (it *mmItem) isBox(g *mmGraph) bool {
	if it.kind != itemNode {
		return false
	}
	s := g.nodes[it.node].shape
	return s != mmStart && s != mmEnd
}

type mmLink struct {
	xo, xi int
	track  int // -1 when the link is a straight vertical
}

type mmChain struct {
	items               []int // top to bottom
	edge                mmEdge
	headTop, headBottom bool
	links               []mmLink
}

type mmGap struct {
	padAbove, borderBottom, tracks, borderTop, padBelow int
	y                                                   int
	links                                               []*mmLink
	// arrowUp / arrowDown mark that a head is drawn right below the upper
	// node or right above the lower node
}

func (gp *mmGap) rows() int {
	return gp.padAbove + gp.borderBottom + gp.tracks + gp.borderTop + gp.padBelow
}
func (gp *mmGap) borderBottomY() int { return gp.y + gp.padAbove }
func (gp *mmGap) trackY(t int) int   { return gp.y + gp.padAbove + gp.borderBottom + t }
func (gp *mmGap) borderTopY() int    { return gp.y + gp.padAbove + gp.borderBottom + gp.tracks }

type mmBox struct {
	x0, y0, x1, y1 int
	title          string
	group          int
}

type mmLayout struct {
	g      *mmGraph
	wrap   int
	items  []*mmItem
	ranks  [][]int
	chains []*mmChain
	gaps   []mmGap
	rankY  []int
	rankH  []int
	boxes  []mmBox
	up     [][]int
	down   [][]int

	groupTop, groupBottom []int // rank span per subgraph, -1 when empty

	width, height int
}

func wrapText(s string, width int) []string {
	var out []string
	for _, para := range strings.Split(s, "\n") {
		words := strings.Fields(para)
		if len(words) == 0 {
			out = append(out, "")
			continue
		}
		cur := ""
		flush := func() {
			if cur != "" {
				out = append(out, cur)
				cur = ""
			}
		}
		for _, w := range words {
			for runewidth.StringWidth(w) > width {
				flush()
				cut, n := 0, 0
				for _, r := range w {
					rw := runewidth.RuneWidth(r)
					if n+rw > width {
						break
					}
					n += rw
					cut += len(string(r))
				}
				if cut == 0 {
					cut = len(string([]rune(w)[:1]))
				}
				out = append(out, w[:cut])
				w = w[cut:]
			}
			switch {
			case cur == "":
				cur = w
			case runewidth.StringWidth(cur)+1+runewidth.StringWidth(w) <= width:
				cur += " " + w
			default:
				flush()
				cur = w
			}
		}
		flush()
	}
	return out
}

func maxWidth(lines []string) int {
	m := 0
	for _, l := range lines {
		m = max(m, runewidth.StringWidth(l))
	}
	return m
}

// layers assigns each node a layer and reports which edges point upwards
// (back edges) and so are drawn reversed.
func (g *mmGraph) layers() ([]int, []bool) {
	n := len(g.nodes)
	adj := make([][]int, n)
	for i, e := range g.edges {
		adj[e.from] = append(adj[e.from], i)
	}
	state := make([]int, n)
	reversed := make([]bool, len(g.edges))
	var dfs func(u int)
	dfs = func(u int) {
		state[u] = 1
		for _, e := range adj[u] {
			switch v := g.edges[e].to; state[v] {
			case 0:
				dfs(v)
			case 1:
				reversed[e] = true
			}
		}
		state[u] = 2
	}
	for u := 0; u < n; u++ {
		if state[u] == 0 {
			dfs(u)
		}
	}

	indeg := make([]int, n)
	out := make([][]int, n)
	for i, e := range g.edges {
		a, b := e.from, e.to
		if reversed[i] {
			a, b = b, a
		}
		out[a] = append(out[a], b)
		indeg[b]++
	}
	layer := make([]int, n)
	var queue []int
	for u := 0; u < n; u++ {
		if indeg[u] == 0 {
			queue = append(queue, u)
		}
	}
	for len(queue) > 0 {
		u := queue[0]
		queue = queue[1:]
		for _, v := range out[u] {
			layer[v] = max(layer[v], layer[u]+1)
			if indeg[v]--; indeg[v] == 0 {
				queue = append(queue, v)
			}
		}
	}
	return layer, reversed
}

func layoutMermaid(g *mmGraph, wrap int) (*mmLayout, error) {
	l := &mmLayout{g: g, wrap: wrap}
	layer, reversed := g.layers()

	maxLayer := 0
	for _, v := range layer {
		maxLayer = max(maxLayer, v)
	}
	nRanks := 2*maxLayer + 1
	l.ranks = make([][]int, nRanks)

	add := func(it *mmItem) int {
		l.items = append(l.items, it)
		l.ranks[it.rank] = append(l.ranks[it.rank], len(l.items)-1)
		return len(l.items) - 1
	}

	// node items
	nodeItem := make([]int, len(g.nodes))
	for i, n := range g.nodes {
		it := &mmItem{kind: itemNode, node: i, rank: 2 * layer[i], group: n.sub}
		switch n.shape {
		case mmStart, mmEnd:
			it.w, it.h = 1, 1
		case mmRect, mmRound, mmDiamond:
			lines := wrapText(n.label, wrap)
			for _, loop := range n.loops {
				text := "↻"
				if loop != "" {
					text += " " + loop
				}
				lines = append(lines, wrapText(text, wrap)...)
			}
			it.lines = lines
			it.w = max(maxWidth(lines)+4, 5)
			it.h = len(lines) + 2
		}
		nodeItem[i] = add(it)
	}

	// edge chains
	for ei, e := range g.edges {
		top, bot := e.from, e.to
		headTop, headBottom := e.arrowFrom, e.arrowTo
		if reversed[ei] {
			top, bot = bot, top
			headTop, headBottom = headBottom, headTop
		}
		rt, rb := 2*layer[top], 2*layer[bot]
		group := -1
		if g.nodes[top].sub >= 0 && g.nodes[top].sub == g.nodes[bot].sub {
			group = g.nodes[top].sub
		}
		var odds []int
		for r := rt + 1; r < rb; r += 2 {
			odds = append(odds, r)
		}
		labelRank := -1
		if e.label != "" && len(odds) > 0 {
			labelRank = odds[len(odds)/2]
		}
		ch := &mmChain{edge: e, headTop: headTop, headBottom: headBottom}
		ch.items = append(ch.items, nodeItem[top])
		for r := rt + 1; r < rb; r++ {
			it := &mmItem{kind: itemDummy, rank: r, w: 1, h: 1, group: group, dotted: e.dotted}
			if r == labelRank {
				it.kind = itemLabel
				it.lines = wrapText(e.label, wrap)
				it.w = 2 + maxWidth(it.lines)
				it.h = len(it.lines)
			}
			ch.items = append(ch.items, add(it))
		}
		ch.items = append(ch.items, nodeItem[bot])
		ch.links = make([]mmLink, len(ch.items)-1)
		l.chains = append(l.chains, ch)
	}

	l.up = make([][]int, len(l.items))
	l.down = make([][]int, len(l.items))
	degree := make([]int, len(l.items))
	for _, ch := range l.chains {
		for k := 0; k+1 < len(ch.items); k++ {
			a, b := ch.items[k], ch.items[k+1]
			l.down[a] = append(l.down[a], b)
			l.up[b] = append(l.up[b], a)
		}
	}
	for i := range l.items {
		degree[i] = max(len(l.up[i]), len(l.down[i]))
	}

	// box heights are uniform within a rank; widths leave room for the ports
	l.rankH = make([]int, nRanks)
	boxH := make([]int, nRanks)
	for _, it := range l.items {
		if it.isBox(g) {
			boxH[it.rank] = max(boxH[it.rank], it.h)
		}
	}
	for i, it := range l.items {
		if it.isBox(g) {
			it.h = boxH[it.rank]
			it.w = max(it.w, degree[i]+2)
		}
		it.anchor = it.w / 2
		if it.kind != itemNode || !it.isBox(g) {
			it.anchor = 0
			if it.kind == itemNode {
				it.anchor = 0
			}
		}
		l.rankH[it.rank] = max(l.rankH[it.rank], it.h)
	}

	l.spans()
	l.orderRanks()
	l.placeX()
	l.clearGroupBands()
	l.assignPorts()
	l.straighten()
	l.assignRows()
	if err := l.buildBoxes(); err != nil {
		return nil, err
	}
	return l, nil
}

// spans records the first and last rank of every subgraph.
func (l *mmLayout) spans() {
	n := len(l.g.subs)
	l.groupTop = make([]int, n)
	l.groupBottom = make([]int, n)
	for i := range l.groupTop {
		l.groupTop[i], l.groupBottom[i] = -1, -1
	}
	for _, it := range l.items {
		if it.kind != itemNode || it.group < 0 {
			continue
		}
		g := it.group
		if l.groupTop[g] < 0 || it.rank < l.groupTop[g] {
			l.groupTop[g] = it.rank
		}
		if it.rank > l.groupBottom[g] {
			l.groupBottom[g] = it.rank
		}
	}
}

func (l *mmLayout) orderRanks() {
	pos := make([]float64, len(l.items))
	refresh := func(r int) {
		for i, id := range l.ranks[r] {
			pos[id] = float64(i)
		}
	}
	for r := range l.ranks {
		refresh(r)
	}

	sortRank := func(r int, nbr [][]int) {
		key := func(id int) float64 {
			if len(nbr[id]) == 0 {
				return pos[id]
			}
			s := 0.0
			for _, n := range nbr[id] {
				s += pos[n]
			}
			return s / float64(len(nbr[id]))
		}
		type unit struct {
			ids []int
			key float64
		}
		var units []*unit
		byGroup := map[int]*unit{}
		for _, id := range l.ranks[r] {
			if g := l.items[id].group; g >= 0 {
				u, ok := byGroup[g]
				if !ok {
					u = &unit{}
					byGroup[g] = u
					units = append(units, u)
				}
				u.ids = append(u.ids, id)
				continue
			}
			units = append(units, &unit{ids: []int{id}})
		}
		for _, u := range units {
			sort.SliceStable(u.ids, func(a, b int) bool { return key(u.ids[a]) < key(u.ids[b]) })
			for _, id := range u.ids {
				u.key += key(id)
			}
			u.key /= float64(len(u.ids))
		}
		sort.SliceStable(units, func(a, b int) bool { return units[a].key < units[b].key })
		var ordered []int
		for _, u := range units {
			ordered = append(ordered, u.ids...)
		}
		l.ranks[r] = ordered
		refresh(r)
	}

	for pass := 0; pass < 8; pass++ {
		if pass%2 == 0 {
			for r := 1; r < len(l.ranks); r++ {
				sortRank(r, l.up)
			}
		} else {
			for r := len(l.ranks) - 2; r >= 0; r-- {
				sortRank(r, l.down)
			}
		}
	}
}

// separation is the number of blank columns required between two
// neighbors of a rank.
func (l *mmLayout) separation(a, b *mmItem) int {
	if a.group == b.group {
		if a.isBox(l.g) && b.isBox(l.g) {
			return 3
		}
		return 2
	}
	s := 1
	if a.group >= 0 {
		s += 2
	}
	if b.group >= 0 {
		s += 2
	}
	return s
}

func isotonic(z []float64) []float64 {
	type block struct {
		sum float64
		n   int
	}
	var st []block
	mean := func(b block) float64 { return b.sum / float64(b.n) }
	for _, v := range z {
		st = append(st, block{v, 1})
		for len(st) > 1 && mean(st[len(st)-2]) > mean(st[len(st)-1]) {
			a, b := st[len(st)-2], st[len(st)-1]
			st = append(st[:len(st)-2], block{a.sum + b.sum, a.n + b.n})
		}
	}
	out := make([]float64, 0, len(z))
	for _, b := range st {
		for i := 0; i < b.n; i++ {
			out = append(out, mean(b))
		}
	}
	return out
}

func (l *mmLayout) placeX() {
	// start packed to the left
	for _, ids := range l.ranks {
		x := 0
		for i, id := range ids {
			it := l.items[id]
			it.x = x + it.anchor
			x += it.w
			if i+1 < len(ids) {
				x += l.separation(it, l.items[ids[i+1]])
			}
		}
	}

	relax := func(r int) {
		ids := l.ranks[r]
		if len(ids) == 0 {
			return
		}
		z := make([]float64, len(ids))
		c := make([]float64, len(ids))
		for i, id := range ids {
			it := l.items[id]
			desired, n := 0.0, 0
			for _, nb := range l.up[id] {
				desired += float64(l.items[nb].x)
				n++
			}
			for _, nb := range l.down[id] {
				desired += float64(l.items[nb].x)
				n++
			}
			if n == 0 {
				desired = float64(it.x)
			} else {
				desired /= float64(n)
			}
			if i > 0 {
				prev := l.items[ids[i-1]]
				c[i] = c[i-1] + float64(prev.w+l.separation(prev, it)-prev.anchor+it.anchor)
			}
			z[i] = desired - c[i]
		}
		y := isotonic(z)
		for i, id := range ids {
			l.items[id].x = int(math.Round(y[i])) + int(c[i])
		}
	}
	for iter := 0; iter < 12; iter++ {
		for r := 0; r < len(l.ranks); r++ {
			relax(r)
		}
		for r := len(l.ranks) - 1; r >= 0; r-- {
			relax(r)
		}
	}
}

// groupBand is the horizontal extent a subgraph box needs, borders included.
func (l *mmLayout) groupBand(gi int) (int, int) {
	bl, br := math.MaxInt, math.MinInt
	for _, it := range l.items {
		if it.group == gi {
			bl = min(bl, it.left()-2)
			br = max(br, it.right()+2)
		}
	}
	return bl, br
}

// clearGroupBands pushes everything that is not part of a subgraph out of
// the band the subgraph box covers, shifting the rest of the rank along.
func (l *mmLayout) clearGroupBands() {
	for iter := 0; iter < 8; iter++ {
		changed := false
		for gi := range l.g.subs {
			if l.groupTop[gi] < 0 {
				continue
			}
			bl, br := l.groupBand(gi)
			for r := l.groupTop[gi]; r <= l.groupBottom[gi]; r++ {
				ids := l.ranks[r]
				first, last := -1, -1
				for i, id := range ids {
					if l.items[id].group == gi {
						if first < 0 {
							first = i
						}
						last = i
					}
				}
				for i, id := range ids {
					f := l.items[id]
					if f.group == gi {
						continue
					}
					left := f.x < (bl+br)/2
					if first >= 0 {
						left = i < first
						if i > first && i < last {
							continue
						}
					}
					if left {
						if d := f.right() - (bl - 2); d > 0 {
							for _, other := range ids[:i+1] {
								l.items[other].x -= d
							}
							changed = true
						}
					} else if d := br + 2 - f.left(); d > 0 {
						for _, other := range ids[i:] {
							l.items[other].x += d
						}
						changed = true
					}
				}
			}
		}
		if !changed {
			return
		}
	}
}

// straighten moves the dummies and labels of every chain under the column
// where the chain leaves its top node, so that an edge drops straight down
// and jogs sideways only once, just above its target.
func (l *mmLayout) straighten() {
	rankPos := make([]int, len(l.items))
	for _, ids := range l.ranks {
		for i, id := range ids {
			rankPos[id] = i
		}
	}
	for _, ch := range l.chains {
		for k := 1; k+1 < len(ch.items); k++ {
			id := ch.items[k]
			it := l.items[id]
			want := ch.links[k-1].xo
			ids := l.ranks[it.rank]
			lo, hi := math.MinInt, math.MaxInt
			if p := rankPos[id]; p > 0 {
				prev := l.items[ids[p-1]]
				lo = prev.right() + 1 + l.separation(prev, it) + it.anchor
			}
			if p := rankPos[id]; p+1 < len(ids) {
				next := l.items[ids[p+1]]
				hi = next.left() - l.separation(it, next) - it.w + it.anchor
			}
			for gi := range l.g.subs {
				if it.group == gi || l.groupTop[gi] < 0 || it.rank < l.groupTop[gi] || it.rank > l.groupBottom[gi] {
					continue
				}
				bl, br := l.groupBand(gi)
				if it.x < (bl+br)/2 {
					hi = min(hi, bl-2-it.w+1+it.anchor)
				} else {
					lo = max(lo, br+2+it.anchor)
				}
			}
			if lo <= hi {
				want = min(max(want, lo), hi)
				it.x = want
			}
			ch.links[k-1].xi = it.x
			ch.links[k].xo = it.x
		}
	}
}

// assignPorts decides the columns where every link leaves and enters.
func (l *mmLayout) assignPorts() {
	type ref struct {
		ch *mmChain
		k  int
	}
	outs := make([][]ref, len(l.items))
	ins := make([][]ref, len(l.items))
	for _, ch := range l.chains {
		for k := range ch.links {
			outs[ch.items[k]] = append(outs[ch.items[k]], ref{ch, k})
			ins[ch.items[k+1]] = append(ins[ch.items[k+1]], ref{ch, k})
		}
	}

	spread := func(it *mmItem, n, i int) int {
		if !it.isBox(l.g) {
			return it.x
		}
		return it.left() + 1 + (2*i+1)*(it.w-2)/(2*n)
	}
	for id, it := range l.items {
		o := outs[id]
		sort.SliceStable(o, func(a, b int) bool {
			return l.items[o[a].ch.items[o[a].k+1]].x < l.items[o[b].ch.items[o[b].k+1]].x
		})
		for i, r := range o {
			r.ch.links[r.k].xo = spread(it, len(o), i)
		}
		in := ins[id]
		sort.SliceStable(in, func(a, b int) bool {
			return l.items[in[a].ch.items[in[a].k]].x < l.items[in[b].ch.items[in[b].k]].x
		})
		for i, r := range in {
			r.ch.links[r.k].xi = spread(it, len(in), i)
		}
	}
}

func (l *mmLayout) assignRows() {
	nRanks := len(l.ranks)
	l.gaps = make([]mmGap, nRanks+1)

	for _, ch := range l.chains {
		for k := range ch.links {
			lk := &ch.links[k]
			lk.track = -1
			g := &l.gaps[l.items[ch.items[k+1]].rank]
			g.links = append(g.links, lk)
			if k == 0 && ch.headTop {
				g.padAbove = 1
			}
			if k == len(ch.links)-1 {
				g.padBelow = 1
			}
		}
	}
	for gi := range l.groupTop {
		if l.groupTop[gi] < 0 {
			continue
		}
		top, bottom := &l.gaps[l.groupTop[gi]], &l.gaps[l.groupBottom[gi]+1]
		top.borderTop, top.padBelow = 1, 1
		bottom.borderBottom, bottom.padAbove = 1, 1
	}
	for gi := range l.gaps {
		l.gaps[gi].tracks = assignTracks(l.gaps[gi].links)
	}

	y := 0
	l.rankY = make([]int, nRanks)
	for gi := range l.gaps {
		l.gaps[gi].y = y
		y += l.gaps[gi].rows()
		if gi < nRanks {
			l.rankY[gi] = y
			y += l.rankH[gi]
		}
	}
	l.height = y
	for _, it := range l.items {
		it.y = l.rankY[it.rank]
	}
}

// assignTracks gives every link that has to jog sideways a row of the gap.
// Links whose vertical runs would meet are kept in order, and links whose
// horizontal runs overlap get different rows. It returns the number of rows.
func assignTracks(links []*mmLink) int {
	var jogs []*mmLink
	for _, lk := range links {
		if lk.xo != lk.xi {
			jogs = append(jogs, lk)
		}
	}
	if len(jogs) == 0 {
		return 0
	}

	// a before b when a leaves from the column b arrives at
	before := make([][]int, len(jogs))
	indeg := make([]int, len(jogs))
	for i, a := range jogs {
		for j, b := range jogs {
			if i != j && a.xo == b.xi {
				before[i] = append(before[i], j)
				indeg[j]++
			}
		}
	}
	var order []int
	done := make([]bool, len(jogs))
	for len(order) < len(jogs) {
		progressed := false
		for i := range jogs {
			if !done[i] && indeg[i] == 0 {
				done[i] = true
				order = append(order, i)
				for _, j := range before[i] {
					indeg[j]--
				}
				progressed = true
			}
		}
		if !progressed { // a cycle: give up the ordering for the rest
			for i := range jogs {
				if !done[i] {
					done[i] = true
					order = append(order, i)
				}
			}
		}
	}

	span := func(a *mmLink) (int, int) { return min(a.xo, a.xi), max(a.xo, a.xi) }
	compatible := func(a, b *mmLink) bool {
		a1, a2 := span(a)
		b1, b2 := span(b)
		if a2 < b1 || b2 < a1 {
			return true
		}
		if a2 == b1 {
			return (a.xi == b.xi && a.xi == a2) || (a.xo == b.xo && a.xo == a2)
		}
		if b2 == a1 {
			return (a.xi == b.xi && a.xi == a1) || (a.xo == b.xo && a.xo == a1)
		}
		return false
	}

	n := 0
	for _, i := range order {
		lk := jogs[i]
		t := 0
		for j, other := range jogs {
			if other.track >= 0 && j != i && other.xo == lk.xi && other.track+1 > t {
				t = other.track + 1
			}
		}
		for {
			ok := true
			for _, other := range jogs {
				if other != lk && other.track == t && !compatible(lk, other) {
					ok = false
					break
				}
			}
			if ok {
				break
			}
			t++
		}
		lk.track = t
		n = max(n, t+1)
	}
	return n
}

func (l *mmLayout) buildBoxes() error {
	g := l.g
	for gi, sub := range g.subs {
		if l.groupTop[gi] < 0 {
			continue
		}
		b := mmBox{group: gi, title: sub.title, x0: math.MaxInt, x1: math.MinInt}
		for _, it := range l.items {
			if it.group == gi {
				b.x0 = min(b.x0, it.left()-2)
				b.x1 = max(b.x1, it.right()+2)
			}
		}
		b.y0 = l.gaps[l.groupTop[gi]].borderTopY()
		b.y1 = l.gaps[l.groupBottom[gi]+1].borderBottomY()
		l.boxes = append(l.boxes, b)
	}

	// shift everything right of column 0
	minX, maxX := math.MaxInt, math.MinInt
	for _, it := range l.items {
		minX = min(minX, it.left())
		maxX = max(maxX, it.right())
	}
	for _, b := range l.boxes {
		minX = min(minX, b.x0)
		maxX = max(maxX, b.x1)
	}
	for _, it := range l.items {
		it.x -= minX
	}
	for i := range l.boxes {
		l.boxes[i].x0 -= minX
		l.boxes[i].x1 -= minX
	}
	for _, ch := range l.chains {
		for k := range ch.links {
			ch.links[k].xo -= minX
			ch.links[k].xi -= minX
		}
	}
	maxX -= minX

	// a subgraph box must not swallow nodes or edges that are not its own
	for i := range l.boxes {
		if l.boxOverlaps(i) {
			return mmUnsupported("subgraph %q overlaps other items", l.boxes[i].title)
		}
	}

	// the title sits in the top border: widen the box for it when there is
	// room, otherwise cut the title short
	for i := range l.boxes {
		b := &l.boxes[i]
		need := b.x0 + runewidth.StringWidth(b.title) + 5
		if need <= b.x1 {
			continue
		}
		natural := b.x1
		b.x1 = need
		if l.boxOverlaps(i) {
			b.x1 = natural
			b.title = runewidth.Truncate(b.title, b.x1-b.x0-4, "…")
		} else {
			maxX = max(maxX, b.x1)
		}
	}
	l.width = maxX + 1
	return nil
}

func (l *mmLayout) boxOverlaps(i int) bool {
	b := l.boxes[i]
	for _, it := range l.items {
		if it.group == b.group {
			continue
		}
		top, bottom := it.y, it.y+it.h-1
		if it.kind != itemNode {
			bottom = it.y + l.rankH[it.rank] - 1
		}
		if it.left() <= b.x1 && it.right() >= b.x0 && top <= b.y1 && bottom >= b.y0 {
			return true
		}
	}
	for j, o := range l.boxes {
		if j != i && o.x0 <= b.x1 && o.x1 >= b.x0 && o.y0 <= b.y1 && o.y1 >= b.y0 {
			return true
		}
	}
	return false
}
