import { DIALOG_PAGE_QUICK_SWITCHER } from '@/lib/registries'
import { useDialogsStore } from '@/stores/dialogs'
import { useTreeStore } from '@/stores/tree'
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { PageQuickSwitcherDialog } from './PageQuickSwitcherDialog'

const navigateMock = vi.fn()
vi.mock('react-router', () => ({
  useNavigate: () => navigateMock,
}))

describe('PageQuickSwitcherDialog', () => {
  const callOrder: string[] = []

  beforeEach(() => {
    vi.clearAllMocks()
    callOrder.length = 0

    navigateMock.mockImplementation(() => {
      callOrder.push('navigate')
    })

    useDialogsStore.setState({
      dialogType: DIALOG_PAGE_QUICK_SWITCHER,
      dialogProps: null,
      closeDialog: () => {
        callOrder.push('closeDialog')
        useDialogsStore.setState({ dialogType: null, dialogProps: null })
      },
    })

    useTreeStore.setState({
      openAncestorsForPath: vi.fn(),
      flatPages: [
        {
          id: 'page-1',
          title: 'Target Page',
          path: 'target-page',
          kind: 'page',
          breadcrumb: 'Target Page',
          searchText: 'target page',
          normalizedTitle: 'target page',
          normalizedPath: 'target-page',
          normalizedBreadcrumb: 'target page',
        },
      ],
    })
  })

  it('closes the dialog before navigating to the selected page', async () => {
    const user = userEvent.setup()
    render(<PageQuickSwitcherDialog />)

    await user.click(await screen.findByRole('option'))

    const firstCloseIndex = callOrder.indexOf('closeDialog')
    const navigateIndex = callOrder.indexOf('navigate')

    expect(firstCloseIndex).toBeGreaterThanOrEqual(0)
    expect(navigateIndex).toBeGreaterThanOrEqual(0)
    // Closing before navigating avoids a render race between the route
    // transition and the dialog's own close/exit-animation state, which
    // otherwise causes the dialog to visibly flash back open once (#flicker).
    expect(firstCloseIndex).toBeLessThan(navigateIndex)
  })
})
