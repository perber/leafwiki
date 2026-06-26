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
})
