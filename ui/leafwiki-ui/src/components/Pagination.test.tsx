import '@/lib/i18n'
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it, vi } from 'vitest'
import { Pagination } from './Pagination'

describe('Pagination', () => {
  it('renders nothing when there is only a single page', () => {
    const { container } = render(
      <Pagination total={5} page={0} limit={10} onPageChange={vi.fn()} />,
    )
    expect(container).toBeEmptyDOMElement()
  })

  it('renders translated prev/next labels and the interpolated page counter', () => {
    render(<Pagination total={30} page={1} limit={10} onPageChange={vi.fn()} />)
    expect(screen.getByText('← Prev')).toBeInTheDocument()
    expect(screen.getByText('Next →')).toBeInTheDocument()
    // page is zero-based internally, shown one-based
    expect(screen.getByText('Page 2 of 3')).toBeInTheDocument()
  })

  it('clamps navigation to the valid page range', async () => {
    const onPageChange = vi.fn()
    render(
      <Pagination total={30} page={0} limit={10} onPageChange={onPageChange} />,
    )
    const user = userEvent.setup()

    await user.click(screen.getByText('← Prev'))
    expect(onPageChange).not.toHaveBeenCalled() // already on first page, button disabled

    await user.click(screen.getByText('Next →'))
    expect(onPageChange).toHaveBeenCalledWith(1)
  })
})
