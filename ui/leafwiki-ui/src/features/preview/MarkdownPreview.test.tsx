import { render } from '@testing-library/react'
import { MemoryRouter } from 'react-router'
import type { PageNode } from '@/lib/api/pages'
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
