"""Drive slo in a pseudo-terminal: answer its start-up questions, send keys,
resize the window and read the screen back through emu.Screen."""
import os, sys
sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
from emu import Screen
import pty, time, fcntl, termios, struct, select, re, signal, tempfile

ANSI = re.compile(r"\x1b(\[[0-9;?:<=>]*[ -/]*[@-~]|\][^\x07\x1b]*(\x07|\x1b\\)|[=>78()][0-9A-B]?)")
BAR = re.compile(r"term (\d+)×(\d+) · view (\d+)×(\d+) · col (\d+) wrap (\d+) · (\d+) lines, widest (\d+), ([\d.]+ \w+) · y (\d+)/(\d+) \((\d+)%\) · render ([\d.]+)ms #(\d+)(?: · relayout ([\d.]+)ms #(\d+))?")

class Term:
    def __init__(self, cols, rows, argv, env=None):
        self.cols, self.rows = cols, rows
        self.out = b""
        self.answered = 0
        pid, fd = pty.fork()
        if pid == 0:
            e = dict(os.environ, TERM="xterm-256color", SLO_TEXT_SIZING="off", COLORTERM="truecolor")
            e.update(env or {})
            os.execvpe(argv[0], argv, e)
        self.pid, self.fd = pid, fd
        self.screen = Screen(cols, rows)
        self.size(cols, rows, signal_=False)

    def size(self, cols, rows, signal_=True):
        self.cols, self.rows = cols, rows
        self.screen.resize(cols, rows) if hasattr(self, 'screen') else None
        fcntl.ioctl(self.fd, termios.TIOCSWINSZ, struct.pack("HHHH", rows, cols, 0, 0))
        if signal_:
            os.kill(self.pid, signal.SIGWINCH)

    def pump(self, secs):
        end = time.time() + secs
        while time.time() < end:
            r, _, _ = select.select([self.fd], [], [], 0.05)
            if not r:
                continue
            try:
                data = os.read(self.fd, 65536)
            except OSError:
                return False
            self.out += data
            self.screen.feed(data.decode('utf8', 'replace'))
            self.reply(data)
        return True

    def reply(self, data):
        t = data.decode("utf8", "replace")
        if "\x1b]10;?" in t: self.send("\x1b]10;rgb:ffff/ffff/ffff\x1b\\")
        if "\x1b]11;?" in t: self.send("\x1b]11;rgb:0000/0000/0000\x1b\\")
        if "\x1b[6n" in t: self.send("\x1b[1;1R")
        if "\x1b[c" in t or "\x1b[0c" in t: self.send("\x1b[?62;c")
        if "\x1b[?u" in t: self.send("\x1b[?0u")

    def send(self, s):
        self.answered += 1
        os.write(self.fd, s.encode())

    def key(self, k, wait=0.6):
        os.write(self.fd, k.encode()); self.pump(wait)

    def bars(self):
        txt = ANSI.sub("", self.out.decode("utf8", "replace"))
        return [m.groups() for m in BAR.finditer(txt)]

    def mark(self):
        return len(self.bars())

    def close(self):
        try:
            os.write(self.fd, b"q"); self.pump(0.3); os.kill(self.pid, signal.SIGKILL)
        except Exception: pass
        try: os.waitpid(self.pid, 0)
        except Exception: pass

def fmt(b):
    names = "termW termH viewW viewH col wrap lines widest size y total pct render n relayout m".split()
    return {k: v for k, v in zip(names, b)}

def bar(t):
    for l in t.screen.lines():
        m = BAR.search(l)
        if m: return fmt(m.groups())
    return None

def isolated_env():
    """HOME, config and cache in a fresh temporary directory, so slo neither
    reads nor writes the real ~/.config/slo."""
    home = tempfile.mkdtemp(prefix="slo-tty-")
    return home, {"HOME": home, "XDG_CONFIG_HOME": os.path.join(home, ".config"),
                  "XDG_CACHE_HOME": os.path.join(home, ".cache")}

def args(argv):
    """slo binary and document from the command line: [slo [document]]."""
    slo = argv[1] if len(argv) > 1 else "./slo"
    doc = argv[2] if len(argv) > 2 else "README.md"
    return os.path.abspath(slo), doc
