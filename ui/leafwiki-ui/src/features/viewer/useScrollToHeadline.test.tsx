import { act, renderHook } from '@testing-library/react'
import type { ReactNode } from 'react'
import { MemoryRouter, useNavigate } from 'react-router'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { scrollToHeadlineHash } from '@/lib/scrollToHeadline'
import { useScrollToHeadline } from './useScrollToHeadline'

vi.mock('@/lib/scrollToHeadline', () => ({
  scrollToHeadlineHash: vi.fn(() => () => {}),
}))

const scrollMock = vi.mocked(scrollToHeadlineHash)

type Props = { content: string; isLoading: boolean }

// Mirrors PageViewer: the hook gets the page content and loading flag, while
// a Markdown link (here: navigate) changes the route.
function renderViewer(initialEntry: string, initialProps: Props) {
  const wrapper = ({ children }: { children: ReactNode }) => (
    <MemoryRouter initialEntries={[initialEntry]}>{children}</MemoryRouter>
  )
  return renderHook(
    (props: Props) => {
      useScrollToHeadline(props)
      return useNavigate()
    },
    { wrapper, initialProps },
  )
}

describe('useScrollToHeadline (#1801)', () => {
  beforeEach(() => {
    scrollMock.mockReset()
    scrollMock.mockImplementation(() => () => {})
  })

  it('scrolls to the heading once the other page has loaded after a cross-page link', () => {
    const cancels: ReturnType<typeof vi.fn>[] = []
    scrollMock.mockImplementation(() => {
      const cancel = vi.fn()
      cancels.push(cancel)
      return cancel
    })
    const { result, rerender } = renderViewer('/docs/current', {
      content: '# Current',
      isLoading: false,
    })
    expect(scrollMock).not.toHaveBeenCalled()

    act(() => {
      void result.current('/docs/page-1#heading2')
    })
    // The hash arrives one render before PageViewer starts loading, so a
    // scroll is scheduled against the old page; starting the load must
    // cancel it before it can scroll.
    rerender({ content: '# Current', isLoading: true })
    expect(cancels).toHaveLength(1)
    expect(cancels[0]).toHaveBeenCalledTimes(1)
    rerender({ content: '', isLoading: true })
    const callsWhileLoading = scrollMock.mock.calls.length

    rerender({ content: '# Page 1\n## Heading2', isLoading: false })
    expect(scrollMock).toHaveBeenCalledTimes(callsWhileLoading + 1)
    expect(scrollMock).toHaveBeenLastCalledWith('#heading2')
    expect(cancels.at(-1)).not.toHaveBeenCalled()
  })

  it('scrolls again when the same heading anchor is followed to another page', () => {
    const { result, rerender } = renderViewer('/docs/a#intro', {
      content: '# A\n## Intro',
      isLoading: false,
    })
    expect(scrollMock).toHaveBeenCalledTimes(1)

    act(() => {
      void result.current('/docs/b#intro')
    })
    rerender({ content: '# A\n## Intro', isLoading: true })
    rerender({ content: '# B\n## Intro', isLoading: false })

    expect(scrollMock).toHaveBeenCalledTimes(2)
    expect(scrollMock).toHaveBeenLastCalledWith('#intro')
  })

  it('scrolls on a same-page anchor without a reload', () => {
    const { result } = renderViewer('/docs/page-1', {
      content: '# Page 1\n## Heading2',
      isLoading: false,
    })

    act(() => {
      void result.current('/docs/page-1#heading2')
    })

    expect(scrollMock).toHaveBeenLastCalledWith('#heading2')
  })

  it('does not scroll after a link without a fragment', () => {
    const { result, rerender } = renderViewer('/docs/current', {
      content: '# Current',
      isLoading: false,
    })

    act(() => {
      void result.current('/docs/page-1?q=term')
    })
    rerender({ content: '# Page 1', isLoading: false })

    expect(scrollMock).not.toHaveBeenCalled()
  })
})
