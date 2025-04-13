// stores/AddPageDialogStore.ts

import { createPage, suggestSlug } from '@/lib/api';
import { handleFieldErrors } from '@/lib/handleFieldErrors';
import { toast } from 'sonner';
import { create } from 'zustand';
import { useTreeStore } from './tree';

type AddPageDialogStore = {
    open: boolean
    parentId: string 
    title: string
    slug: string
    loading: boolean
    success: boolean
    parentPath: string
    errors: Record<string, string> | null;
    getSlug: (parentId: string, title: string) => Promise<void>
    openDialog: (parentId: string) => void
    closeDialog: () => void
    setTitle: (title: string) => void
    setFieldErrors: (errors: Record<string, string>) => void;
    setSlug: (slug: string) => void
    createPage: () => Promise<void>
}

export const useAddPageDialogStore = create<AddPageDialogStore>((set, get) => ({
    open: false,
    parentId: "",
    title: '',
    slug: '',
    parentPath: '',
    loading: false,
    success: false,
    errors: null,
    setFieldErrors: (errors) => set({ errors }),
    openDialog: (parentId) => {
        const pagePath = useTreeStore.getState().getPathById(parentId) || ''
        set({ parentId: parentId, parentPath: pagePath, title: "", slug: "", open: true, errors: null, loading: false, success: false })
    },
    closeDialog: () => set({ parentId: "", title: "", parentPath:"", slug: "", open: false, errors: null, loading: false, success: false }),
    setTitle: (title) => set({ title, errors: null }),
    setSlug: (slug) => set({ slug, errors: null }),
    getSlug: async (parentId, title) => {
        try {
            const slug = await suggestSlug(parentId, title)
            set({ slug, errors: null, })
        } catch (error) {
            console.warn('Error generating slug:', error)
            set({ slug: '', errors: { slug: "couldn't generate slug" } })
            toast.error('Error generating slug')
        }
    },
    createPage: async () => {
        try {
            set({ loading: true, errors: null })
            await createPage({ title: get().title, slug: get().slug, parentId: get().parentId })
            set({ loading: false, success: true })
            toast.success('Page created')
            // Reload the tree or navigate to the new page
            useTreeStore.getState().reloadTree()
        } catch (error) {
            console.warn('Error creating page:', error)
            handleFieldErrors(error, get().setFieldErrors, 'Error creating page');
            set({ loading: false, success: false })
        }

    },
}))
