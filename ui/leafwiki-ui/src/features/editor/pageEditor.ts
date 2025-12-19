// zustand store to manage the PageEditor state
// e.g. loading, error, page, dirty, ...

import { getPageByPath, Page, updatePage } from '@/lib/api/pages'
import { useTreeStore } from '@/stores/tree'
import { create } from 'zustand'
import { useBacklinksStore } from '../backlinks/backlinks'
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

    set({ error: null })
    try {
      useProgressbarStore.getState().setLoading(true)
      const titleChanged = page.title !== title
      const slugChanged = page.slug !== slug

      const updatedPage = await updatePage(page.id, title, slug, content)
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

        return { page: state.page }
      })

      // if title or slug changed, we reload the tree to reflect changes
      if (titleChanged || slugChanged) {
        const reloadTree = useTreeStore.getState().reloadTree
        await reloadTree()
      }

      // reload backlinks
      const editorPageID = get().page?.id
      if (editorPageID) {
        const fetchPageBacklinks =
          useBacklinksStore.getState().fetchPageBacklinks
        await fetchPageBacklinks(editorPageID)
      }

      return updatedPage
    } catch (err) {
      if (err instanceof Error) {
        set({ error: err.message })
      } else {
        set({ error: 'An unknown error occurred' })
      }

      throw err
    } finally {
      useProgressbarStore.getState().setLoading(false)
    }
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
      if (err instanceof Error) {
        set({ error: err.message })
      } else {
        set({ error: 'An unknown error occurred' })
      }
    } finally {
      useProgressbarStore.getState().setLoading(false)
    }
  },
}))
