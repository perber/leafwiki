import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import type { ReactNode } from 'react'
import { beforeEach, describe, expect, it, vi } from 'vitest'

const navigate = vi.fn()
const reloadTree = vi.fn().mockResolvedValue(undefined)
const publishDraft = vi.fn()
const discardDraft = vi.fn()
const getPathById = vi.fn()

vi.mock('react-router', () => ({
  useLocation: () => ({ pathname: '/pending-drafts/pending-1/edit' }),
  useNavigate: () => navigate,
  useParams: () => ({}),
}))
vi.mock('react-i18next', () => ({
  initReactI18next: { type: '3rdParty', init: () => {} },
  useTranslation: () => ({ t: (key: string) => key }),
}))
vi.mock('sonner', () => ({ toast: { success: vi.fn(), error: vi.fn() } }))
vi.mock('@/stores/tree', () => ({
  useTreeStore: Object.assign(
    (
      selector: (state: {
        reloadTree: typeof reloadTree
        openNode: () => void
      }) => unknown,
    ) => selector({ reloadTree, openNode: vi.fn() }),
    { getState: () => ({ getPathById }) },
  ),
}))
vi.mock('./pageEditorStore', () => ({
  isDirtyState: () => false,
  usePageEditorStore: Object.assign(
    (selector: (state: Record<string, unknown>) => unknown) => selector(state),
    { getState: () => state, setState: vi.fn(), resetEditorState: vi.fn() },
  ),
}))
vi.mock('./useAutoSave', () => ({ useAutoSave: () => ({ status: 'idle' }) }))
vi.mock('./useNavigationGuard', () => ({ default: () => undefined }))
vi.mock('./useToolbarActions', () => ({ useToolbarActions: () => undefined }))
vi.mock('./MarkdownEditor', () => ({ default: () => <div /> }))
vi.mock('./PageFrontmatterPanel', () => ({
  PageFrontmatterPanel: () => <div />,
}))
vi.mock('@/components/ui/button', () => ({
  Button: ({
    children,
    onClick,
  }: {
    children: ReactNode
    onClick?: () => void
  }) => <button onClick={onClick}>{children}</button>,
}))
vi.mock('@/components/ui/alert-dialog', () => ({
  AlertDialog: ({ children }: { children: ReactNode }) => <>{children}</>,
  AlertDialogTrigger: ({ children }: { children: ReactNode }) => (
    <>{children}</>
  ),
  AlertDialogContent: ({ children }: { children: ReactNode }) => (
    <>{children}</>
  ),
  AlertDialogDescription: ({ children }: { children: ReactNode }) => (
    <>{children}</>
  ),
  AlertDialogFooter: ({ children }: { children: ReactNode }) => <>{children}</>,
  AlertDialogHeader: ({ children }: { children: ReactNode }) => <>{children}</>,
  AlertDialogTitle: ({ children }: { children: ReactNode }) => <>{children}</>,
  AlertDialogCancel: ({ children }: { children: ReactNode }) => <>{children}</>,
  AlertDialogAction: ({
    children,
    onClick,
  }: {
    children: ReactNode
    onClick?: () => void
  }) => <button onClick={onClick}>{children}</button>,
}))

const page = {
  id: 'page-1',
  slug: 'published',
  path: 'published',
  title: 'Published',
  content: '',
  version: 'v1',
  kind: 'page' as const,
}
let state: Record<string, unknown>

import PageEditor from './PageEditor'

function loadDraft(overrides: Record<string, unknown> = {}) {
  state = {
    page,
    initialPage: page,
    isDraft: true,
    isPendingDraft: false,
    pendingParentId: '',
    tags: [],
    frontmatterFields: [],
    frontmatterUnsupported: '',
    frontmatterErrors: {},
    notFound: false,
    error: null,
    savePage: vi.fn(),
    forceOverwrite: vi.fn(),
    setContent: vi.fn(),
    setTags: vi.fn(),
    setFrontmatterFields: vi.fn(),
    loadPageData: vi.fn(),
    loadPendingDraft: vi.fn(),
    publishDraft,
    discardDraft,
    resetEditorState: vi.fn(),
    ...overrides,
  }
}

describe('PageEditor draft lifecycle navigation', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    reloadTree.mockResolvedValue(undefined)
    getPathById.mockReturnValue('parent')
  })

  it('returns to the canonical view after publishing either draft kind', async () => {
    publishDraft.mockResolvedValue(page)
    loadDraft()
    const { rerender } = render(<PageEditor />)
    fireEvent.click(screen.getByText('draft.publish'))
    await waitFor(() =>
      expect(navigate).toHaveBeenCalledWith('/published', { replace: true }),
    )

    navigate.mockClear()
    loadDraft({ isPendingDraft: true })
    rerender(<PageEditor />)
    fireEvent.click(screen.getByText('draft.publish'))
    await waitFor(() =>
      expect(navigate).toHaveBeenCalledWith('/published', { replace: true }),
    )
  })

  it('returns active drafts to the canonical view and pending drafts to their parent after discard', async () => {
    discardDraft.mockResolvedValue(undefined)
    loadDraft()
    const { rerender } = render(<PageEditor />)
    fireEvent.click(screen.getAllByText('draft.discard')[1])
    await waitFor(() =>
      expect(navigate).toHaveBeenCalledWith('/published', { replace: true }),
    )

    navigate.mockClear()
    loadDraft({ isPendingDraft: true, pendingParentId: 'parent-1' })
    rerender(<PageEditor />)
    fireEvent.click(screen.getAllByText('draft.discard')[1])
    await waitFor(() =>
      expect(navigate).toHaveBeenCalledWith('/parent', { replace: true }),
    )
  })
})
