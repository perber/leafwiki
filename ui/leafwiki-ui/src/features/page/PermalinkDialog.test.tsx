import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import type { ReactNode } from 'react'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { PermalinkDialog } from './PermalinkDialog'

vi.mock('@/components/BaseDialog', () => ({
  default: ({
    children,
    dialogDescription,
  }: {
    children?: ReactNode
    dialogDescription?: ReactNode
  }) => (
    <div>
      <div>{dialogDescription}</div>
      {children}
    </div>
  ),
}))

vi.mock('react-i18next', () => ({
  initReactI18next: { type: '3rdParty', init: () => {} },
  useTranslation: () => ({
    t: (key: string, opts?: { title?: string }) =>
      ({
        'permalinkDialog.description': `Permanent link to ${opts?.title ?? ''}`,
        'permalinkDialog.urlLabel': 'Permanent link',
        'permalinkDialog.copyLink': 'Copy link',
        'permalinkDialog.openLink': 'Open link',
        'permalinkDialog.purposeNote':
          'This is a permanent internal link. It keeps working even after the page is renamed or moved.',
        'permalinkDialog.accessNote':
          'It is not a public link. Anyone you send it to still needs to sign in to this wiki.',
        'permalinkDialog.copiedToast': 'Permalink copied',
        'permalinkDialog.copyErrorToast': 'Could not copy permalink',
      })[key] ?? key,
  }),
}))

const copyMock = vi.fn(() => true)
vi.mock('copy-to-clipboard', () => ({
  default: (text: string) => copyMock(text),
}))

vi.mock('sonner', () => ({
  toast: { success: vi.fn(), error: vi.fn() },
}))

const page = { id: 'abc123', slug: 'my-page', title: 'My Page' }

describe('PermalinkDialog', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('explains that the link is permanent and still requires sign-in', () => {
    render(<PermalinkDialog page={page} />)

    expect(
      screen.getByTestId('permalink-dialog-purpose-note').textContent,
    ).toMatch(/permanent internal link/i)
    expect(
      screen.getByTestId('permalink-dialog-access-note').textContent,
    ).toMatch(/not a public link/i)
  })

  it('copies the fully-qualified permalink URL', async () => {
    const user = userEvent.setup()
    render(<PermalinkDialog page={page} />)

    await user.click(screen.getByTestId('permalink-dialog-copy-button'))

    expect(copyMock).toHaveBeenCalledTimes(1)
    expect(copyMock.mock.calls[0][0]).toContain('/p/abc123/my-page')
  })
})
