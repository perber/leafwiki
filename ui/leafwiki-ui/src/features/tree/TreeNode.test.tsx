import { DIALOG_DELETE_PAGE_CONFIRMATION } from '@/lib/registries'
import type { PageNode } from '@/lib/api/pages'
import { useDialogsStore } from '@/stores/dialogs'
import { useTreeStore } from '@/stores/tree'
import { DEFAULT_PAGE_DISPLAY } from '@/lib/pageDisplay'
import { useConfigStore } from '@/stores/config'
import { render, screen, within } from '@testing-library/react'
import type React from 'react'
import { MemoryRouter } from 'react-router'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { TreeNode } from './TreeNode'

vi.mock('react-i18next', () => ({
  initReactI18next: { type: '3rdParty', init: () => {} },
  useTranslation: () => ({
    t: (key: string, opts?: { count?: number }) =>
      opts?.count === undefined ? key : `${key}:${opts.count}`,
  }),
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
  useConfigStore.setState({ pageDisplay: DEFAULT_PAGE_DISPLAY })
})

describe('TreeNode delete-target highlight', () => {
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

describe('TreeNode subpage count', () => {
  const child = (id: string): PageNode => ({
    ...node,
    id,
    path: `docs/${id}`,
    parentId: 'section-1',
  })
  const section = (children: PageNode[] | null): PageNode => ({
    ...node,
    id: 'section-1',
    title: 'Guides',
    path: 'docs/guides',
    kind: 'section',
    children,
  })

  it.each([
    {
      name: 'a section with subpages when the setting is off',
      showSubpageCount: false,
      target: section([child('a'), child('b')]),
    },
    {
      name: 'a page when the setting is on',
      showSubpageCount: true,
      target: node,
    },
  ])('shows no count for $name', ({ showSubpageCount, target }) => {
    useConfigStore.setState({
      pageDisplay: { ...DEFAULT_PAGE_DISPLAY, showSubpageCount },
    })
    renderNode(target)

    expect(
      screen.queryByTestId(`tree-node-subpage-count-${target.id}`),
    ).not.toBeInTheDocument()
  })

  it.each([
    { name: 'two subpages', children: [child('a'), child('b')], want: 2 },
    { name: 'no children array', children: null, want: 0 },
    { name: 'an empty children array', children: [], want: 0 },
  ])(
    'shows the direct subpage count inside the link for $name',
    ({ children, want }) => {
      useConfigStore.setState({
        pageDisplay: { ...DEFAULT_PAGE_DISPLAY, showSubpageCount: true },
      })
      const target = section(children)
      renderNode(target)

      const link = screen.getByTestId(`tree-node-link-${target.id}`)
      const count = within(link).getByTestId(
        `tree-node-subpage-count-${target.id}`,
      )
      expect(within(count).getByText(String(want))).toHaveAttribute(
        'aria-hidden',
        'true',
      )
      expect(
        within(count).getByText(`section.subpageCount:${want}`),
      ).toHaveClass('sr-only')
    },
  )

  it('counts only direct children, not grandchildren', () => {
    useConfigStore.setState({
      pageDisplay: { ...DEFAULT_PAGE_DISPLAY, showSubpageCount: true },
    })
    const nested: PageNode = {
      ...child('nested'),
      kind: 'section',
      children: [child('x'), child('y'), child('z')],
    }
    const target = section([nested])
    renderNode(target)

    expect(
      within(
        screen.getByTestId(`tree-node-subpage-count-${target.id}`),
      ).getByText('1'),
    ).toBeInTheDocument()
  })
})
