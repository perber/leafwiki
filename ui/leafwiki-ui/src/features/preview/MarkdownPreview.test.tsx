import { act, render } from '@testing-library/react'
import { MemoryRouter } from 'react-router'
import type { PageNode } from '@/lib/api/pages'
import * as assetSrcModule from '@/lib/assetSrc'
import { TooltipProvider } from '@/components/ui/tooltip'
import { useDesignModeStore } from '@/features/designtoggle/designmode'
import { useTreeStore } from '@/stores/tree'
import { collectHighlights } from './collectHighlights'
import MarkdownPreview from './MarkdownPreview'

function renderPreview(content: string) {
  return render(
    <TooltipProvider>
      <MarkdownPreview content={content} />
    </TooltipProvider>,
  )
}

describe('MarkdownPreview syntax highlighting', () => {
  beforeEach(() => {
    localStorage.setItem('design-mode', 'light')
    useDesignModeStore.setState({ mode: 'light' })
    window.matchMedia = vi.fn().mockImplementation((query: string) => ({
      matches: query === '(prefers-color-scheme: light)',
      media: query,
      onchange: null,
      addEventListener: vi.fn(),
      removeEventListener: vi.fn(),
      addListener: vi.fn(),
      removeListener: vi.fn(),
      dispatchEvent: vi.fn(),
    }))
  })

  it('highlights bash, shell session, and powershell code fences', () => {
    const content = `\`\`\`bash
echo "$HOME"
\`\`\`

\`\`\`shell
$ echo "$HOME"
\`\`\`

\`\`\`powershell
$path = Join-Path $HOME 'Documents'
if (Test-Path $path) {
  Write-Host 'ok'
}
\`\`\``

    const { container } = renderPreview(content)

    const bashCodeBlock = container.querySelector('code.language-bash.hljs')
    expect(bashCodeBlock).not.toBeNull()
    expect(bashCodeBlock?.querySelector('.hljs-variable')).not.toBeNull()

    const shellCodeBlock = container.querySelector('code.language-shell.hljs')
    expect(shellCodeBlock).not.toBeNull()
    expect(shellCodeBlock?.querySelector('.hljs-meta')).not.toBeNull()
    expect(shellCodeBlock?.querySelector('.hljs-variable')).not.toBeNull()

    const powershellCodeBlock = container.querySelector(
      'code.language-powershell.hljs',
    )
    expect(powershellCodeBlock).not.toBeNull()
    expect(powershellCodeBlock?.querySelector('.hljs-keyword')).not.toBeNull()
    expect(powershellCodeBlock?.querySelector('.hljs-variable')).not.toBeNull()
  })

  it('highlights AutoHotkey code fences', () => {
    const content = `\`\`\`autohotkey
#Requires AutoHotkey v2.0
if WinExist("Untitled - Notepad") {
  WinActivate
}
\`\`\``

    const { container } = renderPreview(content)

    const autohotkeyCodeBlock = container.querySelector(
      'code.language-autohotkey.hljs',
    )
    expect(autohotkeyCodeBlock).not.toBeNull()
    expect(autohotkeyCodeBlock?.querySelector('.hljs-meta')).not.toBeNull()
    expect(autohotkeyCodeBlock?.querySelector('.hljs-string')).not.toBeNull()
  })

  it('shows line numbers when the fence language ends with =', () => {
    const content = `\`\`\`bash=
echo one
echo two
echo three
\`\`\``

    const { container } = renderPreview(content)

    const block = container.querySelector('.markdown-code-block--line-numbers')
    expect(block).not.toBeNull()

    const lineNumbers = container.querySelectorAll(
      '.markdown-code-block__line-number',
    )
    expect(lineNumbers).toHaveLength(3)
    expect(lineNumbers[0]?.textContent).toBe('1')
    expect(lineNumbers[2]?.textContent).toBe('3')

    const highlighted = container.querySelector('code.language-bash.hljs')
    expect(highlighted).not.toBeNull()
    expect(highlighted?.getAttribute('data-line-numbers')).toBe('true')
  })

  it('does not show line numbers for ordinary fences', () => {
    const content = `\`\`\`bash
echo one
echo two
\`\`\``

    const { container } = renderPreview(content)

    expect(
      container.querySelector('.markdown-code-block--line-numbers'),
    ).toBeNull()
    expect(
      container.querySelector('.markdown-code-block__line-number'),
    ).toBeNull()
  })

  it('renders external images from markdown image syntax', () => {
    const { container } = renderPreview(
      '![Remote diagram](https://example.com/diagram.png)',
    )

    const image = container.querySelector('img')
    expect(image).not.toBeNull()
    expect(image?.getAttribute('src')).toBe('https://example.com/diagram.png')
    expect(image?.getAttribute('alt')).toBe('Remote diagram')
  })

  it('renders external images from sanitized inline html', () => {
    const { container } = renderPreview(
      '<img src="https://example.com/banner.png" alt="Remote banner" />',
    )

    const image = container.querySelector('img')
    expect(image).not.toBeNull()
    expect(image?.getAttribute('src')).toBe('https://example.com/banner.png')
    expect(image?.getAttribute('alt')).toBe('Remote banner')
  })

  it('embeds pdfs referenced via markdown image syntax as an iframe', () => {
    const { container } = renderPreview('![Manual](/assets/manual.pdf)')

    expect(container.querySelector('img')).toBeNull()
    const frame = container.querySelector('iframe')
    expect(frame).not.toBeNull()
    expect(frame?.getAttribute('src')).toContain('/assets/manual.pdf')
    expect(frame?.getAttribute('title')).toBe('Manual')
  })

  it('keeps a #page= fragment so the embed opens on that page', () => {
    const { container } = renderPreview('![Manual](/assets/manual.pdf#page=3)')

    const frame = container.querySelector('iframe')
    expect(frame?.getAttribute('src')).toMatch(
      /\/assets\/manual\.pdf\?v=\d+#page=3$/,
    )
  })

  it('links to a pdf from another origin instead of embedding it', () => {
    const { container } = renderPreview(
      '![Manual](https://example.com/manual.pdf)',
    )

    expect(container.querySelector('iframe')).toBeNull()
    expect(
      container.querySelector('a[href="https://example.com/manual.pdf"]'),
    ).not.toBeNull()
  })

  it('normalizes audio/video src through the shared assetSrc helper (no local duplicate)', () => {
    const spy = vi.spyOn(assetSrcModule, 'normalizeAssetSrc')

    const { container } = renderPreview(
      '<audio controls src="/api/assets/page-1/note.mp3"></audio>',
    )

    expect(spy).toHaveBeenCalledWith('/api/assets/page-1/note.mp3')
    const audio = container.querySelector('audio')
    expect(audio?.getAttribute('src')).toBe('/api/assets/page-1/note.mp3')

    spy.mockRestore()
  })

  it('renders inline code with its copy action', () => {
    const { container } = renderPreview('Use `npm run build` here.')

    const inlineCode = container.querySelector('.markdown-inline-code')
    expect(inlineCode?.textContent).toContain('npm run build')
    expect(
      inlineCode?.querySelector(
        '[data-testid="markdown-inline-code-copy-button"]',
      ),
    ).not.toBeNull()
  })

  it('renders ==text== as a mark element', () => {
    const { container } = renderPreview('Some ==highlighted== text.')

    const mark = container.querySelector('mark')
    expect(mark).not.toBeNull()
    expect(mark?.textContent).toBe('highlighted')
  })

  it('supports nested formatting inside a highlighted span', () => {
    const { container } = renderPreview('==**bold highlight**==')

    const mark = container.querySelector('mark')
    expect(mark).not.toBeNull()
    expect(mark?.querySelector('strong')?.textContent).toBe('bold highlight')
  })

  it('does not convert == inside inline code or fenced code blocks', () => {
    const content = [
      'Use `==` as a diff marker.',
      '',
      '```',
      'a == b',
      '```',
    ].join('\n')

    const { container } = renderPreview(content)

    expect(container.querySelector('mark')).toBeNull()
    expect(container.querySelector('code')?.textContent).toContain('==')
  })

  it('exposes plain and coloured highlights to collectHighlights, not code or raw marks', () => {
    const content = [
      'A ==plain== and a =r=coloured== highlight, `==inline==` code.',
      '',
      '<mark>raw</mark>',
      '',
      '```',
      '==fenced==',
      '```',
    ].join('\n')

    const { container } = renderPreview(content)

    expect(collectHighlights(container).map((h) => h.text)).toEqual([
      'plain',
      'coloured',
    ])
  })

  it.each([
    ['a heading', '## Title with ==mark==', ['mark']],
    ['a table cell', '| a | b |\n|---|---|\n| ==cell== | x |', ['cell']],
    ['nested list items', '- ==one==\n  - ==two==', ['one', 'two']],
    ['a blockquote', '> quoted ==mark==', ['mark']],
    ['an alert', '> [!NOTE]\n> alert ==mark==', ['mark']],
    [
      'a closed collapsible',
      ':::collapsed Details\nhidden ==mark==\n:::',
      ['mark'],
    ],
    ['a footnote', 'Text[^1]\n\n[^1]: note ==mark==', ['mark']],
    [
      'a wikilink inside a highlight',
      '==see [[Missing Page|alias]]==',
      ['see alias'],
    ],
    [
      'a soft line break (multi-line)',
      '==line one\nline two==',
      ['line one line two'],
    ],
    [
      'Unicode and special characters',
      '==Ünïcödé 漢字 <&> "q"==',
      ['Ünïcödé 漢字 <&> "q"'],
    ],
    ['an empty marker', 'a ==== b', []],
  ])('collects highlights inside %s', (_name, content, expected) => {
    // Links (footnotes, wikilinks) need a router.
    const { container } = render(
      <MemoryRouter>
        <TooltipProvider>
          <MarkdownPreview content={content} />
        </TooltipProvider>
      </MemoryRouter>,
    )

    expect(collectHighlights(container).map((h) => h.text)).toEqual(expected)
  })

  it('resizes images using the width syntax', () => {
    const { container } = renderPreview(
      '![Resizable image](https://example.com/image.png){width=75%}',
    )

    const image = container.querySelector('img')

    expect(image).not.toBeNull()
    expect(image).toHaveStyle({
      width: '75%',
      height: 'auto',
    })
  })

  it('supports decimal image sizes', () => {
    const { container } = renderPreview(
      '![Resizable image](https://example.com/image.png){width=37.5%}',
    )

    const image = container.querySelector('img')

    expect(image).not.toBeNull()
    expect(image).toHaveStyle({
      width: '37.5%',
      height: 'auto',
    })
  })

  it('does not resize images without the width syntax', () => {
    const { container } = renderPreview(
      '![Normal image](https://example.com/image.png)',
    )

    const image = container.querySelector('img')

    expect(image).not.toBeNull()
    expect(image).not.toHaveStyle({ width: '75%' })
  })

  it('does not resize images with an invalid width', () => {
    const { container } = renderPreview(
      '![Image](https://example.com/image.png){width=101%}',
    )

    const image = container.querySelector('img')

    expect(image).not.toBeNull()
    expect(image).not.toHaveStyle({ width: '101%' })
  })

  it('does not leak the mdast node onto the rendered image element', () => {
    const { container } = renderPreview(
      '![Resizable image](https://example.com/image.png){width=75%}',
    )

    const image = container.querySelector('img')

    expect(image).not.toBeNull()
    expect(image?.hasAttribute('node')).toBe(false)
  })
})

describe('MarkdownPreview wikilinks with a slash in the title', () => {
  const node = (id: string, path: string, title: string): PageNode => ({
    id,
    title,
    slug: path.split('/').pop() ?? path,
    path,
    version: 'v1',
    kind: 'page',
    children: null,
    parentId: null,
  })

  beforeEach(() => {
    useTreeStore.setState({ byId: {}, byPath: {} })
    localStorage.setItem('design-mode', 'light')
    useDesignModeStore.setState({ mode: 'light' })
    window.matchMedia = vi.fn().mockImplementation((query: string) => ({
      matches: query === '(prefers-color-scheme: light)',
      media: query,
      onchange: null,
      addEventListener: vi.fn(),
      removeEventListener: vi.fn(),
      addListener: vi.fn(),
      removeListener: vi.fn(),
      dispatchEvent: vi.fn(),
    }))
  })

  // Regression for the ADR index bug: [[ADR-0011: SMTP as a CLI/ENV-Only
  // Optional Feature]] used to hit the path-hint branch (because of the "/" in
  // "CLI/ENV") and emit a bare markdown link whose destination contained
  // spaces — invalid CommonMark, so the preview showed the raw "[text](url)".
  it('renders a slash-and-space title as a resolved link, not raw text', () => {
    const adr = node(
      'adr-11',
      'ai-gen-infos/adr/adr-0011-smtp',
      'ADR-0011: SMTP as a CLI/ENV-Only Optional Feature',
    )
    useTreeStore.setState({
      byId: { 'adr-11': adr },
      byPath: { 'ai-gen-infos/adr/adr-0011-smtp': adr },
    })

    const { container } = render(
      <MemoryRouter>
        <TooltipProvider>
          <MarkdownPreview content="[[ADR-0011: SMTP as a CLI/ENV-Only Optional Feature]]" />
        </TooltipProvider>
      </MemoryRouter>,
    )

    const link = container.querySelector(
      'a[href="/ai-gen-infos/adr/adr-0011-smtp"]',
    )
    expect(link).not.toBeNull()
    expect(link?.textContent).toBe(
      'ADR-0011: SMTP as a CLI/ENV-Only Optional Feature',
    )
    expect(container.textContent).not.toContain('](')
  })
})

describe('MarkdownPreview title index refresh', () => {
  it('updates rendered wikilinks when the tree title changes', () => {
    const page: PageNode = {
      id: 'rename',
      title: 'Before',
      path: 'renamed-page',
      slug: 'renamed-page',
      version: 'v1',
      kind: 'page',
      children: null,
    }
    useTreeStore.setState({
      byId: { rename: page },
      byPath: { 'renamed-page': page },
    })
    const { container } = render(
      <MemoryRouter>
        <TooltipProvider>
          <MarkdownPreview content="[[Before]] [[After]]" />
        </TooltipProvider>
      </MemoryRouter>,
    )
    expect(
      container.querySelector('a[href="/renamed-page"]')?.textContent,
    ).toBe('Before')
    const renamed = { ...page, title: 'After' }
    act(() =>
      useTreeStore.setState({
        byId: { rename: renamed },
        byPath: { 'renamed-page': renamed },
      }),
    )
    expect(
      container.querySelector('a[href="/renamed-page"]')?.textContent,
    ).toBe('After')
  })
})

describe('MarkdownPreview sanitizes raw HTML', () => {
  beforeEach(() => {
    useTreeStore.setState({ byId: {}, byPath: {} })
    localStorage.setItem('design-mode', 'light')
    useDesignModeStore.setState({ mode: 'light' })
    window.matchMedia = vi.fn().mockImplementation((query: string) => ({
      matches: query === '(prefers-color-scheme: light)',
      media: query,
      onchange: null,
      addEventListener: vi.fn(),
      removeEventListener: vi.fn(),
      addListener: vi.fn(),
      removeListener: vi.fn(),
      dispatchEvent: vi.fn(),
    }))
  })

  function renderRouted(content: string) {
    return render(
      <MemoryRouter>
        <TooltipProvider>
          <MarkdownPreview content={content} />
        </TooltipProvider>
      </MemoryRouter>,
    )
  }

  function eventHandlerAttributes(root: Element) {
    return Array.from(root.querySelectorAll('*')).flatMap((el) =>
      Array.from(el.attributes)
        .map((attr) => attr.name)
        .filter((name) => name.toLowerCase().startsWith('on')),
    )
  }

  function urlAttributes(root: Element) {
    return Array.from(
      root.querySelectorAll('[href], [src], [action], [formaction]'),
    ).flatMap((el) =>
      ['href', 'src', 'action', 'formaction']
        .map((name) => el.getAttribute(name))
        .filter((value): value is string => value !== null),
    )
  }

  it.each([
    ['script tag', 'before <script>window.__xss = 1</script> after', 'script'],
    ['onerror attribute', '<img src="x" onerror="window.__xss = 1">', null],
    ['onclick on a div', '<div onclick="window.__xss = 1">click</div>', null],
    ['svg onload', '<svg onload="window.__xss = 1"><circle /></svg>', 'svg'],
    ['iframe', '<iframe src="https://evil.example"></iframe>', 'iframe'],
    ['object', '<object data="https://evil.example/x.swf"></object>', 'object'],
    ['embed', '<embed src="https://evil.example/x.swf">', 'embed'],
    ['form', '<form action="https://evil.example"><input></form>', 'form'],
    ['style tag', '<style>body { display: none }</style>', 'style'],
    [
      'meta refresh',
      '<meta http-equiv="refresh" content="0;url=https://evil.example">',
      'meta',
    ],
    ['base tag', '<base href="https://evil.example/">', 'base'],
    [
      'video onerror',
      '<video src="x.mp4" onerror="window.__xss = 1"></video>',
      null,
    ],
    [
      'audio onplay',
      '<audio controls src="x.mp3" onplay="window.__xss = 1"></audio>',
      null,
    ],
  ])('strips %s', (_name, content, forbiddenTag) => {
    const { container } = renderRouted(content)

    if (forbiddenTag) {
      expect(container.querySelector(forbiddenTag)).toBeNull()
    }
    expect(eventHandlerAttributes(container)).toEqual([])
    expect(container.innerHTML).not.toContain('window.__xss')
  })

  it.each([
    ['html javascript: href', '<a href="javascript:alert(1)">x</a>'],
    [
      'html JaVaScRiPt: href with entities',
      '<a href="jav&#x09;ascript:alert(1)">x</a>',
    ],
    ['markdown javascript: link', '[x](javascript:alert(1))'],
    ['markdown javascript: image', '![x](javascript:alert(1))'],
    [
      'html data: href',
      '<a href="data:text/html,<script>alert(1)</script>">x</a>',
    ],
    [
      'html data: img src',
      '<img src="data:image/svg+xml,<svg onload=alert(1)>">',
    ],
    ['vbscript: href', '<a href="vbscript:msgbox(1)">x</a>'],
    [
      'video javascript: src',
      '<video controls src="javascript:alert(1)"></video>',
    ],
    [
      'wikilink protocol prefix abuse',
      '<a href="wikilink-notfound:x" onclick="alert(1)">x</a>',
    ],
  ])('drops dangerous URLs: %s', (_name, content) => {
    const { container } = renderRouted(content)

    for (const value of urlAttributes(container)) {
      expect(
        Array.from(value)
          .filter((ch) => ch.charCodeAt(0) > 0x20)
          .join('')
          .toLowerCase(),
      ).not.toMatch(/^(javascript|vbscript|data):/)
    }
    expect(eventHandlerAttributes(container)).toEqual([])
  })

  it.each([
    'position: fixed; top: 0; left: 0; width: 100%; height: 100%',
    'position:fixed;z-index:9999',
    'background-image: url(javascript:alert(1))',
    'behavior: url(x.htc)',
  ])('removes non-whitelisted inline style %j', (style) => {
    const { container } = renderRouted(
      `<div style="${style}" data-testid="styled">overlay</div>`,
    )

    const div = Array.from(container.querySelectorAll('div')).find(
      (el) => el.textContent === 'overlay',
    )
    expect(div).toBeDefined()
    const remaining = div?.getAttribute('style') ?? ''
    expect(remaining).not.toMatch(/position|z-index|url\(|behavior/i)
  })

  function classTokens(root: Element) {
    return new Set(
      Array.from(root.querySelectorAll('[class]')).flatMap((el) =>
        Array.from(el.classList),
      ),
    )
  }

  it.each([
    ['div', '<div class="fixed inset-0 z-50 bg-background">x</div>'],
    ['span', '<span class="fixed inset-0 z-50 opacity-0">x</span>'],
    ['p', '<p class="absolute top-0 left-0">x</p>'],
    ['img', '<img class="fixed inset-0" src="https://x.example/a.png">'],
    ['mark', '<mark class="fixed inset-0">x</mark>'],
    [
      'KaTeX classes',
      '<span class="katex"><span class="brace-left">x</span></span>',
    ],
    ['upper-case attribute', '<div CLASS="fixed inset-0">x</div>'],
    ['mixed-case attribute', '<div ClAsS="fixed inset-0">x</div>'],
    ['tab and newline separators', '<div class="fixed\tinset-0\nz-50">x</div>'],
    ['className attribute', '<div className="fixed inset-0">x</div>'],
    ['unquoted attribute', '<div class=fixed>x</div>'],
    [
      'table cells',
      '<table class="fixed"><tr class="fixed"><td class="fixed inset-0">x</td></tr></table>',
    ],
    [
      'details',
      '<details class="fixed inset-0"><summary>s</summary>x</details>',
    ],
    ['input in a task list', '<input type="checkbox" class="fixed" disabled>'],
    ['math-inline span', '<span class="math-inline fixed">x</span>'],
  ])('drops class names that the app styles: %s', (_name, content) => {
    const { container } = renderRouted(content)

    const tokens = classTokens(container)
    for (const token of [
      'fixed',
      'absolute',
      'inset-0',
      'top-0',
      'left-0',
      'z-50',
      'opacity-0',
      'bg-background',
      'katex',
      'brace-left',
    ]) {
      expect(tokens).not.toContain(token)
    }
  })

  // The li, code, h2 and ul rows were already filtered by the default schema's
  // per-element rules; they guard that an element's own rule never falls back
  // to a free class.
  it.each([
    [
      'mark',
      '<mark class="flexible-marker fixed">x</mark>',
      'mark',
      'flexible-marker',
    ],
    [
      'details',
      '<details class="markdown-collapsible fixed"><summary>s</summary>x</details>',
      'details',
      'markdown-collapsible',
    ],
    [
      'li',
      '<ul><li class="fixed task-list-item">x</li></ul>',
      'li',
      'task-list-item',
    ],
    [
      'code',
      '<p><code class="language-x fixed">x</code></p>',
      'code',
      'language-x',
    ],
    ['h2', '<h2 class="sr-only fixed">x</h2>', 'h2', 'sr-only'],
    [
      'ul',
      '<ul class="contains-task-list fixed"><li>x</li></ul>',
      'ul',
      'contains-task-list',
    ],
  ])(
    'keeps only the allowed name of a mixed class list on %s',
    (_name, content, selector, kept) => {
      const { container } = renderRouted(content)

      const el = container.querySelector(selector)
      expect(el?.classList.contains(kept)).toBe(true)
      expect(el?.classList.contains('fixed')).toBe(false)
    },
  )

  it('keeps the class names markdown features emit', () => {
    const { container } = renderRouted(`- [x] done

Text[^1] ==marked== and $$a^2$$.

[^1]: Note.

\`\`\`js
const a = 1
\`\`\``)

    expect(container.querySelector('ul.contains-task-list')).not.toBeNull()
    expect(container.querySelector('li.task-list-item')).not.toBeNull()
    expect(
      container.querySelector('.markdown-footnotes.footnotes'),
    ).not.toBeNull()
    expect(container.querySelector('h2.sr-only')).not.toBeNull()
    expect(
      container.querySelector('mark.flexible-marker.flexible-marker-default'),
    ).not.toBeNull()
    expect(container.querySelector('code.language-js.hljs')).not.toBeNull()
    expect(container.querySelector('.katex .msupsub')).not.toBeNull()
  })

  it('keeps the class names of colored markers', () => {
    const { container } = renderRouted('=r=red== and =b=blue==')

    expect(container.querySelector('mark.flexible-marker-red')).not.toBeNull()
    expect(container.querySelector('mark.flexible-marker-blue')).not.toBeNull()
  })

  it.each([
    ['flexible-marker', true],
    ['flexible-marker-default', true],
    ['flexible-marker-empty', true],
    ['flexible-marker-red', true],
    ['flexible-marker-', false],
    ['flexible-marker-red-fixed', false],
    ['flexible-marker-RED', false],
    ['flexible-marker-1', false],
    ['xflexible-marker', false],
  ])('marker class %s is kept: %s', (name, kept) => {
    const { container } = renderRouted(`<mark class="${name}">x</mark>`)

    expect(container.querySelector('mark')?.classList.contains(name)).toBe(kept)
  })

  it.each([
    [
      'collapsible block',
      ':::collapsible Title\nBody\n:::',
      'details.markdown-collapsible',
    ],
    [
      'collapsed block',
      ':::collapsed Title\nBody\n:::',
      'details.markdown-collapsible',
    ],
    ['blockquote', '> quoted', 'blockquote'],
    [
      'shoutout block',
      ':::warning\nBody\n:::',
      'aside.markdown-shoutout.markdown-shoutout--warning',
    ],
    ['code fence with line numbers', '```go=\na := 1\n```', 'code.language-go'],
    [
      'plain code fence with line numbers',
      '```=\na\n```',
      'code.language-plaintext',
    ],
    ['table', '| a | b |\n|---|---|\n| 1 | 2 |', '.table-wrapper table'],
    ['math in a list', '- item $$a^2$$', 'li .katex'],
    ['math in a table', '| a |\n|---|\n| $$a^2$$ |', 'td .katex'],
    ['math in a shoutout', ':::info\n$$a^2$$\n:::', 'aside .katex'],
    [
      'math in a collapsible',
      ':::collapsible T\n$$a^2$$\n:::',
      'details .katex',
    ],
    [
      'raw HTML language-math code',
      '<code class="language-math">a^2</code>',
      '.katex',
    ],
  ])('keeps %s', (_name, content, selector) => {
    const { container } = renderRouted(content)

    expect(container.querySelector(selector)).not.toBeNull()
  })

  it('renders an undefined macro as a KaTeX error, not as raw TeX only', () => {
    const { container } = renderRouted('$$\n\\undefinedmacro{x}\n$$')

    expect(container.textContent).toContain('undefinedmacro')
    expect(
      container.querySelector('.katex-error, .katex [style*="color"]'),
    ).not.toBeNull()
  })

  it('does not trust math in raw HTML code elements', () => {
    const { container } = renderRouted(
      '<code class="language-math">\\htmlClass{fixed inset-0}{x}\\href{https://e.example/}{y}</code>',
    )

    expect(container.querySelector('.fixed, .inset-0')).toBeNull()
    expect(container.querySelector('a[href^="https://e.example"]')).toBeNull()
  })

  it('keeps whitelisted inline style and allowed media/mark tags', () => {
    const { container } = renderRouted(
      '<p style="color: red; position: fixed">kept</p>\n\n<mark>hi</mark>',
    )

    const p = Array.from(container.querySelectorAll('p')).find(
      (el) => el.textContent === 'kept',
    )
    expect(p?.getAttribute('style') ?? '').toMatch(/color:\s*red/)
    expect(p?.getAttribute('style') ?? '').not.toMatch(/position/)
    expect(container.querySelector('mark')?.textContent).toBe('hi')
  })
})

describe('MarkdownPreview math', () => {
  beforeEach(() => {
    localStorage.setItem('design-mode', 'light')
    useDesignModeStore.setState({ mode: 'light' })
    window.matchMedia = vi.fn().mockImplementation((query: string) => ({
      matches: query === '(prefers-color-scheme: light)',
      media: query,
      onchange: null,
      addEventListener: vi.fn(),
      removeEventListener: vi.fn(),
      addListener: vi.fn(),
      removeListener: vi.fn(),
      dispatchEvent: vi.fn(),
    }))
  })

  it.each([
    ['a $$ block', '$$\na^2\n$$'],
    ['a math fence', '```math\na^2\n```'],
  ])('renders %s as display math', (_name, content) => {
    const { container } = renderPreview(content)

    const display = container.querySelector('.katex-display .katex')
    expect(display).not.toBeNull()
    expect(display?.querySelector('.msupsub')).not.toBeNull()
  })

  it('renders $$ inside a paragraph as inline math', () => {
    const { container } = renderPreview('Before $$a^2$$ after')

    expect(container.querySelector('p .katex')).not.toBeNull()
    expect(container.querySelector('.katex-display')).toBeNull()
  })

  it('renders the drawn parts of math, such as the square root sign', () => {
    const { container } = renderPreview('$$\n\\sqrt{x}\n$$')

    expect(container.querySelector('.katex .hide-tail svg path')).not.toBeNull()
  })

  it.each([
    ['\\htmlClass', '$$\n\\htmlClass{fixed inset-0 z-50}{x}\n$$', '.fixed'],
    ['\\htmlId', '$$\n\\htmlId{app-root}{x}\n$$', '#app-root'],
    ['\\htmlData', '$$\n\\htmlData{foo=bar}{x}\n$$', '[data-foo]'],
    [
      '\\includegraphics',
      '$$\n\\includegraphics{https://e.example/x.png}\n$$',
      'img',
    ],
    ['\\href', '$$\n\\href{https://e.example/}{x}\n$$', 'a'],
    ['\\url', '$$\n\\url{https://e.example/}\n$$', 'a'],
  ])('does not trust %s in math', (_name, content, forbidden) => {
    const { container } = renderPreview(content)

    expect(container.querySelector('.katex, .katex-error')).not.toBeNull()
    expect(container.querySelector(`.katex ${forbidden}`)).toBeNull()
  })

  it.each([
    '$$\n\\htmlStyle{position:fixed;inset:0}{x}\n$$',
    '$$\n\\color{red;position:fixed}{x}\n$$',
    '$$\n\\textcolor{red;position:fixed}{x}\n$$',
    '$$\n\\rule{1px}{1px;position:fixed}\n$$',
  ])('keeps positioning out of math styles: %s', (content) => {
    const { container } = renderPreview(content)

    const styles = Array.from(container.querySelectorAll('[style]'))
      .map((el) => el.getAttribute('style'))
      .join(';')
    expect(styles).not.toMatch(/position|inset/i)
  })
})

describe('MarkdownPreview plain link tree refresh', () => {
  const target: PageNode = {
    id: 'target',
    title: 'Target',
    path: 'target',
    slug: 'target',
    version: 'v1',
    kind: 'page',
    children: null,
  }
  const moved: PageNode = { ...target, path: 'moved/target' }

  beforeEach(() => {
    useTreeStore.setState({ byId: {}, byPath: {} })
    localStorage.setItem('design-mode', 'light')
    useDesignModeStore.setState({ mode: 'light' })
    window.matchMedia = vi.fn().mockImplementation((query: string) => ({
      matches: query === '(prefers-color-scheme: light)',
      media: query,
      onchange: null,
      addEventListener: vi.fn(),
      removeEventListener: vi.fn(),
      addListener: vi.fn(),
      removeListener: vi.fn(),
      dispatchEvent: vi.fn(),
    }))
  })

  function renderTargetLink() {
    return render(
      <MemoryRouter>
        <TooltipProvider>
          <MarkdownPreview content="[Target](/target)" />
        </TooltipProvider>
      </MemoryRouter>,
    )
  }

  it('resolves a plain markdown link when the tree arrives later', () => {
    const { container } = renderTargetLink()
    expect(container.querySelector('a[href="/target"]')).toBeNull()

    act(() =>
      useTreeStore.setState({
        byId: { target },
        byPath: { target },
      }),
    )

    expect(container.querySelector('a[href="/target"]')?.textContent).toBe(
      'Target',
    )
  })

  it('unresolves a plain markdown link when the target is moved away', () => {
    useTreeStore.setState({ byId: { target }, byPath: { target } })
    const { container } = renderTargetLink()
    expect(container.querySelector('a[href="/target"]')).not.toBeNull()

    act(() =>
      useTreeStore.setState({
        byId: { target: moved },
        byPath: { 'moved/target': moved },
      }),
    )
    expect(container.querySelector('a[href="/target"]')).toBeNull()
  })

  it.each([
    { markdown: '[Target](/target#section)', href: '/target#section' },
    { markdown: '[Target](target?q=1#section)', href: '/target?q=1#section' },
    {
      markdown: '<a href="/target#section">Target</a>',
      href: '/target#section',
    },
  ])(
    'keeps the heading anchor of $markdown through the sanitizer (#1801)',
    ({ markdown, href }) => {
      useTreeStore.setState({ byId: { target }, byPath: { target } })
      const { container } = render(
        <MemoryRouter>
          <TooltipProvider>
            <MarkdownPreview content={markdown} />
          </TooltipProvider>
        </MemoryRouter>,
      )

      const link = container.querySelector('a')
      expect(link?.getAttribute('href')).toBe(href)
      expect(link?.textContent).toBe('Target')
    },
  )
})
