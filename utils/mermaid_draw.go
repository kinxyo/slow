package utils

import (
	"strings"

	"github.com/mattn/go-runewidth"
)

const (
	mU uint8 = 1 << iota
	mD
	mL
	mR
)

type mmCell struct {
	r      rune // overrides the line characters when set
	mask   uint8
	dotted bool
	round  bool
	cont   bool // second column of a wide rune
}

type mmCanvas struct {
	w, h  int
	cells [][]mmCell
}

func newMMCanvas(w, h int) *mmCanvas {
	c := &mmCanvas{w: w, h: h, cells: make([][]mmCell, h)}
	for y := range c.cells {
		c.cells[y] = make([]mmCell, w)
	}
	return c
}

func (c *mmCanvas) at(x, y int) *mmCell {
	if x < 0 || y < 0 || x >= c.w || y >= c.h {
		return &mmCell{}
	}
	return &c.cells[y][x]
}

func (c *mmCanvas) or(x, y int, m uint8, dotted bool) {
	cell := c.at(x, y)
	cell.dotted = (cell.mask == 0 || cell.dotted) && dotted
	cell.mask |= m
}

// vline connects (x,y1) down to (x,y2).
func (c *mmCanvas) vline(x, y1, y2 int, dotted bool) {
	for y := y1; y <= y2; y++ {
		var m uint8
		if y > y1 {
			m |= mU
		}
		if y < y2 {
			m |= mD
		}
		c.or(x, y, m, dotted)
	}
}

// hline connects (x1,y) to (x2,y).
func (c *mmCanvas) hline(x1, x2, y int, dotted bool) {
	if x1 > x2 {
		x1, x2 = x2, x1
	}
	for x := x1; x <= x2; x++ {
		var m uint8
		if x > x1 {
			m |= mL
		}
		if x < x2 {
			m |= mR
		}
		c.or(x, y, m, dotted)
	}
}

func (c *mmCanvas) put(x, y int, r rune) {
	cell := c.at(x, y)
	cell.r = r
}

func (c *mmCanvas) text(x, y int, s string) {
	for _, r := range s {
		w := runewidth.RuneWidth(r)
		if w == 0 {
			continue
		}
		c.put(x, y, r)
		if w == 2 {
			c.at(x+1, y).cont = true
		}
		x += w
	}
}

var mmLineRunes = map[uint8]rune{
	mU: '│', mD: '│', mU | mD: '│',
	mL: '─', mR: '─', mL | mR: '─',
	mD | mR: '┌', mD | mL: '┐', mU | mR: '└', mU | mL: '┘',
	mU | mD | mR: '├', mU | mD | mL: '┤', mL | mR | mD: '┬', mL | mR | mU: '┴',
	mU | mD | mL | mR: '┼',
}

var mmRoundRunes = map[uint8]rune{
	mD | mR: '╭', mD | mL: '╮', mU | mR: '╰', mU | mL: '╯',
}

func (cell *mmCell) rune() rune {
	if cell.r != 0 {
		return cell.r
	}
	if cell.mask == 0 {
		return ' '
	}
	if cell.round {
		if r, ok := mmRoundRunes[cell.mask]; ok {
			return r
		}
	}
	if cell.dotted {
		switch cell.mask {
		case mU, mD, mU | mD:
			return '┆'
		case mL, mR, mL | mR:
			return '┄'
		}
	}
	return mmLineRunes[cell.mask]
}

func (c *mmCanvas) String() string {
	var sb strings.Builder
	for y, row := range c.cells {
		var line strings.Builder
		for x := range row {
			if row[x].cont {
				continue
			}
			line.WriteRune(row[x].rune())
		}
		sb.WriteString(strings.TrimRight(line.String(), " "))
		if y < len(c.cells)-1 {
			sb.WriteByte('\n')
		}
	}
	return sb.String()
}

// exitY is the row a link leaves an item from, entryY the row it arrives at.
func (l *mmLayout) exitY(it *mmItem) int {
	if it.kind == itemNode {
		return it.y + it.h - 1
	}
	return it.y + l.rankH[it.rank] - 1
}

func (l *mmLayout) entryY(it *mmItem) int { return it.y }

func (l *mmLayout) draw() string {
	g := l.g
	c := newMMCanvas(l.width, l.height)

	for _, b := range l.boxes {
		c.hline(b.x0, b.x1, b.y0, true)
		c.hline(b.x0, b.x1, b.y1, true)
		c.vline(b.x0, b.y0, b.y1, true)
		c.vline(b.x1, b.y0, b.y1, true)
		for _, p := range [][2]int{{b.x0, b.y0}, {b.x1, b.y0}, {b.x0, b.y1}, {b.x1, b.y1}} {
			c.at(p[0], p[1]).round = true
		}
	}

	for _, ch := range l.chains {
		dotted := ch.edge.dotted
		for k, lk := range ch.links {
			p, q := l.items[ch.items[k]], l.items[ch.items[k+1]]
			gp := &l.gaps[q.rank]
			start, end := l.exitY(p), l.entryY(q)
			arrowUp := k == 0 && ch.headTop
			arrowDown := k == len(ch.links)-1 && ch.headBottom
			if arrowUp {
				start++
			}
			if arrowDown {
				end--
			}
			if lk.xo == lk.xi {
				c.vline(lk.xo, start, end, dotted)
			} else {
				yt := gp.trackY(lk.track)
				c.vline(lk.xo, start, yt, dotted)
				c.hline(lk.xo, lk.xi, yt, dotted)
				c.vline(lk.xi, yt, end, dotted)
			}
			if arrowUp {
				c.put(lk.xo, start, '▲')
			}
			if arrowDown {
				c.put(lk.xi, end, '▼')
			}
		}
	}

	for _, it := range l.items {
		switch {
		case it.kind == itemNode && it.isBox(g):
			l.drawBox(c, it)
		case it.kind == itemNode:
			if g.nodes[it.node].shape == mmStart {
				c.put(it.x, it.y, '●')
			} else {
				c.put(it.x, it.y, '◉')
			}
		default:
			bottom := it.y + l.rankH[it.rank] - 1
			c.vline(it.x, it.y, bottom, it.dotted)
			for i, line := range it.lines {
				c.text(it.x+2, it.y+i, line)
			}
		}
	}

	for _, b := range l.boxes {
		c.text(b.x0+2, b.y0, " "+b.title+" ")
	}
	return c.String()
}

func (l *mmLayout) drawBox(c *mmCanvas, it *mmItem) {
	n := l.g.nodes[it.node]
	x0, y0 := it.left(), it.y
	x1, y1 := it.right(), it.y+it.h-1
	c.hline(x0, x1, y0, false)
	c.hline(x0, x1, y1, false)
	c.vline(x0, y0, y1, false)
	c.vline(x1, y0, y1, false)
	for _, p := range [][2]int{{x0, y0}, {x1, y0}, {x0, y1}, {x1, y1}} {
		cell := c.at(p[0], p[1])
		cell.round = n.shape == mmRound
		cell.dotted = false
	}
	if n.shape == mmDiamond {
		c.put(x0, y0, '╱')
		c.put(x1, y0, '╲')
		c.put(x0, y1, '╲')
		c.put(x1, y1, '╱')
	}
	inner := it.w - 2
	off := (it.h - 2 - len(it.lines)) / 2
	for i, line := range it.lines {
		pad := (inner - runewidth.StringWidth(line)) / 2
		c.text(x0+1+pad, y0+1+off+i, line)
	}
}
