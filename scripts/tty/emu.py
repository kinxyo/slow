import re

class Screen:
    """Just enough of a terminal to read what a Bubble Tea program leaves on screen."""
    def __init__(self, cols, rows):
        self.resize(cols, rows)
        self.x = self.y = 0
        self.buf = ""

    def resize(self, cols, rows):
        self.cols, self.rows = cols, rows
        old = getattr(self, "grid", None)
        self.grid = [[" "] * cols for _ in range(rows)]
        if old:
            for y in range(min(rows, len(old))):
                for x in range(min(cols, len(old[0]))):
                    self.grid[y][x] = old[y][x]
        self.x = min(getattr(self, "x", 0), cols - 1)
        self.y = min(getattr(self, "y", 0), rows - 1)

    CSI = re.compile(r"\x1b\[([0-9;:?<=>]*)([ -/]*)([@-~])")
    OSC = re.compile(r"\x1b\][^\x07\x1b]*(\x07|\x1b\\)")

    def feed(self, data):
        s = self.buf + data
        self.buf = ""
        i = 0
        while i < len(s):
            c = s[i]
            if c == "\x1b":
                m = self.OSC.match(s, i) or self.CSI.match(s, i)
                if not m:
                    if i + 1 >= len(s) or s[i + 1] in "[]":
                        self.buf = s[i:]; return
                    if s[i + 1] == "M": self.y = max(0, self.y - 1)  # reverse index: up a line
                    i += 2; continue
                if m.re is self.CSI:
                    self.csi(m.group(1), m.group(3))
                i = m.end(); continue
            if c == "\r": self.x = 0
            elif c == "\n": self.y = min(self.y + 1, self.rows - 1)
            elif c == "\b": self.x = max(0, self.x - 1)
            elif c >= " ":
                if self.x >= self.cols: self.x = 0; self.y = min(self.y + 1, self.rows - 1)
                self.grid[self.y][self.x] = c; self.x += 1
                self.last = c
            i += 1

    def csi(self, p, f):
        if p.startswith(("?", "<", "=", ">")): return
        n = [int(v) if v.isdigit() else 0 for v in re.split("[;:]", p)] if p else []
        a = n[0] if n and n[0] else 1
        if f in "Hf":
            self.y = min(max((n[0] if n and n[0] else 1) - 1, 0), self.rows - 1)
            self.x = min(max((n[1] if len(n) > 1 and n[1] else 1) - 1, 0), self.cols - 1)
        elif f == "A": self.y = max(0, self.y - a)
        elif f == "B": self.y = min(self.rows - 1, self.y + a)
        elif f == "C": self.x = min(self.cols - 1, self.x + a)
        elif f == "D": self.x = max(0, self.x - a)
        elif f == "G": self.x = min(max(a - 1, 0), self.cols - 1)
        elif f == "d": self.y = min(max(a - 1, 0), self.rows - 1)
        elif f == "K":
            k = n[0] if n else 0
            r = self.grid[self.y]
            if k == 0: r[self.x:] = [" "] * (self.cols - self.x)
            elif k == 1: r[:self.x + 1] = [" "] * (self.x + 1)
            else: r[:] = [" "] * self.cols
        elif f == "J":
            k = n[0] if n else 0
            if k in (2, 3): self.grid = [[" "] * self.cols for _ in range(self.rows)]
            elif k == 0:
                self.grid[self.y][self.x:] = [" "] * (self.cols - self.x)
                for y in range(self.y + 1, self.rows): self.grid[y] = [" "] * self.cols
        elif f == "X":
            for k in range(a):
                if self.x + k < self.cols: self.grid[self.y][self.x + k] = " "
        elif f == "b":
            for _ in range(a):
                if self.x < self.cols: self.grid[self.y][self.x] = getattr(self, "last", " "); self.x += 1
        elif f == "@":
            r = self.grid[self.y]; r[self.x:self.x] = [" "] * a; del r[self.cols:]
        elif f == "P":
            r = self.grid[self.y]; del r[self.x:self.x + a]; r.extend([" "] * (self.cols - len(r)))

    def lines(self):
        return ["".join(r).rstrip() for r in self.grid]
