import { markdown } from '@codemirror/lang-markdown'
import { ensureSyntaxTree, forceParsing } from '@codemirror/language'
import { EditorState } from '@codemirror/state'
import { EditorView } from '@codemirror/view'
import { afterEach, describe, expect, it } from 'vitest'
import { noSpellcheckRanges, spellcheck } from './spellcheck'

// The parser works against a time budget; force a complete tree so the
// tests don't depend on machine load.
function parse(doc: string) {
  const state = EditorState.create({ doc, extensions: [markdown()] })
  const tree = ensureSyntaxTree(state, doc.length, 10_000)
  if (!tree) throw new Error('markdown parse did not finish')
  return tree
}

function excludedTexts(doc: string): string[] {
  return noSpellcheckRanges(parse(doc), 0, doc.length).map(({ from, to }) =>
    doc.slice(from, to),
  )
}

describe('noSpellcheckRanges', () => {
  it('leaves prose alone', () => {
    expect(excludedTexts('# Überschrift\n\nEin ganz normaler Satz.')).toEqual(
      [],
    )
  })

  it('excludes inline code', () => {
    expect(excludedTexts('Run `npm ci` first.')).toEqual(['`npm ci`'])
  })

  it('excludes fenced code blocks', () => {
    const block = '```go\nfunc main() {}\n```'
    expect(excludedTexts(`Text\n\n${block}\n\nMore text`)).toEqual([block])
  })

  it('excludes indented code blocks', () => {
    expect(excludedTexts('Text\n\n    fmt.Println()\n')).toEqual([
      'fmt.Println()',
    ])
  })

  it('excludes link and image URLs but keeps the link text', () => {
    expect(
      excludedTexts(
        'See [the docs](https://example.com/dcs) and ![Bild](img.png)',
      ),
    ).toEqual(['https://example.com/dcs', 'img.png'])
  })

  it('excludes autolinks', () => {
    expect(excludedTexts('Go to <https://example.com>.')).toEqual([
      '<https://example.com>',
    ])
  })

  it('excludes inline HTML tags but keeps the text between them', () => {
    expect(excludedTexts('Ein <kbd class="key">Wort</kbd> hier')).toEqual([
      '<kbd class="key">',
      '</kbd>',
    ])
  })

  it('only returns ranges that overlap the requested window', () => {
    const doc = '`a`\n\nprose\n\n`b`'
    const from = doc.indexOf('`b`')
    expect(
      noSpellcheckRanges(parse(doc), from, doc.length).map(({ from, to }) =>
        doc.slice(from, to),
      ),
    ).toEqual(['`b`'])
  })
})

describe('spellcheck extension', () => {
  let view: EditorView | null = null

  afterEach(() => {
    view?.destroy()
    view = null
  })

  function mount(doc: string) {
    view = new EditorView({
      state: EditorState.create({ doc, extensions: [markdown(), spellcheck] }),
      parent: document.body,
    })
    forceParsing(view, doc.length, 10_000)
    return view
  }

  it('turns the browser spellcheck on for the content element', () => {
    expect(mount('Hallo').contentDOM.getAttribute('spellcheck')).toBe('true')
  })

  it('keeps autocorrect and autocapitalize off', () => {
    const content = mount('Hallo').contentDOM
    expect(content.getAttribute('autocorrect')).toBe('off')
    expect(content.getAttribute('autocapitalize')).toBe('off')
  })

  it('marks code with spellcheck="false"', () => {
    const marked = mount('Run `npm ci` now').contentDOM.querySelectorAll(
      '[spellcheck="false"]',
    )
    expect(Array.from(marked, (el) => el.textContent)).toEqual(['`npm ci`'])
  })

  it('updates the marks when the document changes', () => {
    const editor = mount('Run it now')
    editor.dispatch({ changes: { from: 4, to: 6, insert: '`make ci`' } })
    forceParsing(editor, editor.state.doc.length, 10_000)
    const marked = editor.contentDOM.querySelectorAll('[spellcheck="false"]')
    expect(Array.from(marked, (el) => el.textContent)).toEqual(['`make ci`'])
  })
})
