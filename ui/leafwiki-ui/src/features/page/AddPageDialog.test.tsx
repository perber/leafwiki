import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import type { BaseDialogConfirmButton } from '@/components/BaseDialog'
import type { ReactNode } from 'react'
import { MemoryRouter } from 'react-router'
import { beforeEach, describe, expect, it, vi } from 'vitest'

const api = vi.hoisted(() => ({
  createPendingDraft: vi.fn(),
  createPage: vi.fn(),
}))
const navigate = vi.fn()

vi.mock('@/lib/api/pages', () => ({ NODE_KIND_PAGE: 'page', ...api }))
vi.mock('@/stores/tree', () => ({
  useTreeStore: (
    selector: (state: {
      reloadTree: () => Promise<void>
      getPathById: (id: string) => string
    }) => unknown,
  ) => selector({ reloadTree: vi.fn(), getPathById: () => '' }),
}))
vi.mock('@/lib/routePath', () => ({
  buildEditUrl: (path: string) => `/e/${path}`,
  buildPendingDraftEditUrl: (id: string) => `/pending-drafts/${id}/edit`,
}))
vi.mock('react-router', () => ({
  MemoryRouter: ({ children }: { children: ReactNode }) => children,
  useNavigate: () => navigate,
}))
vi.mock('react-i18next', () => ({
  initReactI18next: { type: '3rdParty', init: () => {} },
  useTranslation: () => ({ t: (key: string) => key }),
}))
vi.mock('sonner', () => ({
  toast: { success: vi.fn(), error: vi.fn(), warning: vi.fn() },
}))
vi.mock('@/components/BaseDialog', () => ({
  default: ({
    children,
    onConfirm,
    onClose,
    dialogTitle,
    buttons,
  }: {
    children: ReactNode
    onConfirm: (type: string) => Promise<boolean>
    onClose: () => boolean
    dialogTitle: string
    buttons: BaseDialogConfirmButton[]
  }) => (
    <div>
      <h1>{dialogTitle}</h1>
      {children}
      <button onClick={onClose}>close</button>
      {buttons.map((button) => (
        <button
          key={button.actionType}
          data-variant={button.variant}
          onClick={() => onConfirm(button.actionType)}
        >
          {button.label}
        </button>
      ))}
    </div>
  ),
}))
vi.mock('@/components/FormInput', () => ({
  FormInput: ({ onChange }: { onChange: (value: string) => void }) => (
    <input data-testid="title" onChange={(e) => onChange(e.target.value)} />
  ),
}))
vi.mock('./SlugInputWithSuggestion', () => ({
  SlugInputWithSuggestion: ({
    onSlugChange,
    onSlugTouchedChange,
    onLastSlugTitleChange,
  }: {
    onSlugChange: (value: string) => void
    onSlugTouchedChange: (touched: boolean) => void
    onLastSlugTitleChange: (title: string) => void
  }) => (
    <input
      data-testid="slug"
      onChange={(e) => {
        onSlugChange(e.target.value)
        onSlugTouchedChange(true)
        onLastSlugTitleChange('Draft')
      }}
    />
  ),
}))

import { AddPageDialog } from './AddPageDialog'

describe('AddPageDialog draft option', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    api.createPendingDraft.mockResolvedValue({ page: { id: 'pending-1' } })
    api.createPage.mockResolvedValue(undefined)
  })

  it('keeps the normal page flow by default', async () => {
    render(
      <MemoryRouter>
        <AddPageDialog parentId="" />
      </MemoryRouter>,
    )
    expect(screen.getByText('addDialog.titlePage')).toBeInTheDocument()
    expect(
      screen.getByRole('checkbox', { name: 'addDialog.createAsDraft' }),
    ).toHaveAttribute('data-state', 'unchecked')
    expect(screen.getByText('addDialog.create')).toBeInTheDocument()
    expect(screen.getByText('addDialog.create')).toHaveAttribute(
      'data-variant',
      'secondary',
    )
    expect(screen.getByText('addDialog.createAndEdit')).toBeInTheDocument()
    expect(screen.getByText('addDialog.createAndEdit')).toHaveAttribute(
      'data-variant',
      'default',
    )
    fireEvent.change(screen.getByTestId('title'), {
      target: { value: 'Draft' },
    })
    fireEvent.change(screen.getByTestId('slug'), {
      target: { value: 'draft' },
    })
    fireEvent.click(screen.getByText('addDialog.create'))
    await waitFor(() =>
      expect(api.createPage).toHaveBeenCalledWith({
        title: 'Draft',
        slug: 'draft',
        parentId: '',
        kind: 'page',
      }),
    )
    expect(api.createPendingDraft).not.toHaveBeenCalled()
  })

  it('keeps page buttons stable and creates a pending draft when selected', async () => {
    render(
      <MemoryRouter>
        <AddPageDialog parentId="" />
      </MemoryRouter>,
    )
    const checkbox = screen.getByRole('checkbox', {
      name: 'addDialog.createAsDraft',
    })
    fireEvent.click(checkbox)
    expect(checkbox).toHaveAttribute('data-state', 'checked')
    fireEvent.click(screen.getByText('close'))
    expect(checkbox).toHaveAttribute('data-state', 'unchecked')
    fireEvent.click(checkbox)
    expect(screen.getByText('addDialog.create')).toHaveAttribute(
      'data-variant',
      'secondary',
    )
    expect(screen.getByText('addDialog.createAndEdit')).toHaveAttribute(
      'data-variant',
      'default',
    )
    fireEvent.change(screen.getByTestId('title'), {
      target: { value: 'Draft' },
    })
    fireEvent.change(screen.getByTestId('slug'), {
      target: { value: 'draft' },
    })
    fireEvent.click(screen.getByText('addDialog.create'))
    await waitFor(() =>
      expect(api.createPendingDraft).toHaveBeenCalledWith({
        title: 'Draft',
        slug: 'draft',
        parentId: '',
      }),
    )
    expect(navigate).toHaveBeenCalledWith('/pending-drafts/pending-1/edit')
    expect(api.createPage).not.toHaveBeenCalled()
    await waitFor(() =>
      expect(checkbox).toHaveAttribute('data-state', 'unchecked'),
    )
  })

  it('opens the pending editor after Create & Edit when draft creation is selected', async () => {
    render(
      <MemoryRouter>
        <AddPageDialog parentId="" />
      </MemoryRouter>,
    )
    fireEvent.click(
      screen.getByRole('checkbox', { name: 'addDialog.createAsDraft' }),
    )
    fireEvent.change(screen.getByTestId('title'), {
      target: { value: 'Draft' },
    })
    fireEvent.change(screen.getByTestId('slug'), {
      target: { value: 'draft' },
    })
    fireEvent.click(screen.getByText('addDialog.createAndEdit'))
    await waitFor(() =>
      expect(api.createPendingDraft).toHaveBeenCalledWith({
        title: 'Draft',
        slug: 'draft',
        parentId: '',
      }),
    )
    expect(navigate).toHaveBeenCalledWith('/pending-drafts/pending-1/edit')
  })
})
