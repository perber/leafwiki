# Developer documentation

How LeafWiki is built, for contributors and their coding agents. Setup and contribution rules
are in [CONTRIBUTING.md](../../CONTRIBUTING.md); agent instructions in [AGENTS.md](../../AGENTS.md).

- [Rules](./rules.md): the short list of rules for every change; read this first.
- [Conventions](./conventions.md): the reference behind the rules, by area.
- [Source map](./source-map.md): what every backend and frontend directory does.
- [Architecture](./architecture.md): layering, init order, storage model (derived vs. real
  state), auth model, Markdown rendering pipeline.
- [Processes](./processes.md): core workflows end to end: page save, revisioning, wikilink
  resolution, resync, editor auto-save, tree rename/move, auth.
- [Architecture decision records](./adr/README.md): why things are the way they are, when to
  write a new ADR, and the template.

These pages name code by package, file and function, not line numbers, so they don't go stale
with every edit. If you find a statement that no longer matches the code, fix it in the same PR.
