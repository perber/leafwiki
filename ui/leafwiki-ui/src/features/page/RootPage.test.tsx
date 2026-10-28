import { render, screen } from '@testing-library/react'
import { describe, expect, it, vi } from 'vitest'
import RootPage from './RootPage'

type TreeNode = { path: string }
const treeState: { tree: { children: TreeNode[] } | null } = { tree: null }

vi.mock('@/stores/tree', () => ({
  useTreeStore: () => treeState,
}))

vi.mock('@/features/viewer/PageViewer', () => ({
  default: ({ routePath }: { routePath?: string }) => (
    <div data-testid="page-viewer">{routePath}</div>
  ),
}))

describe('RootPage', () => {
  it('renders the first page in place, without redirecting off "/"', () => {
    treeState.tree = { children: [{ path: 'home' }, { path: 'other' }] }

    render(<RootPage />)

    expect(screen.getByTestId('page-viewer')).toHaveTextContent('/home')
  })

  it('renders nothing until the tree is loaded', () => {
    treeState.tree = null

    const { container } = render(<RootPage />)

    expect(container).toBeEmptyDOMElement()
  })

  it('renders nothing when the wiki has no pages', () => {
    treeState.tree = { children: [] }

    const { container } = render(<RootPage />)

    expect(container).toBeEmptyDOMElement()
  })
})
