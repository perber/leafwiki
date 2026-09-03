import { render, screen } from '@testing-library/react'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import BackupSettings from './BackupSettings'

vi.mock('react-i18next', () => ({
  initReactI18next: { type: '3rdParty', init: () => {} },
  useTranslation: () => ({ t: (key: string) => key }),
}))

vi.mock('sonner', () => ({
  toast: { success: vi.fn(), error: vi.fn() },
}))

vi.mock('@/lib/useDateTimeFormat', () => ({
  useDateTimeFormat: () => ({ formatDateTime: () => '' }),
}))

vi.mock('../viewer/setTitle', () => ({ useSetTitle: () => {} }))

// BackupConfigForm has its own test file; stub it so these tests only exercise
// the wrapper's status view + "show the form iff not env-managed" decision.
vi.mock('./BackupConfigForm', () => ({
  default: () => <div data-testid="backup-config-form" />,
}))

let storeState: Record<string, unknown>

vi.mock('@/stores/backup', () => ({
  useBackupStore: () => storeState,
}))

function makeState(overrides: Partial<Record<string, unknown>> = {}) {
  return {
    enabled: true,
    envManaged: false,
    bootError: '',
    lastBackupAt: null,
    lastError: '',
    needsIntervention: false,
    conflictDetails: '',
    isLoading: false,
    isPolling: false,
    pollingFromAt: null,
    statusError: '',
    loadStatus: vi.fn(),
    triggerPush: vi.fn(),
    forcePush: vi.fn(),
    stopPolling: vi.fn(),
    loadConfig: vi.fn(),
    ...overrides,
  }
}

describe('BackupSettings', () => {
  beforeEach(() => {
    storeState = makeState()
  })

  it('renders the configuration form when the backup is settings-managed', () => {
    render(<BackupSettings />)
    expect(screen.getByTestId('backup-config-form')).toBeInTheDocument()
    expect(screen.queryByText('config.envManagedHint')).not.toBeInTheDocument()
  })

  it('hides the form and shows a hint when the backup is env-managed', () => {
    storeState = makeState({ envManaged: true })
    render(<BackupSettings />)
    expect(screen.getByText('config.envManagedHint')).toBeInTheDocument()
    expect(screen.queryByTestId('backup-config-form')).not.toBeInTheDocument()
  })
})
