import { useDialogsStore } from '@/stores/dialogs'
import { usePageEditorStore } from '@/features/editor/pageEditorStore'
import { DIALOG_EDIT_PAGE_METADATA } from '@/lib/registries'
import type { PageNode } from '@/lib/api/pages'
import { fireEvent, render, screen } from '@testing-library/react'
import type React from 'react'
import { MemoryRouter } from 'react-router-dom'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import TreeNodeActionsMenu from './TreeNodeActionsMenu'
import { useTreeNodeActionsMenusStore } from './treeNodeActionsMenus'

vi.mock('@/components/TooltipWrapper', () => ({
  TooltipWrapper: ({ children }: { children: React.ReactNode }) => children,
}))

vi.mock('@/components/ui/dropdown-menu', () => ({
  DropdownMenu: ({ children }: { children: React.ReactNode }) => (
    <div>{children}</div>
  ),
  DropdownMenuContent: ({ children }: { children: React.ReactNode }) => (
    <div>{children}</div>
  ),
  DropdownMenuItem: ({
    children,
    onClick,
    className,
    'data-testid': testId,
  }: {
    children: React.ReactNode
    onClick?: () => void
    className?: string
    'data-testid'?: string
  }) => (
    <button className={className} data-testid={testId} onClick={onClick}>
      {children}
    </button>
  ),
  DropdownMenuSeparator: () => <hr />,
  DropdownMenuTrigger: ({ children }: { children: React.ReactNode }) => (
    <>{children}</>
  ),
}))

vi.mock('react-i18next', () => ({
  initReactI18next: {
    type: '3rdParty',
    init: () => {},
  },
  useTranslation: () => ({ t: (key: string) => key }),
}))

vi.mock('@/lib/api/pages', async () => {
  const actual =
    await vi.importActual<typeof import('@/lib/api/pages')>('@/lib/api/pages')
  return {
    ...actual,
    applyPageRefactor: vi.fn(),
    convertPage: vi.fn(),
    getPageByPath: vi.fn(),
    pinPage: vi.fn(),
    previewPageRefactor: vi.fn(),
    updatePage: vi.fn(),
  }
})

const node: PageNode = {
  id: 'page-1',
  title: 'Getting Started',
  slug: 'getting-started',
  path: 'docs/getting-started',
  version: 'v1',
  parentId: 'docs',
  children: null,
  kind: 'page',
}

describe('TreeNodeActionsMenu', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    useDialogsStore.setState({ dialogType: null, dialogProps: null })
    usePageEditorStore.setState({ page: null })
    useTreeNodeActionsMenusStore.setState({ openMenuNodeId: node.id })
  })

  it('opens the metadata dialog for renaming a tree node', () => {
    render(
      <MemoryRouter>
        <TreeNodeActionsMenu node={node} />
      </MemoryRouter>,
    )

    fireEvent.click(screen.getByTestId('tree-view-action-button-rename'))

    const dialogState = useDialogsStore.getState()

    expect(dialogState.dialogType).toBe(DIALOG_EDIT_PAGE_METADATA)
    expect(dialogState.dialogProps).toMatchObject({
      parentId: node.parentId,
      currentId: node.id,
      itemKind: node.kind,
      title: node.title,
      slug: node.slug,
    })
    expect(dialogState.dialogProps?.onChange).toEqual(expect.any(Function))
  })
})
