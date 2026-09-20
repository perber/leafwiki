import type { BackupConfig } from '@/lib/api/backup'
import { fireEvent, render, screen, within } from '@testing-library/react'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import BackupConfigForm from './BackupConfigForm'

vi.mock('react-i18next', () => ({
  initReactI18next: { type: '3rdParty', init: () => {} },
  useTranslation: () => ({ t: (key: string) => key }),
}))

vi.mock('sonner', () => ({ toast: { success: vi.fn(), error: vi.fn() } }))

const baseConfig: BackupConfig = {
  remoteUrl: 'https://github.com/acme/wiki-backup.git',
  path: '',
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
    expect(screen.getByText('config.httpUsername')).toBeInTheDocument()
    expect(screen.queryByLabelText('config.sshKey')).not.toBeInTheDocument()
  })

  it('shows the SSH key fields for a git@ remote', () => {
    storeState = makeState({
      config: { ...baseConfig, remoteUrl: 'git@github.com:acme/wiki.git' },
    })
    render(<BackupConfigForm />)
    expect(screen.getByLabelText('config.sshKey')).toBeInTheDocument()
    expect(screen.queryByText('config.httpUsername')).not.toBeInTheDocument()
  })

  it('swaps credential fields when the remote URL scheme changes', () => {
    storeState = makeState({
      config: { ...baseConfig, remoteUrl: 'https://example.com/wiki.git' },
    })
    render(<BackupConfigForm />)
    expect(screen.getByText('config.httpUsername')).toBeInTheDocument()

    fireEvent.change(screen.getByLabelText('config.remoteUrl'), {
      target: { value: 'git@github.com:acme/wiki.git' },
    })
    expect(screen.getByLabelText('config.sshKey')).toBeInTheDocument()
    expect(screen.queryByText('config.httpUsername')).not.toBeInTheDocument()
  })

  it('seeds the repository path field from the loaded config and submits a trimmed value', async () => {
    storeState = makeState({ config: { ...baseConfig, path: 'docs/wiki' } })
    render(<BackupConfigForm />)
    const pathInput = screen.getByLabelText('config.path') as HTMLInputElement
    expect(pathInput.value).toBe('docs/wiki')

    fireEvent.change(pathInput, { target: { value: '  docs/other  ' } })
    fireEvent.click(screen.getByRole('button', { name: 'config.saveButton' }))

    expect(storeState.saveConfig).toHaveBeenCalledWith(
      expect.objectContaining({ path: 'docs/other' }),
    )
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

  it('does not render an editable, autofillable HTTP username field when a username is already stored', () => {
    storeState = makeState({
      config: { ...baseConfig, remoteUrl: 'https://example.com/wiki.git' },
    })
    render(<BackupConfigForm />)
    expect(
      screen.queryByLabelText('config.httpUsername'),
    ).not.toBeInTheDocument()
    expect(screen.getByText(baseConfig.httpUsername)).toBeInTheDocument()
    // one Change button for the username, one for the password
    expect(
      screen.getAllByRole('button', { name: 'config.changeSecretButton' }),
    ).toHaveLength(2)
  })

  it('reveals an editable HTTP username field pre-filled with the current value after Change is clicked', () => {
    storeState = makeState({
      config: { ...baseConfig, remoteUrl: 'https://example.com/wiki.git' },
    })
    render(<BackupConfigForm />)
    fireEvent.click(
      screen.getAllByRole('button', { name: 'config.changeSecretButton' })[0],
    )
    const usernameInput = screen.getByLabelText('config.httpUsername')
    expect(usernameInput).toHaveValue(baseConfig.httpUsername)
    expect(usernameInput).toHaveAttribute('autocomplete', 'off')
  })

  it('shows the HTTP username field directly when nothing is configured yet', () => {
    storeState = makeState({
      config: {
        ...baseConfig,
        remoteUrl: 'https://example.com/wiki.git',
        httpUsername: '',
      },
    })
    render(<BackupConfigForm />)
    expect(screen.getByLabelText('config.httpUsername')).toBeInTheDocument()
  })

  it('does not render an editable, autofillable HTTP password field when a credential is already stored', () => {
    storeState = makeState({
      config: { ...baseConfig, remoteUrl: 'https://example.com/wiki.git' },
    })
    render(<BackupConfigForm />)
    expect(
      screen.queryByLabelText('config.httpPassword'),
    ).not.toBeInTheDocument()
    expect(screen.getByText('config.secretStoredLabel')).toBeInTheDocument()
    const passwordSection = screen
      .getByText('config.httpPassword')
      .closest('div')!
    expect(
      within(passwordSection).getByRole('button', {
        name: 'config.changeSecretButton',
      }),
    ).toBeInTheDocument()
  })

  it('reveals an empty, non-autofillable HTTP password field only after Change is clicked', () => {
    storeState = makeState({
      config: { ...baseConfig, remoteUrl: 'https://example.com/wiki.git' },
    })
    render(<BackupConfigForm />)
    const passwordSection = screen
      .getByText('config.httpPassword')
      .closest('div')!
    fireEvent.click(
      within(passwordSection).getByRole('button', {
        name: 'config.changeSecretButton',
      }),
    )
    const passwordInput = screen.getByLabelText('config.httpPassword')
    expect(passwordInput).toBeInTheDocument()
    expect(passwordInput).toHaveValue('')
    expect(passwordInput).toHaveAttribute('autocomplete', 'new-password')
  })

  it('shows the HTTP password field directly when no credential is stored yet', () => {
    storeState = makeState({
      config: {
        ...baseConfig,
        remoteUrl: 'https://example.com/wiki.git',
        hasHttpPassword: false,
      },
    })
    render(<BackupConfigForm />)
    expect(screen.getByLabelText('config.httpPassword')).toBeInTheDocument()
    expect(
      screen.queryByText('config.secretStoredLabel'),
    ).not.toBeInTheDocument()
  })

  it('does not render an editable, autofillable SSH key field when a key is already stored', () => {
    storeState = makeState({
      config: {
        ...baseConfig,
        remoteUrl: 'git@github.com:acme/wiki.git',
        hasSshKey: true,
      },
    })
    render(<BackupConfigForm />)
    expect(screen.queryByLabelText('config.sshKey')).not.toBeInTheDocument()
    expect(screen.getByText('config.secretStoredLabel')).toBeInTheDocument()

    fireEvent.click(
      screen.getByRole('button', { name: 'config.changeSecretButton' }),
    )
    expect(screen.getByLabelText('config.sshKey')).toHaveValue('')
  })

  it('saving without touching a stored HTTP password submits a blank value so the backend keeps it', async () => {
    const saveConfig = vi.fn().mockResolvedValue(undefined)
    storeState = makeState({
      config: { ...baseConfig, remoteUrl: 'https://example.com/wiki.git' },
      saveConfig,
    })
    render(<BackupConfigForm />)
    fireEvent.click(screen.getByRole('button', { name: 'config.saveButton' }))
    await vi.waitFor(() => expect(saveConfig).toHaveBeenCalled())
    expect(saveConfig.mock.calls[0][0]).toMatchObject({ httpPassword: '' })
  })
})
