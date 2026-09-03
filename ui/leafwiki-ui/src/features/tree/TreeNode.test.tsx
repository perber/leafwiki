import { DIALOG_DELETE_PAGE_CONFIRMATION } from '@/lib/registries'
import type { PageNode } from '@/lib/api/pages'
import { useDialogsStore } from '@/stores/dialogs'
import { useTreeStore } from '@/stores/tree'
import { render, screen } from '@testing-library/react'
import type React from 'react'
import { MemoryRouter } from 'react-router'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { TreeNode } from './TreeNode'

vi.mock('react-i18next', () => ({
  initReactI18next: { type: '3rdParty', init: () => {} },
  useTranslation: () => ({ t: (key: string) => key }),
}))

vi.mock('@/components/TooltipWrapper', () => ({
  TooltipWrapper: ({ children }: { children: React.ReactNode }) => children,
}))

// The actions menu pulls in a large dependency tree of its own (dropdown
// primitives, favorites, page editor state, etc.) that this test doesn't
// care about -- it only asserts on the row's delete-target highlight.
vi.mock('./TreeNodeActionsMenu', () => ({
  default: () => <div />,
}))

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

const otherNode: PageNode = {
  ...node,
  id: 'page-2',
  title: 'Other Page',
  path: 'docs/other-page',
}

function renderNode(target: PageNode = node) {
  return render(
    <MemoryRouter>
      <TreeNode node={target} />
    </MemoryRouter>,
  )
}

describe('TreeNode delete-target highlight', () => {
  beforeEach(() => {
    window.matchMedia = vi.fn().mockImplementation(() => ({
      matches: false,
      media: '(max-width: 767px)',
      onchange: null,
      addEventListener: vi.fn(),
      removeEventListener: vi.fn(),
      addListener: vi.fn(),
      removeListener: vi.fn(),
      dispatchEvent: vi.fn(),
    }))

    useDialogsStore.setState({ dialogType: null, dialogProps: null })
    useTreeStore.setState({ activeNodeId: null, openNodeIdSet: {} })
  })

  it('has no delete-target styling when no dialog is open', () => {
    renderNode()

    const row = screen.getByTestId(`tree-node-${node.id}`)
    expect(row).not.toHaveAttribute('data-delete-target')
    expect(row.className).not.toContain('tree-node--delete-target')
  })

  it('stays highlighted while the delete confirmation dialog targets this node', () => {
    useDialogsStore.setState({
      dialogType: DIALOG_DELETE_PAGE_CONFIRMATION,
      dialogProps: { pageId: node.id, redirectTo: '/' },
    })

    renderNode()

    const row = screen.getByTestId(`tree-node-${node.id}`)
    expect(row).toHaveAttribute('data-delete-target', 'true')
    expect(row.className).toContain('tree-node--delete-target')
  })

  it('does not highlight a row when the dialog targets a different node', () => {
    useDialogsStore.setState({
      dialogType: DIALOG_DELETE_PAGE_CONFIRMATION,
      dialogProps: { pageId: otherNode.id, redirectTo: '/' },
    })

    renderNode(node)

    const row = screen.getByTestId(`tree-node-${node.id}`)
    expect(row).not.toHaveAttribute('data-delete-target')
    expect(row.className).not.toContain('tree-node--delete-target')
  })

  it('does not highlight the row once the dialog is closed again', () => {
    useDialogsStore.setState({
      dialogType: DIALOG_DELETE_PAGE_CONFIRMATION,
      dialogProps: { pageId: node.id, redirectTo: '/' },
    })
    const { rerender } = renderNode()
    expect(screen.getByTestId(`tree-node-${node.id}`)).toHaveAttribute(
      'data-delete-target',
      'true',
    )

    useDialogsStore.setState({ dialogType: null, dialogProps: null })
    rerender(
      <MemoryRouter>
        <TreeNode node={node} />
      </MemoryRouter>,
    )

    expect(screen.getByTestId(`tree-node-${node.id}`)).not.toHaveAttribute(
      'data-delete-target',
    )
  })
})
