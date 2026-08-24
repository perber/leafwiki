import '@/lib/i18n'
import { useUserSettingsStore } from '@/stores/userSettings'
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { PreferencesPanel } from './PreferencesPanel'

// jsdom has no real layout/pointer-capture engine, which Radix's Select
// relies on when opening its popover — stub the bits it touches.
beforeEach(() => {
  window.HTMLElement.prototype.scrollIntoView = vi.fn()
  window.HTMLElement.prototype.hasPointerCapture = vi
    .fn()
    .mockReturnValue(false)
  window.HTMLElement.prototype.releasePointerCapture = vi.fn()
})

describe('PreferencesPanel', () => {
  const setLanguage = vi.fn()
  const toggleAutoSave = vi.fn()

  beforeEach(() => {
    vi.clearAllMocks()
    useUserSettingsStore.setState({
      autoSave: true,
      language: 'en',
      loaded: true,
      setLanguage,
      toggleAutoSave,
    })
  })

  it('shows the currently selected language', () => {
    render(<PreferencesPanel />)

    expect(screen.getByText('English')).toBeInTheDocument()
  })

  it('lets the user pick a different shipped language', async () => {
    const user = userEvent.setup({ delay: null, pointerEventsCheck: 0 })
    render(<PreferencesPanel />)

    await user.click(screen.getByTestId('preferences-language-select'))
    await user.click(await screen.findByText('Deutsch'))

    expect(setLanguage).toHaveBeenCalledWith('de')
  })
})
