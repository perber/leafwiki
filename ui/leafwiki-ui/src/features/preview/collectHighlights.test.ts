import { describe, expect, it } from 'vitest'
import { collectHighlights } from './collectHighlights'

function root(html: string): HTMLElement {
  const el = document.createElement('div')
  el.innerHTML = html
  return el
}

const marker = (text: string, color = 'default') =>
  `<mark class="flexible-marker flexible-marker-${color}">${text}</mark>`

describe('collectHighlights', () => {
  it.each([
    ['an empty root', '', []],
    [
      'markers in document order, coloured ones included',
      `<p>${marker('first')}</p><ul><li>${marker('second', 'red')}</li></ul><p>${marker('third')}</p>`,
      ['first', 'second', 'third'],
    ],
    [
      'the text content of nested formatting',
      marker('<strong>bold</strong> and <em>italic</em>'),
      ['bold and italic'],
    ],
    [
      'no empty or whitespace-only markers',
      `${marker('')}${marker('  ')}<mark class="flexible-marker flexible-marker-empty"></mark>${marker('kept')}`,
      ['kept'],
    ],
    [
      'no search-query or raw HTML marks',
      `<mark class="search-query-highlight">query</mark><mark>raw</mark>${marker('kept')}`,
      ['kept'],
    ],
  ])('collects %s', (_name, html, expected) => {
    expect(collectHighlights(root(html)).map((h) => h.text)).toEqual(expected)
  })

  it('returns each label with its marker element', () => {
    const container = root(`<p>${marker('  padded  ')}</p>`)

    expect(collectHighlights(container)).toEqual([
      { text: 'padded', element: container.querySelector('mark') },
    ])
  })
})
