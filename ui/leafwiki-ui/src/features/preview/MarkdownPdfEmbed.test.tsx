import { render, screen } from '@testing-library/react'
import { describe, expect, it } from 'vitest'
import { MarkdownPdfEmbed } from './MarkdownPdfEmbed'

let mockIsMobile = false

vi.mock('@/lib/useIsMobile', () => ({
  useIsMobile: () => mockIsMobile,
}))

describe('MarkdownPdfEmbed', () => {
  beforeEach(() => {
    mockIsMobile = false
  })

  it('renders an iframe pointing at the pdf source', () => {
    render(<MarkdownPdfEmbed src="/assets/manual.pdf" alt="Manual" />)

    const frame = document.querySelector('iframe')
    expect(frame).not.toBeNull()
    expect(frame?.getAttribute('src')).toContain('/assets/manual.pdf')
    expect(frame?.getAttribute('title')).toBe('Manual')
  })

  it('keeps a #page= fragment through cache-busting versioning', () => {
    render(<MarkdownPdfEmbed src="/assets/manual.pdf#page=3" alt="Manual" />)

    const frame = document.querySelector('iframe')
    const src = frame?.getAttribute('src') ?? ''
    expect(src).toContain('/assets/manual.pdf')
    expect(src).toMatch(/\?v=\d+#page=3$/)
  })

  it('falls back to a translated title when no alt text is given', () => {
    render(<MarkdownPdfEmbed src="/assets/manual.pdf" />)

    const frame = document.querySelector('iframe')
    expect(frame?.getAttribute('title')).toBe('PDF')
  })

  it('renders a fallback link to open the pdf in a new tab', () => {
    render(<MarkdownPdfEmbed src="https://example.com/doc.pdf" alt="Doc" />)

    const link = screen.getByRole('link')
    expect(link.getAttribute('href')).toBe('https://example.com/doc.pdf')
    expect(link.getAttribute('target')).toBe('_blank')
  })

  it('does not iframe a pdf from another origin, only links to it', () => {
    render(<MarkdownPdfEmbed src="https://example.com/doc.pdf" alt="Doc" />)

    expect(document.querySelector('iframe')).toBeNull()

    const link = screen.getByRole('link')
    expect(link.getAttribute('href')).toBe('https://example.com/doc.pdf')
    expect(link.getAttribute('target')).toBe('_blank')
  })

  it('shows a file card instead of an iframe on mobile', () => {
    mockIsMobile = true
    render(<MarkdownPdfEmbed src="/assets/manual.pdf" alt="Manual" />)

    expect(document.querySelector('iframe')).toBeNull()

    const link = screen.getByRole('link')
    expect(link.getAttribute('href')).toContain('/assets/manual.pdf')
    expect(link.getAttribute('target')).toBe('_blank')
    expect(link).toHaveTextContent('Manual')
    expect(link).toHaveTextContent('manual.pdf')
    expect(link).toHaveTextContent('Open in new tab')
  })

  it('uses the decoded file name as card title when there is no alt text', () => {
    mockIsMobile = true
    render(<MarkdownPdfEmbed src="/assets/Benutzer%20Handbuch.pdf#page=2" />)

    const link = screen.getByRole('link')
    expect(link).toHaveTextContent('Benutzer Handbuch.pdf')
    expect(link.textContent?.match(/Benutzer Handbuch\.pdf/g)).toHaveLength(1)
  })
})
