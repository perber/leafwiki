// zustand store to manage the PageEditor state
// e.g. loading, error, page, dirty, ...

import {
  applyPageRefactor,
  getPageByPath,
  Page,
  previewPageRefactor,
  updatePage,
} from '@/lib/api/pages'
import { mapApiError } from '@/lib/api/errors'
import { useConfigStore } from '@/stores/config'
import { useTreeStore } from '@/stores/tree'
import { create } from 'zustand'
import { useLinkStatusStore } from '../links/linkstatus_store'
import { confirmPageRefactor } from '../page/pageRefactorDialog'
import { useProgressbarStore } from '../progressbar/progressbar'

interface PageEditorState {
  title: string // current title in the editor
  slug: string // current slug in the editor
  content: string // current markdown content in the editor
  error: string | null // error message, if any
  page: Page | null // current page being edited
  initialPage: Page | null // initial page data when loaded
  setTitle: (title: string) => void // set the current title
  setSlug: (slug: string) => void // set the current slug
  setContent: (content: string) => void // set the current markdown content
  setError: (error: string | null) => void // set the error message
  setPage: (page: Page | null) => void // set the current page
  savePage: () => Promise<Page | null | undefined> // save the current page
  forceOverwrite: () => Promise<Page | null | undefined> // re-fetch server version, then save
  loadPageData: (path: string) => Promise<void> // load page data by path
}

const isDirtyState = (s: PageEditorState) => {
  const { page, title, slug, content } = s
  if (!page) return false
  return page.title !== title || page.slug !== slug || page.content !== content
}

export const usePageEditorStore = create<PageEditorState>((set, get) => ({
  error: null,
  page: null,
  title: '',
  path: '',
  slug: '',
  content: '',
  lastStoredPage: null,
  initialPage: null,
  setTitle: (title) => set({ title }),
  setSlug: (slug) => set({ slug }),
  setContent: (content) => set({ content }),
  setError: (error) => set({ error }),
  setPage: (page) => set({ page }),
  savePage: async () => {
    const { page, title, slug, content } = get()
    if (!page || !isDirtyState(get())) return

    try {
      useProgressbarStore.getState().setLoading(true)
      const titleChanged = page.title !== title
      const slugChanged = page.slug !== slug
      const enableLinkRefactor = useConfigStore.getState().enableLinkRefactor

      let updatedPage: Page | null = null

      if (slugChanged && enableLinkRefactor) {
        const preview = await previewPageRefactor(page.id, {
          kind: 'rename',
          title,
          slug,
        })
        const rewriteLinks = await confirmPageRefactor(preview)
        if (rewriteLinks === null) {
          return null
        }

        updatedPage = await applyPageRefactor(page.id, {
          kind: 'rename',
          version: page.version,
          title,
          slug,
          content,
          rewriteLinks,
        })
      } else {
        updatedPage = await updatePage(
          page.id,
          page.version,
          title,
          slug,
          content,
        )
      }

      // only update the page.content to avoid overwriting other fields
      set((state) => {
        if (!state.page) return {}

        if (
          updatedPage?.content === null ||
          updatedPage?.content === undefined
        ) {
          throw new Error('Updated page content is null or undefined')
        }
        state.page.title = updatedPage.title
        state.page.slug = updatedPage.slug
        state.page.content = updatedPage.content
        state.page.path = updatedPage.path
        state.page.version = updatedPage.version

        return { page: state.page }
      })

      // sync tree: full reload on structural changes, version-only patch otherwise
      if (titleChanged || slugChanged) {
        await useTreeStore.getState().reloadTree()
      } else if (updatedPage?.id && updatedPage?.version) {
        useTreeStore.getState().patchNodeVersion(updatedPage.id, updatedPage.version)
      }

      // reload backlinks
      const editorPageID = get().page?.id
      if (editorPageID) {
        const fetchLinkStatusForPage =
          useLinkStatusStore.getState().fetchLinkStatusForPage
        await fetchLinkStatusForPage(editorPageID)
      }

      return updatedPage
    } finally {
      useProgressbarStore.getState().setLoading(false)
    }
  },
  forceOverwrite: async () => {
    const { page } = get()
    if (!page?.path) return

    const fresh = await getPageByPath(page.path)
    set((state) => {
      if (!state.page) return {}
      state.page.version = fresh.version
      return { page: state.page }
    })
    return get().savePage()
  },
  loadPageData: async (path: string) => {
    set({ error: null, page: null, initialPage: null })
    useProgressbarStore.getState().setLoading(true)
    try {
      const page = await getPageByPath(path)
      set({
        page,
        initialPage: { ...page },
        title: page.title,
        slug: page.slug,
        content: page.content,
      })
    } catch (err) {
      const mapped = mapApiError(err, 'An unknown error occurred')
      set({
        error: mapped.message,
      })
    } finally {
      useProgressbarStore.getState().setLoading(false)
    }
  },
}))
