import '@/lib/i18n'
import { useSessionStore } from '@/stores/session'
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { TotpPanel } from './TotpPanel'

vi.mock('sonner', () => ({
  toast: { error: vi.fn(), success: vi.fn() },
}))

vi.mock('@/lib/api/totp', () => ({
  startTOTPSetup: vi.fn(),
  confirmTOTPSetup: vi.fn(),
  disableTOTP: vi.fn(),
  getTOTPStatus: vi.fn(),
}))

vi.mock('qrcode.react', () => ({
  QRCodeSVG: () => <div data-testid="totp-setup-qr" />,
}))

import { toast } from 'sonner'
import * as totpAPI from '@/lib/api/totp'

const baseUser = {
  id: 'user-1',
  username: 'admin',
  email: 'admin@example.com',
  role: 'admin' as const,
}

beforeEach(() => {
  vi.clearAllMocks()
})

describe('TotpPanel — setup flow (totpEnabled: false)', () => {
  beforeEach(() => {
    useSessionStore.setState({
      user: { ...baseUser, totpEnabled: false },
      isRefreshing: false,
      accessTokenExpiresAt: null,
    })
  })

  it('walks through password, verify and recovery steps, then enables TOTP', async () => {
    const user = userEvent.setup({ delay: null })
    ;(totpAPI.startTOTPSetup as ReturnType<typeof vi.fn>).mockResolvedValue({
      secret: 'JBSWY3DPEHPK3PXP',
      otpAuthUrl: 'otpauth://totp/LeafWiki:admin?secret=JBSWY3DPEHPK3PXP',
    })
    ;(totpAPI.confirmTOTPSetup as ReturnType<typeof vi.fn>).mockResolvedValue({
      recoveryCodes: ['AB12-CD34', 'EF56-GH78'],
    })

    render(<TotpPanel />)

    await user.type(
      screen.getByTestId('totp-setup-password'),
      'current-password',
    )
    await user.click(screen.getByTestId('totp-setup-continue'))

    await waitFor(() =>
      expect(totpAPI.startTOTPSetup).toHaveBeenCalledWith('current-password'),
    )
    expect(
      await screen.findByTestId('totp-setup-manual-key'),
    ).toHaveTextContent('JBSWY3DPEHPK3PXP')

    await user.type(screen.getByTestId('totp-setup-code'), '654321')
    await user.click(screen.getByTestId('totp-setup-enable'))

    await waitFor(() =>
      expect(totpAPI.confirmTOTPSetup).toHaveBeenCalledWith('654321'),
    )
    expect(
      await screen.findByTestId('totp-setup-recovery-codes'),
    ).toHaveTextContent('AB12-CD34')
    expect(useSessionStore.getState().user?.totpEnabled).toBe(true)
    expect(toast.success).toHaveBeenCalled()
  })

  it('shows a field error and clears the password on a wrong password, staying on step 1', async () => {
    const user = userEvent.setup({ delay: null })
    ;(totpAPI.startTOTPSetup as ReturnType<typeof vi.fn>).mockRejectedValue(
      new Error('wrong password'),
    )

    render(<TotpPanel />)

    const passwordInput = screen.getByTestId('totp-setup-password')
    await user.type(passwordInput, 'wrong-password')
    await user.click(screen.getByTestId('totp-setup-continue'))

    await waitFor(() => expect(totpAPI.startTOTPSetup).toHaveBeenCalled())
    expect(passwordInput).toHaveValue('')
    expect(
      screen.queryByTestId('totp-setup-manual-key'),
    ).not.toBeInTheDocument()
    expect(useSessionStore.getState().user?.totpEnabled).toBe(false)
  })

  it('disables the continue/enable button until the current step has the required input', async () => {
    const user = userEvent.setup({ delay: null })
    ;(totpAPI.startTOTPSetup as ReturnType<typeof vi.fn>).mockResolvedValue({
      secret: 'JBSWY3DPEHPK3PXP',
      otpAuthUrl: 'otpauth://totp/LeafWiki:admin?secret=JBSWY3DPEHPK3PXP',
    })

    render(<TotpPanel />)

    expect(screen.getByTestId('totp-setup-continue')).toBeDisabled()
    await user.type(
      screen.getByTestId('totp-setup-password'),
      'current-password',
    )
    expect(screen.getByTestId('totp-setup-continue')).not.toBeDisabled()

    await user.click(screen.getByTestId('totp-setup-continue'))
    await screen.findByTestId('totp-setup-manual-key')
    expect(screen.getByTestId('totp-setup-enable')).toBeDisabled()
  })
})

describe('TotpPanel — disable flow (totpEnabled: true)', () => {
  beforeEach(() => {
    useSessionStore.setState({
      user: { ...baseUser, totpEnabled: true },
      isRefreshing: false,
      accessTokenExpiresAt: null,
    })
  })

  it('disables TOTP with a valid password and code', async () => {
    const user = userEvent.setup({ delay: null })
    ;(totpAPI.disableTOTP as ReturnType<typeof vi.fn>).mockResolvedValue(
      undefined,
    )

    render(<TotpPanel />)

    await user.type(
      screen.getByTestId('totp-disable-password'),
      'current-password',
    )
    await user.type(screen.getByTestId('totp-disable-code'), '123456')
    await user.click(screen.getByTestId('totp-disable-confirm'))

    await waitFor(() =>
      expect(totpAPI.disableTOTP).toHaveBeenCalledWith(
        'current-password',
        '123456',
      ),
    )
    expect(useSessionStore.getState().user?.totpEnabled).toBe(false)
    expect(toast.success).toHaveBeenCalled()
  })

  it('clears both fields and keeps TOTP enabled when the password is wrong', async () => {
    const user = userEvent.setup({ delay: null })
    ;(totpAPI.disableTOTP as ReturnType<typeof vi.fn>).mockRejectedValue(
      new Error('wrong password'),
    )

    render(<TotpPanel />)

    const passwordInput = screen.getByTestId('totp-disable-password')
    const codeInput = screen.getByTestId('totp-disable-code')
    await user.type(passwordInput, 'wrong-password')
    await user.type(codeInput, '123456')
    await user.click(screen.getByTestId('totp-disable-confirm'))

    await waitFor(() => expect(totpAPI.disableTOTP).toHaveBeenCalled())
    expect(passwordInput).toHaveValue('')
    expect(codeInput).toHaveValue('')
    expect(useSessionStore.getState().user?.totpEnabled).toBe(true)
  })

  it('disables the confirm button until both password and code are filled in', async () => {
    const user = userEvent.setup({ delay: null })
    render(<TotpPanel />)

    expect(screen.getByTestId('totp-disable-confirm')).toBeDisabled()

    await user.type(screen.getByTestId('totp-disable-password'), 'pw')
    expect(screen.getByTestId('totp-disable-confirm')).toBeDisabled()

    await user.type(screen.getByTestId('totp-disable-code'), '123456')
    expect(screen.getByTestId('totp-disable-confirm')).not.toBeDisabled()
  })
})
