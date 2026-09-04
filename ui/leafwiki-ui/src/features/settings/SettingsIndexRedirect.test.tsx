import { render, screen } from '@testing-library/react'
import { MemoryRouter, Route, Routes } from 'react-router'
import { describe, expect, it, vi } from 'vitest'
import SettingsIndexRedirect from './SettingsIndexRedirect'

const mockUseSettingsSectionContext = vi.fn()

vi.mock('@/lib/registries/settingsSectionRegistry', async (importOriginal) => {
  const actual =
    await importOriginal<
      typeof import('@/lib/registries/settingsSectionRegistry')
    >()
  return {
    ...actual,
    useSettingsSectionContext: () => mockUseSettingsSectionContext(),
  }
})

function renderIndex() {
  return render(
    <MemoryRouter initialEntries={['/settings']}>
      <Routes>
        <Route path="/settings" element={<SettingsIndexRedirect />} />
        <Route
          path="/settings/account"
          element={<div data-testid="landed-account">account</div>}
        />
        <Route
          path="/settings/branding"
          element={<div data-testid="landed-branding">branding</div>}
        />
        <Route path="/" element={<div data-testid="landed-root">root</div>} />
      </Routes>
    </MemoryRouter>,
  )
}

describe('SettingsIndexRedirect', () => {
  it('redirects a regular authenticated user to the account section (the first section visible to any role)', () => {
    mockUseSettingsSectionContext.mockReturnValue({ role: 'viewer' })

    renderIndex()

    expect(screen.getByTestId('landed-account')).toBeInTheDocument()
  })

  it('falls back to / when no section is visible for the current context', () => {
    mockUseSettingsSectionContext.mockReturnValue({ role: undefined })

    renderIndex()

    expect(screen.getByTestId('landed-root')).toBeInTheDocument()
  })
})
