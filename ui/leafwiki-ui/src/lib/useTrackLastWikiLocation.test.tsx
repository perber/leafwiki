import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { MemoryRouter, useNavigate } from 'react-router'
import { beforeEach, describe, expect, it } from 'vitest'
import { useLastWikiLocationStore } from '@/stores/lastWikiLocation'
import { useTrackLastWikiLocation } from './useTrackLastWikiLocation'

beforeEach(() => {
  useLastWikiLocationStore.setState({ location: null })
})

function Harness() {
  useTrackLastWikiLocation()
  const navigate = useNavigate()
  return (
    <div>
      <button onClick={() => navigate('/docs/intro')}>go-intro</button>
      <button onClick={() => navigate('/e/docs/intro')}>go-edit</button>
      <button onClick={() => navigate('/settings-guide')}>
        go-settings-slug
      </button>
      <button
        onClick={() =>
          navigate('/settings/account', { state: { leafwikiVisitId: 'nope' } })
        }
      >
        go-settings
      </button>
    </div>
  )
}

function renderHarness(initialEntry: string) {
  return render(
    <MemoryRouter initialEntries={[initialEntry]}>
      <Harness />
    </MemoryRouter>,
  )
}

describe('useTrackLastWikiLocation', () => {
  it('remembers the current non-settings location on mount', () => {
    renderHarness('/docs/getting-started')

    expect(useLastWikiLocationStore.getState().location).toMatchObject({
      pathname: '/docs/getting-started',
    })
  })

  it('updates as the user navigates between view and edit routes', async () => {
    const user = userEvent.setup()
    renderHarness('/docs/getting-started')

    await user.click(screen.getByText('go-intro'))
    expect(useLastWikiLocationStore.getState().location?.pathname).toBe(
      '/docs/intro',
    )

    await user.click(screen.getByText('go-edit'))
    expect(useLastWikiLocationStore.getState().location?.pathname).toBe(
      '/e/docs/intro',
    )
  })

  it('does not overwrite the remembered location when entering settings', async () => {
    const user = userEvent.setup()
    renderHarness('/docs/getting-started')

    await user.click(screen.getByText('go-intro'))
    await user.click(screen.getByText('go-settings'))

    expect(useLastWikiLocationStore.getState().location?.pathname).toBe(
      '/docs/intro',
    )
  })

  it('still tracks a normal page whose slug merely starts with "settings"', async () => {
    const user = userEvent.setup()
    renderHarness('/docs/getting-started')

    await user.click(screen.getByText('go-settings-slug'))

    expect(useLastWikiLocationStore.getState().location?.pathname).toBe(
      '/settings-guide',
    )
  })
})
