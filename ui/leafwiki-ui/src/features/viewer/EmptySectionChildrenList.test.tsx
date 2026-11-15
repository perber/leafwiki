import type { Page, PageNode } from '@/lib/api/pages'
import { DEFAULT_PAGE_DISPLAY } from '@/lib/pageDisplay'
import { useConfigStore } from '@/stores/config'
import { useSessionStore } from '@/stores/session'
import { useTreeStore } from '@/stores/tree'
import { render, screen, within } from '@testing-library/react'
import { MemoryRouter } from 'react-router'
import { beforeEach, describe, expect, it } from 'vitest'
import EmptySectionChildrenList from './EmptySectionChildrenList'

const leaf = (id: string, parentId: string): PageNode => ({
  id,
  title: id,
  slug: id,
  path: `docs/${parentId}/${id}`,
  version: 'v1',
  parentId,
  children: null,
  kind: 'page',
})

// docs (section, no content) -> "Guides" (section, 3 pages), "Empty"
// (section, no children), "Intro" (page)
const guides: PageNode = {
  ...leaf('guides', 'docs'),
  title: 'Guides',
  kind: 'section',
  children: [
    leaf('a', 'guides'),
    leaf('b', 'guides'),
    { ...leaf('c', 'guides'), kind: 'section', children: [leaf('d', 'c')] },
  ],
}
const empty: PageNode = {
  ...leaf('empty', 'docs'),
  title: 'Empty',
  kind: 'section',
  children: [],
}
const single: PageNode = {
  ...leaf('single', 'docs'),
  title: 'Single',
  kind: 'section',
  children: [leaf('only', 'single')],
}
const intro: PageNode = { ...leaf('intro', 'docs'), title: 'Intro' }
const docsNode: PageNode = {
  ...leaf('docs', 'root'),
  path: 'docs',
  title: 'Docs',
  kind: 'section',
  children: [guides, empty, single, intro],
}
const docsPage: Page = {
  id: 'docs',
  slug: 'docs',
  path: 'docs',
  title: 'Docs',
  content: '',
  version: 'v1',
  kind: 'section',
}

function renderList(showSubpageCount: boolean) {
  useConfigStore.setState({
    pageDisplay: { ...DEFAULT_PAGE_DISPLAY, showSubpageCount },
  })
  return render(
    <MemoryRouter>
      <EmptySectionChildrenList page={docsPage} />
    </MemoryRouter>,
  )
}

function itemFor(title: string) {
  return screen.getByRole('link', { name: new RegExp(`^${title}`) })
    .parentElement as HTMLElement
}

beforeEach(() => {
  useSessionStore.setState({ user: null })
  useConfigStore.setState({ publicAccess: true, authDisabled: false })
  useTreeStore.setState({
    tree: { ...docsNode, id: 'root', children: [docsNode] },
    getPageById: (id: string) => (id === docsNode.id ? docsNode : null),
  })
})

describe('EmptySectionChildrenList subpage count', () => {
  it('shows no count when the setting is off', () => {
    renderList(false)

    expect(screen.queryAllByTestId(/^subpage-count-/)).toHaveLength(0)
  })

  it.each([
    { title: 'Guides', visible: '3', spoken: '3 subpages' },
    { title: 'Empty', visible: '0', spoken: '0 subpages' },
    { title: 'Single', visible: '1', spoken: '1 subpage' },
  ])(
    'shows $visible direct subpages at the sub-section $title',
    ({ title, visible, spoken }) => {
      renderList(true)

      const count = within(itemFor(title)).getByTestId(/^subpage-count-/)
      expect(within(count).getByText(visible)).toHaveAttribute(
        'aria-hidden',
        'true',
      )
      expect(within(count).getByText(spoken)).toHaveClass('sr-only')
    },
  )

  it('shows no count at a page in the list', () => {
    renderList(true)

    expect(
      within(itemFor('Intro')).queryByTestId(/^subpage-count-/),
    ).not.toBeInTheDocument()
  })
})
