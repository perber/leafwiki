import { render, screen } from '@testing-library/react'
import {
  createMemoryRouter,
  Outlet,
  RouterProvider,
  useLocation,
} from 'react-router'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { createLeafWikiRouter } from './router'

// Replace every lazy section component with a synchronous stub so the test
// exercises real route matching (redirect -> guard -> section) without
// loading the section modules and their stores.
vi.mock('./lazy-routes', async (importOriginal) => {
  const actual = await importOriginal<Record<string, unknown>>()
  return Object.fromEntries(
    Object.keys(actual).map((name) => [
      name,
      () => <div data-testid={`section-${name}`} />,
    ]),
  )
})

const mockContext = vi.fn()
vi.mock('@/lib/registries/settingsSectionRegistry', async (importOriginal) => {
  const actual =
    await importOriginal<
      typeof import('@/lib/registries/settingsSectionRegistry')
    >()
  return { ...actual, useSettingsSectionContext: () => mockContext() }
})

const baseCtx = {
  authDisabled: false,
  gitBackupEnabled: false,
  gitBackupEnvManaged: false,
  snapshotEnabled: false,
  enableApiKeyManagement: false,
  totpAvailable: false,
  userManagementUrl: undefined,
  editorLimit: 0,
}

function LocationProbe() {
  const location = useLocation()
  return <div data-testid="location">{location.pathname + location.search}</div>
}

// renderSettingsAt mounts the real /settings children from
// createLeafWikiRouter under a bare layout (no AuthWrapper/SettingsLayout,
// which need a live session) and navigates to `url`.
function renderSettingsAt(url: string) {
  const appRouter = createLeafWikiRouter(false, false, false, '', true)
  const settingsRoute = appRouter.routes.find((r) => r.path === '/settings')!
  const router = createMemoryRouter(
    [
      {
        path: '/settings',
        element: (
          <>
            <Outlet />
            <LocationProbe />
          </>
        ),
        children: settingsRoute.children,
      },
      { path: '*', element: <LocationProbe /> },
    ],
    { initialEntries: [url] },
  )
  return render(<RouterProvider router={router} />)
}

describe('old settings URLs redirect through the target section guard', () => {
  beforeEach(() => {
    mockContext.mockReset()
  })

  // The redirect target is a fixed string, so a query string on the old
  // bookmark is dropped; nothing reads one on the Display section.
  it.each([
    ['/settings/toc-display'],
    ['/settings/toc-display/'],
    ['/settings/toc-display?x=1'],
  ])('lands an admin on the Display section from %s', async (url) => {
    mockContext.mockReturnValue({ ...baseCtx, role: 'admin' })

    renderSettingsAt(url)

    expect(
      await screen.findByTestId('section-DisplaySettings'),
    ).toBeInTheDocument()
    expect(screen.getByTestId('location').textContent).toBe('/settings/display')
  })

  // Anonymous users never get here in the app (AuthWrapper sends them to
  // /login first); the row proves the guard alone still holds.
  it.each([
    ['an editor', { role: 'editor' }, '/settings/account'],
    ['a viewer', { role: 'viewer' }, '/settings/account'],
    ['an anonymous user', { role: undefined }, '/'],
  ])(
    'does not let %s reach the Display section via the old URL',
    async (_name, roleCtx, expectedLocation) => {
      mockContext.mockReturnValue({ ...baseCtx, ...roleCtx })

      renderSettingsAt('/settings/toc-display')

      await vi.waitFor(() =>
        expect(screen.getByTestId('location').textContent).toBe(
          expectedLocation,
        ),
      )
      expect(
        screen.queryByTestId('section-DisplaySettings'),
      ).not.toBeInTheDocument()
    },
  )

  it('does not treat a nested path under the old segment as the old section', async () => {
    mockContext.mockReturnValue({ ...baseCtx, role: 'admin' })

    renderSettingsAt('/settings/toc-display/extra')

    expect(
      screen.queryByTestId('section-DisplaySettings'),
    ).not.toBeInTheDocument()
    expect(screen.getByTestId('location').textContent).toBe(
      '/settings/toc-display/extra',
    )
  })
})
