import { markdown, markdownLanguage } from '@codemirror/lang-markdown'
import { EditorSelection, EditorState, Transaction } from '@codemirror/state'
import { EditorView, keymap, runScopeHandlers } from '@codemirror/view'
import { describe, expect, it } from 'vitest'
import {
  replaceFilenameInText,
  tabIndentKeyBinding,
  tabIndentUnit,
} from './editorCommands'

// Runs the Tab / Shift-Tab binding against a real EditorState and returns the
// resulting document and cursor head. `head`/`anchor` default to a collapsed
// cursor; `extraCursors` adds further collapsed cursors.
function runTab(
  doc: string,
  {
    anchor,
    head = anchor,
    shift = false,
    extraCursors = [],
    readOnly = false,
  }: {
    anchor: number
    head?: number
    shift?: boolean
    extraCursors?: number[]
    readOnly?: boolean
  },
) {
  const state = EditorState.create({
    doc,
    selection: EditorSelection.create([
      EditorSelection.range(anchor, head),
      ...extraCursors.map((cursor) => EditorSelection.cursor(cursor)),
    ]),
    extensions: [
      markdown(),
      tabIndentUnit,
      EditorState.allowMultipleSelections.of(true),
      EditorState.readOnly.of(readOnly),
    ],
  })
  let next = state
  const command = shift ? tabIndentKeyBinding.shift : tabIndentKeyBinding.run
  const handled = command!({
    state,
    dispatch: (tr: Transaction) => {
      next = tr.state
    },
  } as unknown as EditorView)
  return {
    handled,
    doc: next.doc.toString(),
    head: next.selection.main.head,
  }
}

function runTabAtEnd(doc: string) {
  return runTab(doc, { anchor: doc.length })
}

// Whether Markdown really reads the document as a list inside a list item.
function hasNestedList(doc: string) {
  let nested = false
  markdownLanguage.parser.parse(doc).iterate({
    enter: (node) => {
      if (
        /^(Bullet|Ordered)List$/.test(node.name) &&
        node.node.parent?.name === 'ListItem'
      ) {
        nested = true
      }
    },
  })
  return nested
}

describe('tabIndentKeyBinding', () => {
  it('inserts an indent at the cursor instead of shifting the whole line', () => {
    // Cursor sits mid-line inside "## Heading" -> "## Hea|ding"
    const { handled, doc } = runTab('## Heading', { anchor: 6 })
    expect(handled).toBe(true)
    expect(doc).toBe('## Hea\tding')
    // Regression: the old `indentWithTab` produced "\t## Heading".
    expect(doc.startsWith('\t')).toBe(false)
  })

  it('indents the whole bullet line when the cursor is right after the list marker', () => {
    // Simulates pressing Enter on a list item ("- item 1\n- ") and then pressing Tab
    const initial = '- item 1\n- '
    const { handled, doc, head } = runTab(initial, { anchor: initial.length })
    expect(handled).toBe(true)
    expect(doc).toBe('- item 1\n\t- ')
    expect(head).toBe('- item 1\n\t- '.length)
  })

  it.each([
    { label: 'dash', input: '- a\n- ', expected: '- a\n\t- ' },
    { label: 'asterisk', input: '* a\n* ', expected: '* a\n\t* ' },
    { label: 'plus', input: '+ a\n+ ', expected: '+ a\n\t+ ' },
    {
      label: 'item below a nested sibling',
      input: '- a\n\t- b\n\t- ',
      expected: '- a\n\t- b\n\t\t- ',
    },
    {
      label: 'item that returned to the outer level',
      input: '- a\n\t- b\n- ',
      expected: '- a\n\t- b\n\t- ',
    },
    {
      label: 'unchecked task',
      input: '- [ ] a\n- [ ] ',
      expected: '- [ ] a\n\t- [ ] ',
    },
    {
      label: 'checked task',
      input: '- [x] a\n- [x] ',
      expected: '- [x] a\n\t- [x] ',
    },
    { label: 'loose list', input: '- a\n\n- ', expected: '- a\n\n\t- ' },
    {
      label: 'item after a continuation line',
      input: '- a\n  more\n- ',
      expected: '- a\n  more\n\t- ',
    },
    {
      label: 'item in a blockquote',
      input: '> - a\n> - ',
      expected: '> - a\n> \t- ',
    },
  ])('nests a $label', ({ input, expected }) => {
    const { handled, doc } = runTabAtEnd(input)
    expect(handled).toBe(true)
    expect(doc).toBe(expected)
    expect(hasNestedList(doc)).toBe(true)
  })

  it.each([
    { label: 'item 1.', input: '1. a\n1. ', expected: '1. a\n\t1. ' },
    { label: 'item 2.', input: '1. a\n2. ', expected: '1. a\n\t1. ' },
    { label: 'item 2)', input: '1) a\n2) ', expected: '1) a\n\t1) ' },
    { label: 'item 10.', input: '9. a\n10. ', expected: '9. a\n\t1. ' },
  ])(
    'restarts numbering when nesting ordered $label, because only 1 can start a nested list',
    ({ input, expected }) => {
      const { handled, doc, head } = runTabAtEnd(input)
      expect(handled).toBe(true)
      expect(doc).toBe(expected)
      expect(head).toBe(expected.length)
      expect(hasNestedList(doc)).toBe(true)
    },
  )

  it('nests an item that already has text when the cursor is right after the marker', () => {
    const { doc, head } = runTab('- a\n- b', { anchor: '- a\n- '.length })
    expect(doc).toBe('- a\n\t- b')
    expect(head).toBe('- a\n\t- '.length)
  })

  it.each([
    { label: 'first item of a list', input: '- ' },
    { label: 'first item of an ordered list', input: '1. ' },
    { label: 'first task', input: '- [ ] ' },
    { label: 'first item below a paragraph', input: 'text\n\n- ' },
    { label: 'first item of a nested list', input: '- a\n\t- ' },
    { label: 'item in a different list', input: '- a\n1. ' },
    { label: 'list line in a code fence', input: '```\n- a\n- ' },
    { label: 'numbered line in a code fence', input: '```\n1. a\n2. ' },
    { label: 'marker without a space', input: '- a\n-' },
  ])(
    'inserts a literal tab for $label, since indenting would break the list',
    ({ input }) => {
      const { handled, doc } = runTabAtEnd(input)
      expect(handled).toBe(true)
      expect(doc).toBe(`${input}\t`)
    },
  )

  it('nests every list item when all cursors qualify', () => {
    const initial = '- a\n- b\n- c'
    const { doc } = runTab(initial, {
      anchor: '- a\n- '.length,
      extraCursors: ['- a\n- b\n- '.length],
    })
    expect(doc).toBe('- a\n\t- b\n\t- c')
  })

  it('indents a line once when two cursors sit behind the same marker', () => {
    const { doc } = runTab('- a\n-  b', {
      anchor: '- a\n- '.length,
      extraCursors: ['- a\n-  '.length],
    })
    expect(doc).toBe('- a\n\t-  b')
  })

  it('falls back to literal tabs when any cursor does not qualify', () => {
    const { doc } = runTab('- a\n- b', {
      anchor: '- a'.length,
      extraCursors: ['- a\n- '.length],
    })
    expect(doc).toBe('- a\t\n- \tb')
  })

  it('leaves a read-only document alone', () => {
    const initial = '- a\n- '
    const { handled, doc } = runTab(initial, {
      anchor: initial.length,
      readOnly: true,
    })
    expect(handled).toBe(false)
    expect(doc).toBe(initial)
  })

  it('moves a nested item back out on Shift-Tab', () => {
    const { doc } = runTab('- a\n\t- b', {
      anchor: '- a\n\t- '.length,
      shift: true,
    })
    expect(doc).toBe('- a\n- b')
  })

  it('inserts a literal tab at the cursor when inside list item text', () => {
    // Cursor sits mid-text inside a list item: "- hello| world"
    const initial = '- hello world'
    const { handled, doc } = runTab(initial, { anchor: '- hello'.length })
    expect(handled).toBe(true)
    expect(doc).toBe('- hello\t world')
  })

  it('nests the new item when typing Enter then Tab in a real editor', () => {
    const view = new EditorView({
      state: EditorState.create({
        doc: '- a',
        selection: { anchor: '- a'.length },
        extensions: [
          markdown(),
          tabIndentUnit,
          keymap.of([tabIndentKeyBinding]),
        ],
      }),
    })
    const press = (key: string) =>
      runScopeHandlers(view, new KeyboardEvent('keydown', { key }), 'editor')

    expect(press('Enter')).toBe(true)
    expect(press('Tab')).toBe(true)

    expect(view.state.doc.toString()).toBe('- a\n\t- ')
    expect(view.state.selection.main.head).toBe('- a\n\t- '.length)
    view.destroy()
  })

  it('indents every line when the selection spans multiple lines', () => {
    const { doc } = runTab('alpha\nbravo', {
      anchor: 0,
      head: 'alpha\nbravo'.length,
    })
    expect(doc).toBe('\talpha\n\tbravo')
  })

  it('dedents the current line on Shift-Tab', () => {
    const { doc } = runTab('\t\tindented', { anchor: 4, shift: true })
    expect(doc).toBe('\tindented')
  })
})

describe('replaceFilenameInText', () => {
  it('updates both the src filename and the alt text when the alt text matches the old filename', () => {
    const doc = '![old-name.png](/assets/old-name.png)'
    const result = replaceFilenameInText(doc, 'old-name.png', 'new-name.png')
    expect(result).toBe('![new-name.png](/assets/new-name.png)')
  })

  it('updates the src filename but preserves custom alt text that does not match the old filename', () => {
    const doc = '![a nice photo](/assets/old-name.png)'
    const result = replaceFilenameInText(doc, 'old-name.png', 'new-name.png')
    expect(result).toBe('![a nice photo](/assets/new-name.png)')
  })

  it('updates plain (non-image) markdown links the same way', () => {
    const doc = '[old-name.pdf](/assets/old-name.pdf)'
    const result = replaceFilenameInText(doc, 'old-name.pdf', 'new-name.pdf')
    expect(result).toBe('[new-name.pdf](/assets/new-name.pdf)')
  })

  it('handles filenames containing regex-special characters', () => {
    const doc = '![old (1).png](/assets/old (1).png)'
    const result = replaceFilenameInText(doc, 'old (1).png', 'new.png')
    expect(result).toBe('![new.png](/assets/new.png)')
  })

  it('replaces every occurrence in the document', () => {
    const doc = [
      '![old-name.png](/assets/old-name.png)',
      'See also [old-name.png](/assets/old-name.png) below.',
    ].join('\n')
    const result = replaceFilenameInText(doc, 'old-name.png', 'new-name.png')
    expect(result).toBe(
      [
        '![new-name.png](/assets/new-name.png)',
        'See also [new-name.png](/assets/new-name.png) below.',
      ].join('\n'),
    )
  })

  it('does not match across multiple links on the same line', () => {
    const doc =
      '![foo.png](/assets/foo.png) and ![old-name.png](/assets/old-name.png)'
    const result = replaceFilenameInText(doc, 'old-name.png', 'new-name.png')
    expect(result).toBe(
      '![foo.png](/assets/foo.png) and ![new-name.png](/assets/new-name.png)',
    )
  })
})
