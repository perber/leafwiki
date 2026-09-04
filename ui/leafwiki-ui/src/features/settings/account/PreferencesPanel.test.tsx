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
  const setDateFormat = vi.fn()
  const setTimeFormat = vi.fn()

  beforeEach(() => {
    vi.clearAllMocks()
    useUserSettingsStore.setState({
      autoSave: true,
      language: 'en',
      dateFormat: 'locale',
      timeFormat: 'locale',
      loaded: true,
      setLanguage,
      toggleAutoSave,
      setDateFormat,
      setTimeFormat,
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

  it('renders the date and time format pickers', () => {
    render(<PreferencesPanel />)

    expect(
      screen.getByTestId('preferences-dateformat-select'),
    ).toBeInTheDocument()
    expect(
      screen.getByTestId('preferences-timeformat-select'),
    ).toBeInTheDocument()
  })

  it('associates each format picker with its visible label', () => {
    render(<PreferencesPanel />)

    // Accessible name comes from the label via aria-labelledby, not the
    // selected value — so the two pickers are distinguishable to AT.
    expect(
      screen.getByRole('combobox', { name: 'Language' }),
    ).toBeInTheDocument()
    expect(
      screen.getByRole('combobox', { name: 'Date format' }),
    ).toBeInTheDocument()
    expect(
      screen.getByRole('combobox', { name: 'Time format' }),
    ).toBeInTheDocument()
  })

  it('lets the user pick a date format', async () => {
    const user = userEvent.setup({ delay: null, pointerEventsCheck: 0 })
    render(<PreferencesPanel />)

    await user.click(screen.getByTestId('preferences-dateformat-select'))
    await user.click(await screen.findByText('27.08.2026'))

    expect(setDateFormat).toHaveBeenCalledWith('dmy_dot')
  })

  it('lets the user pick a time format', async () => {
    const user = userEvent.setup({ delay: null, pointerEventsCheck: 0 })
    render(<PreferencesPanel />)

    await user.click(screen.getByTestId('preferences-timeformat-select'))
    await user.click(await screen.findByText('24-hour (14:30)'))

    expect(setTimeFormat).toHaveBeenCalledWith('24h')
  })
})
