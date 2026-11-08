// Browser-native spellcheck for the Markdown editor (#1731).
//
// CodeMirror renders its content element with spellcheck="false", so the
// browser never checks what people type. This turns it back on and marks the
// parts of the Markdown that are not prose (code, URLs, HTML tags) with
// spellcheck="false" so they don't drown the real typos in red. The language
// follows the browser's spellcheck settings and the `lang` attribute, which
// tracks the UI language (see src/lib/documentLang.ts).
//
// autocorrect and autocapitalize stay at CodeMirror's "off": on mobile they
// rewrite Markdown syntax while you type it.

import { syntaxTree } from '@codemirror/language'
import type { Range } from '@codemirror/state'
import {
  Decoration,
  type DecorationSet,
  EditorView,
  ViewPlugin,
  type ViewUpdate,
} from '@codemirror/view'

// Lezer Markdown node names whose text is not prose. Children of a match are
// skipped, so an Autolink is excluded as a whole, not twice.
const NON_PROSE_NODES = new Set([
  'InlineCode',
  'FencedCode',
  'CodeBlock',
  'URL',
  'Autolink',
  // Inline tags only: an HTMLBlock spans its prose too.
  'HTMLTag',
])

const noSpellcheckMark = Decoration.mark({
  attributes: { spellcheck: 'false' },
})

export type TextRange = { from: number; to: number }
type SyntaxTree = ReturnType<typeof syntaxTree>

/** Non-prose ranges of a Markdown syntax tree that overlap [from, to]. */
export function noSpellcheckRanges(
  tree: SyntaxTree,
  from: number,
  to: number,
): TextRange[] {
  const ranges: TextRange[] = []
  tree.iterate({
    from,
    to,
    enter(node) {
      if (!NON_PROSE_NODES.has(node.name)) return
      ranges.push({ from: node.from, to: node.to })
      return false
    },
  })
  return ranges
}

function buildDecorations(view: EditorView): DecorationSet {
  const marks: Range<Decoration>[] = []
  // The tree may still be partial; the plugin rebuilds once parsing advances.
  const tree = syntaxTree(view.state)
  for (const { from, to } of view.visibleRanges) {
    for (const range of noSpellcheckRanges(tree, from, to)) {
      if (range.from < range.to) {
        marks.push(noSpellcheckMark.range(range.from, range.to))
      }
    }
  }
  // Visible ranges are disjoint, but a block spanning two of them shows up in
  // both; Decoration.set sorts, and duplicates are harmless.
  return Decoration.set(marks, true)
}

const noSpellcheckPlugin = ViewPlugin.fromClass(
  class {
    decorations: DecorationSet

    constructor(view: EditorView) {
      this.decorations = buildDecorations(view)
    }

    update(update: ViewUpdate) {
      if (
        update.docChanged ||
        update.viewportChanged ||
        syntaxTree(update.startState) !== syntaxTree(update.state)
      ) {
        this.decorations = buildDecorations(update.view)
      }
    }
  },
  { decorations: (plugin) => plugin.decorations },
)

export const spellcheck = [
  EditorView.contentAttributes.of({ spellcheck: 'true' }),
  noSpellcheckPlugin,
]
