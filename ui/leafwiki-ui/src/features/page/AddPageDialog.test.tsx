import { createPage, NODE_KIND_SECTION } from '@/lib/api/pages'
import { normalizeHotkeyCombo } from '@/lib/hotkeys'
import { DIALOG_ADD_PAGE } from '@/lib/registries'
import { buildEditUrl } from '@/lib/routePath'
import { useDialogsStore } from '@/stores/dialogs'
import { useHotKeysStore } from '@/stores/hotkeys'
import { useTreeStore } from '@/stores/tree'
import { act, render, screen } from '@testing-library/react'
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
    useHotKeysStore.setState({ registeredHotkeys: {} })

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

  // The Enter key runs the dialog's 'dialog.confirm' hotkey, which the global
  // key handler in AppLayout dispatches; the test calls it the same way.
  async function pressEnter() {
    const hotkey = useHotKeysStore
      .getState()
      .registeredHotkeys[normalizeHotkeyCombo('Enter')].at(-1)!
    await act(async () => {
      await hotkey.action()
    })
  }

  // #1562
  it('creates a section on Enter without opening it, like the Create button', async () => {
    const user = userEvent.setup()
    render(<AddPageDialog parentId="" nodeKind={NODE_KIND_SECTION} />)

    await user.type(screen.getByTestId('add-page-title-input'), 'My Section')
    await user.type(screen.getByTestId('add-page-slug-input'), 'my-section')
    await pressEnter()

    expect(createPage).toHaveBeenCalledWith(
      expect.objectContaining({ slug: 'my-section', kind: NODE_KIND_SECTION }),
    )
    expect(callOrder).toContain('closeDialog')
    expect(navigateMock).not.toHaveBeenCalled()
  })

  it('creates a page on Enter and opens it in the editor', async () => {
    const user = userEvent.setup()
    render(<AddPageDialog parentId="" />)

    await user.type(screen.getByTestId('add-page-title-input'), 'My Page')
    await user.type(screen.getByTestId('add-page-slug-input'), 'my-page')
    await pressEnter()

    expect(navigateMock).toHaveBeenCalledWith(buildEditUrl('my-page'))
  })
})
