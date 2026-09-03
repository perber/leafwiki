import type { BackupConfig } from '@/lib/api/backup'
import { fireEvent, render, screen } from '@testing-library/react'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import BackupConfigForm from './BackupConfigForm'

vi.mock('react-i18next', () => ({
  initReactI18next: { type: '3rdParty', init: () => {} },
  useTranslation: () => ({ t: (key: string) => key }),
}))

vi.mock('sonner', () => ({ toast: { success: vi.fn(), error: vi.fn() } }))

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

vi.mock('@/stores/backup', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@/stores/backup')>()
  return { ...actual, useBackupStore: () => storeState }
})

function makeState(overrides: Partial<Record<string, unknown>> = {}) {
  return {
    enabled: true,
    config: baseConfig,
    configLoading: false,
    configError: '',
    encryptionKeyAvailable: true,
    minIntervalMinutes: 2,
    maxIntervalMinutes: 1440,
    testConfig: vi.fn(),
    saveConfig: vi.fn(),
    disable: vi.fn(),
    ...overrides,
  }
}

describe('BackupConfigForm', () => {
  beforeEach(() => {
    storeState = makeState()
  })

  it('rejects an interval below the minimum and disables Save', () => {
    render(<BackupConfigForm />)
    fireEvent.change(screen.getByLabelText('config.intervalMinutes'), {
      target: { value: '1' },
    })
    expect(screen.getByText('config.intervalOutOfRange')).toBeInTheDocument()
    expect(
      screen.getByRole('button', { name: 'config.saveButton' }),
    ).toBeDisabled()
  })

  it('accepts an interval within range', () => {
    render(<BackupConfigForm />)
    fireEvent.change(screen.getByLabelText('config.intervalMinutes'), {
      target: { value: '120' },
    })
    expect(
      screen.queryByText('config.intervalOutOfRange'),
    ).not.toBeInTheDocument()
    expect(
      screen.getByRole('button', { name: 'config.saveButton' }),
    ).not.toBeDisabled()
  })

  it('shows HTTP credential fields for an https remote', () => {
    storeState = makeState({
      config: { ...baseConfig, remoteUrl: 'https://example.com/wiki.git' },
    })
    render(<BackupConfigForm />)
    expect(screen.getByLabelText('config.httpUsername')).toBeInTheDocument()
    expect(screen.queryByLabelText('config.sshKey')).not.toBeInTheDocument()
  })

  it('shows the SSH key fields for a git@ remote', () => {
    storeState = makeState({
      config: { ...baseConfig, remoteUrl: 'git@github.com:acme/wiki.git' },
    })
    render(<BackupConfigForm />)
    expect(screen.getByLabelText('config.sshKey')).toBeInTheDocument()
    expect(
      screen.queryByLabelText('config.httpUsername'),
    ).not.toBeInTheDocument()
  })

  it('swaps credential fields when the remote URL scheme changes', () => {
    storeState = makeState({
      config: { ...baseConfig, remoteUrl: 'https://example.com/wiki.git' },
    })
    render(<BackupConfigForm />)
    expect(screen.getByLabelText('config.httpUsername')).toBeInTheDocument()

    fireEvent.change(screen.getByLabelText('config.remoteUrl'), {
      target: { value: 'git@github.com:acme/wiki.git' },
    })
    expect(screen.getByLabelText('config.sshKey')).toBeInTheDocument()
    expect(
      screen.queryByLabelText('config.httpUsername'),
    ).not.toBeInTheDocument()
  })

  it('keeps credential fields editable and shows a note when secrets are stored unencrypted', () => {
    storeState = makeState({
      encryptionKeyAvailable: false,
      config: { ...baseConfig, remoteUrl: 'git@github.com:acme/wiki.git' },
    })
    render(<BackupConfigForm />)
    expect(
      screen.getByText('config.credentialsUnencryptedHint'),
    ).toBeInTheDocument()
    expect(screen.getByLabelText('config.sshKey')).not.toBeDisabled()
  })
})
