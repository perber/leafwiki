import '@/lib/i18n'
import { DIALOG_TOTP_SETUP } from '@/lib/registries'
import { useDialogsStore } from '@/stores/dialogs'
import { useSessionStore } from '@/stores/session'
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { TOTPSetupDialog } from './TOTPSetupDialog'

vi.mock('sonner', () => ({
  toast: { error: vi.fn(), success: vi.fn() },
}))

vi.mock('@/lib/api/totp', () => ({
  startTOTPSetup: vi.fn(),
  confirmTOTPSetup: vi.fn(),
  disableTOTP: vi.fn(),
  getTOTPStatus: vi.fn(),
}))

// qrcode.react renders an actual <canvas>/<svg> via a library that isn't
// relevant to this component's behavior; stub it to keep the test focused.
vi.mock('qrcode.react', () => ({
  QRCodeSVG: () => <div data-testid="totp-setup-qr" />,
}))

import { toast } from 'sonner'
import * as totpAPI from '@/lib/api/totp'

const sessionUser = {
  id: 'user-1',
  username: 'admin',
  email: 'admin@example.com',
  role: 'admin' as const,
  totpEnabled: false,
}

describe('TOTPSetupDialog', () => {
  beforeEach(() => {
    useDialogsStore.setState({
      dialogType: DIALOG_TOTP_SETUP,
      dialogProps: null,
    })
    useSessionStore.setState({
      user: sessionUser,
      isRefreshing: false,
      accessTokenExpiresAt: null,
    })
    vi.clearAllMocks()
  })

  it('walks through password, verify and recovery steps, then enables TOTP', async () => {
    const user = userEvent.setup()
    ;(totpAPI.startTOTPSetup as ReturnType<typeof vi.fn>).mockResolvedValue({
      secret: 'JBSWY3DPEHPK3PXP',
      otpAuthUrl: 'otpauth://totp/LeafWiki:admin?secret=JBSWY3DPEHPK3PXP',
    })
    ;(totpAPI.confirmTOTPSetup as ReturnType<typeof vi.fn>).mockResolvedValue({
      recoveryCodes: ['AB12-CD34', 'EF56-GH78'],
    })

    render(<TOTPSetupDialog />)

    await user.type(
      screen.getByTestId('totp-setup-password'),
      'current-password',
    )
    await user.click(screen.getByTestId('totp-setup-dialog-button-confirm'))

    await waitFor(() =>
      expect(totpAPI.startTOTPSetup).toHaveBeenCalledWith('current-password'),
    )
    expect(
      await screen.findByTestId('totp-setup-manual-key'),
    ).toHaveTextContent('JBSWY3DPEHPK3PXP')

    await user.type(screen.getByTestId('totp-setup-code'), '654321')
    await user.click(screen.getByTestId('totp-setup-dialog-button-confirm'))

    await waitFor(() =>
      expect(totpAPI.confirmTOTPSetup).toHaveBeenCalledWith('654321'),
    )
    expect(
      await screen.findByTestId('totp-setup-recovery-codes'),
    ).toHaveTextContent('AB12-CD34')
    expect(useSessionStore.getState().user?.totpEnabled).toBe(true)
    expect(toast.success).toHaveBeenCalled()

    // Dialog closes on "Done" without another API call.
    await user.click(screen.getByTestId('totp-setup-dialog-button-confirm'))
    expect(totpAPI.startTOTPSetup).toHaveBeenCalledTimes(1)
    expect(totpAPI.confirmTOTPSetup).toHaveBeenCalledTimes(1)
  })

  it('shows a field error and clears the password on a wrong password, staying on step 1', async () => {
    const user = userEvent.setup()
    ;(totpAPI.startTOTPSetup as ReturnType<typeof vi.fn>).mockRejectedValue(
      new Error('wrong password'),
    )

    render(<TOTPSetupDialog />)

    const passwordInput = screen.getByTestId('totp-setup-password')
    await user.type(passwordInput, 'wrong-password')
    await user.click(screen.getByTestId('totp-setup-dialog-button-confirm'))

    await waitFor(() => expect(totpAPI.startTOTPSetup).toHaveBeenCalled())
    expect(passwordInput).toHaveValue('')
    expect(
      screen.queryByTestId('totp-setup-manual-key'),
    ).not.toBeInTheDocument()
    expect(useSessionStore.getState().user?.totpEnabled).toBe(false)
  })

  it('shows a field error and clears the code on a wrong verification code, staying on step 2', async () => {
    const user = userEvent.setup()
    ;(totpAPI.startTOTPSetup as ReturnType<typeof vi.fn>).mockResolvedValue({
      secret: 'JBSWY3DPEHPK3PXP',
      otpAuthUrl: 'otpauth://totp/LeafWiki:admin?secret=JBSWY3DPEHPK3PXP',
    })
    ;(totpAPI.confirmTOTPSetup as ReturnType<typeof vi.fn>).mockRejectedValue(
      new Error('wrong code'),
    )

    render(<TOTPSetupDialog />)

    await user.type(
      screen.getByTestId('totp-setup-password'),
      'current-password',
    )
    await user.click(screen.getByTestId('totp-setup-dialog-button-confirm'))
    await screen.findByTestId('totp-setup-manual-key')

    const codeInput = screen.getByTestId('totp-setup-code')
    await user.type(codeInput, '000000')
    await user.click(screen.getByTestId('totp-setup-dialog-button-confirm'))

    await waitFor(() => expect(totpAPI.confirmTOTPSetup).toHaveBeenCalled())
    expect(codeInput).toHaveValue('')
    expect(
      screen.queryByTestId('totp-setup-recovery-codes'),
    ).not.toBeInTheDocument()
    expect(useSessionStore.getState().user?.totpEnabled).toBe(false)
  })

  it('disables the confirm button until the current step has the required input', async () => {
    const user = userEvent.setup()
    ;(totpAPI.startTOTPSetup as ReturnType<typeof vi.fn>).mockResolvedValue({
      secret: 'JBSWY3DPEHPK3PXP',
      otpAuthUrl: 'otpauth://totp/LeafWiki:admin?secret=JBSWY3DPEHPK3PXP',
    })

    render(<TOTPSetupDialog />)

    expect(
      screen.getByTestId('totp-setup-dialog-button-confirm'),
    ).toBeDisabled()
    await user.type(
      screen.getByTestId('totp-setup-password'),
      'current-password',
    )
    expect(
      screen.getByTestId('totp-setup-dialog-button-confirm'),
    ).not.toBeDisabled()

    await user.click(screen.getByTestId('totp-setup-dialog-button-confirm'))
    await screen.findByTestId('totp-setup-manual-key')
    expect(
      screen.getByTestId('totp-setup-dialog-button-confirm'),
    ).toBeDisabled()
  })
})
