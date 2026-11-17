import { describe, expect, it } from 'vitest'
import { isSSRPreview } from './ssrPreview'

function root(html: string): HTMLElement {
  const el = document.createElement('div')
  el.append(document.createRange().createContextualFragment(html))
  return el
}

describe('isSSRPreview', () => {
  it.each([
    ['?ssr=', '<article><h1>Setup</h1></article>', true],
    ['?ssr=1', '<article><h1>Setup</h1></article>', true],
    ['?ssr', '<article><h1>Setup</h1></article>', true],
    ['?a=1&ssr=1', '<article><h1>Setup</h1></article>', true],
    ['', '<article><h1>Setup</h1></article>', false],
    ['?ssrx=1', '<article><h1>Setup</h1></article>', false],
    // No server body (private wiki, unknown page, SPA route): the SPA mounts,
    // so the parameter never leaves a blank page.
    ['?ssr=1', '', false],
    ['?ssr=1', '{{__BODY__}}', false],
  ])('search %j with #root %j → %s', (search, html, want) => {
    expect(isSSRPreview(search, root(html))).toBe(want)
  })
})
