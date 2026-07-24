import '@/lib/i18n'
import { DIALOG_TOTP_DISABLE } from '@/lib/registries'
import { useDialogsStore } from '@/stores/dialogs'
import { useSessionStore } from '@/stores/session'
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { TOTPDisableDialog } from './TOTPDisableDialog'

vi.mock('sonner', () => ({
  toast: { error: vi.fn(), success: vi.fn() },
}))

vi.mock('@/lib/api/totp', () => ({
  startTOTPSetup: vi.fn(),
  confirmTOTPSetup: vi.fn(),
  disableTOTP: vi.fn(),
  getTOTPStatus: vi.fn(),
}))

import { toast } from 'sonner'
import * as totpAPI from '@/lib/api/totp'

const sessionUser = {
  id: 'user-1',
  username: 'admin',
  email: 'admin@example.com',
  role: 'admin' as const,
  totpEnabled: true,
}

describe('TOTPDisableDialog', () => {
  beforeEach(() => {
    useDialogsStore.setState({
      dialogType: DIALOG_TOTP_DISABLE,
      dialogProps: null,
    })
    useSessionStore.setState({
      user: sessionUser,
      isRefreshing: false,
      accessTokenExpiresAt: null,
    })
    vi.clearAllMocks()
  })

  it('disables TOTP with a valid password and code, closing the dialog', async () => {
    const user = userEvent.setup()
    ;(totpAPI.disableTOTP as ReturnType<typeof vi.fn>).mockResolvedValue(
      undefined,
    )

    render(<TOTPDisableDialog />)

    await user.type(
      screen.getByTestId('totp-disable-password'),
      'current-password',
    )
    await user.type(screen.getByTestId('totp-disable-code'), '123456')
    await user.click(screen.getByTestId('totp-disable-dialog-button-confirm'))

    await waitFor(() =>
      expect(totpAPI.disableTOTP).toHaveBeenCalledWith(
        'current-password',
        '123456',
      ),
    )
    expect(useSessionStore.getState().user?.totpEnabled).toBe(false)
    expect(toast.success).toHaveBeenCalled()
  })

  it('accepts a recovery code in place of a TOTP code', async () => {
    const user = userEvent.setup()
    ;(totpAPI.disableTOTP as ReturnType<typeof vi.fn>).mockResolvedValue(
      undefined,
    )

    render(<TOTPDisableDialog />)

    await user.type(
      screen.getByTestId('totp-disable-password'),
      'current-password',
    )
    await user.type(screen.getByTestId('totp-disable-code'), 'AB12-CD34')
    await user.click(screen.getByTestId('totp-disable-dialog-button-confirm'))

    await waitFor(() =>
      expect(totpAPI.disableTOTP).toHaveBeenCalledWith(
        'current-password',
        'AB12-CD34',
      ),
    )
  })

  it('clears both fields and shows the same error on both when the password is wrong', async () => {
    const user = userEvent.setup()
    ;(totpAPI.disableTOTP as ReturnType<typeof vi.fn>).mockRejectedValue(
      new Error('wrong password'),
    )

    render(<TOTPDisableDialog />)

    const passwordInput = screen.getByTestId('totp-disable-password')
    const codeInput = screen.getByTestId('totp-disable-code')
    await user.type(passwordInput, 'wrong-password')
    await user.type(codeInput, '123456')
    await user.click(screen.getByTestId('totp-disable-dialog-button-confirm'))

    await waitFor(() => expect(totpAPI.disableTOTP).toHaveBeenCalled())
    expect(passwordInput).toHaveValue('')
    expect(codeInput).toHaveValue('')
    expect(useSessionStore.getState().user?.totpEnabled).toBe(true)
  })

  it('clears both fields when the code is wrong, keeping TOTP enabled', async () => {
    const user = userEvent.setup()
    ;(totpAPI.disableTOTP as ReturnType<typeof vi.fn>).mockRejectedValue(
      new Error('wrong code'),
    )

    render(<TOTPDisableDialog />)

    const passwordInput = screen.getByTestId('totp-disable-password')
    const codeInput = screen.getByTestId('totp-disable-code')
    await user.type(passwordInput, 'current-password')
    await user.type(codeInput, '000000')
    await user.click(screen.getByTestId('totp-disable-dialog-button-confirm'))

    await waitFor(() => expect(totpAPI.disableTOTP).toHaveBeenCalled())
    expect(passwordInput).toHaveValue('')
    expect(codeInput).toHaveValue('')
    expect(useSessionStore.getState().user?.totpEnabled).toBe(true)
  })

  it('disables the confirm button until both password and code are filled in', async () => {
    const user = userEvent.setup()
    render(<TOTPDisableDialog />)

    expect(
      screen.getByTestId('totp-disable-dialog-button-confirm'),
    ).toBeDisabled()

    await user.type(screen.getByTestId('totp-disable-password'), 'pw')
    expect(
      screen.getByTestId('totp-disable-dialog-button-confirm'),
    ).toBeDisabled()

    await user.type(screen.getByTestId('totp-disable-code'), '123456')
    expect(
      screen.getByTestId('totp-disable-dialog-button-confirm'),
    ).not.toBeDisabled()
  })
})
