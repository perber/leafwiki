import { afterEach, describe, expect, it, vi } from 'vitest'

vi.mock('child_process', async (importOriginal) => {
  const actual = await importOriginal<typeof import('child_process')>()
  return {
    ...actual,
    execSync: vi.fn(() => {
      throw new Error(
        'scripts/resolve-version.sh not reachable (simulated Docker frontend-build stage)',
      )
    }),
  }
})

import {
  backendProxyPattern,
  chunkFileName,
  resolveAppVersion,
  stripDevShellBody,
} from './vite.config'

describe('resolveAppVersion', () => {
  const originalAppVersion = process.env.APP_VERSION

  afterEach(() => {
    if (originalAppVersion === undefined) {
      delete process.env.APP_VERSION
    } else {
      process.env.APP_VERSION = originalAppVersion
    }
  })

  it('returns APP_VERSION without needing scripts/resolve-version.sh to be reachable', () => {
    // The Docker frontend-build stage only COPYs ui/leafwiki-ui/, so
    // scripts/resolve-version.sh (and bash, on node:*-alpine) generally
    // isn't present there. Every real build sets APP_VERSION via
    // --build-arg regardless, so this must not depend on the script
    // succeeding.
    process.env.APP_VERSION = 'v9.9.9-test'

    expect(resolveAppVersion()).toBe('v9.9.9-test')
  })
})

describe('chunkFileName', () => {
  it('never emits a file name starting with "_" or "."', () => {
    // Go's //go:embed skips files starting with "_" or "." when embedding a
    // directory, so such a chunk (e.g. mermaid's lodash "_baseUniq") would
    // 404 at runtime and break the lazy import that depends on it.
    expect(chunkFileName({ name: '_baseUniq' })).toBe(
      'static/baseUniq-[hash].js',
    )
    expect(chunkFileName({ name: '.hidden' })).toBe('static/hidden-[hash].js')
  })

  it('leaves regular chunk names untouched', () => {
    expect(chunkFileName({ name: 'codemirror' })).toBe(
      'static/codemirror-[hash].js',
    )
  })
})

describe('stripDevShellBody', () => {
  it('empties #root in the dev server, where Go does not fill the shell', () => {
    // The Go server replaces {{__BODY__}} with contributed markup; the Vite
    // dev server serves index.html as is, so the placeholder would flash as
    // text until React mounts.
    expect(
      stripDevShellBody(
        '<title>{{__PAGE_TITLE__}}</title><div id="root">{{__BODY__}}</div>',
      ),
    ).toBe('<title>{{__PAGE_TITLE__}}</title><div id="root"></div>')
  })
})

describe('backendProxyPattern', () => {
  // Vite treats a proxy key starting with "^" as a regex and tests it against
  // the request URL; a plain key would be a startsWith match.
  const proxied = (url: string) => new RegExp(backendProxyPattern).test(url)

  it('proxies the backend paths', () => {
    expect(proxied('/api/pages')).toBe(true)
    expect(proxied('/api')).toBe(true)
    expect(proxied('/api?x=1')).toBe(true)
    expect(proxied('/assets/abc/image.png')).toBe(true)
    expect(proxied('/branding/logo.png')).toBe(true)
  })

  it('leaves page slugs that only start with a backend prefix to the SPA', () => {
    // A reload on such a page in `make dev` must hit Vite, not the backend,
    // which has no embedded frontend in dev and answers 404.
    expect(proxied('/api-guide')).toBe(false)
    expect(proxied('/assets-list')).toBe(false)
    expect(proxied('/branding-guide/logos')).toBe(false)
    expect(proxied('/docs/api')).toBe(false)
  })
})
