import { EditorSelection, EditorState, Transaction } from '@codemirror/state'
import type { EditorView } from '@codemirror/view'
import { describe, expect, it } from 'vitest'
import {
  replaceFilenameInText,
  tabIndentKeyBinding,
  tabIndentUnit,
} from './editorCommands'

// Runs the Tab / Shift-Tab binding against a real EditorState and returns the
// resulting document and cursor head. `head`/`anchor` default to a collapsed cursor.
function runTab(
  doc: string,
  {
    anchor,
    head = anchor,
    shift = false,
  }: { anchor: number; head?: number; shift?: boolean },
) {
  const state = EditorState.create({
    doc,
    selection: EditorSelection.single(anchor, head),
    extensions: [tabIndentUnit],
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
    // Regression: the old "insertTab" usage produced "- item 1\n- \t"
    expect(doc).not.toBe('- item 1\n- \t')
  })

  it.each([
    { label: 'dash bullet', input: '- ', expected: '\t- ' },
    { label: 'asterisk bullet', input: '* ', expected: '\t* ' },
    { label: 'plus bullet', input: '+ ', expected: '\t+ ' },
    { label: 'already indented bullet', input: '\t- ', expected: '\t\t- ' },
    { label: 'numbered dot list', input: '1. ', expected: '\t1. ' },
    { label: 'numbered paren list', input: '1) ', expected: '\t1) ' },
    { label: 'unchecked task list', input: '- [ ] ', expected: '\t- [ ] ' },
    { label: 'checked task list', input: '- [x] ', expected: '\t- [x] ' },
  ])('indents the whole line when cursor follows $label ($input)', ({ input, expected }) => {
    const { handled, doc } = runTab(input, { anchor: input.length })
    expect(handled).toBe(true)
    expect(doc).toBe(expected)
  })

  it('inserts a literal tab at the cursor when inside list item text', () => {
    // Cursor sits mid-text inside a list item: "- hello| world"
    const initial = '- hello world'
    const { handled, doc } = runTab(initial, { anchor: '- hello'.length })
    expect(handled).toBe(true)
    expect(doc).toBe('- hello\t world')
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