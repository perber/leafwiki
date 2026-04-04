import * as importAPI from '@/lib/api/import'
import { ApiError } from '@/lib/api/auth'
import { toast } from 'sonner'
import { create } from 'zustand'
import { useTreeStore } from './tree'

// Helper to normalize error messages from various error types
function getErrorMessage(err: unknown): string {
  if (err instanceof Error) {
    return err.message
  }
  if (typeof err === 'object' && err !== null) {
    const errObj = err as Record<string, unknown>
    if (typeof errObj.error === 'string') {
      return errObj.error
    }
    if (typeof errObj.message === 'string') {
      return errObj.message
    }
  }
  return String(err)
}

type ImportStore = {
  creatingImportPlan: boolean
  executingImportPlan: boolean
  cancelingImportPlan: boolean
  loadingImportPlan: boolean
  importPlan: importAPI.ImportPlan | null
  importResult: importAPI.ImportResult | null
  createImportPlan: (sourcePath: File) => Promise<void>
  loadImportPlan: () => Promise<void>
  executeImportPlan: () => Promise<void>
  cancelImportPlan: () => Promise<void>
}

const IMPORT_POLL_INTERVAL_MS = 1000
const IMPORT_POLL_RETRY_LIMIT = 3

function sleep(ms: number): Promise<void> {
  return new Promise((resolve) => window.setTimeout(resolve, ms))
}

async function pollImportPlanUntilSettled(
  initialPlan: importAPI.ImportPlan,
  set: (partial: Partial<ImportStore>) => void,
): Promise<importAPI.ImportPlan> {
  let currentPlan = initialPlan
  let consecutivePollErrors = 0

  while (currentPlan.execution_status === 'running') {
    await sleep(IMPORT_POLL_INTERVAL_MS)

    try {
      currentPlan = await importAPI.getImportPlan()
      consecutivePollErrors = 0
      set({
        importPlan: currentPlan,
        importResult: currentPlan.execution_result ?? null,
      })
    } catch (err) {
      consecutivePollErrors++
      if (consecutivePollErrors >= IMPORT_POLL_RETRY_LIMIT) {
        throw err
      }
    }
  }

  return currentPlan
}

export const useImportStore = create<ImportStore>((set, get) => ({
  importPlan: null,
  creatingImportPlan: false,
  executingImportPlan: false,
  cancelingImportPlan: false,
  loadingImportPlan: false,
  importResult: null,
  createImportPlan: async (sourcePath: File) => {
    set({ creatingImportPlan: true, importPlan: null, importResult: null })
    try {
      const importPlan = await importAPI.createImportPlanFromZip(sourcePath)
      toast.success('Import plan created successfully')
      set({ importPlan })
    } catch (err) {
      toast.error('Failed to create import plan: ' + getErrorMessage(err))
    } finally {
      set({ creatingImportPlan: false })
    }
  },
  loadImportPlan: async () => {
    set({ loadingImportPlan: true })
    try {
      let importPlan = await importAPI.getImportPlan()
      set({
        importPlan,
        importResult: importPlan.execution_result ?? null,
      })

      if (importPlan.execution_status === 'running') {
        set({ executingImportPlan: true })
        importPlan = await pollImportPlanUntilSettled(importPlan, set)
        set({
          importPlan,
          importResult: importPlan.execution_result ?? null,
        })
      }
    } catch (err) {
      if (err instanceof ApiError && err.status === 404) {
        set({ importPlan: null, importResult: null })
        return
      }
      toast.error('Failed to load import plan: ' + getErrorMessage(err))
      return
    } finally {
      set({ loadingImportPlan: false, executingImportPlan: false })
    }
  },
  executeImportPlan: async () => {
    const importPlan = get().importPlan
    if (importPlan === null) {
      toast.error('No import plan to execute')
      return
    }
    try {
      set({ executingImportPlan: true, importResult: null })
      let currentPlan = await importAPI.executeImportPlan()
      set({ importPlan: currentPlan, importResult: null })

      currentPlan = await pollImportPlanUntilSettled(currentPlan, set)

      if (currentPlan.execution_status === 'completed') {
        toast.success('Import completed successfully')
        set({
          importPlan: currentPlan,
          importResult: currentPlan.execution_result ?? null,
        })
      } else if (currentPlan.execution_status === 'canceled') {
        toast.success('Import canceled')
        set({
          importPlan: currentPlan,
          importResult: currentPlan.execution_result ?? null,
        })
      } else if (currentPlan.execution_status === 'failed') {
        set({ importPlan: currentPlan })
        throw new Error(
          currentPlan.execution_error || 'Import execution failed',
        )
      }
    } catch (err) {
      toast.error('Failed to execute import plan: ' + getErrorMessage(err))
    } finally {
      set({ executingImportPlan: false })
      // reload tree
      useTreeStore.getState().reloadTree()
    }
  },
  cancelImportPlan: async () => {
    const importPlan = get().importPlan
    if (importPlan === null) {
      toast.error('No import plan to clear')
      return
    }
    try {
      set({ cancelingImportPlan: true })
      const response = await importAPI.cancelImportPlan()

      if (importPlan.execution_status === 'running' && response) {
        set({ importPlan: response })
        const finalPlan = await pollImportPlanUntilSettled(response, set)
        toast.success(
          finalPlan.execution_status === 'canceled'
            ? 'Import canceled'
            : 'Import finished before cancellation completed',
        )
        set({
          importPlan: finalPlan,
          importResult: finalPlan.execution_result ?? null,
        })
        useTreeStore.getState().reloadTree()
        return
      }

      toast.success('Import plan cleared')
      set({ importPlan: null, importResult: null })
    } catch (err) {
      toast.error(
        'Failed to cancel or clear import plan: ' + getErrorMessage(err),
      )
    } finally {
      set({ cancelingImportPlan: false })
    }
  },
}))
