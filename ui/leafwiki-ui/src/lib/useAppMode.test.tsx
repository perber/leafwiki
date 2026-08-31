import { render, screen } from '@testing-library/react'
import { MemoryRouter } from 'react-router'
import { describe, expect, it } from 'vitest'
import { useAppMode } from './useAppMode'

function ModeProbe() {
  return <span>{useAppMode()}</span>
}

function renderAt(path: string) {
  render(
    <MemoryRouter initialEntries={[path]}>
      <ModeProbe />
    </MemoryRouter>,
  )
}

describe('useAppMode', () => {
  it.each(['/e/published-page', '/pending-drafts/pending-1/edit'])(
    'treats %s as edit mode',
    (path) => {
      renderAt(path)

      expect(screen.getByText('edit')).toBeInTheDocument()
    },
  )

  it('does not classify other pending-draft paths as edit mode', () => {
    renderAt('/pending-drafts/pending-1')

    expect(screen.getByText('view')).toBeInTheDocument()
  })
})
