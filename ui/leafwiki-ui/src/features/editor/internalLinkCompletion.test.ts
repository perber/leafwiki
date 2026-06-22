import {
  CompletionContext,
  autocompletion,
  completionStatus,
  type Completion,
  currentCompletions,
  startCompletion,
} from '@codemirror/autocomplete'
import { EditorState } from '@codemirror/state'
import { EditorView } from '@codemirror/view'
import { describe, it, expect, beforeEach, vi } from 'vitest'
import {
  hasSuppressedExternalPrefix,
  buildMarkdownLinkOptions,
  buildWikiLinkOptions,
  wikiLinkCompletionSource,
} from './internalLinkCompletion'
import type { FlatPageSearchItem } from '@/lib/pageSearch'
import { useTreeStore } from '@/stores/tree'

const item = (
  title: string,
  path: string,
  breadcrumb?: string,
): FlatPageSearchItem => ({
  id: path,
  title,
  path,
  kind: 'page',
  breadcrumb: breadcrumb ?? title,
  searchText: `${title} ${path}`,
  normalizedTitle: title.toLowerCase().trim(),
  normalizedPath: path.toLowerCase().trim(),
  normalizedBreadcrumb: (breadcrumb ?? title).toLowerCase().trim(),
})

function expectCompletionApply(
  completion: Completion | undefined,
): (
  view: EditorView,
  completion: Completion,
  from: number,
  to: number,
) => void {
  expect(completion).toBeTruthy()
  expect(typeof completion?.apply).toBe('function')
  return completion!.apply as (
    view: EditorView,
    completion: Completion,
    from: number,
    to: number,
  ) => void
}

describe('hasSuppressedExternalPrefix', () => {
  it('suppresses http', () => {
    expect(hasSuppressedExternalPrefix('http')).toBe(true)
  })

  it('suppresses http: and https:', () => {
    expect(hasSuppressedExternalPrefix('http:')).toBe(true)
    expect(hasSuppressedExternalPrefix('https:')).toBe(true)
  })

  it('suppresses full external URLs', () => {
    expect(hasSuppressedExternalPrefix('https://example.com')).toBe(true)
    expect(hasSuppressedExternalPrefix('mailto:user@example.com')).toBe(true)
  })

  it('suppresses with leading whitespace', () => {
    expect(hasSuppressedExternalPrefix('  https://example.com')).toBe(true)
  })

  it('is case-insensitive', () => {
    expect(hasSuppressedExternalPrefix('HTTP://example.com')).toBe(true)
    expect(hasSuppressedExternalPrefix('MAILTO:user@example.com')).toBe(true)
  })

  it('does not suppress internal paths', () => {
    expect(hasSuppressedExternalPrefix('/docs/intro')).toBe(false)
    expect(hasSuppressedExternalPrefix('docs/intro')).toBe(false)
    expect(hasSuppressedExternalPrefix('')).toBe(false)
  })

  it('suppresses mailto as a plain word without colon', () => {
    expect(hasSuppressedExternalPrefix('mailto')).toBe(true)
  })
})

describe('wikiLinkCompletionSource', () => {
  beforeEach(() => {
    useTreeStore.setState({ flatPages: [] })
  })

  it('replaces a single trailing closing bracket', () => {
    useTreeStore.setState({
      flatPages: [item('Target Page', 'docs/target-page')],
    })

    const doc = '[[Target]'
    const state = EditorState.create({ doc })
    const context = new CompletionContext(state, doc.length - 1, true)
    const result = wikiLinkCompletionSource(context)

    expect(result).not.toBeNull()
    expect(result?.from).toBe(2)
    expect(result?.to).toBe(doc.length)
    expect(result?.options[0]?.apply).toEqual(expect.any(Function))
  })

  it('does not replace text after the cursor on the same line', () => {
    useTreeStore.setState({
      flatPages: [item('Target Page', 'docs/target-page')],
    })

    const doc = 'Before [[Target after text'
    const cursorPos = 'Before [[Target'.length
    const state = EditorState.create({ doc })
    const context = new CompletionContext(state, cursorPos, true)
    const result = wikiLinkCompletionSource(context)

    expect(result).not.toBeNull()
    const completionResult = result!
    expect(completionResult.from).toBe('Before [['.length)
    expect(completionResult.to).toBe(cursorPos)

    const parent = document.createElement('div')
    document.body.appendChild(parent)
    const view = new EditorView({
      state: EditorState.create({
        doc,
        selection: { anchor: cursorPos },
      }),
      parent,
    })

    try {
      const apply = expectCompletionApply(completionResult.options[0])
      apply(
        view,
        completionResult.options[0]!,
        completionResult.from,
        completionResult.to!,
      )
      expect(view.state.doc.toString()).toBe(
        'Before [[Target Page]] after text',
      )
    } finally {
      view.destroy()
      parent.remove()
    }
  })

  it('does not replace an adjacent wikilink later on the same line', () => {
    useTreeStore.setState({
      flatPages: [item('Target Page', 'docs/target-page')],
    })

    const doc = 'Before [[Target and [[Existing]]'
    const cursorPos = 'Before [[Target'.length
    const state = EditorState.create({ doc })
    const context = new CompletionContext(state, cursorPos, true)
    const result = wikiLinkCompletionSource(context)

    expect(result).not.toBeNull()
    const completionResult = result!
    expect(completionResult.from).toBe('Before [['.length)
    expect(completionResult.to).toBe(cursorPos)

    const parent = document.createElement('div')
    document.body.appendChild(parent)
    const view = new EditorView({
      state: EditorState.create({
        doc,
        selection: { anchor: cursorPos },
      }),
      parent,
    })

    try {
      const apply = expectCompletionApply(completionResult.options[0])
      apply(
        view,
        completionResult.options[0]!,
        completionResult.from,
        completionResult.to!,
      )
      expect(view.state.doc.toString()).toBe(
        'Before [[Target Page]] and [[Existing]]',
      )
    } finally {
      view.destroy()
      parent.remove()
    }
  })

  it('does not replace an adjacent markdown link later on the same line', () => {
    useTreeStore.setState({
      flatPages: [item('Target Page', 'docs/target-page')],
    })

    const doc = 'Before [[Target and [Existing](/docs/existing)'
    const cursorPos = 'Before [[Target'.length
    const state = EditorState.create({ doc })
    const context = new CompletionContext(state, cursorPos, true)
    const result = wikiLinkCompletionSource(context)

    expect(result).not.toBeNull()
    const completionResult = result!
    expect(completionResult.from).toBe('Before [['.length)
    expect(completionResult.to).toBe(cursorPos)

    const parent = document.createElement('div')
    document.body.appendChild(parent)
    const view = new EditorView({
      state: EditorState.create({
        doc,
        selection: { anchor: cursorPos },
      }),
      parent,
    })

    try {
      const apply = expectCompletionApply(completionResult.options[0])
      apply(
        view,
        completionResult.options[0]!,
        completionResult.from,
        completionResult.to!,
      )
      expect(view.state.doc.toString()).toBe(
        'Before [[Target Page]] and [Existing](/docs/existing)',
      )
    } finally {
      view.destroy()
      parent.remove()
    }
  })

  it('does not consume until a later closing wikilink on the same line', () => {
    useTreeStore.setState({
      flatPages: [item('Target Page', 'docs/target-page')],
    })

    const doc = '[[Hello]] asdf [[Target more text [[Hello]]'
    const cursorPos = '[[Hello]] asdf [[Target'.length
    const state = EditorState.create({ doc })
    const context = new CompletionContext(state, cursorPos, true)
    const result = wikiLinkCompletionSource(context)

    expect(result).not.toBeNull()
    const completionResult = result!
    expect(completionResult.from).toBe('[[Hello]] asdf [['.length)
    expect(completionResult.to).toBe(cursorPos)

    const parent = document.createElement('div')
    document.body.appendChild(parent)
    const view = new EditorView({
      state: EditorState.create({
        doc,
        selection: { anchor: cursorPos },
      }),
      parent,
    })

    try {
      const apply = expectCompletionApply(completionResult.options[0])
      apply(
        view,
        completionResult.options[0]!,
        completionResult.from,
        completionResult.to!,
      )
      expect(view.state.doc.toString()).toBe(
        '[[Hello]] asdf [[Target Page]] more text [[Hello]]',
      )
    } finally {
      view.destroy()
      parent.remove()
    }
  })

  it('applies completion without replacing later text in the editor', async () => {
    useTreeStore.setState({
      flatPages: [item('Target Page', 'docs/target-page')],
    })

    const parent = document.createElement('div')
    document.body.appendChild(parent)

    const doc = '[[Hello]] asdf [[Target more text [[Hello]]'
    const cursorPos = '[[Hello]] asdf [[Target'.length

    const view = new EditorView({
      state: EditorState.create({
        doc,
        selection: { anchor: cursorPos },
        extensions: [
          autocompletion({
            override: [wikiLinkCompletionSource],
          }),
        ],
      }),
      parent,
    })

    try {
      expect(startCompletion(view)).toBe(true)

      await vi.waitFor(() => {
        expect(completionStatus(view.state)).toBe('active')
        expect(currentCompletions(view.state).length).toBeGreaterThan(0)
      })

      const completion = currentCompletions(view.state)[0]
      const apply = expectCompletionApply(completion)
      apply(view, completion!, cursorPos - 'Target'.length, cursorPos)
      expect(view.state.doc.toString()).toBe(
        '[[Hello]] asdf [[Target Page]] more text [[Hello]]',
      )
    } finally {
      view.destroy()
      parent.remove()
    }
  })
})

describe('buildMarkdownLinkOptions', () => {
  it('maps items to completion options with path-based apply', () => {
    const options = buildMarkdownLinkOptions([
      item('Intro', 'docs/intro', 'Docs / Intro'),
    ])
    expect(options).toHaveLength(1)
    expect(options[0].label).toBe('Intro')
    expect(options[0].displayLabel).toBe('Intro')
    expect(options[0].apply).toBe('/docs/intro')
    expect(options[0].info).toBe('Docs / Intro')
    expect(options[0].path).toBe('docs/intro')
  })

  it('returns one option per item', () => {
    const options = buildMarkdownLinkOptions([
      item('Page A', 'a'),
      item('Page B', 'b'),
      item('Page C', 'c'),
    ])
    expect(options).toHaveLength(3)
    expect(options.map((o) => o.apply)).toEqual(['/a', '/b', '/c'])
  })
})

describe('buildWikiLinkOptions', () => {
  it('maps items to completion options with title-based apply handler', () => {
    const options = buildWikiLinkOptions([
      item('Intro', 'docs/intro', 'Docs / Intro'),
    ])
    expect(options).toHaveLength(1)
    expect(options[0].label).toBe('Intro')
    expect(options[0].displayLabel).toBe('Intro')
    expect(options[0].apply).toEqual(expect.any(Function))
    expect(options[0].info).toBe('Docs / Intro')
    expect(options[0].path).toBe('docs/intro')
  })

  it('uses the page title (not path) when applied', () => {
    const options = buildWikiLinkOptions([item('My Page', 'folder/my-page')])
    const parent = document.createElement('div')
    document.body.appendChild(parent)

    const view = new EditorView({
      state: EditorState.create({
        doc: '[[My',
        selection: { anchor: '[[My'.length },
      }),
      parent,
    })

    try {
      const apply = expectCompletionApply(options[0])
      apply(view, options[0], 2, '[[My'.length)
      expect(view.state.doc.toString()).toBe('[[My Page]]')
    } finally {
      view.destroy()
      parent.remove()
    }
  })

  it('returns one option per item', () => {
    const options = buildWikiLinkOptions([
      item('Page A', 'a'),
      item('Page B', 'b'),
    ])
    expect(options).toHaveLength(2)
    expect(options.map((o) => typeof o.apply)).toEqual(['function', 'function'])
  })
})
