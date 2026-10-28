import { describe, expect, it } from 'vitest'
import { documentTitle } from './documentTitle'

describe('documentTitle', () => {
  // Same format as seo.DocumentTitle in internal/wiki/seo/page_meta.go, which
  // the server puts into the shell of public pages.
  it.each([
    {
      name: 'page title, dash, site name',
      title: 'Setup',
      siteName: 'Team Wiki',
      want: 'Setup - Team Wiki',
    },
    {
      name: 'site name alone without a title',
      title: '',
      siteName: 'Team Wiki',
      want: 'Team Wiki',
    },
    {
      name: 'title is used as is',
      title: 'A <b> & "c"',
      siteName: 'LeafWiki',
      want: 'A <b> & "c" - LeafWiki',
    },
  ])('$name', ({ title, siteName, want }) => {
    expect(documentTitle(title, siteName)).toBe(want)
  })
})
