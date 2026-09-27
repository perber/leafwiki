import { createPage } from '@/lib/api/pages'
import { DIALOG_ADD_PAGE } from '@/lib/registries'
import { useDialogsStore } from '@/stores/dialogs'
import { useTreeStore } from '@/stores/tree'
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { AddPageDialog } from './AddPageDialog'

const navigateMock = vi.fn()
vi.mock('react-router', () => ({
  useNavigate: () => navigateMock,
}))

vi.mock('@/lib/api/pages', async () => {
  const actual =
    await vi.importActual<typeof import('@/lib/api/pages')>('@/lib/api/pages')
  return {
    ...actual,
    createPage: vi.fn(),
  }
})

describe('AddPageDialog', () => {
  const callOrder: string[] = []

  beforeEach(() => {
    vi.clearAllMocks()
    callOrder.length = 0

    navigateMock.mockImplementation(() => {
      callOrder.push('navigate')
    })

    useDialogsStore.setState({
      dialogType: DIALOG_ADD_PAGE,
      dialogProps: null,
      closeDialog: () => {
        callOrder.push('closeDialog')
        useDialogsStore.setState({ dialogType: null, dialogProps: null })
      },
    })

    useTreeStore.setState({
      reloadTree: vi.fn().mockResolvedValue(undefined),
      getPathById: () => '',
    })

    vi.mocked(createPage).mockResolvedValue(
      undefined as unknown as ReturnType<typeof createPage>,
    )
  })

  it('closes the dialog before navigating to the newly created page', async () => {
    const user = userEvent.setup()
    render(<AddPageDialog parentId="" />)

    await user.type(screen.getByTestId('add-page-title-input'), 'My Page')
    await user.type(screen.getByTestId('add-page-slug-input'), 'my-page')

    await user.click(screen.getByTestId('add-page-dialog-button-confirm'))

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
