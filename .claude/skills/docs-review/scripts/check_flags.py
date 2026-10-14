#!/usr/bin/env python3
"""Compare `leafwiki --help` (generated from the flag definitions) with the docs.

Usage: check_flags.py <help-output-file> <doc.md> [<doc.md> ...]

Reports flags/env vars that exist in the binary but are missing from the docs,
names documented but no longer existing, and default values that differ.
Exit code 1 if anything is off.
"""
import re
import sys

help_text = open(sys.argv[1]).read()
docs = {p: open(p).read() for p in sys.argv[2:]}
alldocs = "\n".join(docs.values())

# --flag type  usage (default: X) [$ENV]
code = {}
for m in re.finditer(r"^\s+--([a-z0-9-]+)(?: [a-z]+)?\s{2,}(.*?)\[\$([A-Z0-9_]+)\]\s*$", help_text, re.M):
    flag, usage, env = m.groups()
    d = re.search(r"\(default: (.*?)\)\s*$", usage.strip())
    default = d.group(1).strip('"') if d else None
    code[flag] = (env, default)
code_envs = {e for e, _ in code.values()} | {"LEAFWIKI_LOG_LEVEL"}

problems = []
for flag, (env, default) in sorted(code.items()):
    if f"`--{flag}`" not in alldocs:
        problems.append(f"missing flag in docs: --{flag}")
    if f"`{env}`" not in alldocs:
        problems.append(f"missing env var in docs: {env}")

doc_flags = set(re.findall(r"`--([a-z0-9][a-z0-9-]+)(?:[=` ])", alldocs))
for f in sorted(doc_flags - set(code) - {"help", "version", "non-interactive", "env-file", "detach"}):
    problems.append(f"documented flag not in binary: --{f}")
for e in sorted(set(re.findall(r"`(LEAFWIKI_[A-Z0-9_]+)`", alldocs)) - code_envs):
    problems.append(f"documented env var not in binary: {e}")

# Defaults: check table rows "| `--flag` | ... | `default` | ..."
for path, text in docs.items():
    for row in re.finditer(r"^\|\s*`--([a-z0-9-]+)`\s*\|(.*)$", text, re.M):
        flag, rest = row.groups()
        if flag not in code or code[flag][1] is None:
            continue
        cells = [c.strip() for c in rest.split("|")]
        want = code[flag][1]
        if not any(c.strip("`") == want for c in cells):
            problems.append(f"{path}: default for --{flag} should be `{want}` (row: {' | '.join(cells[:3])})")

print("\n".join(problems) if problems else "flags/env vars/defaults: OK")
sys.exit(1 if problems else 0)
