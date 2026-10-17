#!/usr/bin/env python3
"""Check relative links and #anchors in Markdown files (GitHub slug rules).

Usage: check_links.py <file.md> [<file.md> ...]   (run from the repo root)
External http(s)/mailto links are skipped. Exit code 1 on any broken link.
"""
import os
import re
import sys

def slugs(path):
    out, seen = set(), {}
    in_code = False
    for line in open(path, encoding="utf-8"):
        if line.lstrip().startswith("```"):
            in_code = not in_code
            continue
        m = None if in_code else re.match(r"^#{1,6}\s+(.*?)\s*#*\s*$", line)
        if not m:
            continue
        s = m.group(1).strip().lower()
        s = re.sub(r"<[^>]+>", "", s)
        s = re.sub(r"[^\w\- ]", "", s).replace(" ", "-")
        n = seen.get(s, 0)
        seen[s] = n + 1
        out.add(s if n == 0 else f"{s}-{n}")
    return out

broken = []
for src in sys.argv[1:]:
    text = open(src, encoding="utf-8").read()
    text = re.sub(r"```.*?```", "", text, flags=re.S)
    text = re.sub(r"`[^`\n]*`", "", text)  # inline code holds examples, not links
    for target in re.findall(r"\]\(([^)\s]+)(?:\s+\"[^\"]*\")?\)", text) + re.findall(r'src="([^"]+)"', text):
        if re.match(r"^(https?:|mailto:)", target):
            continue
        path, _, anchor = target.partition("#")
        dest = os.path.normpath(os.path.join(os.path.dirname(src), path)) if path else src
        if path and not os.path.exists(dest):
            broken.append(f"{src}: missing file {target}")
            continue
        if anchor and dest.endswith(".md") and anchor not in slugs(dest):
            broken.append(f"{src}: missing anchor {target}")

print("\n".join(broken) if broken else "links: OK")
sys.exit(1 if broken else 0)
