import { render, screen } from '@testing-library/react'
import { describe, expect, it } from 'vitest'
import { MarkdownPdfEmbed } from './MarkdownPdfEmbed'

describe('MarkdownPdfEmbed', () => {
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
})
