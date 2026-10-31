"""Print the first screens of a document as slo shows them, 180×45.

    go build -o slo . && python3 scripts/tty/screens.py ./slo testfile.md [pages]
"""
import os, sys
sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
from drive import Term, isolated_env, args

slo, doc = args(sys.argv)
pages = int(sys.argv[3]) if len(sys.argv) > 3 else 4
_, env = isolated_env()
t = Term(180, 45, [slo, "-t", doc], env=env)
t.pump(3.0)
for _ in range(pages):
    print("\n".join("|" + l.rstrip() for l in t.screen.lines()))
    print("=" * 60)
    t.key("f", 0.8)
t.close()
