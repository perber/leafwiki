import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { scrollToHeadlineHash } from './scrollToHeadline'

let replaceStateSpy: ReturnType<typeof vi.spyOn>

beforeEach(() => {
  replaceStateSpy = vi
    .spyOn(history, 'replaceState')
    .mockImplementation(() => {})
  // scrollToHeadlineHash exits early without a scroll-container
  const sc = document.createElement('div')
  sc.id = 'scroll-container'
  document.body.appendChild(sc)
})

afterEach(() => {
  document.body.innerHTML = ''
  vi.restoreAllMocks()
})

function addHeading(id: string): HTMLElement {
  const el = document.createElement('h2')
  el.id = id
  el.scrollIntoView = vi.fn()
  document.body.appendChild(el)
  return el
}

describe('scrollToHeadlineHash — hash update', () => {
  it('calls history.replaceState with the hash when heading is found', () => {
    addHeading('my-heading')
    scrollToHeadlineHash('#my-heading', { waitForStableLayout: false })
    expect(replaceStateSpy).toHaveBeenCalledWith(null, '', '#my-heading')
  })

  it('does not call replaceState when heading is not found', () => {
    scrollToHeadlineHash('#nonexistent', { waitForStableLayout: false })
    expect(replaceStateSpy).not.toHaveBeenCalled()
  })

  it('passes encoded hash to replaceState as-is', () => {
    addHeading('hello world')
    scrollToHeadlineHash('#hello%20world', { waitForStableLayout: false })
    expect(replaceStateSpy).toHaveBeenCalledWith(null, '', '#hello%20world')
  })
})
