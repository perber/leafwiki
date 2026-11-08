# ADR-0021: The page side panel renders sections composed by the page viewer

## Status

Proposed (2026-11-08)

## Context

The page viewer shows a side panel (`features/preview/TocSidePanel.tsx`) and, in the
subheader, a dropdown with the same content (`features/preview/TocDropdownButton.tsx`).
Both listed two hard-coded groups: the page's headings ("On this page") and its attachments
("Attached Media"). Each panel had its own branches for the two groups, the panel title came
from `tocTitleKey(entryCount, downloadCount)`, and `features/viewer/PageViewer.tsx` decided
whether to show the panel at all with `showToc || attachments.length > 0`.

Issue [#1521](https://github.com/perber/leafwiki/issues/1521) adds a third group (the page's
highlights, between headings and attachments). Under that shape it would touch both panels, the
title function and the visibility check, and every further group would do the same, which
[rule 1](../rules.md) rules out.

The content of every group comes from per-page data (parsed headings, fetched attachments)
and, for headings, from per-page state (the scroll spy's active heading).

## Decision

- `features/preview/pagePanelSection.ts` defines the seam:
  `PagePanelSection = { id, titleKey, items: PagePanelItem[] }` and a closed set of item kinds,
  `anchor` (label, optional level, `active`, optional `onSelect`) and `download` (`href`,
  `fileName`). Each item carries a `testId` suffix; the side panel prefixes `toc-`, the dropdown
  `toc-dropdown-`.
- Both panels take a plain `sections` prop and render it generically: empty sections are
  skipped, the first non-empty one is titled by the panel header or trigger button
  (`panelTitleKey`), every later one gets its own heading. The panels don't know which
  sections exist.
- Producers live in the feature that owns the data and import the seam type from
  `features/preview`, following the existing viewer → preview direction:
  `features/preview/useHeadingsSection.ts` (headings come from the Markdown preview pipeline)
  owns the scroll spy and the click-to-scroll behaviour, `features/viewer/attachmentsSection.ts`
  (`buildAttachmentsSection(attachments)`, the viewer fetches the attachments) maps
  attachments to download items. The highlights producer is split in two:
  `features/preview/collectHighlights.ts` is a pure function that reads the `==text==` marks
  out of rendered preview output (it takes the root as a parameter and sits next to the
  pipeline that renders the marks, so a change to how marks render is made in one feature),
  and `features/viewer/useHighlightsSection.ts` builds the section, because it reads the
  viewer's content root (`.page-viewer__content`) after each render. A section `id` is unique
  within one list; the panels use it as React key.
- `features/viewer/PageViewer.tsx` composes the list (`[headings, highlights, attachments]`)
  and passes the same array to both panels, so one scroll spy serves both. Whether the panel
  is shown is derived from the sections (`sections.some(s => s.items.length > 0)`); the
  "always show TOC" rule stays in what the viewer feeds the headings section. The inline TOC
  dropdown in `MarkdownPreview.tsx` passes just the headings section.

Adding a section is one more entry in that list. Adding an item kind (a new render primitive)
changes the seam and both panels.

## Alternatives considered

- **A registry in `lib/registries/`**, like dialogs and sidebar panels: those register
  components once at startup. Sections are built from per-page data and hook state on every
  render, so a registry entry would still need the viewer to hand it the page, the attachments
  and the scroll spy; it adds indirection without removing the composition step.
- **Sections render their own React nodes**: the same section has to look different in the
  side panel and in the dropdown (Radix menu items), and the collapsed panel needs to drop
  items out of the tab order. Render primitives keep that in the panels.
- **Keep the branches and add a third one**: touches four places per new group.

## Consequences

- A new group (highlights, backlinks, …) is a producer plus one entry in `PageViewer`; the
  panel title and visibility follow without changes.
- A group that needs a new kind of row (e.g. with an icon or a secondary line) has to add an
  item kind to `PagePanelItem` and render it in both panels.
- Revisit if sections need to be contributed by features the viewer shouldn't know about;
  then a registry of section producers becomes worth its indirection.
