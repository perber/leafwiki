import { asApiLocalizedError, mapApiError } from '@/lib/api/errors'
import { useDialogsStore } from '@/stores/dialogs'
import { useEditorStore, type AutoSaveStatus } from '@/stores/editor'
import { useEffect, useReducer, useRef, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { DIALOG_UNSAVED_CHANGES } from '@/lib/registries'
import { validateEditorFrontmatterMetadata } from './frontmatter'
import { isDirtyState, usePageEditorStore } from './pageEditorStore'

const DEBOUNCE_MS = 2000

export function useAutoSave(): { status: AutoSaveStatus } {
  // status is kept in local useState (not just the store) because the main debounce
  // effect depends on it to reschedule after a save completes.
  const [status, setStatus] = useState<AutoSaveStatus>('idle')
  const { t } = useTranslation('editor')
  // Incremented when the unsaved-changes dialog is dismissed via Cancel so the
  // debounce effect re-runs and reschedules the auto-save.
  const [retriggerCount, dispatchRetrigger] = useReducer(
    (c: number) => c + 1,
    0,
  )

  const autoSave = useEditorStore((s) => s.autoSave)
  const content = usePageEditorStore((s) => s.content)
  const tags = usePageEditorStore((s) => s.tags)
  const frontmatterFields = usePageEditorStore((s) => s.frontmatterFields)
  const page = usePageEditorStore((s) => s.page)
  const slug = usePageEditorStore((s) => s.slug)
  const dirty = usePageEditorStore(isDirtyState)

  // Track the page version so we can detect a manual save after a conflict
  const lastPageVersionRef = useRef<string | undefined>(page?.version)
  const debounceTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null)
  const isSavingRef = useRef(false)
  const statusRef = useRef<AutoSaveStatus>('idle')
  // Incremented on navigation or autoSave-off to invalidate in-flight save callbacks
  const generationRef = useRef(0)

  const updateStatus = (next: AutoSaveStatus) => {
    statusRef.current = next
    setStatus(next)
    useEditorStore.getState().setAutoSaveStatus(next)
  }

  const clearDebounce = () => {
    if (debounceTimerRef.current !== null) {
      clearTimeout(debounceTimerRef.current)
      debounceTimerRef.current = null
    }
  }

  // Cancel the debounce when the unsaved-changes dialog opens; reschedule when it
  // closes via Cancel. Uses Zustand subscribe (not a React selector) to avoid
  // re-rendering PageEditor — a React subscription cascades into useNavigationGuard
  // recreating its callbacks and calling openDialog a second time (double button).
  // prevState.dialogType distinguishes open vs close without a mutable closure var.
  useEffect(() => {
    return useDialogsStore.subscribe((state, prevState) => {
      if (state.dialogType === DIALOG_UNSAVED_CHANGES) {
        clearDebounce()
      } else if (prevState.dialogType === DIALOG_UNSAVED_CHANGES) {
        // Dialog just closed. Fire a retrigger so the debounce effect reschedules
        // if content is still dirty (Cancel path). On Leave anyway the unmount
        // cleanup runs clearDebounce() before the 2 s timer fires, so this is safe.
        dispatchRetrigger()
      }
    })
  }, [])

  // When autoSave is disabled: cancel timers, invalidate in-flight saves, and reset.
  // isSavingRef must be cleared here too: if a silent save was awaiting when auto-save
  // was toggled off, the callback returns early (generation mismatch) without resetting
  // isSavingRef — leaving it stuck as true and blocking all future auto-saves.
  useEffect(() => {
    if (!autoSave) {
      clearDebounce()
      generationRef.current++
      isSavingRef.current = false
      updateStatus('idle')
    }
  }, [autoSave])

  // When page identity changes (navigation to a different page): reset everything
  const pageId = page?.id
  const pageVersion = page?.version
  useEffect(() => {
    clearDebounce()
    isSavingRef.current = false
    generationRef.current++ // invalidate any in-flight save callback
    updateStatus('idle')
    lastPageVersionRef.current = pageVersion
  }, [pageId, pageVersion])

  // Detect manual save after a paused conflict: page.version advances
  useEffect(() => {
    if (
      statusRef.current === 'paused' &&
      pageVersion !== undefined &&
      pageVersion !== lastPageVersionRef.current
    ) {
      lastPageVersionRef.current = pageVersion
      updateStatus('idle')
    } else if (pageVersion !== undefined) {
      lastPageVersionRef.current = pageVersion
    }
  }, [pageVersion])

  // Also clear 'paused' when the editor becomes clean (user reverted all changes)
  useEffect(() => {
    if (statusRef.current === 'paused' && !dirty) {
      updateStatus('idle')
    }
  }, [dirty])

  // Main debounce effect: watch content, tags, frontmatterFields.
  // Also depends on `status` so it re-runs after a save completes and can reschedule
  // for edits that arrived while a save was in flight.
  // Also depends on `retriggerCount` to reschedule after the unsaved-changes dialog
  // is dismissed via Cancel.
  useEffect(() => {
    // Do nothing if auto-save is off
    if (!autoSave) return

    // Do nothing if paused (waiting for manual save after conflict)
    if (statusRef.current === 'paused') return

    // Do nothing if nothing is dirty
    if (!dirty) return

    // Do NOT auto-save if slug changed (slug change requires manual save)
    if (!page || page.slug !== slug) return

    // Don't stack saves
    if (isSavingRef.current) return

    clearDebounce()

    const generation = generationRef.current

    debounceTimerRef.current = setTimeout(async () => {
      debounceTimerRef.current = null

      // Abort if navigation or autoSave toggle happened since scheduling
      if (generationRef.current !== generation) return

      // Re-check conditions at fire time
      if (useDialogsStore.getState().dialogType === DIALOG_UNSAVED_CHANGES)
        return
      const state = usePageEditorStore.getState()
      const currentDirty = isDirtyState(state)
      if (!currentDirty) return
      if (!state.page || state.page.slug !== state.slug) return
      if (isSavingRef.current) return

      // Skip auto-save if metadata is currently invalid — don't surface errors while the user is mid-edit
      const validationErrors = validateEditorFrontmatterMetadata(
        state.tags,
        state.frontmatterFields,
      )
      if (Object.keys(validationErrors).length > 0) return

      isSavingRef.current = true
      updateStatus('saving')

      try {
        const result = await usePageEditorStore
          .getState()
          .savePage({ silent: true })

        if (generationRef.current !== generation) return

        if (result === undefined) {
          // savePage returned early (not dirty — a concurrent save already completed)
          isSavingRef.current = false
          updateStatus('idle')
          return
        }

        isSavingRef.current = false
        updateStatus('idle')
        toast.success(t('autoSave.savedToast'), { duration: 2000 })
      } catch (err) {
        if (generationRef.current !== generation) return

        isSavingRef.current = false
        const localized = asApiLocalizedError(err)
        if (localized?.code === 'page_version_conflict') {
          updateStatus('paused')
          toast(t('autoSave.versionConflictToast'), { duration: 6000 })
        } else {
          updateStatus('idle')
          // Skip toast for validation errors — they are already shown inline via frontmatterErrors
          const currentErrors = usePageEditorStore.getState().frontmatterErrors
          if (Object.keys(currentErrors).length === 0) {
            const mapped = mapApiError(err, t('autoSave.errorFallback'))
            toast.error(mapped.message, { duration: 4000 })
          }
        }
      }
    }, DEBOUNCE_MS)

    return () => {
      clearDebounce()
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [
    content,
    tags,
    frontmatterFields,
    autoSave,
    dirty,
    slug,
    page?.slug,
    status,
    retriggerCount,
  ])

  // Cleanup on unmount: invalidate any in-flight save so its continuation doesn't
  // call setStatus after the component is gone. isSavingRef is also reset so a
  // remounted instance starts clean. If a debounce was pending (content changed
  // but the 2-second timer hadn't fired yet), flush it immediately so edits made
  // just before navigation are not lost.
  useEffect(() => {
    return () => {
      // eslint-disable-next-line react-hooks/exhaustive-deps
      const hadPendingTimer = debounceTimerRef.current !== null
      // eslint-disable-next-line react-hooks/exhaustive-deps
      const wasAlreadySaving = isSavingRef.current

      // ESLint warns about ref.current in cleanup assuming a stale read — here we
      // intentionally write (increment) at unmount time, which is exactly correct.
      // eslint-disable-next-line react-hooks/exhaustive-deps
      generationRef.current++
      isSavingRef.current = false
      clearDebounce()
      useEditorStore.getState().setAutoSaveStatus('idle')

      // Flush: fire the pending save immediately so edits aren't silently dropped
      if (
        hadPendingTimer &&
        !wasAlreadySaving &&
        statusRef.current !== 'paused'
      ) {
        const editorState = useEditorStore.getState()
        const pageEditorState = usePageEditorStore.getState()
        const validationErrors = validateEditorFrontmatterMetadata(
          pageEditorState.tags,
          pageEditorState.frontmatterFields,
        )
        if (
          editorState.autoSave &&
          isDirtyState(pageEditorState) &&
          pageEditorState.page?.slug === pageEditorState.slug &&
          Object.keys(validationErrors).length === 0
        ) {
          pageEditorState.savePage({ silent: true }).catch(() => {})
        }
      }
    }
  }, [])

  return { status }
}
