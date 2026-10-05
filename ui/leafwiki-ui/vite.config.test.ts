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

import { chunkFileName, resolveAppVersion } from './vite.config'

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
    expect(chunkFileName({ name: '_baseUniq' })).toBe('static/baseUniq-[hash].js')
    expect(chunkFileName({ name: '.hidden' })).toBe('static/hidden-[hash].js')
  })

  it('leaves regular chunk names untouched', () => {
    expect(chunkFileName({ name: 'codemirror' })).toBe(
      'static/codemirror-[hash].js',
    )
  })
})
