import '@/lib/i18n'
import { User } from '@/lib/api/users'
import { useSessionStore } from '@/stores/session'
import { render, screen } from '@testing-library/react'
import { beforeEach, describe, expect, it } from 'vitest'
import { ChangePasswordButton } from './ChangePasswordButton'

const admin: User = {
  id: 'admin-id',
  username: 'admin',
  email: 'admin@example.com',
  role: 'admin',
  totpEnabled: false,
  mustSetPassword: false,
}

const other: User = {
  id: 'other-id',
  username: 'jane',
  email: 'jane@example.com',
  role: 'editor',
  totpEnabled: false,
  mustSetPassword: false,
}

describe('ChangePasswordButton', () => {
  beforeEach(() => {
    useSessionStore.setState({ user: admin })
  })

  it('renders for another user', () => {
    render(<ChangePasswordButton user={other} />)
    expect(screen.getByRole('button')).toBeInTheDocument()
  })

  it('does not render for the current user (own password is changed in account settings)', () => {
    render(<ChangePasswordButton user={admin} />)
    expect(screen.queryByRole('button')).not.toBeInTheDocument()
  })
})
