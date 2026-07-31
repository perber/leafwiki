import { NODE_KIND_PAGE, NODE_KIND_SECTION, sortPages } from '@/lib/api/pages'
import type { PageNode } from '@/lib/api/pages'
import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import type { ReactNode } from 'react'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { SortPagesDialog } from './SortPagesDialog'

vi.mock('@/components/BaseDialog', () => ({
  default: ({
    children,
    buttons,
    onConfirm,
  }: {
    children?: ReactNode
    buttons?: { label: string; actionType: string }[]
    onConfirm: (type: string) => Promise<boolean>
  }) => (
    <div>
      {children}
      {buttons?.map((button) => (
        <button
          key={button.actionType}
          data-testid={`sort-pages-dialog-button-${button.actionType}`}
          onClick={() => void onConfirm(button.actionType)}
        >
          {button.label}
        </button>
      ))}
    </div>
  ),
}))

vi.mock('react-i18next', () => ({
  initReactI18next: {
    type: '3rdParty',
    init: () => {},
  },
  useTranslation: () => ({
    t: (key: string) =>
      ({
        'sortDialog.ascending': 'A → Z',
        'sortDialog.descending': 'Z → A',
        'sortDialog.sectionsFirst': 'Sections First',
        'sortDialog.sectionsLast': 'Sections Last',
        'sortDialog.save': 'Save',
      })[key] ?? key,
  }),
}))

vi.mock('@/lib/api/pages', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@/lib/api/pages')>()
  return {
    ...actual,
    sortPages: vi.fn(),
  }
})

vi.mock('@/stores/tree', () => ({
  useTreeStore: (
    selector: (state: { reloadTree: () => Promise<void> }) => unknown,
  ) => selector({ reloadTree: vi.fn() }),
}))

vi.mock('sonner', () => ({
  toast: { success: vi.fn() },
}))

const child = (
  id: string,
  title: string,
  kind: PageNode['kind'],
): PageNode => ({
  id,
  title,
  slug: id,
  path: `/${id}`,
  version: '1',
  children: null,
  kind,
})

const parent: PageNode = {
  id: 'root',
  title: 'Root',
  slug: 'root',
  path: '/',
  version: '1',
  kind: NODE_KIND_SECTION,
  children: [
    child('home', 'Home', NODE_KIND_PAGE),
    child('guides', 'Guides', NODE_KIND_SECTION),
    child('about', 'About', NODE_KIND_PAGE),
    child('development', 'Development', NODE_KIND_SECTION),
  ],
}

const displayedTitles = () =>
  screen
    .getAllByTestId(/^sort-page-title-/)
    .map((element) => element.textContent)

describe('SortPagesDialog', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('sorts sections first and saves the grouped order', async () => {
    const user = userEvent.setup()
    render(<SortPagesDialog parent={parent} />)

    await user.click(screen.getByTestId('sort-sections-first-button'))

    expect(displayedTitles()).toEqual([
      'Development',
      'Guides',
      'About',
      'Home',
    ])

    fireEvent.click(screen.getByTestId('sort-pages-dialog-button-confirm'))

    await waitFor(() => {
      expect(sortPages).toHaveBeenCalledWith('root', [
        'development',
        'guides',
        'about',
        'home',
      ])
    })
  })

  it('sorts sections last using the selected descending direction', async () => {
    const user = userEvent.setup()
    render(<SortPagesDialog parent={parent} />)

    await user.click(screen.getByTestId('sort-za-button'))
    await user.click(screen.getByTestId('sort-sections-last-button'))

    expect(displayedTitles()).toEqual([
      'Home',
      'About',
      'Guides',
      'Development',
    ])
  })

  it('keeps the existing alphabetical sort across all children', async () => {
    const user = userEvent.setup()
    render(<SortPagesDialog parent={parent} />)

    await user.click(screen.getByTestId('sort-az-button'))

    expect(displayedTitles()).toEqual([
      'About',
      'Development',
      'Guides',
      'Home',
    ])
  })
})
