import type { PageNode } from '@/lib/api/pages'
import { useTreeStore } from '@/stores/tree'
import { fireEvent, render, screen } from '@testing-library/react'
import { MemoryRouter } from 'react-router'
import { beforeEach, describe, expect, it, vi } from 'vitest'

const dnd = vi.hoisted(() => ({ useDraggable: vi.fn(), useDroppable: vi.fn() }))

vi.mock('@dnd-kit/core', () => ({
  useDraggable: dnd.useDraggable,
  useDroppable: dnd.useDroppable,
}))
vi.mock('@/lib/useIsMobile', () => ({ useIsMobile: () => false }))
vi.mock('@/lib/useIsReadOnly', () => ({ useIsReadOnly: () => false }))
vi.mock('react-i18next', () => ({
  initReactI18next: { type: '3rdParty', init: () => {} },
  useTranslation: () => ({ t: (key: string) => key }),
}))
vi.mock('./TreeNodeActionsMenu', () => ({ default: () => <span>menu</span> }))
vi.mock('./TreeViewActionButton', () => ({
  TreeViewActionButton: ({ actionName }: { actionName: string }) => (
    <button data-testid={`tree-view-action-button-${actionName}`} />
  ),
}))

import { TreeNode } from './TreeNode'

const pending: PageNode = {
  id: 'pending-1',
  title: 'Pending',
  slug: 'pending',
  path: 'published/pending',
  version: 'pending',
  kind: 'page',
  children: [],
  draft: 'pending',
}

describe('TreeNode draft overlay', () => {
  beforeEach(() => {
    dnd.useDraggable.mockReturnValue({ setNodeRef: vi.fn(), listeners: {} })
    dnd.useDroppable.mockReturnValue({ setNodeRef: vi.fn() })
    useTreeStore.setState({
      openNodeIdSet: {},
      activeNodeId: null,
    })
  })

  it('keeps a leaf parent expandable and renders pending rows as private non-draggable links', () => {
    const parent = {
      ...pending,
      id: 'page-1',
      title: 'Published',
      slug: 'published',
      path: 'published',
      draft: undefined,
      children: [pending],
    }
    render(
      <MemoryRouter>
        <TreeNode node={parent} />
      </MemoryRouter>,
    )

    expect(
      screen.getByTestId('tree-node-toggle-icon-page-1'),
    ).toBeInTheDocument()
    fireEvent.click(screen.getByTestId('tree-node-toggle-icon-page-1'))
    expect(screen.getByTestId('tree-node-link-pending-1')).toHaveAttribute(
      'href',
      '/pending-drafts/pending-1/edit',
    )
    expect(
      screen.getByLabelText('treeActions.pendingDraftMarker'),
    ).toBeInTheDocument()
    expect(screen.queryByText('menu')).not.toBeInTheDocument()
    expect(dnd.useDraggable).toHaveBeenCalledWith(
      expect.objectContaining({ id: 'pending-1', disabled: true }),
    )
    expect(dnd.useDroppable).toHaveBeenCalledWith(
      expect.objectContaining({ id: 'pending-1', disabled: true }),
    )
  })
})
