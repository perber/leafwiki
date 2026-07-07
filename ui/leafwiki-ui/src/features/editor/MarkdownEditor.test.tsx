import { act, fireEvent, render } from '@testing-library/react'
import { useEffect, useRef } from 'react'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import MarkdownEditor from './MarkdownEditor'

// Track each time MarkdownCodeEditor mounts and what initialValue it received.
// We use useEffect([]) so this only fires on mount, not on re-renders.
const mountSpy = vi.fn()
let capturedOnChange: ((val: string) => void) | null = null
let mockEditorState = {
  previewVisible: false,
  previewStacked: false,
}

vi.mock('./MarkdownCodeEditor', () => ({
  default: function MockMarkdownCodeEditor({
    initialValue,
    onChange,
  }: {
    initialValue: string
    resetKey: string
    onChange: (v: string) => void
    onCursorLineChange?: (line: number) => void
    editorViewRef: React.RefObject<unknown>
    lineWrap?: boolean
  }) {
    const initialValueAtMount = useRef(initialValue)
    useEffect(() => {
      mountSpy(initialValueAtMount.current)
    }, [])

    capturedOnChange = onChange
    return <div data-testid="code-editor" data-initial-value={initialValue} />
  },
}))

vi.mock('../preview/MarkdownPreview', () => ({
  default: () => <div data-testid="preview" />,
}))

vi.mock('./MarkdownToolbar', () => ({
  default: () => <div data-testid="toolbar" />,
}))

let mockIsMobile = false
vi.mock('@/lib/useIsMobile', () => ({
  useIsMobile: () => mockIsMobile,
}))

vi.mock('@/stores/config', () => ({
  useConfigStore: () => 5 * 1024 * 1024,
}))

vi.mock('@/stores/editor', () => ({
  useEditorStore: () => ({
    previewVisible: mockEditorState.previewVisible,
    togglePreview: vi.fn(),
    previewStacked: mockEditorState.previewStacked,
    togglePreviewLayout: vi.fn(),
    lineWrap: true,
  }),
}))

vi.mock('./pageEditorStore', () => ({
  usePageEditorStore: () => null,
}))

vi.mock('sonner', () => ({
  toast: { error: vi.fn(), success: vi.fn() },
}))

vi.mock('@/lib/api/assets', () => ({
  uploadAsset: vi.fn(),
}))

vi.mock('@/lib/api/errors', () => ({
  mapApiError: vi.fn(() => ({ message: 'error' })),
}))

vi.mock('@codemirror/commands', () => ({
  historyField: {},
  redo: vi.fn(() => false),
  undo: vi.fn(() => false),
}))

vi.mock('@codemirror/view', () => ({
  EditorView: class {},
}))

vi.mock('@/lib/config', () => ({
  formatBytes: (n: number) => `${n}B`,
  IMAGE_EXTENSIONS: ['png', 'jpg', 'gif', 'webp'],
}))

vi.mock('../preview/rehypeLineNumber', () => ({
  slugifyHeadline: (s: string) => s,
}))

describe('MarkdownEditor – breakpoint remount preserves content', () => {
  beforeEach(() => {
    mountSpy.mockClear()
    capturedOnChange = null
    window.localStorage.clear()
    mockIsMobile = false
    mockEditorState = {
      ...mockEditorState,
      previewVisible: false,
      previewStacked: false,
    }
  })

  it('passes edited content (not original initialValue) to MarkdownCodeEditor on remount', async () => {
    const onChange = vi.fn()
    const { rerender } = render(
      <MarkdownEditor
        initialValue="original content"
        pageId="page-1"
        onChange={onChange}
      />,
    )

    // Initial mount — editor receives original content
    expect(mountSpy).toHaveBeenCalledTimes(1)
    expect(mountSpy).toHaveBeenCalledWith('original content')

    // User types something
    act(() => {
      capturedOnChange?.('edited content')
    })

    // Switch to mobile — MarkdownCodeEditor remounts in the mobile branch
    mountSpy.mockClear()
    mockIsMobile = true
    rerender(
      <MarkdownEditor
        initialValue="original content"
        pageId="page-1"
        onChange={onChange}
      />,
    )

    // Remounted editor must receive the edited content, not the original
    expect(mountSpy).toHaveBeenCalledTimes(1)
    expect(mountSpy).toHaveBeenCalledWith('edited content')
  })

  it('does not remount MarkdownCodeEditor when the user types', async () => {
    const onChange = vi.fn()
    render(
      <MarkdownEditor
        initialValue="original content"
        pageId="page-1"
        onChange={onChange}
      />,
    )

    // Clear after initial mount
    mountSpy.mockClear()

    // Simulate several keystrokes
    act(() => {
      capturedOnChange?.('o')
    })
    act(() => {
      capturedOnChange?.('or')
    })
    act(() => {
      capturedOnChange?.('ori')
    })

    // MarkdownCodeEditor must not have remounted
    expect(mountSpy).not.toHaveBeenCalled()
  })

  it('renders stacked desktop layout classes when preview is stacked', () => {
    mockEditorState = {
      ...mockEditorState,
      previewVisible: true,
      previewStacked: true,
    }

    const { container } = render(
      <MarkdownEditor
        initialValue="original content"
        pageId="page-1"
        onChange={vi.fn()}
      />,
    )

    const layout = container.querySelector('.markdown-editor__stacked-layout')
    const divider = container.querySelector(
      '.markdown-editor__divider--stacked',
    )

    expect(layout).not.toBeNull()
    expect(divider).not.toBeNull()
  })

  it('resizes side-by-side desktop panes when dragging the divider', () => {
    mockEditorState = {
      ...mockEditorState,
      previewVisible: true,
      previewStacked: false,
    }

    const { getByTestId, container } = render(
      <MarkdownEditor
        initialValue="original content"
        pageId="page-1"
        onChange={vi.fn()}
      />,
    )

    const divider = getByTestId('editor-preview-resize-handle')
    const layout = divider.parentElement as HTMLDivElement
    layout.getBoundingClientRect = vi.fn(
      () =>
        ({
          width: 1000,
        }) as DOMRect,
    )

    fireEvent.mouseDown(divider, { clientX: 500 })
    fireEvent.mouseMove(document, { clientX: 600 })
    fireEvent.mouseUp(document)

    const editorPane = container.querySelector(
      '.markdown-editor__editor-pane--half',
    ) as HTMLDivElement

    expect(editorPane.style.flex).toBe('0 0 60%')
    expect(divider).toHaveAttribute('aria-valuenow', '60')
  })

  it('does not resize panes in stacked desktop preview mode', () => {
    mockEditorState = {
      ...mockEditorState,
      previewVisible: true,
      previewStacked: true,
    }

    const { getByTestId, container } = render(
      <MarkdownEditor
        initialValue="original content"
        pageId="page-1"
        onChange={vi.fn()}
      />,
    )

    const divider = getByTestId('editor-preview-resize-handle')

    fireEvent.mouseDown(divider, { clientX: 500 })
    fireEvent.mouseMove(document, { clientX: 600 })
    fireEvent.mouseUp(document)

    const editorPane = container.querySelector(
      '.markdown-editor__editor-pane--stacked',
    ) as HTMLDivElement

    expect(editorPane.style.flex).toBe('')
    expect(divider).not.toHaveAttribute('aria-valuenow')
  })
})
