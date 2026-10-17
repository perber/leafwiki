import { act, renderHook } from '@testing-library/react'
import type { MouseEvent as ReactMouseEvent } from 'react'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { useSplitPaneResize } from './useSplitPaneResize'

const STORAGE_KEY = 'leafwiki-editor-pane-width'

function mouseDown(clientX: number, clientY: number) {
  return {
    clientX,
    clientY,
    preventDefault: vi.fn(),
    stopPropagation: vi.fn(),
  } as unknown as ReactMouseEvent<HTMLDivElement>
}

afterEach(() => {
  window.localStorage.clear()
})

describe('useSplitPaneResize', () => {
  it('starts at 50% without a stored width and clamps a stored one', () => {
    const { result: fresh } = renderHook(() =>
      useSplitPaneResize({
        isMobile: false,
        showPreview: true,
        previewStacked: false,
      }),
    )
    expect(fresh.current.editorPaneWidth).toBe(50)

    window.localStorage.setItem(STORAGE_KEY, '90')
    const { result: stored } = renderHook(() =>
      useSplitPaneResize({
        isMobile: false,
        showPreview: true,
        previewStacked: false,
      }),
    )
    expect(stored.current.editorPaneWidth).toBe(75)
  })

  it('resizes horizontally by drag distance, clamps and persists the width', () => {
    const { result } = renderHook(() =>
      useSplitPaneResize({
        isMobile: false,
        showPreview: true,
        previewStacked: false,
      }),
    )
    // No split element is attached, so the window width is the reference.
    const width = window.innerWidth

    act(() => result.current.handleSplitResize(mouseDown(100, 0)))
    expect(result.current.isResizingSplit).toBe(true)

    act(() => {
      document.dispatchEvent(
        new MouseEvent('mousemove', { clientX: 100 + width * 0.1 }),
      )
    })
    expect(result.current.editorPaneWidth).toBeCloseTo(60)

    act(() => {
      document.dispatchEvent(
        new MouseEvent('mousemove', { clientX: 100 + width }),
      )
    })
    expect(result.current.editorPaneWidth).toBe(75)

    act(() => {
      document.dispatchEvent(new MouseEvent('mouseup'))
    })
    expect(result.current.isResizingSplit).toBe(false)
    expect(window.localStorage.getItem(STORAGE_KEY)).toBe('75')

    // Listeners are gone after mouseup.
    act(() => {
      document.dispatchEvent(new MouseEvent('mousemove', { clientX: 0 }))
    })
    expect(result.current.editorPaneWidth).toBe(75)
  })

  it('uses the vertical axis when the preview is stacked', () => {
    const { result } = renderHook(() =>
      useSplitPaneResize({
        isMobile: false,
        showPreview: true,
        previewStacked: true,
      }),
    )
    const height = window.innerHeight

    act(() => result.current.handleSplitResize(mouseDown(0, 50)))
    act(() => {
      document.dispatchEvent(
        new MouseEvent('mousemove', {
          clientX: 9999,
          clientY: 50 - height * 0.1,
        }),
      )
    })
    expect(result.current.editorPaneWidth).toBeCloseTo(40)
  })

  it('ignores drags on mobile or with the preview hidden', () => {
    for (const options of [
      { isMobile: true, showPreview: true, previewStacked: false },
      { isMobile: false, showPreview: false, previewStacked: false },
    ]) {
      const { result } = renderHook(() => useSplitPaneResize(options))
      const event = mouseDown(0, 0)
      act(() => result.current.handleSplitResize(event))
      expect(result.current.isResizingSplit).toBe(false)
      expect(event.preventDefault).not.toHaveBeenCalled()
    }
  })
})

describe('useSplitPaneResize edge cases', () => {
  const opts = { isMobile: false, showPreview: true, previewStacked: false }

  it.each([
    ['NaN text', 'abc', 50],
    ['empty string', '', 50],
    ['below min', '3', 25],
    ['negative', '-40', 25],
    ['above max', '1000', 75],
    ['Infinity', 'Infinity', 75],
    ['valid float', '37.5', 37.5],
    ['numeric prefix', '60px', 60],
  ])('stored width %s -> %s', (_name, stored, expected) => {
    window.localStorage.setItem(STORAGE_KEY, stored)
    const { result } = renderHook(() => useSplitPaneResize(opts))
    expect(result.current.editorPaneWidth).toBe(expected)
  })

  it('persists the initial width on mount', () => {
    renderHook(() => useSplitPaneResize(opts))
    expect(window.localStorage.getItem(STORAGE_KEY)).toBe('50')
  })

  it('removes document listeners when unmounted during an active drag', () => {
    const add = vi.spyOn(document, 'addEventListener')
    const remove = vi.spyOn(document, 'removeEventListener')
    const { result, unmount } = renderHook(() => useSplitPaneResize(opts))
    act(() => result.current.handleSplitResize(mouseDown(0, 0)))
    const moveAdds = add.mock.calls.filter((c) => c[0] === 'mousemove')
    const upAdds = add.mock.calls.filter((c) => c[0] === 'mouseup')
    expect(moveAdds).toHaveLength(1)
    expect(upAdds).toHaveLength(1)

    unmount()
    expect(
      remove.mock.calls.filter(
        (c) => c[0] === 'mousemove' && c[1] === moveAdds[0][1],
      ).length,
    ).toBeGreaterThan(0)
    expect(
      remove.mock.calls.filter(
        (c) => c[0] === 'mouseup' && c[1] === upAdds[0][1],
      ).length,
    ).toBeGreaterThan(0)
    add.mockRestore()
    remove.mockRestore()
  })

  it('a second drag after mouseup works and does not stack listeners', () => {
    const add = vi.spyOn(document, 'addEventListener')
    const { result } = renderHook(() => useSplitPaneResize(opts))
    for (let i = 0; i < 2; i++) {
      act(() => result.current.handleSplitResize(mouseDown(0, 0)))
      act(() => {
        document.dispatchEvent(new MouseEvent('mouseup'))
      })
    }
    expect(add.mock.calls.filter((c) => c[0] === 'mousemove')).toHaveLength(2)
    expect(result.current.isResizingSplit).toBe(false)
    add.mockRestore()
  })

  it('uses the split element size when attached', () => {
    const { result } = renderHook(() => useSplitPaneResize(opts))
    const el = document.createElement('div')
    el.getBoundingClientRect = () => ({ width: 1000, height: 500 }) as DOMRect
    result.current.desktopSplitRef.current = el
    act(() => result.current.handleSplitResize(mouseDown(0, 0)))
    act(() => {
      document.dispatchEvent(new MouseEvent('mousemove', { clientX: 100 }))
    })
    expect(result.current.editorPaneWidth).toBeCloseTo(60)
  })
})
