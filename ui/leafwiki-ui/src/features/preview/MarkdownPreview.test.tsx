import { act, render } from '@testing-library/react'
import { MemoryRouter } from 'react-router'
import type { PageNode } from '@/lib/api/pages'
import * as assetSrcModule from '@/lib/assetSrc'
import { TooltipProvider } from '@/components/ui/tooltip'
import { useDesignModeStore } from '@/features/designtoggle/designmode'
import { useTreeStore } from '@/stores/tree'
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
})
