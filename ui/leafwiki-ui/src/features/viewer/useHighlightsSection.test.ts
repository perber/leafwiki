import { renderHook } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { useHighlightsSection } from './useHighlightsSection'

const marker = (text: string) =>
  `<mark class="flexible-marker flexible-marker-default">${text}</mark>`

function renderContent(html: string) {
  const article = document.createElement('article')
  article.className = 'page-viewer__content'
  article.innerHTML = html
  document.body.appendChild(article)
  return article
}

afterEach(() => {
  document.body.innerHTML = ''
})

describe('useHighlightsSection', () => {
  it('maps each highlight of the rendered page to an anchor item', () => {
    renderContent(`<p>${marker('first')} and ${marker('second')}</p>`)

    const { result } = renderHook(() =>
      useHighlightsSection({ content: 'page', isLoading: false }),
    )

    expect(result.current.id).toBe('highlights')
    expect(result.current.titleKey).toBe('toc.highlights')
    expect(result.current.items).toEqual([
      expect.objectContaining({
        kind: 'anchor',
        key: 'highlight-0',
        label: 'first',
        testId: 'highlight-0',
      }),
      expect.objectContaining({
        kind: 'anchor',
        key: 'highlight-1',
        label: 'second',
        testId: 'highlight-1',
      }),
    ])
    // No scroll spy: highlights are never the active entry.
    for (const item of result.current.items) {
      if (item.kind !== 'anchor') throw new Error('expected an anchor item')
      expect(item.active).toBeUndefined()
    }
  })

  it('is empty without highlights, or while the page is loading', () => {
    renderContent(`<p>${marker('ignored while loading')}</p>`)
    const { result, rerender } = renderHook(
      ({ isLoading }) => useHighlightsSection({ content: 'page', isLoading }),
      { initialProps: { isLoading: true } },
    )
    expect(result.current.items).toEqual([])

    document.body.innerHTML = ''
    renderContent('<p>No highlights.</p>')
    rerender({ isLoading: false })
    expect(result.current.items).toEqual([])
  })

  it('recomputes when the content changes', () => {
    const article = renderContent(`<p>${marker('old')}</p>`)
    const { result, rerender } = renderHook(
      ({ content }) => useHighlightsSection({ content, isLoading: false }),
      { initialProps: { content: 'old page' } },
    )
    expect(result.current.items.map((item) => item.label)).toEqual(['old'])

    article.innerHTML = `<p>${marker('new')}</p>`
    rerender({ content: 'new page' })

    expect(result.current.items.map((item) => item.label)).toEqual(['new'])
  })

  it('scrolls the highlight at that index into the centre on select', () => {
    const article = renderContent(
      `<p>${marker('first')}${marker('second')}</p>`,
    )
    const { result } = renderHook(() =>
      useHighlightsSection({ content: 'page', isLoading: false }),
    )

    // Re-rendered markup: the click resolves the element at click time.
    article.innerHTML = `<p>${marker('first')}${marker('second')}</p>`
    const marks = article.querySelectorAll('mark')
    const scrolls = Array.from(marks).map((mark) => {
      const spy = vi.fn()
      mark.scrollIntoView = spy
      return spy
    })
    const hashBefore = window.location.hash

    const item = result.current.items[1]
    if (item.kind !== 'anchor') throw new Error('expected an anchor item')
    item.onSelect?.()

    expect(scrolls[0]).not.toHaveBeenCalled()
    expect(scrolls[1]).toHaveBeenCalledWith(
      expect.objectContaining({ block: 'center' }),
    )
    expect(window.location.hash).toBe(hashBefore)
  })

  it('opens every closed collapsible around the mark before scrolling', () => {
    const article = renderContent(
      `<details><summary>Outer</summary><details><summary>Inner</summary><p>${marker('hidden')}</p></details></details><details><summary>Other</summary></details>`,
    )
    // A collapsible around the whole content root stays as it is.
    const wrapper = document.createElement('details')
    article.replaceWith(wrapper)
    wrapper.appendChild(article)
    const mark = article.querySelector('mark')!
    const [outer, inner, other] = Array.from(
      article.querySelectorAll('details'),
    )
    let openWhenScrolled: boolean[] = []
    mark.scrollIntoView = vi.fn(() => {
      openWhenScrolled = [outer.open, inner.open]
    })
    const { result } = renderHook(() =>
      useHighlightsSection({ content: 'page', isLoading: false }),
    )

    const item = result.current.items[0]
    if (item.kind !== 'anchor') throw new Error('expected an anchor item')
    item.onSelect?.()

    expect(openWhenScrolled).toEqual([true, true])
    expect(mark.scrollIntoView).toHaveBeenCalledWith({
      behavior: 'smooth',
      block: 'center',
    })
    expect(other.open).toBe(false)
    expect(wrapper.open).toBe(false)
  })
})
