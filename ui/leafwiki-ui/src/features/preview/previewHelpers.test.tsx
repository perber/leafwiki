import {
  getSemanticShoutoutTitle,
  getShoutoutConfig,
  isPdfSrc,
  normalizeFootnoteHref,
  transformMarkdownUrl,
} from './previewHelpers'

describe('transformMarkdownUrl', () => {
  it.each([
    ['https://example.com/a?b=c#d', 'https://example.com/a?b=c#d'],
    ['/relative/path', '/relative/path'],
    ['#anchor', '#anchor'],
    ['mailto:a@example.com', 'mailto:a@example.com'],
    ['wikilink-notfound:Some%20Title', 'wikilink-notfound:Some%20Title'],
    ['wikilink-ambiguous:Title', 'wikilink-ambiguous:Title'],
    ['javascript:alert(1)', ''],
    ['JavaScript:alert(1)', ''],
    ['data:text/html;base64,PHNjcmlwdD4=', ''],
    ['vbscript:msgbox(1)', ''],
    // The wikilink pass-through matches the exact prefix only.
    ['wikilink-notfoundjavascript:alert(1)', ''],
    ['', ''],
  ])('%j -> %j', (input, expected) => {
    expect(transformMarkdownUrl(input)).toBe(expected)
  })
})

describe('normalizeFootnoteHref', () => {
  it.each([
    [undefined, undefined],
    ['', ''],
    ['#user-content-fn-1', '#user-content-fn-1'],
    ['#user-content-fnref-1', '#user-content-fnref-1'],
    ['#other', '#other'],
    [
      'https://example.com/#user-content-fn-1',
      'https://example.com/#user-content-fn-1',
    ],
  ])('%j -> %j', (input, expected) => {
    expect(normalizeFootnoteHref(input)).toBe(expected)
  })
})

describe('isPdfSrc', () => {
  it.each([
    [undefined, false],
    ['', false],
    ['/assets/manual.pdf', true],
    ['/assets/MANUAL.PDF', true],
    ['/assets/manual.pdf?download=1', true],
    ['/assets/manual.pdf#page=3', true],
    ['/assets/manual.pdf.png', false],
    ['/assets/pdf', false],
    ['/assets/manual.png?x=.pdf', false],
    ['/assets/übersicht äöü.pdf', true],
  ])('%j -> %j', (input, expected) => {
    expect(isPdfSrc(input)).toBe(expected)
  })
})

describe('getShoutoutConfig', () => {
  it.each([
    ['[!INFO]', { kind: 'info' }],
    ['  [!WARNING]  ', { kind: 'warning' }],
    ['[!CUSTOM_KIND-2]', { kind: 'custom_kind-2' }],
    ['[!info]', null],
    ['[!]', null],
    ['[!INFO] trailing', null],
    ['', null],
  ])('paragraph %j -> %j', (marker, expected) => {
    expect(getShoutoutConfig([<p key="p">{marker}</p>])).toEqual(expected)
  })

  it('reads the marker through nested elements and skips leading whitespace', () => {
    expect(
      getShoutoutConfig([
        '\n  ',
        <p key="p">
          <span>[!</span>
          <strong>NOTE</strong>]
        </p>,
      ]),
    ).toEqual({ kind: 'note' })
  })

  it('ignores markers that are not in a leading paragraph', () => {
    expect(getShoutoutConfig([<div key="d">[!INFO]</div>])).toBeNull()
    expect(getShoutoutConfig('[!INFO]')).toBeNull()
    expect(getShoutoutConfig(null)).toBeNull()
  })
})

describe('getSemanticShoutoutTitle', () => {
  it.each([
    ['info', 'Info'],
    ['success', 'Success'],
    ['warning', 'Warning'],
    ['error', 'Error'],
    ['note', null],
    ['', null],
  ])('%j -> %j', (kind, expected) => {
    expect(getSemanticShoutoutTitle(kind)).toBe(expected)
  })
})
