import { DIALOG_PAGE_REFACTOR_CONFIRMATION } from '@/lib/registries'
import { useDialogsStore } from '@/stores/dialogs'
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { PageRefactorDialog } from './PageRefactorDialog'

const preview = {
  kind: 'rename' as const,
  pageId: 'page-1',
  oldPath: '/docs/alpha',
  newPath: '/docs/beta',
  affectedPages: [],
  counts: {
    affectedPages: 1,
    matchedLinks: 2,
  },
  warnings: [],
}

describe('PageRefactorDialog', () => {
  beforeEach(() => {
    useDialogsStore.setState({
      dialogType: DIALOG_PAGE_REFACTOR_CONFIRMATION,
      dialogProps: null,
    })
  })

  it('lets the editor save without rewriting links when enabled', async () => {
    const user = userEvent.setup()
    const onResolve = vi.fn()

    render(
      <PageRefactorDialog
        preview={preview}
        onResolve={onResolve}
        allowSkipRewrite
      />,
    )

    await user.click(
      screen.getByTestId('page-refactor-dialog-button-save-without-rewrite'),
    )

    expect(onResolve).toHaveBeenCalledWith(false)
  })

  it('keeps cancel as an abort path by default', async () => {
    const user = userEvent.setup()
    const onResolve = vi.fn()

    render(<PageRefactorDialog preview={preview} onResolve={onResolve} />)

    expect(
      screen.queryByTestId('page-refactor-dialog-button-save-without-rewrite'),
    ).not.toBeInTheDocument()

    await user.click(screen.getByTestId('page-refactor-dialog-button-cancel'))

    expect(onResolve).toHaveBeenCalledWith(null)
  })
})
