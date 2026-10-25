#!/usr/bin/env python3
# The work list: what rugaroo compiled to a NotImplementedError stub (a gem method, when arm or branch that does not
# compile, decisions 205/244/251) that MRI actually reached running the traced command (work/calls.txt, lines.txt).
import re, sys, collections
go, work = sys.argv[1], sys.argv[2]
calls = collections.defaultdict(set)
for l in open(f"{work}/calls.txt"):
    p, _, m = l.strip().partition("\t")
    calls[m].add(p)
lines = set(l.strip() for l in open(f"{work}/lines.txt"))
src = open(go).read()
hits = set()
for who, why in re.findall(r'rugaroo: ((?:[^"\\]|\\.)*?) does not compile: ((?:[^"\\]|\\.)*)', src):
    why = why.encode().decode("unicode_escape")
    loc = re.match(r'(\S+?:\d+): ', why)
    if who.startswith("a when arm") or who.startswith("a branch"):
        if loc and loc.group(1) in lines:
            hits.add((who, why))
        continue
    m = re.match(r'(\S+?)(?:#|\.)(\S+)$', who)
    if not m:
        continue
    meth = m.group(2)
    if loc and loc.group(1).rsplit(":", 1)[0] in calls.get(meth, ()) or why.startswith("Go rejects it") and meth in calls:
        hits.add((who, why))
for who, why in sorted(hits):
    print(f"{who}\t{re.sub(r'\S*/Library/Homebrew/', '', why)[:240]}")
print(f"{len(hits)} hits", file=sys.stderr)
