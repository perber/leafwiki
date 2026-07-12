import { beforeEach, describe, expect, it } from 'vitest'
import type { Page } from '@/lib/api/pages'
import { usePageEditorStore } from './pageEditorStore'

const fakePage: Page = {
  id: 'page-1',
  title: 'Getting Started',
  slug: 'getting-started',
  path: 'docs/getting-started',
  kind: 'page',
  content: 'Hello world',
  version: 'v1',
  tags: ['guide'],
  properties: { owner: 'alice' },
} as Page

describe('pageEditorStore.resetEditorState', () => {
  beforeEach(() => {
    usePageEditorStore.setState(usePageEditorStore.getInitialState())
  })

  it('clears page back to null so stale currentEditorPageId reads disappear', () => {
    usePageEditorStore.setState({
      page: fakePage,
      initialPage: fakePage,
      title: fakePage.title,
      slug: fakePage.slug,
      content: fakePage.content,
      tags: fakePage.tags,
      frontmatterFields: [{ key: 'owner', value: 'alice', type: 'text' }],
      notFound: true,
      error: 'stale error',
    })

    usePageEditorStore.getState().resetEditorState()

    const state = usePageEditorStore.getState()
    expect(state.page).toBeNull()
    expect(state.initialPage).toBeNull()
    expect(state.title).toBe('')
    expect(state.slug).toBe('')
    expect(state.content).toBe('')
    expect(state.tags).toEqual([])
    expect(state.frontmatterFields).toEqual([])
    expect(state.notFound).toBe(false)
    expect(state.error).toBeNull()
  })

  it('leaves the store in a clean state when nothing was ever loaded', () => {
    usePageEditorStore.getState().resetEditorState()

    expect(usePageEditorStore.getState().page).toBeNull()
  })
})
