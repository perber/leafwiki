import { renderHook, waitFor } from '@testing-library/react'
import { afterEach, beforeAll, describe, expect, it, vi } from 'vitest'
import { useMermaidInjector } from './useMermaidInjector'

vi.mock('@/lib/i18n', () => ({
  default: { t: (key: string) => key },
}))

beforeAll(() => {
  // jsdom has no layout engine; mermaid measures labels with these.
  const proto = SVGElement.prototype as unknown as Record<string, unknown>
  proto.getBBox = () => ({ x: 0, y: 0, width: 100, height: 20 })
  proto.getComputedTextLength = () => 100
})

afterEach(() => {
  document.body.innerHTML = ''
})

async function renderDiagram(code: string): Promise<SVGSVGElement> {
  const sandbox = document.createElement('div')
  sandbox.id = 'mermaid-renderer'
  const container = document.createElement('div')
  document.body.append(sandbox, container)
  const onError = vi.fn()

  renderHook(() =>
    useMermaidInjector({
      containerRef: { current: container },
      code,
      theme: 'default',
      onError,
    }),
  )

  await waitFor(() => expect(container.querySelector('svg')).not.toBeNull(), {
    timeout: 10000,
  })
  expect(onError.mock.calls.every(([error]) => error === null)).toBe(true)
  return container.querySelector('svg') as SVGSVGElement
}

describe('useMermaidInjector label HTML', () => {
  it('drops form controls from HTML labels and keeps their text', async () => {
    const svg = await renderDiagram(`flowchart TD
  A["<form action='https://elsewhere.example/' method='post'>Name <input name='u'> <textarea>t</textarea> <select><option>o</option></select> <button>Send</button></form>"]`)

    expect(
      svg.querySelector('form, input, textarea, select, option, button'),
    ).toBeNull()
    expect(svg.textContent).toContain('Name')
  }, 15000)

  it('drops inline style and class from HTML labels', async () => {
    const svg = await renderDiagram(`flowchart TD
  A["<div style='position:fixed;inset:0;z-index:99999'>covered</div><span class='fixed inset-0 z-50'>wide</span>"]`)

    const label = svg.querySelector('foreignObject')
    expect(label).not.toBeNull()
    expect(label?.innerHTML).not.toMatch(/position:\s*fixed|z-index/)
    expect(label?.querySelector('.fixed, .inset-0, .z-50')).toBeNull()
    expect(svg.textContent).toContain('covered')
    expect(svg.textContent).toContain('wide')
  }, 15000)

  it('drops style elements and iframes from HTML labels', async () => {
    const svg = await renderDiagram(`flowchart TD
  A["<style>body{display:none}</style><iframe src='https://elsewhere.example/'></iframe>text"]`)

    const label = svg.querySelector('foreignObject')
    expect(label?.querySelector('style, iframe')).toBeNull()
    expect(svg.textContent).toContain('text')
  }, 15000)

  it('keeps text formatting, line breaks and links in HTML labels', async () => {
    const svg = await renderDiagram(`flowchart TD
  A["<b>bold</b> <i>italic</i><br>next <a href='https://example.com/'>link</a>"] --> B`)

    const label = svg.querySelector('foreignObject')
    expect(label?.querySelector('b')?.textContent).toBe('bold')
    expect(label?.querySelector('i')?.textContent).toBe('italic')
    expect(label?.querySelector('br')).not.toBeNull()
    expect(label?.querySelector('a')?.getAttribute('href')).toBe(
      'https://example.com/',
    )
  }, 15000)

  it('keeps the styles mermaid generates for nodes and the theme', async () => {
    const svg = await renderDiagram(`flowchart TD
  A[one] --> B[two]
  classDef hot fill:#f00
  class A hot
  style B fill:#0f0`)

    expect(svg.querySelector('style')).not.toBeNull()
    expect(svg.innerHTML).toContain('fill:#f00')
    expect(svg.innerHTML).toContain('fill:#0f0')
    expect(svg.querySelector('.nodeLabel')).not.toBeNull()
  }, 15000)

  it('drops dialogs and popovers from HTML labels', async () => {
    const svg = await renderDiagram(`flowchart TD
  A["<dialog open>dlg</dialog><div popover id='p'>pop</div><a href='https://example.com/' popovertarget='p' commandfor='p' command='show-popover'>link</a>"]`)

    const label = svg.querySelector('foreignObject')
    expect(label?.querySelector('dialog')).toBeNull()
    expect(
      label?.querySelector(
        '[popover], [popovertarget], [commandfor], [command]',
      ),
    ).toBeNull()
    expect(svg.textContent).toContain('pop')
  }, 15000)

  // Rows marked "guard" already pass without labelSanitizeConfig (sequence
  // text is SVG text; DOMPurify drops nested foreignObject content). They pin
  // that those paths stay safe.
  const markup =
    "<input name='u'><button>b</button><span class='fixed inset-0 z-50' style='position:fixed;z-index:99999'>txt</span>"
  // Sequence and state diagrams end a statement at ";", so these cases use
  // one declaration only.
  const lineMarkup =
    "<input name='u'><button>b</button><span class='fixed inset-0 z-50' style='position:fixed'>txt</span>"

  function expectNoAuthorControlsOrStyles(svg: SVGSVGElement) {
    expect(
      svg.querySelector('form, input, textarea, select, option, button'),
    ).toBeNull()
    expect(svg.querySelector('.fixed, .inset-0, .z-50')).toBeNull()
    const styles = Array.from(svg.querySelectorAll('[style]'))
      .map((el) => el.getAttribute('style'))
      .join(';')
    expect(styles).not.toMatch(/position:\s*fixed|z-index:\s*99999/)
  }

  it.each([
    ['an edge label', `flowchart TD\n  A -->|"${markup}"| B`],
    ['a markdown string', `flowchart TD\n  A["\`**bold** ${markup}\`"]`],
    [
      'upper-case tags and attributes',
      `flowchart TD\n  A["<INPUT NAME='u'><SPAN CLASS='fixed inset-0 z-50' STYLE='position:fixed;z-index:99999'>txt</SPAN>"]`,
    ],
    [
      'controls nested in svg markup (guard)',
      `flowchart TD\n  A["<svg><foreignObject><input name='u'><button>b</button></foreignObject></svg>txt"]`,
    ],
    [
      'controls inside details',
      `flowchart TD\n  A["<details open><summary>s</summary>${markup}</details>"]`,
    ],
    [
      'a sequence note (guard)',
      `sequenceDiagram\n  A->>B: hi\n  Note over A: ${lineMarkup}`,
    ],
    ['a sequence message (guard)', `sequenceDiagram\n  A->>B: ${lineMarkup}`],
    ['a class label', `classDiagram\n  class A["${markup}"]`],
    ['a state description', `stateDiagram-v2\n  s1: ${lineMarkup}`],
    [
      'an init directive that relaxes the label config',
      `%%{init: {"dompurifyConfig": {"FORBID_TAGS": [], "FORBID_ATTR": []}}}%%\nflowchart TD\n  A["${markup}"]`,
    ],
    [
      'front matter that relaxes the label config',
      `---\nconfig:\n  dompurifyConfig:\n    FORBID_TAGS: []\n    FORBID_ATTR: []\n---\nflowchart TD\n  A["${markup}"]`,
    ],
    [
      'an init directive that turns html labels off and on',
      `%%{init: {"flowchart": {"htmlLabels": true}, "htmlLabels": true}}%%\nflowchart TD\n  A["${markup}"]`,
    ],
  ])(
    'drops form controls, style and class from %s',
    async (_name, code) => {
      const svg = await renderDiagram(code)

      expectNoAuthorControlsOrStyles(svg)
    },
    15000,
  )

  it('still sanitizes labels after a diagram with a config directive', async () => {
    await renderDiagram(
      `%%{init: {"dompurifyConfig": {"FORBID_TAGS": [], "FORBID_ATTR": []}}}%%\nflowchart TD\n  A[one]`,
    )
    document.body.innerHTML = ''

    const svg = await renderDiagram(`flowchart TD\n  A["${markup}"]`)

    expectNoAuthorControlsOrStyles(svg)
  }, 15000)

  it('keeps links that open in a new tab, with rel=noopener', async () => {
    const svg = await renderDiagram(`flowchart TD
  A["<a href='https://example.com/' target='_blank'>link</a>"]`)

    const link = svg.querySelector('foreignObject a')
    expect(link?.getAttribute('target')).toBe('_blank')
    expect(link?.getAttribute('rel')).toBe('noopener')
  }, 15000)

  it('keeps markdown string formatting', async () => {
    const svg = await renderDiagram('flowchart TD\n  A["`**bold** _it_`"]')

    expect(svg.querySelector('strong')?.textContent).toBe('bold')
    expect(svg.querySelector('em')?.textContent).toBe('it')
  }, 15000)
})
