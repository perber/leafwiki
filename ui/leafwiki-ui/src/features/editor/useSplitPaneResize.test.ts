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
