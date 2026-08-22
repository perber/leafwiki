import '@/lib/i18n'
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { MemoryRouter, Route, Routes } from 'react-router'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import ForgotPasswordForm from './ForgotPasswordForm'

const requestPasswordResetMock = vi.fn()

vi.mock('@/lib/api/auth', () => ({
  requestPasswordReset: (...args: unknown[]) =>
    requestPasswordResetMock(...args),
}))

function renderForm() {
  return render(
    <MemoryRouter initialEntries={['/forgot-password']}>
      <Routes>
        <Route path="/forgot-password" element={<ForgotPasswordForm />} />
        <Route path="/login" element={<div>Login page</div>} />
      </Routes>
    </MemoryRouter>,
  )
}

describe('ForgotPasswordForm', () => {
  beforeEach(() => {
    requestPasswordResetMock.mockReset()
  })

  it('shows the same generic confirmation for a successful request', async () => {
    requestPasswordResetMock.mockResolvedValue(undefined)
    const user = userEvent.setup()
    renderForm()

    await user.type(screen.getByTestId('forgot-password-identifier'), 'alice')
    await user.click(screen.getByTestId('forgot-password-submit'))

    await waitFor(() =>
      expect(screen.getByText('Check your email')).toBeInTheDocument(),
    )
    expect(requestPasswordResetMock).toHaveBeenCalledWith('alice')
  })

  // The backend's enumeration protection is worthless if the frontend
  // reacts differently to a network/server error than to success — both
  // must land on the identical generic confirmation.
  it('shows the same generic confirmation even when the request fails', async () => {
    requestPasswordResetMock.mockRejectedValue(new Error('boom'))
    const user = userEvent.setup()
    renderForm()

    await user.type(
      screen.getByTestId('forgot-password-identifier'),
      'no-such-user',
    )
    await user.click(screen.getByTestId('forgot-password-submit'))

    await waitFor(() =>
      expect(screen.getByText('Check your email')).toBeInTheDocument(),
    )
  })
})
