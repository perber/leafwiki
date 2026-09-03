import { TooltipProvider } from '@/components/ui/tooltip'
import * as backupApi from '@/lib/api/backup'
import { useConfigStore } from '@/stores/config'
import { useSessionStore } from '@/stores/session'
import { render, screen, waitFor } from '@testing-library/react'
import type { ReactNode } from 'react'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { BackupWarningIndicator } from './BackupWarningIndicator'

const wrap = (ui: ReactNode) => render(<TooltipProvider>{ui}</TooltipProvider>)

vi.mock('react-i18next', () => ({
  initReactI18next: { type: '3rdParty', init: () => {} },
  useTranslation: () => ({ t: (k: string) => k }),
}))
vi.mock('react-router', () => ({ useNavigate: () => vi.fn() }))
vi.mock('@/lib/api/backup', () => ({ fetchBackupAlert: vi.fn() }))

const asAdmin = () =>
  useSessionStore.setState({
    user: { id: '1', username: 'a', role: 'admin' },
  } as unknown as Parameters<typeof useSessionStore.setState>[0])

describe('BackupWarningIndicator', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    asAdmin()
    useConfigStore.setState({ gitBackupConfigured: false })
  })

  it('does not poll or render when no backup is configured', () => {
    wrap(<BackupWarningIndicator />)
    expect(backupApi.fetchBackupAlert).not.toHaveBeenCalled()
    expect(
      screen.queryByRole('button', { name: 'warningAriaLabel' }),
    ).not.toBeInTheDocument()
  })

  it('shows the warning for a configured backup reporting an error, even when it is not running', async () => {
    vi.mocked(backupApi.fetchBackupAlert).mockResolvedValue({
      needsIntervention: false,
      hasError: true,
    })
    useConfigStore.setState({ gitBackupConfigured: true })

    wrap(<BackupWarningIndicator />)

    await waitFor(() => expect(backupApi.fetchBackupAlert).toHaveBeenCalled())
    await waitFor(() =>
      expect(
        screen.getByRole('button', { name: 'warningAriaLabel' }),
      ).toBeInTheDocument(),
    )
  })
})
