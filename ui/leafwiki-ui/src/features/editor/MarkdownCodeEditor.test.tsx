import { act, render } from '@testing-library/react'
import { createRef } from 'react'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import type { EditorView as EditorViewType } from '@codemirror/view'

// Mock CodeMirror — we only care about counting EditorState.create calls,
// which is the proxy for "did CodeMirror reinitialize?"
const createSpy = vi.fn(() => ({ doc: '', extensions: [] }))

vi.mock('@codemirror/state', () => ({
  EditorState: { create: (args: unknown) => createSpy(args) },
  Compartment: class {
    of() {
      return {}
    }
    reconfigure() {
      return {}
    }
  },
}))

const destroySpy = vi.fn()
const dispatchSpy = vi.fn()
vi.mock('@codemirror/view', () => ({
  EditorView: Object.assign(
    class {
      dom = document.createElement('div')
      state = { doc: { toString: () => '' }, selection: { main: { head: 0 } } }
      destroy = destroySpy
      dispatch = dispatchSpy
      focus() {}
      constructor({ parent }: { parent: HTMLElement }) {
        parent?.appendChild(this.dom)
      }
    },
    {
      theme: () => ({}),
      lineWrapping: {},
      updateListener: { of: () => ({}) },
      domEventHandlers: () => ({}),
    },
  ),
  keymap: { of: () => ({}) },
}))

vi.mock('@codemirror/commands', () => ({
  defaultKeymap: [],
  history: () => ({}),
  historyKeymap: [],
  indentWithTab: {},
}))

vi.mock('@codemirror/lang-markdown', () => ({ markdown: () => ({}) }))
vi.mock('@codemirror/search', () => ({
  search: () => ({}),
  searchKeymap: [],
  openSearchPanel: vi.fn(),
}))
vi.mock('@codemirror/autocomplete', () => ({
  autocompletion: () => ({}),
  closeCompletion: vi.fn(),
  completionStatus: vi.fn(),
}))
vi.mock('@codemirror/theme-one-dark', () => ({ oneDark: {} }))
vi.mock('@fsegurai/codemirror-theme-github-light', () => ({ githubLight: {} }))

vi.mock('../designtoggle/designmode', () => ({
  useDesignModeStore: () => ({ mode: 'light' }),
}))

vi.mock('./internalLinkCompletion', () => ({
  internalLinkCompletionSource: {},
  wikiLinkCompletionSource: {},
}))

import MarkdownCodeEditor from './MarkdownCodeEditor'

describe('MarkdownCodeEditor – resetKey controls reinitialization', () => {
  beforeEach(() => {
    createSpy.mockClear()
    destroySpy.mockClear()
  })

  it('initializes CodeMirror once on mount', () => {
    const editorViewRef = createRef<EditorViewType | null>()
    render(
      <MarkdownCodeEditor
        initialValue="hello"
        resetKey="page-1"
        onChange={vi.fn()}
        editorViewRef={editorViewRef}
      />,
    )
    expect(createSpy).toHaveBeenCalledTimes(1)
    expect(createSpy.mock.calls[0]?.[0]).toMatchObject({ doc: 'hello' })
  })

  it('does NOT reinitialize when initialValue changes (user is typing)', () => {
    const editorViewRef = createRef<EditorViewType | null>()
    const { rerender } = render(
      <MarkdownCodeEditor
        initialValue="hello"
        resetKey="page-1"
        onChange={vi.fn()}
        editorViewRef={editorViewRef}
      />,
    )

    createSpy.mockClear()

    // Simulate typing by passing new initialValue with same resetKey
    act(() => {
      rerender(
        <MarkdownCodeEditor
          initialValue="hello world"
          resetKey="page-1"
          onChange={vi.fn()}
          editorViewRef={editorViewRef}
        />,
      )
    })

    act(() => {
      rerender(
        <MarkdownCodeEditor
          initialValue="hello world!"
          resetKey="page-1"
          onChange={vi.fn()}
          editorViewRef={editorViewRef}
        />,
      )
    })

    // CodeMirror must NOT have been recreated
    expect(createSpy).not.toHaveBeenCalled()
  })

  it('reinitializes with new content when resetKey changes (page navigation)', () => {
    const editorViewRef = createRef<EditorViewType | null>()
    const { rerender } = render(
      <MarkdownCodeEditor
        initialValue="page A content"
        resetKey="page-1"
        onChange={vi.fn()}
        editorViewRef={editorViewRef}
      />,
    )

    createSpy.mockClear()

    // Navigate to page B
    act(() => {
      rerender(
        <MarkdownCodeEditor
          initialValue="page B content"
          resetKey="page-2"
          onChange={vi.fn()}
          editorViewRef={editorViewRef}
        />,
      )
    })

    expect(createSpy).toHaveBeenCalledTimes(1)
    expect(createSpy.mock.calls[0][0]).toMatchObject({ doc: 'page B content' })
  })
})
