import type { BackupConfig } from '@/lib/api/backup'
import { fireEvent, render, screen } from '@testing-library/react'
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

const baseConfig: BackupConfig = {
  remoteUrl: 'https://github.com/acme/wiki-backup.git',
  branch: 'main',
  authorName: 'Backup Bot',
  authorEmail: 'bot@example.com',
  authMode: 'https',
  sshKeyPath: '',
  sshKnownHostsPath: '',
  httpUsername: 'acme-bot',
  hasSshKey: false,
  hasHttpPassword: true,
  intervalMinutes: 30,
}

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
    config: baseConfig,
    configLoading: false,
    configError: '',
    encryptionKeyAvailable: true,
    minIntervalMinutes: 2,
    maxIntervalMinutes: 1440,
    loadConfig: vi.fn(),
    testConfig: vi.fn(),
    saveConfig: vi.fn(),
    disable: vi.fn(),
    ...overrides,
  }
}

describe('BackupSettings', () => {
  beforeEach(() => {
    storeState = makeState()
  })

  it('renders the configuration form when the backup is settings-managed', () => {
    render(<BackupSettings />)
    expect(screen.getByText('config.remoteUrl')).toBeInTheDocument()
    expect(
      screen.getByRole('button', { name: 'config.saveButton' }),
    ).toBeInTheDocument()
    expect(screen.queryByText('config.envManagedHint')).not.toBeInTheDocument()
  })

  it('hides the form and shows a hint when the backup is env-managed', () => {
    storeState = makeState({ envManaged: true })
    render(<BackupSettings />)
    expect(screen.getByText('config.envManagedHint')).toBeInTheDocument()
    expect(screen.queryByText('config.remoteUrl')).not.toBeInTheDocument()
    expect(
      screen.queryByRole('button', { name: 'config.saveButton' }),
    ).not.toBeInTheDocument()
  })

  it('rejects an interval below the minimum and disables Save', () => {
    render(<BackupSettings />)
    const interval = screen.getByLabelText('config.intervalMinutes')
    fireEvent.change(interval, { target: { value: '1' } })
    expect(screen.getByText('config.intervalOutOfRange')).toBeInTheDocument()
    expect(
      screen.getByRole('button', { name: 'config.saveButton' }),
    ).toBeDisabled()
  })

  it('accepts an interval within range', () => {
    render(<BackupSettings />)
    const interval = screen.getByLabelText('config.intervalMinutes')
    fireEvent.change(interval, { target: { value: '120' } })
    expect(
      screen.queryByText('config.intervalOutOfRange'),
    ).not.toBeInTheDocument()
    expect(
      screen.getByRole('button', { name: 'config.saveButton' }),
    ).not.toBeDisabled()
  })
})
