import '@/lib/i18n'
import { useHotKeysStore } from '@/stores/hotkeys'
import { useLastWikiLocationStore } from '@/stores/lastWikiLocation'
import { useToolbarStore } from '@/features/toolbar/toolbarStore'
import { act, render, screen } from '@testing-library/react'
import { MemoryRouter, Route, Routes, useLocation } from 'react-router'
import { beforeEach, describe, expect, it } from 'vitest'
import SettingsLayout from './SettingsLayout'

beforeEach(() => {
  useToolbarStore.setState({ buttons: [] })
  useHotKeysStore.setState({ registeredHotkeys: {} })
  useLastWikiLocationStore.setState({ location: null })
})

function LocationProbe() {
  const location = useLocation()
  return (
    <div>
      <span data-testid="pathname">{location.pathname}</span>
      <span data-testid="state">{JSON.stringify(location.state)}</span>
    </div>
  )
}

function renderSettings() {
  return render(
    <MemoryRouter initialEntries={['/settings/account']}>
      <Routes>
        <Route path="/settings" element={<SettingsLayout />}>
          <Route
            path="account"
            element={<div data-testid="settings-account">account</div>}
          />
        </Route>
        <Route path="*" element={<LocationProbe />} />
      </Routes>
    </MemoryRouter>,
  )
}

function triggerExit() {
  const button = useToolbarStore.getState().buttons[0]
  expect(button?.id).toBe('exit-settings')
  act(() => {
    button.action()
  })
}

describe('SettingsLayout exit', () => {
  it('returns to the remembered wiki location, re-supplying its navigation state', () => {
    useLastWikiLocationStore.setState({
      location: {
        pathname: '/docs/getting-started',
        search: '',
        state: { leafwikiVisitId: 'visit-42' },
      },
    })

    renderSettings()
    triggerExit()

    expect(screen.getByTestId('pathname')).toHaveTextContent(
      '/docs/getting-started',
    )
    expect(screen.getByTestId('state')).toHaveTextContent('visit-42')
  })

  it('falls back to / when there is no remembered location', () => {
    renderSettings()
    triggerExit()

    expect(screen.getByTestId('pathname')).toHaveTextContent('/')
  })
})
