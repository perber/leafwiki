import * as importAPI from '@/lib/api/import'
import { toast } from 'sonner'
import { create } from 'zustand'
import { useTreeStore } from './tree'

type ImportStore = {
  creatingImportPlan: boolean
  executingImportPlan: boolean
  importPlan: importAPI.ImportPlan | null
  createImportPlan: (sourcePath: File) => Promise<void>
  loadImportPlan: () => Promise<void>
  executeImportPlan: () => Promise<void>
}

export const useImportStore = create<ImportStore>((set, get) => ({
  importPlan: null,
  creatingImportPlan: false,
  executingImportPlan: false,
  createImportPlan: async (sourcePath: File) => {
    set({ creatingImportPlan: true })
    try {
      const importPlan = await importAPI.createImportPlanFromZip(sourcePath)
      toast.success('Import plan created successfully')
      set({ importPlan })
    } catch (err) {
      toast.error('Failed to create import plan: ' + (err as Error).message)
    } finally {
      set({ creatingImportPlan: false })
    }
  },
  loadImportPlan: async () => {
    try {
      const importPlan = await importAPI.getImportPlan()
      set({ importPlan })
    } catch (err) {
      toast.error('Failed to load import plan: ' + (err as Error).message)
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
      set({ executingImportPlan: true })
      await importAPI.executeImportPlan()
      toast.success('Import completed successfully')
      set({ importPlan: null })
    } catch (err) {
      if ('error' in (err as { error: string })) {
        toast.error(
          'Failed to execute import plan: ' + (err as { error: string }).error,
        )
      } else {
        toast.error('Failed to execute import plan: unknown error')
      }
    } finally {
      set({ executingImportPlan: false })
      // reload tree
      useTreeStore.getState().reloadTree()
    }
  },
}))
