import { render } from '@testing-library/react'
import { useDesignModeStore } from '@/features/designtoggle/designmode'
import MarkdownPreview from './MarkdownPreview'

describe('MarkdownPreview syntax highlighting', () => {
  beforeEach(() => {
    localStorage.setItem('design-mode', 'light')
    useDesignModeStore.setState({ mode: 'light' })
    window.matchMedia = vi.fn().mockImplementation(() => ({
      matches: true,
      media: '(prefers-color-scheme: light)',
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

    const { container } = render(<MarkdownPreview content={content} />)

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

    const { container } = render(<MarkdownPreview content={content} />)

    const autohotkeyCodeBlock = container.querySelector(
      'code.language-autohotkey.hljs',
    )
    expect(autohotkeyCodeBlock).not.toBeNull()
    expect(autohotkeyCodeBlock?.querySelector('.hljs-meta')).not.toBeNull()
    expect(autohotkeyCodeBlock?.querySelector('.hljs-string')).not.toBeNull()
  })

  it('renders external images from markdown image syntax', () => {
    const { container } = render(
      <MarkdownPreview content="![Remote diagram](https://example.com/diagram.png)" />,
    )

    const image = container.querySelector('img')
    expect(image).not.toBeNull()
    expect(image?.getAttribute('src')).toBe('https://example.com/diagram.png')
    expect(image?.getAttribute('alt')).toBe('Remote diagram')
  })

  it('renders external images from sanitized inline html', () => {
    const { container } = render(
      <MarkdownPreview content='<img src="https://example.com/banner.png" alt="Remote banner" />' />,
    )

    const image = container.querySelector('img')
    expect(image).not.toBeNull()
    expect(image?.getAttribute('src')).toBe('https://example.com/banner.png')
    expect(image?.getAttribute('alt')).toBe('Remote banner')
  })
})
