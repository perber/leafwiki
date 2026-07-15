import { fetchWithAuth } from './auth'

export type TOTPSetupStart = {
  secret: string
  otpAuthUrl: string
}

export type TOTPSetupConfirm = {
  recoveryCodes: string[]
}

export type TOTPStatus = {
  enabled: boolean
  recoveryCodesRemaining: number
}

export async function startTOTPSetup(
  currentPassword: string,
): Promise<TOTPSetupStart> {
  return (await fetchWithAuth('/api/users/me/totp/setup/start', {
    method: 'POST',
    body: JSON.stringify({ currentPassword }),
  })) as TOTPSetupStart
}

export async function confirmTOTPSetup(
  code: string,
): Promise<TOTPSetupConfirm> {
  return (await fetchWithAuth('/api/users/me/totp/setup/confirm', {
    method: 'POST',
    body: JSON.stringify({ code }),
  })) as TOTPSetupConfirm
}

export async function disableTOTP(
  currentPassword: string,
  code: string,
): Promise<void> {
  await fetchWithAuth('/api/users/me/totp/disable', {
    method: 'POST',
    body: JSON.stringify({ currentPassword, code }),
  })
}

export async function getTOTPStatus(): Promise<TOTPStatus> {
  return (await fetchWithAuth('/api/users/me/totp/status')) as TOTPStatus
}
