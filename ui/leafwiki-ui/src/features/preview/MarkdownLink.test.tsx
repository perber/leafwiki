import type { PageNode } from '@/lib/api/pages'
import { TooltipProvider } from '@/components/ui/tooltip'
import { DIALOG_CREATE_PAGE_BY_PATH } from '@/lib/registries'
import { useConfigStore } from '@/stores/config'
import { useDialogsStore } from '@/stores/dialogs'
import { useSessionStore } from '@/stores/session'
import { useTreeStore } from '@/stores/tree'
import { act, fireEvent, render, screen } from '@testing-library/react'
import { MemoryRouter, useLocation } from 'react-router'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { MarkdownLink } from './MarkdownLink'

vi.mock('@/lib/api/pages', async () => {
  const actual =
    await vi.importActual<typeof import('@/lib/api/pages')>('@/lib/api/pages')
  return {
    ...actual,
    suggestSlug: vi.fn(),
  }
})

vi.mock('react-i18next', () => ({
  initReactI18next: {
    type: '3rdParty',
    init: () => {},
  },
  useTranslation: () => ({ t: (key: string) => key }),
}))

import { suggestSlug } from '@/lib/api/pages'
const mockSuggestSlug = suggestSlug as ReturnType<typeof vi.fn>

function makeFolder(id: string, path: string): PageNode {
  return {
    id,
    title: path,
    slug: path,
    path,
    version: 'v1',
    children: null,
    kind: 'section',
    parentId: null,
  }
}

function makePage(id: string, path: string, parentId: string | null): PageNode {
  return {
    id,
    title: path,
    slug: path,
    path,
    version: 'v1',
    children: null,
    kind: 'page',
    parentId,
  }
}

describe('MarkdownLink wikilink-notfound', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    useDialogsStore.setState({ dialogType: null, dialogProps: null })
    useTreeStore.setState({ byId: {}, byPath: {} })
  })

  it.each([
    {
      name: 'creates the new page in the current page folder, not the root',
      folder: makeFolder('folder-1', 'docs'),
      currentPage: makePage('page-1', 'docs/current', 'folder-1'),
      title: 'TCP/IP',
      slug: 'tcp-ip',
      initialPath: 'docs/tcp-ip',
    },
    {
      name: 'creates the new page at the root when the current page is top-level',
      folder: null,
      currentPage: makePage('page-2', 'root-page', null),
      title: 'Foo',
      slug: 'foo',
      initialPath: 'foo',
    },
  ])('$name', async ({ folder, currentPage, title, slug, initialPath }) => {
    const pages = folder ? [folder, currentPage] : [currentPage]
    useTreeStore.setState({
      byId: Object.fromEntries(pages.map((page) => [page.id, page])),
      byPath: Object.fromEntries(pages.map((page) => [page.path, page])),
    })
    mockSuggestSlug.mockResolvedValue(slug)

    render(
      <MemoryRouter>
        <MarkdownLink
          href={`wikilink-notfound:${encodeURIComponent(title)}`}
          path={currentPage.path}
        >
          {title}
        </MarkdownLink>
      </MemoryRouter>,
    )

    await act(async () => {
      fireEvent.click(screen.getByRole('button'))
      await Promise.resolve()
    })

    expect(mockSuggestSlug).toHaveBeenCalledWith(folder?.id ?? '', title)
    expect(useDialogsStore.getState().dialogType).toBe(
      DIALOG_CREATE_PAGE_BY_PATH,
    )
    expect(useDialogsStore.getState().dialogProps).toMatchObject({
      initialPath,
      initialTitle: title,
    })
  })
})

describe('MarkdownLink web URLs', () => {
  beforeEach(() => {
    useDialogsStore.setState({ dialogType: null, dialogProps: null })
    useTreeStore.setState({ byId: {}, byPath: {} })
  })

  it.each([
    {
      name: 'same-origin page',
      href: `${window.location.origin}/view/docs/page`,
    },
    {
      name: 'base path, query and fragment',
      href: `${window.location.origin}/wiki/view/docs/page?mode=read#heading`,
    },
    {
      name: 'same-origin resource',
      href: `${window.location.origin}/assets/file.pdf?download=1#page=2`,
    },
    {
      name: 'uppercase scheme',
      href: `${window.location.origin.toUpperCase()}/view/docs/page`,
    },
    {
      name: 'protocol-relative same origin',
      href: `//${window.location.host}/wiki/view/docs/page?mode=read#heading`,
    },
  ])('opens $name in the current tab without a tree lookup', ({ href }) => {
    render(
      <MemoryRouter>
        <MarkdownLink href={href} target="_blank">
          Page
        </MarkdownLink>
      </MemoryRouter>,
    )

    const link = screen.getByRole('link', { name: 'Page' })
    expect(link).toHaveAttribute('href', href)
    expect(link).not.toHaveAttribute('target')
    expect(useDialogsStore.getState().dialogType).toBeNull()
  })

  it.each([
    {
      name: 'external HTTPS',
      href: 'https://example.com/wiki/page?mode=read#heading',
    },
    { name: 'external HTTP', href: 'http://example.com/page' },
    { name: 'uppercase external scheme', href: 'HTTPS://example.com/page' },
    { name: 'protocol-relative external URL', href: '//example.com/wiki/page' },
    {
      name: 'different port',
      href: `${window.location.protocol}//${window.location.hostname}:12345/page`,
    },
    {
      name: 'different protocol',
      href: `${window.location.protocol === 'http:' ? 'https:' : 'http:'}//${window.location.host}/page`,
    },
    {
      name: 'hostname suffix',
      href: `${window.location.protocol}//${window.location.hostname}.example.com/page`,
    },
    {
      name: 'credentials before external host',
      href: `${window.location.protocol}//${window.location.host}@example.com/page`,
    },
    { name: 'malformed web URL', href: 'https://[invalid/page' },
    { name: 'malformed uppercase URL', href: 'HTTPS://[invalid/page' },
    { name: 'malformed protocol-relative URL', href: '//[invalid/page' },
  ])('keeps $name in a protected new tab', ({ href }) => {
    render(
      <MemoryRouter>
        <MarkdownLink href={href}>External</MarkdownLink>
      </MemoryRouter>,
    )

    const link = screen.getByRole('link', { name: 'External' })
    expect(link).toHaveAttribute('href', href)
    expect(link).toHaveAttribute('target', '_blank')
    expect(link).toHaveAttribute('rel', 'noopener noreferrer')
  })

  it('preserves relative wiki navigation', () => {
    const page = makePage('page', 'docs/current/child', null)
    useTreeStore.setState({
      byId: { page },
      byPath: { 'docs/current/child': page },
    })

    render(
      <MemoryRouter>
        <MarkdownLink href="child" path="docs/current">
          Child
        </MarkdownLink>
      </MemoryRouter>,
    )

    const link = screen.getByRole('link', { name: 'Child' })
    expect(link).toHaveAttribute('href', '/docs/current/child')
    expect(link).not.toHaveAttribute('target')
  })

  it.each([
    { name: 'relative asset', href: 'assets/file.pdf' },
    { name: 'root-relative asset', href: '/assets/file.pdf' },
  ])('keeps $name in a protected new tab', ({ href }) => {
    render(
      <MemoryRouter>
        <MarkdownLink href={href}>File</MarkdownLink>
      </MemoryRouter>,
    )

    const link = screen.getByRole('link', { name: 'File' })
    expect(link).toHaveAttribute('href', '/assets/file.pdf')
    expect(link).toHaveAttribute('target', '_blank')
    expect(link).toHaveAttribute('rel', 'noopener noreferrer')
  })
})

describe('MarkdownLink fragments and queries (#1801)', () => {
  const target = makePage('t', 'docs/page-1', null)

  beforeEach(() => {
    useDialogsStore.setState({ dialogType: null, dialogProps: null })
    useTreeStore.setState({
      byId: { t: target },
      byPath: { 'docs/page-1': target },
    })
  })

  it.each([
    {
      name: 'absolute link with a heading',
      href: '/docs/page-1#heading2',
      path: 'docs/current',
      expected: '/docs/page-1#heading2',
    },
    {
      name: 'relative link with a heading',
      href: 'page-1#heading2',
      path: 'docs',
      expected: '/docs/page-1#heading2',
    },
    {
      name: 'relative parent link with a heading',
      href: '../page-1#heading2',
      path: 'docs/current',
      expected: '/docs/page-1#heading2',
    },
    {
      name: 'link with a query and a heading',
      href: '/docs/page-1?q=term#heading2',
      path: 'docs/current',
      expected: '/docs/page-1?q=term#heading2',
    },
    {
      name: 'link with a trailing slash and a heading',
      href: '/docs/page-1/#heading2',
      path: 'docs/current',
      expected: '/docs/page-1#heading2',
    },
    {
      name: 'link with a non-ASCII heading, encoded once',
      href: '/docs/page-1#%C3%BCberschrift',
      path: 'docs/current',
      expected: '/docs/page-1#%C3%BCberschrift',
    },
    {
      name: 'markup-like fragment, percent-encoded instead of parsed',
      href: '/docs/page-1#"><img src=x onerror=alert(1)>',
      path: 'docs/current',
      expected: '/docs/page-1#%22%3E%3Cimg%20src=x%20onerror=alert(1)%3E',
    },
    {
      name: 'NUL escape in the fragment, kept as written',
      href: '/docs/page-1#%00',
      path: 'docs/current',
      expected: '/docs/page-1#%00',
    },
    {
      name: 'second # inside the fragment',
      href: '/docs/page-1#a#b',
      path: 'docs/current',
      expected: '/docs/page-1#a#b',
    },
    {
      name: '? after # stays part of the fragment',
      href: '/docs/page-1#a?b',
      path: 'docs/current',
      expected: '/docs/page-1#a?b',
    },
    {
      name: 'empty query before the fragment',
      href: 'page-1?#heading2',
      path: 'docs',
      expected: '/docs/page-1#heading2',
    },
    {
      name: 'dot-relative link back to the current page',
      href: './#heading2',
      path: 'docs/page-1',
      expected: '/docs/page-1#heading2',
    },
    {
      name: 'backslash parent link',
      href: '..\\page-1#heading2',
      path: 'docs/current',
      expected: '/docs/page-1#heading2',
    },
    {
      name: 'very long fragment',
      href: `/docs/page-1#${'h'.repeat(5000)}`,
      path: 'docs/current',
      expected: `/docs/page-1#${'h'.repeat(5000)}`,
    },
  ])('keeps the fragment: $name', ({ href, path, expected }) => {
    const { container } = render(
      <MemoryRouter>
        <MarkdownLink href={href} path={path}>
          Page
        </MarkdownLink>
      </MemoryRouter>,
    )

    const link = screen.getByRole('link', { name: 'Page' })
    expect(link).toHaveAttribute('href', expected)
    expect(link).not.toHaveAttribute('target')
    expect(container.querySelector('img')).toBeNull()
  })

  function LocationProbe() {
    const location = useLocation()
    return (
      <output data-testid="location">
        {location.pathname + location.search + location.hash}
      </output>
    )
  }

  it('navigates to the target page with the query and heading in the location', () => {
    render(
      <MemoryRouter initialEntries={['/docs/current']}>
        <MarkdownLink href="page-1?q=term#heading2" path="docs">
          Page
        </MarkdownLink>
        <LocationProbe />
      </MemoryRouter>,
    )

    fireEvent.click(screen.getByRole('link', { name: 'Page' }))

    expect(screen.getByTestId('location')).toHaveTextContent(
      '/docs/page-1?q=term#heading2',
    )
  })

  it('keeps the fragment in the edit-mode preview, resolved against the edited page', () => {
    // Without a path prop the link resolves against window.location, which in
    // the editor is /e/<page>.
    window.history.replaceState(null, '', '/e/docs/current')
    try {
      render(
        <MemoryRouter initialEntries={['/e/docs/current']}>
          <MarkdownLink href="../page-1#heading2">Page</MarkdownLink>
        </MemoryRouter>,
      )

      expect(screen.getByRole('link', { name: 'Page' })).toHaveAttribute(
        'href',
        '/docs/page-1#heading2',
      )
    } finally {
      window.history.replaceState(null, '', '/')
    }
  })

  it('keeps a same-page fragment as a plain anchor', () => {
    render(
      <MemoryRouter>
        <MarkdownLink href="#heading2">Section</MarkdownLink>
      </MemoryRouter>,
    )

    const link = screen.getByRole('link', { name: 'Section' })
    expect(link).toHaveAttribute('href', '#heading2')
    expect(link).not.toHaveAttribute('target')
  })

  it('offers to create a missing target without the fragment in its path', () => {
    render(
      <MemoryRouter>
        <MarkdownLink href="/docs/missing?q=term#heading2">
          Missing
        </MarkdownLink>
      </MemoryRouter>,
    )

    fireEvent.click(screen.getByRole('button', { name: 'Missing' }))

    expect(useDialogsStore.getState().dialogType).toBe(
      DIALOG_CREATE_PAGE_BY_PATH,
    )
    expect(useDialogsStore.getState().dialogProps).toMatchObject({
      initialPath: 'docs/missing',
    })
  })
})

describe('MarkdownLink tree updates', () => {
  const target = makePage('t', 'target', null)

  beforeEach(() => {
    useDialogsStore.setState({ dialogType: null, dialogProps: null })
    useTreeStore.setState({ byId: {}, byPath: {} })
  })

  it('resolves a link when its target appears after render', () => {
    render(
      <MemoryRouter>
        <MarkdownLink href="/target">Target</MarkdownLink>
      </MemoryRouter>,
    )
    expect(screen.getByRole('button', { name: 'Target' })).toBeInTheDocument()
    expect(screen.queryByRole('link')).toBeNull()

    act(() =>
      useTreeStore.setState({
        byId: { t: target },
        byPath: { target },
      }),
    )

    expect(screen.getByRole('link', { name: 'Target' })).toHaveAttribute(
      'href',
      '/target',
    )
  })

  it('resolves a relative link when its target appears after render', () => {
    const child = makePage('c', 'docs/current/child', null)
    render(
      <MemoryRouter>
        <MarkdownLink href="child" path="docs/current">
          Child
        </MarkdownLink>
      </MemoryRouter>,
    )
    expect(screen.queryByRole('link')).toBeNull()

    act(() =>
      useTreeStore.setState({
        byId: { c: child },
        byPath: { 'docs/current/child': child },
      }),
    )

    expect(screen.getByRole('link', { name: 'Child' })).toHaveAttribute(
      'href',
      '/docs/current/child',
    )
  })

  it('stops navigating when the target disappears after render (editor)', () => {
    useTreeStore.setState({ byId: { t: target }, byPath: { target } })
    render(
      <MemoryRouter>
        <MarkdownLink href="/target">Target</MarkdownLink>
      </MemoryRouter>,
    )
    expect(screen.getByRole('link', { name: 'Target' })).toBeInTheDocument()

    act(() => useTreeStore.setState({ byId: {}, byPath: {} }))

    expect(screen.getByRole('button', { name: 'Target' })).toBeInTheDocument()
    expect(screen.queryByRole('link')).toBeNull()
  })

  describe('read-only', () => {
    const config = useConfigStore.getState()
    const session = useSessionStore.getState()

    beforeEach(() => {
      useConfigStore.setState({ publicAccess: true, authDisabled: false })
      useSessionStore.setState({ user: null })
      // BrokenLinkTooltip renders TooltipWrapper, which reads matchMedia.
      window.matchMedia = vi.fn().mockImplementation(() => ({
        matches: false,
        addEventListener: vi.fn(),
        removeEventListener: vi.fn(),
      }))
    })

    // Runs before Testing Library's cleanup, so the component is still mounted.
    afterEach(() => {
      act(() => {
        useConfigStore.setState({
          publicAccess: config.publicAccess,
          authDisabled: config.authDisabled,
        })
        useSessionStore.setState({ user: session.user })
      })
    })

    it('shows broken text when the target disappears after render (read-only)', () => {
      useTreeStore.setState({ byId: { t: target }, byPath: { target } })
      render(
        <MemoryRouter>
          <TooltipProvider>
            <MarkdownLink href="/target">Target</MarkdownLink>
          </TooltipProvider>
        </MemoryRouter>,
      )
      expect(screen.getByRole('link', { name: 'Target' })).toBeInTheDocument()

      act(() => useTreeStore.setState({ byId: {}, byPath: {} }))

      expect(screen.queryByRole('link')).toBeNull()
      expect(screen.queryByRole('button')).toBeNull()
      expect(screen.getByText('Target')).toHaveClass('text-error')
    })
  })
})
