import * as importAPI from '@/lib/api/import'
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
  importPlan: importAPI.ImportPlan | null
  importResult: importAPI.ImportResult | null
  createImportPlan: (sourcePath: File) => Promise<void>
  loadImportPlan: () => Promise<void>
  executeImportPlan: () => Promise<void>
}

export const useImportStore = create<ImportStore>((set, get) => ({
  importPlan: null,
  creatingImportPlan: false,
  executingImportPlan: false,
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
    set({ creatingImportPlan: true, importPlan: null, importResult: null })
    try {
      const importPlan = await importAPI.getImportPlan()
      set({ importPlan })
    } catch (err) {
      toast.error('Failed to load import plan: ' + getErrorMessage(err))
      return
    } finally {
      set({ creatingImportPlan: false })
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
      const importResult = await importAPI.executeImportPlan()
      toast.success('Import completed successfully')
      set({ importPlan: null, importResult })
    } catch (err) {
      toast.error('Failed to execute import plan: ' + getErrorMessage(err))
    } finally {
      set({ executingImportPlan: false })
      // reload tree
      useTreeStore.getState().reloadTree()
    }
  },
}))
