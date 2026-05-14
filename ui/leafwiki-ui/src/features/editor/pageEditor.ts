// zustand store to manage the PageEditor state
// e.g. loading, error, page, dirty, ...

import {
  applyPageRefactor,
  getPageByPath,
  Page,
  previewPageRefactor,
  updatePage,
} from '@/lib/api/pages'
import { isPageNotFoundError, mapApiError } from '@/lib/api/errors'
import { useConfigStore } from '@/stores/config'
import { useTreeStore } from '@/stores/tree'
import { create } from 'zustand'
import { useLinkStatusStore } from '../links/linkstatus_store'
import { confirmPageRefactor } from '../page/pageRefactorDialog'
import { useProgressbarStore } from '../progressbar/progressbar'
import {
  buildEditorFrontmatter,
  EditorFrontmatterField,
  parseEditorFrontmatter,
  validateEditorFrontmatterMetadata,
} from './frontmatter'

interface PageEditorState {
  title: string // current title in the editor
  slug: string // current slug in the editor
  content: string // current markdown content in the editor
  tags: string[] // convenient tag editor state
  frontmatterFields: EditorFrontmatterField[]
  frontmatterUnsupported: string
  frontmatterErrors: Record<string, string>
  error: string | null // error message, if any
  notFound: boolean
  page: Page | null // current page being edited
  initialPage: Page | null // initial page data when loaded
  setTitle: (title: string) => void // set the current title
  setSlug: (slug: string) => void // set the current slug
  setContent: (content: string) => void // set the current markdown content
  setTags: (tags: string[]) => void
  setFrontmatterFields: (fields: EditorFrontmatterField[]) => void
  setFrontmatterErrors: (errors: Record<string, string>) => void
  setError: (error: string | null) => void // set the error message
  setPage: (page: Page | null) => void // set the current page
  savePage: () => Promise<Page | null | undefined> // save the current page
  forceOverwrite: () => Promise<Page | null | undefined> // re-fetch server version, then save
  loadPageData: (path: string) => Promise<void> // load page data by path
}

const isDirtyState = (s: PageEditorState) => {
  const {
    page,
    title,
    slug,
    content,
    tags,
    frontmatterFields,
    frontmatterUnsupported,
  } = s
  if (!page) return false
  return (
    page.title !== title ||
    page.slug !== slug ||
    page.content !== content ||
    (page.frontmatter ?? '') !==
      buildEditorFrontmatter({
        tags,
        fields: frontmatterFields,
        unsupportedRaw: frontmatterUnsupported,
      })
  )
}

export const usePageEditorStore = create<PageEditorState>((set, get) => ({
  error: null,
  notFound: false,
  page: null,
  title: '',
  path: '',
  slug: '',
  content: '',
  tags: [],
  frontmatterFields: [],
  frontmatterUnsupported: '',
  frontmatterErrors: {},
  lastStoredPage: null,
  initialPage: null,
  setTitle: (title) => set({ title }),
  setSlug: (slug) => set({ slug }),
  setContent: (content) => set({ content }),
  setTags: (tags) =>
    set((state) => ({
      tags,
      frontmatterErrors: {
        ...state.frontmatterErrors,
        tags: '',
      },
    })),
  setFrontmatterFields: (frontmatterFields) =>
    set((state) => {
      const nextErrors = { ...state.frontmatterErrors }
      for (const key of Object.keys(nextErrors)) {
        if (key.startsWith('properties.')) {
          delete nextErrors[key]
        }
      }

      return {
        frontmatterFields,
        frontmatterErrors: nextErrors,
      }
    }),
  setFrontmatterErrors: (frontmatterErrors) => set({ frontmatterErrors }),
  setError: (error) => set({ error }),
  setPage: (page) => set({ page }),
  savePage: async () => {
    const {
      page,
      title,
      slug,
      content,
      tags,
      frontmatterFields,
      frontmatterUnsupported,
    } = get()
    if (!page || !isDirtyState(get())) return

    const frontmatterErrors = validateEditorFrontmatterMetadata(
      tags,
      frontmatterFields,
    )
    if (Object.keys(frontmatterErrors).length > 0) {
      set({ frontmatterErrors })
      throw new Error('Please fix metadata errors before saving.')
    }

    try {
      useProgressbarStore.getState().setLoading(true)
      set({ frontmatterErrors: {} })
      const titleChanged = page.title !== title
      const slugChanged = page.slug !== slug
      const enableLinkRefactor = useConfigStore.getState().enableLinkRefactor
      const frontmatter = buildEditorFrontmatter({
        tags,
        fields: frontmatterFields,
        unsupportedRaw: frontmatterUnsupported,
      })
      const frontmatterChanged = (page.frontmatter ?? '') !== frontmatter

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

        if (updatedPage && frontmatterChanged) {
          updatedPage = await updatePage(
            updatedPage.id,
            updatedPage.version,
            title,
            slug,
            content,
            frontmatter,
          )
        }
      } else {
        updatedPage = await updatePage(
          page.id,
          page.version,
          title,
          slug,
          content,
          frontmatter,
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
        state.page.frontmatter = updatedPage.frontmatter ?? frontmatter

        return { page: state.page }
      })

      // sync tree: full reload on structural changes, version-only patch otherwise
      if (titleChanged || slugChanged) {
        await useTreeStore.getState().reloadTree()
      } else if (updatedPage?.id && updatedPage?.version) {
        useTreeStore
          .getState()
          .patchNodeVersion(updatedPage.id, updatedPage.version)
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
    set({
      error: null,
      notFound: false,
      page: null,
      initialPage: null,
      frontmatterErrors: {},
    })
    useProgressbarStore.getState().setLoading(true)
    try {
      const page = await getPageByPath(path)
      const parsedFrontmatter = parseEditorFrontmatter(page.frontmatter)
      set({
        page,
        initialPage: { ...page },
        notFound: false,
        title: page.title,
        slug: page.slug,
        content: page.content,
        tags: parsedFrontmatter.tags,
        frontmatterFields: parsedFrontmatter.fields,
        frontmatterUnsupported: parsedFrontmatter.unsupportedRaw,
      })
    } catch (err) {
      if (isPageNotFoundError(err)) {
        set({
          error: null,
          notFound: true,
        })
        return
      }

      const mapped = mapApiError(err, 'An unknown error occurred')
      set({
        error: mapped.message,
        notFound: false,
      })
    } finally {
      useProgressbarStore.getState().setLoading(false)
    }
  },
}))
