"""Resize slo while it shows a document and print the debug bar after each
step: which resizes render again and which only lay the text out again.

    go build -o slo . && python3 scripts/tty/resize.py ./slo testfile.md
"""
import os, sys
sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
from drive import Term, isolated_env, args, bar

slo, doc = args(sys.argv)
home, env = isolated_env()
t = Term(200, 50, [slo, "-t", doc], env=env)
t.pump(3.0)
t.key("D", 1.0)

def show(label):
    b = bar(t)
    keys = ("termW", "viewW", "col", "wrap", "y", "total", "render", "n", "relayout", "m")
    print(f"{label:28}", {k: b[k] for k in keys} if b else "NO BAR")

show("start 200x50")
t.size(260, 50); t.pump(1.5); show("resize 260 (same wrap)")
t.size(80, 50);  t.pump(3.0); show("resize 80 (wrap 100->80)")
t.size(200, 50); t.pump(3.0); show("resize 200 (wrap 80->100)")
t.key("-", 3.0); show("press '-' (column 90)")
t.key("G", 1.0); show("press G (bottom)")
t.size(230, 50); t.pump(1.5); show("resize 230 at the bottom")
t.close()
