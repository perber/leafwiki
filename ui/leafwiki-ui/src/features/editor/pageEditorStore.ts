// zustand store to manage the PageEditor state
// e.g. loading, error, page, dirty, ...

import {
  applyPageRefactor,
  discardDraft,
  discardPendingDraft,
  getDraft,
  getPendingDraft,
  getPageByPath,
  NODE_KIND_PAGE,
  Page,
  publishDraft,
  publishPendingDraft,
  previewPageRefactor,
  saveDraft,
  savePendingDraft,
  startDraft,
  updatePage,
} from '@/lib/api/pages'
import {
  asApiLocalizedError,
  isPageNotFoundError,
  mapApiError,
} from '@/lib/api/errors'
import i18next from '@/lib/i18n'
import { useConfigStore } from '@/stores/config'
import { useTreeStore } from '@/stores/tree'
import { create } from 'zustand'
import { useLinkStatusStore } from '../links/linkstatus_store'
import { confirmPageRefactor } from '../page/pageRefactorDialogState'
import { useProgressbarStore } from '../progressbar/progressbarStore'
import { useViewerStore } from '../viewer/viewer'
import {
  EditorFrontmatterField,
  validateEditorFrontmatterMetadata,
} from './frontmatter'

export interface PageEditorState {
  title: string // current title in the editor
  slug: string // current slug in the editor
  content: string // current markdown content in the editor
  tags: string[] // convenient tag editor state
  frontmatterFields: EditorFrontmatterField[]
  frontmatterUnsupported: string
  frontmatterErrors: Record<string, string>
  error: string | null // error message, if any
  isLoading: boolean
  notFound: boolean
  page: Page | null // current page being edited
  initialPage: Page | null // initial page data when loaded
  isDraft: boolean
  isPendingDraft: boolean
  pendingParentId: string
  setTitle: (title: string) => void // set the current title
  setSlug: (slug: string) => void // set the current slug
  setContent: (content: string) => void // set the current markdown content
  setTags: (tags: string[]) => void
  setFrontmatterFields: (fields: EditorFrontmatterField[]) => void
  setFrontmatterErrors: (errors: Record<string, string>) => void
  setError: (error: string | null) => void // set the error message
  setPage: (page: Page | null) => void // set the current page
  savePage: (options?: { silent?: boolean }) => Promise<Page | null | undefined> // save the current page
  forceOverwrite: () => Promise<Page | null | undefined> // re-fetch server version, then save
  loadPageData: (path: string) => Promise<void> // load page data by path
  loadPendingDraft: (id: string) => Promise<void>
  startDraft: () => Promise<void>
  publishDraft: () => Promise<Page | null>
  discardDraft: () => Promise<void>
  resetEditorState: () => void // clear the store back to its pristine (no page loaded) shape
}

function tagsChanged(current: string[], original: string[]): boolean {
  if (current.length !== original.length) return true
  const a = [...current].sort()
  const b = [...original].sort()
  return a.some((v, i) => v !== b[i])
}

function propertiesChanged(
  fields: EditorFrontmatterField[],
  original: Record<string, unknown>,
): boolean {
  const editable = fields.filter((f) => !f.internal && f.type === 'text')
  const origKeys = Object.keys(original)
  if (editable.length !== origKeys.length) return true
  return editable.some((f) => String(original[f.key] ?? '') !== f.value)
}

function buildEditableProperties(
  fields: EditorFrontmatterField[],
): Record<string, string> {
  const properties: Record<string, string> = {}

  for (const field of fields) {
    if (!field.internal && field.type === 'text' && field.key) {
      properties[field.key] = field.value
    }
  }

  return properties
}

export const isDirtyState = (s: PageEditorState) => {
  const { page, title, slug, content, tags, frontmatterFields } = s
  if (!page) return false
  return (
    page.title !== title ||
    page.slug !== slug ||
    page.content !== content ||
    tagsChanged(tags, page.tags ?? []) ||
    propertiesChanged(frontmatterFields, page.properties ?? {})
  )
}

// Module-level mutex: prevents concurrent auto-saves from stacking.
// Manual saves (silent=false) bypass this so Ctrl+S is never blocked by an in-flight auto-save.
let isSavingMutex = false

let loadController: AbortController | null = null

export const usePageEditorStore = create<PageEditorState>((set, get) => ({
  error: null,
  isLoading: false,
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
  isDraft: false,
  isPendingDraft: false,
  pendingParentId: '',
  setTitle: (title) => set({ title }),
  setSlug: (slug) => set({ slug }),
  setContent: (content) => set({ content }),
  setTags: (tags) =>
    set((state) => {
      const nextErrors = { ...state.frontmatterErrors }
      delete nextErrors.tags
      return { tags, frontmatterErrors: nextErrors }
    }),
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
  savePage: async (options?: { silent?: boolean }) => {
    const { page, title, slug, content, tags, frontmatterFields } = get()
    if (!page || !isDirtyState(get())) return

    const frontmatterErrors = validateEditorFrontmatterMetadata(
      tags,
      frontmatterFields,
    )
    if (Object.keys(frontmatterErrors).length > 0) {
      set({ frontmatterErrors })
      throw new Error(
        i18next.t('pageEditor.metadataErrorsBeforeSave', { ns: 'editor' }),
      )
    }

    // Only block concurrent auto-saves; manual saves always proceed
    if (isSavingMutex && options?.silent) return
    isSavingMutex = true

    const properties = buildEditableProperties(frontmatterFields)

    try {
      if (!options?.silent) useProgressbarStore.getState().setLoading(true)
      set({ frontmatterErrors: {} })
      const titleChanged = page.title !== title
      const slugChanged = page.slug !== slug
      const enableLinkRefactor = useConfigStore.getState().enableLinkRefactor
      const frontmatterChanged =
        tagsChanged(tags, page.tags ?? []) ||
        propertiesChanged(frontmatterFields, page.properties ?? {})

      let updatedPage: Page | null = null

      if (get().isPendingDraft) {
        updatedPage = (
          await savePendingDraft(
            page.id,
            title,
            slug,
            content,
            tags,
            properties,
          )
        ).page
      } else if (get().isDraft) {
        updatedPage = (
          await saveDraft(page.id, title, content, tags, properties)
        ).page
      } else {
        if (slugChanged && enableLinkRefactor) {
          const preview = await previewPageRefactor(page.id, {
            kind: 'rename',
            title,
            slug,
          })
          const rewriteLinks = await confirmPageRefactor(preview, {
            allowSkipRewrite: true,
          })
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
              tags,
              properties,
            )
          }
        } else {
          updatedPage = await updatePage(
            page.id,
            page.version,
            title,
            slug,
            content,
            tags,
            properties,
          )
        }
      }

      const nextTags = updatedPage?.tags ?? tags
      const nextProperties =
        updatedPage && updatedPage.properties
          ? updatedPage.properties
          : properties

      // Keep the local page snapshot canonical after save so metadata-only
      // updates do not remain dirty when the API omits empty collections.
      set((state) => {
        if (!state.page) return {}

        if (
          updatedPage?.content === null ||
          updatedPage?.content === undefined
        ) {
          throw new Error(
            i18next.t('pageEditor.contentNullFallback', { ns: 'editor' }),
          )
        }
        state.page.title = updatedPage.title
        state.page.slug = updatedPage.slug
        state.page.content = updatedPage.content
        state.page.path = updatedPage.path
        state.page.version = updatedPage.version
        state.page.tags = nextTags
        state.page.properties = nextProperties

        return {
          page: state.page,
          tags: nextTags,
          frontmatterFields: state.frontmatterFields.map((field) => {
            if (field.internal || field.type !== 'text') {
              return field
            }

            return {
              ...field,
              value: nextProperties[field.key] ?? field.value,
            }
          }),
        }
      })

      // Draft saves never touch the canonical tree or its indexes.
      if (!get().isDraft) {
        if (titleChanged || slugChanged) {
          await useTreeStore.getState().reloadTree()
        } else if (updatedPage?.id && updatedPage?.version) {
          useTreeStore
            .getState()
            .patchNodeVersion(updatedPage.id, updatedPage.version)
        }
      }

      const viewerPage = useViewerStore.getState().page
      if (
        !get().isDraft &&
        viewerPage?.id &&
        viewerPage.id === updatedPage?.id &&
        updatedPage
      ) {
        useViewerStore.setState({
          page: updatedPage,
          notFound: false,
          error: null,
        })
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
      isSavingMutex = false
      if (!options?.silent) useProgressbarStore.getState().setLoading(false)
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
    loadController?.abort()
    loadController = new AbortController()
    const { signal } = loadController

    useProgressbarStore.getState().setLoading(true)
    set({
      error: null,
      isLoading: true,
      notFound: false,
      page: null,
      initialPage: null,
      frontmatterErrors: {},
    })
    try {
      const canonicalPage = await getPageByPath(path, signal)
      let page = canonicalPage
      let isDraft = false
      try {
        const draft = await getDraft(canonicalPage.id, signal)
        page = draft.page
        isDraft = true
      } catch (err) {
        if (asApiLocalizedError(err)?.code !== 'draft_not_found') {
          throw err
        }
        if (canonicalPage.kind === NODE_KIND_PAGE) {
          try {
            const draft = await startDraft(canonicalPage.id)
            page = draft.page
            isDraft = true
          } catch (startErr) {
            if (asApiLocalizedError(startErr)?.code !== 'draft_exists') {
              throw startErr
            }
            const draft = await getDraft(canonicalPage.id, signal)
            page = draft.page
            isDraft = true
          }
        }
        if (signal.aborted) return
        if (isDraft) await useTreeStore.getState().reloadTree({ silent: true })
      }
      const fields: EditorFrontmatterField[] = Object.entries(
        page.properties ?? {},
      ).map(([key, value]) => ({
        key,
        value: String(value ?? ''),
        type: 'text' as const,
      }))
      if (signal.aborted) return
      set({
        page,
        initialPage: { ...page },
        notFound: false,
        title: page.title,
        slug: page.slug,
        content: page.content,
        tags: page.tags ?? [],
        frontmatterFields: fields,
        frontmatterUnsupported: '',
        isDraft,
        isPendingDraft: false,
        pendingParentId: '',
      })
    } catch (err) {
      if (signal.aborted) return

      if (isPageNotFoundError(err)) {
        set({
          error: null,
          notFound: true,
        })
        return
      }

      const mapped = mapApiError(
        err,
        i18next.t('pageEditor.unknownErrorFallback', { ns: 'editor' }),
      )
      set({
        error: mapped.message,
        notFound: false,
      })
    } finally {
      if (!signal.aborted) {
        set({ isLoading: false })
        useProgressbarStore.getState().setLoading(false)
      }
    }
  },
  loadPendingDraft: async (id: string) => {
    loadController?.abort()
    loadController = new AbortController()
    const { signal } = loadController
    useProgressbarStore.getState().setLoading(true)
    try {
      const response = await getPendingDraft(id, signal)
      const page = response.page
      if (signal.aborted) return
      set({
        page,
        initialPage: { ...page },
        title: page.title,
        slug: page.slug,
        content: page.content,
        tags: page.tags ?? [],
        frontmatterFields: Object.entries(page.properties ?? {}).map(
          ([key, value]) => ({
            key,
            value: String(value ?? ''),
            type: 'text' as const,
          }),
        ),
        notFound: false,
        error: null,
        isDraft: true,
        isPendingDraft: true,
        pendingParentId: response.parentId,
      })
    } catch (err) {
      if (signal.aborted) return
      set({
        error: mapApiError(
          err,
          i18next.t('pageEditor.unknownErrorFallback', { ns: 'editor' }),
        ).message,
        notFound: false,
      })
    } finally {
      if (!signal.aborted) {
        set({ isLoading: false })
        useProgressbarStore.getState().setLoading(false)
      }
    }
  },
  startDraft: async () => {
    const { page } = get()
    if (!page) return
    const response = await startDraft(page.id)
    const draftPage = response.page
    set({
      page: draftPage,
      initialPage: { ...draftPage },
      title: draftPage.title,
      slug: draftPage.slug,
      content: draftPage.content,
      tags: draftPage.tags ?? [],
      frontmatterFields: Object.entries(draftPage.properties ?? {}).map(
        ([key, value]) => ({
          key,
          value: String(value ?? ''),
          type: 'text' as const,
        }),
      ),
      isDraft: true,
    })
  },
  publishDraft: async () => {
    if (isDirtyState(get())) {
      const saved = await get().savePage()
      if (!saved && isDirtyState(get())) return null
    }
    const { page } = get()
    if (!page) return null
    const published = get().isPendingDraft
      ? await publishPendingDraft(page.id)
      : await publishDraft(page.id)
    set({
      page: published,
      initialPage: { ...published },
      title: published.title,
      slug: published.slug,
      content: published.content,
      tags: published.tags ?? [],
      isDraft: false,
      isPendingDraft: false,
      pendingParentId: '',
    })
    return published
  },
  discardDraft: async () => {
    const { page } = get()
    if (!page) return
    if (get().isPendingDraft) {
      await discardPendingDraft(page.id)
      set({
        page: null,
        initialPage: null,
        isDraft: false,
        isPendingDraft: false,
      })
      return
    }
    await discardDraft(page.id)
    const published = await getPageByPath(page.path)
    set({
      page: published,
      initialPage: { ...published },
      title: published.title,
      slug: published.slug,
      content: published.content,
      tags: published.tags ?? [],
      frontmatterFields: Object.entries(published.properties ?? {}).map(
        ([key, value]) => ({
          key,
          value: String(value ?? ''),
          type: 'text' as const,
        }),
      ),
      isDraft: false,
      isPendingDraft: false,
      pendingParentId: '',
    })
  },
  // Called when PageEditor unmounts so `page` (and thus currentEditorPageId
  // reads elsewhere, e.g. TreeNodeActionsMenu's rename/delete guards) doesn't
  // keep pointing at the last-edited page indefinitely after the editor closes.
  resetEditorState: () => {
    loadController?.abort()
    set({
      error: null,
      isLoading: false,
      notFound: false,
      page: null,
      title: '',
      slug: '',
      content: '',
      tags: [],
      frontmatterFields: [],
      frontmatterUnsupported: '',
      frontmatterErrors: {},
      initialPage: null,
      isDraft: false,
      isPendingDraft: false,
      pendingParentId: '',
    })
  },
}))
