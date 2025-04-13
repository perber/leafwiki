// stores/AddPageDialogStore.ts

import { movePage } from '@/lib/api';
import { handleFieldErrors } from '@/lib/handleFieldErrors';
import { toast } from 'sonner';
import { create } from 'zustand';
import { useTreeStore } from './tree';

type MovePageDialogStore = {
    open: boolean
    pageId: string
    loading: boolean
    success: boolean
    parentId: string
    path: string | null
    errors: Record<string, string> | null;
    getParentId: (parentId: string) => string
    getTree: () => any
    getPathById: (id: string) => string | null
    openDialog: (pageId: string) => void
    closeDialog: () => void
    setFieldErrors: (errors: Record<string, string>) => void;
    movePage: (newParentId: string) => Promise<void>
}

export const useMovePageDialogStore = create<MovePageDialogStore>((set, get) => ({
    open: false,
    pageId: "",
    parentId: "",
    path: "",
    loading: false,
    success: false,
    errors: null,
    setFieldErrors: (errors) => set({ errors }),
    getTree: () => {
        return useTreeStore.getState().tree
    },
    getPathById: (id) => {
        return useTreeStore.getState().getPathById(id)
    },
    getParentId: (pageId) => {
        if (!pageId) return ''
        console.log('getParentId', { pageId })
        const tree = get().getTree()
        const findParent = (node: any): string | null => {
            for (const child of node.children || []) {
                if (child.id === pageId) return node.id
                const found = findParent(child)
                if (found) return found
            }
            return null
        }

        const parentId = findParent(tree)
        return parentId || ''
    },
    openDialog: (pageId) => {
        // Look for parent id in the tree
        const parentId = get().getParentId(pageId)
        const path = get().getPathById(pageId)
        set({ pageId, parentId, path, open: true, errors: null, loading: false, success: false })
    },
    closeDialog: () => set({ open: false, pageId: "", path: "", parentId: "", errors: null, loading: false, success: false }),
    movePage: async (newParentId) => {
        try {
            set({ loading: true, errors: null })
            await movePage(get().pageId, newParentId)
            await useTreeStore.getState().reloadTree()
            const path = get().getPathById(get().pageId)
            toast.success('Page moved successfully')
            set({ loading: false, success: true, path })
        } catch (err: any) {
            console.log(err)
            handleFieldErrors(err, get().setFieldErrors, 'Error moving page')
            set({ loading: false, success: false })
        }
    },
}))
