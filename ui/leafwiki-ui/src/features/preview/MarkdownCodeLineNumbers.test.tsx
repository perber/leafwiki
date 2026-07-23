import { render } from '@testing-library/react'
import { useDesignModeStore } from '@/features/designtoggle/designmode'
import MarkdownPreview from './MarkdownPreview'

describe('MarkdownPreview code block line numbers (#1313)', () => {
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

  it('renders a numbered gutter when the fence language ends with "="', () => {
    const content = '```js=\nconst a = 1\nconst b = 2\nconst c = 3\n```'
    const { container } = render(<MarkdownPreview content={content} />)

    const gutter = container.querySelector(
      '[data-testid="markdown-code-line-numbers"]',
    )
    expect(gutter).not.toBeNull()

    const numbers = gutter?.querySelectorAll(
      '.markdown-code-block__line-number',
    )
    expect(numbers?.length).toBe(3)
    expect(Array.from(numbers ?? []).map((n) => n.textContent)).toEqual([
      '1',
      '2',
      '3',
    ])

    expect(
      container.querySelector('.markdown-code-block--line-numbers'),
    ).not.toBeNull()

    // The trailing "=" is stripped so the real language is still highlighted.
    const code = container.querySelector('code.md-line-numbers')
    expect(code).not.toBeNull()
    expect(code?.getAttribute('class')).toContain('language-js')
    expect(code?.getAttribute('class')).not.toContain('language-js=')
    expect(code?.classList.contains('hljs')).toBe(true)
    expect(code?.querySelector('.hljs-keyword')).not.toBeNull()
  })

  it('counts every line, including the last non-empty line', () => {
    const content = '```python=\nx = 1\ny = 2\n```'
    const { container } = render(<MarkdownPreview content={content} />)

    const numbers = container.querySelectorAll(
      '.markdown-code-block__line-number',
    )
    expect(numbers.length).toBe(2)
  })

  it('does not render line numbers for a plain fenced block', () => {
    const content = '```js\nconst a = 1\nconst b = 2\n```'
    const { container } = render(<MarkdownPreview content={content} />)

    expect(
      container.querySelector('[data-testid="markdown-code-line-numbers"]'),
    ).toBeNull()
    expect(
      container.querySelector('.markdown-code-block--line-numbers'),
    ).toBeNull()

    const code = container.querySelector('code.language-js.hljs')
    expect(code).not.toBeNull()
    expect(code?.classList.contains('md-line-numbers')).toBe(false)
  })
})
