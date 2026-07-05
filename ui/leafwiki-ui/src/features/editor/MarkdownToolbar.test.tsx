import type { ReactNode, RefObject } from 'react'
import { fireEvent, render, screen } from '@testing-library/react'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import MarkdownToolbar from './MarkdownToolbar'
import type { MarkdownEditorRef } from './MarkdownEditor'

let mockIsMobile = false

vi.mock('@/components/TooltipWrapper', () => ({
  TooltipWrapper: ({ children }: { children: ReactNode }) => children,
}))

vi.mock('@/lib/useIsMobile', () => ({
  useIsMobile: () => mockIsMobile,
}))

vi.mock('react-i18next', async (importOriginal) => {
  const actual = await importOriginal<typeof import('react-i18next')>()
  return {
    ...actual,
    useTranslation: () => ({
      t: (key: string) => key,
    }),
  }
})

vi.mock('@/stores/dialogs', () => ({
  useDialogsStore: (
    selector: (state: { openDialog: ReturnType<typeof vi.fn> }) => unknown,
  ) => selector({ openDialog: vi.fn() }),
}))

vi.mock('@/stores/editor', () => ({
  useEditorStore: (
    selector: (state: {
      lineWrap: boolean
      toggleLineWrap: ReturnType<typeof vi.fn>
      autoSave: boolean
      toggleAutoSave: ReturnType<typeof vi.fn>
      autoSaveStatus: 'idle'
    }) => unknown,
  ) =>
    selector({
      lineWrap: true,
      toggleLineWrap: vi.fn(),
      autoSave: true,
      toggleAutoSave: vi.fn(),
      autoSaveStatus: 'idle',
    }),
}))

describe('MarkdownToolbar paste controls', () => {
  const pasteRichMock = vi.fn()
  const pastePlainMock = vi.fn()
  const editorRef: RefObject<MarkdownEditorRef> = {
    current: {
      canUndo: () => false,
      canRedo: () => false,
      getMarkdown: () => '',
      insertWrappedText: vi.fn(),
      insertHeading: vi.fn(),
      insertAtCursor: vi.fn(),
      replaceSelection: vi.fn(),
      pasteRich: pasteRichMock,
      pastePlain: pastePlainMock,
      focus: vi.fn(),
      undo: vi.fn(),
      redo: vi.fn(),
      editorViewRef: { current: null },
    },
  }

  beforeEach(() => {
    mockIsMobile = false
    pasteRichMock.mockClear()
    pastePlainMock.mockClear()
  })

  it('shows paste buttons directly on desktop', () => {
    render(
      <MarkdownToolbar
        editorRef={editorRef}
        pageId="page-1"
        previewVisible={false}
        previewStacked={false}
        onTogglePreview={vi.fn()}
        onTogglePreviewLayout={vi.fn()}
      />,
    )

    expect(screen.getByTestId('paste-rich-button')).toBeInTheDocument()
    expect(screen.getByTestId('paste-plain-button')).toBeInTheDocument()
  })

  it('keeps paste actions only inside the mobile dropdown', async () => {
    mockIsMobile = true

    render(
      <MarkdownToolbar
        editorRef={editorRef}
        pageId="page-1"
        previewVisible={false}
        previewStacked={false}
        onTogglePreview={vi.fn()}
        onTogglePreviewLayout={vi.fn()}
      />,
    )

    expect(screen.queryByTestId('paste-rich-button')).not.toBeInTheDocument()
    expect(screen.queryByTestId('paste-plain-button')).not.toBeInTheDocument()

    fireEvent.pointerDown(screen.getByLabelText('toolbar.moreOptionsAriaLabel'))

    expect(await screen.findByText('toolbar.pasteRich')).toBeInTheDocument()
    expect(await screen.findByText('toolbar.pastePlain')).toBeInTheDocument()
  })
})
