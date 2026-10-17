import { act, renderHook } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { usePreviewScrollSync } from './usePreviewScrollSync'

type Props = {
  showPreview: boolean
  previewContent: string
  assetVersion: number
}

let rafCallbacks: Map<number, FrameRequestCallback>
let nextId: number

beforeEach(() => {
  rafCallbacks = new Map()
  nextId = 1
  vi.stubGlobal('requestAnimationFrame', (cb: FrameRequestCallback) => {
    const id = nextId++
    rafCallbacks.set(id, cb)
    return id
  })
  vi.stubGlobal('cancelAnimationFrame', (id: number) => {
    rafCallbacks.delete(id)
  })
})
afterEach(() => vi.unstubAllGlobals())

const flush = () => {
  const cbs = [...rafCallbacks.values()]
  rafCallbacks.clear()
  cbs.forEach((cb) => cb(0))
}

function makePreview(lines: number[]) {
  const preview = document.createElement('div')
  const content = document.createElement('div')
  preview.appendChild(content)
  lines.forEach((l) => {
    const el = document.createElement('p')
    el.setAttribute('data-line', String(l))
    content.appendChild(el)
  })
  Object.defineProperty(preview, 'clientHeight', { value: 100 })
  Object.defineProperty(preview, 'scrollHeight', { value: 1000 })
  preview.scrollTo = vi.fn() as never
  return preview
}

function setup(props: Props, lines = [1, 5], lineText = 'plain') {
  const view = { state: { doc: { line: () => ({ text: lineText }) } } }
  const hook = renderHook(
    (p: Props) =>
      usePreviewScrollSync({ editorViewRef: { current: view as never }, ...p }),
    { initialProps: props },
  )
  const preview = makePreview(lines)
  act(() => hook.result.current.setPreviewRef(preview))
  return { ...hook, preview }
}

const base: Props = { showPreview: true, previewContent: 'a', assetVersion: 0 }

describe('usePreviewScrollSync', () => {
  it('coalesces rapid cursor moves into one frame (last wins)', () => {
    const { result, preview } = setup(base)
    result.current.onCursorLineChange(1)
    result.current.onCursorLineChange(5)
    expect(rafCallbacks.size).toBe(1)
    const target = preview.querySelector("[data-line='5']") as HTMLElement
    target.getBoundingClientRect = () => ({ top: 600 }) as DOMRect
    preview.getBoundingClientRect = () => ({ top: 0 }) as DOMRect
    flush()
    expect(preview.scrollTo).toHaveBeenCalledTimes(1)
    expect(preview.scrollTo).toHaveBeenCalledWith({
      top: 550,
      behavior: 'smooth',
    })
  })

  it('falls back to the nearest earlier block for a line without data-line', () => {
    const { result, preview } = setup(base)
    const target = preview.querySelector("[data-line='1']") as HTMLElement
    target.getBoundingClientRect = () => ({ top: 400 }) as DOMRect
    preview.getBoundingClientRect = () => ({ top: 0 }) as DOMRect
    result.current.onCursorLineChange(3)
    flush()
    expect(preview.scrollTo).toHaveBeenCalledTimes(1)
  })

  it.each([
    // [name, target top, preview scrollTop, expected scrollTo calls]
    ['within the 16px threshold', 116, 66, 0],
    ['exactly at the threshold', 116, 50, 0],
    ['just past the threshold', 117, 50, 1],
  ])('scroll threshold: %s', (_n, targetTop, scrollTop, calls) => {
    const { result, preview } = setup(base)
    // desired = targetTop + scrollTop - 50 (half the 100px viewport)
    preview.scrollTop = scrollTop
    preview.getBoundingClientRect = () => ({ top: 0 }) as DOMRect
    const target = preview.querySelector("[data-line='5']") as HTMLElement
    target.getBoundingClientRect = () =>
      ({ top: targetTop - scrollTop }) as DOMRect
    result.current.onCursorLineChange(5)
    flush()
    expect(preview.scrollTo).toHaveBeenCalledTimes(calls)
  })

  it('does not scroll without any target block', () => {
    const { result, preview } = setup(base, [])
    result.current.onCursorLineChange(2)
    flush()
    expect(preview.scrollTo).not.toHaveBeenCalled()
  })

  it('clamps the scroll target to 0 and to the max scroll height', () => {
    const { result, preview } = setup(base)
    preview.getBoundingClientRect = () => ({ top: 0 }) as DOMRect
    const first = preview.querySelector("[data-line='1']") as HTMLElement
    const last = preview.querySelector("[data-line='5']") as HTMLElement
    // Rect tops are viewport-relative: block 1 sits 10px into the content,
    // seen while the preview is scrolled down by 400px.
    first.getBoundingClientRect = () => ({ top: 10 - 400 }) as DOMRect
    last.getBoundingClientRect = () => ({ top: 5000 }) as DOMRect
    preview.scrollTop = 400
    result.current.onCursorLineChange(1)
    flush()
    expect(preview.scrollTo).toHaveBeenLastCalledWith({
      top: 0,
      behavior: 'smooth',
    })
    preview.scrollTop = 0
    result.current.onCursorLineChange(5)
    flush()
    // scrollHeight 1000 - clientHeight 100
    expect(preview.scrollTo).toHaveBeenLastCalledWith({
      top: 900,
      behavior: 'smooth',
    })
  })

  it.each([
    ['a heading carrying the data-line', true, '## Anything'],
    ['a heading found by its slug id (Unicode)', false, '## Größe & Über'],
  ])('scrolls to %s', (_n, withDataLine, lineText) => {
    // Line 3 has no plain block; without heading lookup the fallback would
    // pick the block at line 1 (top 10 -> no scroll).
    const { result, preview } = setup(base, [1], lineText)
    preview.getBoundingClientRect = () => ({ top: 0 }) as DOMRect
    ;(
      preview.querySelector("[data-line='1']") as HTMLElement
    ).getBoundingClientRect = () => ({ top: 10 }) as DOMRect
    const heading = document.createElement('h2')
    if (withDataLine) heading.setAttribute('data-line', '3')
    else heading.id = 'grosse-uber'
    heading.getBoundingClientRect = () => ({ top: 500 }) as DOMRect
    preview.firstElementChild!.appendChild(heading)

    result.current.onCursorLineChange(3)
    flush()
    expect(preview.scrollTo).toHaveBeenCalledWith({
      top: 450,
      behavior: 'smooth',
    })
  })

  it.each([
    ['preview content', { previewContent: 'b' }],
    ['asset version', { assetVersion: 1 }],
  ])(
    're-aligns with behavior auto when the %s changes, only once a line is known and the preview is shown',
    (_n, change) => {
      const { result, rerender, preview } = setup(base)
      preview.getBoundingClientRect = () => ({ top: 0 }) as DOMRect
      ;(
        preview.querySelector("[data-line='5']") as HTMLElement
      ).getBoundingClientRect = () => ({ top: 600 }) as DOMRect

      // No cursor line yet: nothing scheduled.
      rerender({ ...base, ...change })
      expect(rafCallbacks.size).toBe(0)

      result.current.onCursorLineChange(5)
      flush()
      vi.mocked(preview.scrollTo).mockClear()
      preview.scrollTop = 0

      rerender(base)
      expect(rafCallbacks.size).toBe(1)
      flush()
      expect(preview.scrollTo).toHaveBeenCalledWith({
        top: 550,
        behavior: 'auto',
      })

      // Preview hidden: no re-align.
      rerender({ ...base, ...change, showPreview: false })
      expect(rafCallbacks.size).toBe(0)
    },
  )

  it('observes the preview content size and disconnects on change and unmount', () => {
    const observe = vi.fn()
    const disconnect = vi.fn()
    let callback: (() => void) | undefined
    vi.stubGlobal(
      'ResizeObserver',
      class {
        constructor(cb: () => void) {
          callback = cb
        }
        observe = observe
        disconnect = disconnect
      },
    )
    const { result, rerender, preview, unmount } = setup(base)
    // The ref is attached after the first effect run; a content change
    // re-runs the effect with the element in place.
    rerender({ ...base, previewContent: 'b' })
    expect(observe).toHaveBeenCalledWith(preview.firstElementChild)

    // Size change before any cursor line is known: no scroll scheduled.
    callback!()
    expect(rafCallbacks.size).toBe(0)
    result.current.onCursorLineChange(1)
    flush()
    callback!()
    expect(rafCallbacks.size).toBe(1)

    rerender({ ...base, previewContent: 'c' })
    expect(disconnect).toHaveBeenCalledTimes(1)
    unmount()
    expect(disconnect).toHaveBeenCalledTimes(2)
  })

  it('cancels a pending frame on unmount', () => {
    const { result, unmount } = setup(base)
    result.current.onCursorLineChange(1)
    expect(rafCallbacks.size).toBe(1)
    unmount()
    expect(rafCallbacks.size).toBe(0)
  })

  it('does not throw when no preview element is attached', () => {
    const view = { state: { doc: { line: () => ({ text: '' }) } } }
    const { result } = renderHook(() =>
      usePreviewScrollSync({
        editorViewRef: { current: view as never },
        ...base,
      }),
    )
    result.current.onCursorLineChange(1)
    expect(() => flush()).not.toThrow()
  })
})
