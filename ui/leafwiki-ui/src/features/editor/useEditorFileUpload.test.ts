import '@/lib/i18n'
import { act, renderHook } from '@testing-library/react'
import type { ClipboardEvent, DragEvent } from 'react'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { useEditorFileUpload } from './useEditorFileUpload'

const uploadAssetMock = vi.fn()
const toastError = vi.fn()
const toastSuccess = vi.fn()

vi.mock('@/lib/api/assets', () => ({
  uploadAsset: (...args: unknown[]) => uploadAssetMock(...args),
}))
vi.mock('sonner', () => ({
  toast: {
    error: (...a: unknown[]) => toastError(...a),
    success: (...a: unknown[]) => toastSuccess(...a),
  },
}))
vi.mock('@/stores/config', () => ({
  useConfigStore: (sel: (s: { maxAssetUploadSizeBytes: number }) => number) =>
    sel({ maxAssetUploadSizeBytes: 1024 }),
}))

function makeView(doc: string, cursor = 0, dropPos: number | null = 0) {
  const view = {
    state: {
      doc: { length: doc.length },
      selection: { main: { from: cursor } },
    },
    dispatch: vi.fn(),
    focus: vi.fn(),
    posAtCoords: vi.fn(() => dropPos),
  }
  return view
}

function file(name: string, type: string, size = 10) {
  const f = new File(['x'], name, { type })
  Object.defineProperty(f, 'size', { value: size })
  return f
}

function setup(view: ReturnType<typeof makeView> | null) {
  const onDocChange = vi.fn()
  const hook = renderHook(() =>
    useEditorFileUpload({
      editorViewRef: { current: view as never },
      pageId: 'p1',
      onDocChange,
    }),
  )
  return { ...hook, onDocChange }
}

const paste = (clipboardData: unknown) =>
  ({
    clipboardData,
    preventDefault: vi.fn(),
    stopPropagation: vi.fn(),
  }) as unknown as ClipboardEvent<HTMLDivElement> & {
    preventDefault: ReturnType<typeof vi.fn>
  }

const drop = (files: File[] | undefined) =>
  ({
    dataTransfer: files ? { files } : undefined,
    clientX: 1,
    clientY: 2,
    preventDefault: vi.fn(),
    stopPropagation: vi.fn(),
  }) as unknown as DragEvent<HTMLDivElement> & {
    preventDefault: ReturnType<typeof vi.fn>
  }

beforeEach(() => {
  vi.clearAllMocks()
  uploadAssetMock.mockImplementation(async (_id: string, f: File) => ({
    file: `/assets/p1/${f.name}`,
  }))
  vi.spyOn(console, 'error').mockImplementation(() => {})
})

describe('useEditorFileUpload', () => {
  it.each([
    ['image by mime', 'a.bin', 'image/png', '![a.bin](/assets/p1/a.bin)\n'],
    ['image by extension', 'a.PNG', '', '![a.PNG](/assets/p1/a.PNG)\n'],
    ['pdf', 'd.pdf', 'application/pdf', '![d.pdf](/assets/p1/d.pdf)\n'],
    ['other file', 'z.zip', 'application/zip', '[z.zip](/assets/p1/z.zip)\n'],
    ['no extension', 'README', '', '[README](/assets/p1/README)\n'],
    [
      'Unicode name',
      'Größe 日本 ü.jpeg',
      '',
      '![Größe 日本 ü.jpeg](/assets/p1/Größe 日本 ü.jpeg)\n',
    ],
  ])(
    'pastes %s as the right markdown at the cursor',
    async (_n, name, type, md) => {
      const view = makeView('abc', 2)
      const { result, onDocChange } = setup(view)
      await act(() =>
        result.current.handlePaste(paste({ files: [file(name, type)] })),
      )
      expect(view.dispatch).toHaveBeenCalledWith({
        changes: { from: 2, insert: md },
        selection: { anchor: 2 + md.length },
      })
      expect(onDocChange).toHaveBeenCalledWith(view)
      expect(view.focus).toHaveBeenCalled()
    },
  )

  it('paste without files leaves the event untouched', async () => {
    const view = makeView('abc')
    const { result } = setup(view)
    for (const data of [
      undefined,
      { files: [], items: [] },
      { files: [], items: [{ kind: 'string', getAsFile: () => null }] },
    ]) {
      const ev = paste(data)
      await act(() => result.current.handlePaste(ev))
      expect(ev.preventDefault).not.toHaveBeenCalled()
    }
    expect(uploadAssetMock).not.toHaveBeenCalled()
  })

  it('falls back to clipboard items and skips null files', async () => {
    const view = makeView('')
    const { result } = setup(view)
    const ev = paste({
      files: [],
      items: [
        { kind: 'file', getAsFile: () => file('i.png', 'image/png') },
        { kind: 'file', getAsFile: () => null },
      ],
    })
    await act(() => result.current.handlePaste(ev))
    expect(ev.preventDefault).toHaveBeenCalled()
    expect(uploadAssetMock).toHaveBeenCalledTimes(1)
  })

  it('rejects too-large files without uploading (limit is inclusive)', async () => {
    const view = makeView('')
    const { result } = setup(view)
    await act(() =>
      result.current.handlePaste(
        paste({ files: [file('big.png', 'image/png', 1025)] }),
      ),
    )
    expect(uploadAssetMock).not.toHaveBeenCalled()
    expect(toastError).toHaveBeenCalledWith(
      'File too large. Max 1.0 KiB allowed.',
    )
    expect(view.dispatch).not.toHaveBeenCalled()

    await act(() =>
      result.current.handlePaste(
        paste({ files: [file('ok.png', 'image/png', 1024)] }),
      ),
    )
    expect(uploadAssetMock).toHaveBeenCalledTimes(1)
  })

  it.each([
    ['the error message', new Error('disk full'), 'disk full'],
    ['the fallback', new Error(''), 'Failed to upload a.png'],
    ['the fallback for a non-Error', 'nope', 'Failed to upload a.png'],
  ])(
    'shows %s in a toast and does not touch the doc when the upload fails',
    async (_n, err, message) => {
      uploadAssetMock.mockRejectedValueOnce(err)
      const view = makeView('')
      const { result, onDocChange } = setup(view)
      await act(() =>
        result.current.handlePaste(
          paste({ files: [file('a.png', 'image/png')] }),
        ),
      )
      expect(toastError).toHaveBeenCalledWith(message)
      expect(toastSuccess).not.toHaveBeenCalled()
      expect(view.dispatch).not.toHaveBeenCalled()
      expect(onDocChange).not.toHaveBeenCalled()
    },
  )

  it('a failing file does not stop the next one in a multi-file paste', async () => {
    uploadAssetMock.mockRejectedValueOnce(new Error('boom'))
    const view = makeView('')
    const { result } = setup(view)
    await act(() =>
      result.current.handlePaste(
        paste({
          files: [file('a.png', 'image/png'), file('b.png', 'image/png')],
        }),
      ),
    )
    expect(view.dispatch).toHaveBeenCalledTimes(1)
    expect(view.dispatch.mock.calls[0][0].changes.insert).toContain('b.png')
    expect(toastSuccess).toHaveBeenCalledWith('Uploaded b.png')
  })

  it('does nothing after upload when the editor view is gone', async () => {
    const { result, onDocChange } = setup(null)
    await act(() =>
      result.current.handlePaste(
        paste({ files: [file('a.png', 'image/png')] }),
      ),
    )
    expect(uploadAssetMock).toHaveBeenCalled()
    expect(onDocChange).not.toHaveBeenCalled()
  })

  it('drop without files is ignored', async () => {
    const { result } = setup(makeView(''))
    for (const files of [undefined, []]) {
      const ev = drop(files)
      await act(() => result.current.handleDrop(ev))
      expect(ev.preventDefault).not.toHaveBeenCalled()
    }
  })

  it('drop with several files: first at the drop point, the rest at the cursor', async () => {
    const view = makeView('0123456789', 3, 7)
    // Simulate the first insert moving the cursor to its end.
    view.dispatch.mockImplementationOnce(
      (tr: { selection: { anchor: number } }) => {
        view.state.selection.main.from = tr.selection.anchor
      },
    )
    const { result } = setup(view)
    const ev = drop([
      file('a.png', 'image/png'),
      file('b.zip', 'application/zip'),
    ])
    await act(() => result.current.handleDrop(ev))
    expect(ev.preventDefault).toHaveBeenCalled()
    const first = view.dispatch.mock.calls[0][0]
    const second = view.dispatch.mock.calls[1][0]
    expect(first.changes.from).toBe(7)
    expect(second.changes.from).toBe(7 + first.changes.insert.length)
  })

  it('clamps the drop point to the doc length and falls back to the cursor when posAtCoords is null', async () => {
    const view = makeView('abc', 1, 99)
    const { result } = setup(view)
    await act(() =>
      result.current.handleDrop(drop([file('a.png', 'image/png')])),
    )
    expect(view.dispatch.mock.calls[0][0].changes.from).toBe(3)

    const view2 = makeView('abc', 1, null)
    const { result: r2 } = setup(view2)
    await act(() => r2.current.handleDrop(drop([file('a.png', 'image/png')])))
    expect(view2.dispatch.mock.calls[0][0].changes.from).toBe(1)
  })

  it('drop point 0 is honoured (not treated as missing)', async () => {
    const view = makeView('abc', 2, 0)
    const { result } = setup(view)
    await act(() =>
      result.current.handleDrop(drop([file('a.png', 'image/png')])),
    )
    expect(view.dispatch.mock.calls[0][0].changes.from).toBe(0)
  })
})
